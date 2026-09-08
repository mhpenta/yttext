package yttext

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"html"
	"io"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	innertubeClientName    = "ANDROID"
	innertubeClientVersion = "20.10.38"
	innertubeAPIURL        = "https://www.youtube.com/youtubei/v1/player?key=%s"
	userAgent              = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0 Safari/537.36"
	defaultHTTPTimeout     = 30 * time.Second
)

// ErrLanguageUnavailable indicates that no caption track matches the requested language.
var ErrLanguageUnavailable = errors.New("requested transcript language is unavailable")

var (
	videoIDRegex         = regexp.MustCompile(`^[A-Za-z0-9_-]{11}$`)
	innertubeAPIKeyRegex = regexp.MustCompile(`"INNERTUBE_API_KEY"\s*:\s*"([a-zA-Z0-9_-]+)"`)
)

// Segment represents a single caption/subtitle entry.
type Segment struct {
	Text      string  `json:"text"`
	Duration  float64 `json:"duration"`
	Offset    float64 `json:"offset"`
	StartTime float64 `json:"start"`
}

// Transcript represents the full transcript for a video.
type Transcript []Segment

// TranscriptAPI provides methods to interact with YouTube transcript APIs
type TranscriptAPI struct {
	httpClient *http.Client
}

// New creates a client with a 30-second timeout for each HTTP request,
// including reading the response body.
func New() *TranscriptAPI {
	return NewWithHTTPClient(nil)
}

// NewWithHTTPClient creates a TranscriptAPI using client without modifying it.
// A nil client uses the default 30-second request timeout. A supplied client's
// timeout, transport, cookie jar, and redirect policy are respected.
func NewWithHTTPClient(client *http.Client) *TranscriptAPI {
	if client == nil {
		client = &http.Client{Timeout: defaultHTTPTimeout}
	}
	return &TranscriptAPI{httpClient: client}
}

// GetTranscript fetches a transcript for a YouTube video ID.
// An empty languageCode requests English. Unavailable languages return an error.
func (api *TranscriptAPI) GetTranscript(videoID string, languageCode string) (Transcript, error) {
	return api.GetTranscriptContext(context.Background(), videoID, languageCode)
}

// GetTranscriptContext fetches a transcript with cancellation and deadlines
// applying to the entire fetch, including all HTTP requests and response bodies.
func (api *TranscriptAPI) GetTranscriptContext(ctx context.Context, videoID string, languageCode string) (Transcript, error) {
	videoID = strings.TrimSpace(videoID)
	if !videoIDRegex.MatchString(videoID) {
		return nil, fmt.Errorf("invalid YouTube video ID: %q", videoID)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return api.fetchTranscripts(ctx, videoID, languageCode)
}

// GetTranscriptByURL fetches a transcript for a YouTube video URL or video ID.
func (api *TranscriptAPI) GetTranscriptByURL(videoURL string, languageCode string) (Transcript, error) {
	return api.GetTranscriptByURLContext(context.Background(), videoURL, languageCode)
}

// GetTranscriptByURLContext fetches a transcript for a URL or video ID with
// cancellation and deadlines applying to the entire fetch.
func (api *TranscriptAPI) GetTranscriptByURLContext(ctx context.Context, videoURL string, languageCode string) (Transcript, error) {
	videoID, err := api.extractVideoID(videoURL)
	if err != nil {
		return nil, err
	}
	return api.GetTranscriptContext(ctx, videoID, languageCode)
}

// GetTranscripts fetches a transcript for a YouTube video ID.
//
// Deprecated: use GetTranscript.
func (api *TranscriptAPI) GetTranscripts(videoID string, languageCode string) (Transcript, error) {
	return api.GetTranscript(videoID, languageCode)
}

// GetTranscriptsByURL fetches a transcript for a YouTube video URL or video ID.
//
// Deprecated: use GetTranscriptByURL.
func (api *TranscriptAPI) GetTranscriptsByURL(videoURL string, languageCode string) (Transcript, error) {
	return api.GetTranscriptByURL(videoURL, languageCode)
}

// extractVideoID extracts the video ID from a YouTube URL
func (api *TranscriptAPI) extractVideoID(videoURL string) (string, error) {
	videoURL = strings.TrimSpace(videoURL)
	if videoIDRegex.MatchString(videoURL) {
		return videoURL, nil
	}

	u, err := url.Parse(videoURL)
	if err != nil {
		return "", fmt.Errorf("invalid URL: %w", err)
	}

	host := strings.TrimPrefix(strings.ToLower(u.Host), "www.")
	pathParts := strings.Split(strings.TrimPrefix(u.Path, "/"), "/")
	if host == "youtu.be" {
		if id := strings.TrimSpace(pathParts[0]); videoIDRegex.MatchString(id) {
			return id, nil
		}
	}

	if host == "youtube.com" || strings.HasSuffix(host, ".youtube.com") {
		q := u.Query()
		if v := q.Get("v"); v != "" {
			if videoIDRegex.MatchString(v) {
				return v, nil
			}
			return "", fmt.Errorf("invalid YouTube video ID in URL: %s", v)
		}

		if len(pathParts) >= 2 && (pathParts[0] == "embed" || pathParts[0] == "shorts" || pathParts[0] == "live") && videoIDRegex.MatchString(pathParts[1]) {
			return pathParts[1], nil
		}
	}

	return "", fmt.Errorf("could not extract video ID from URL: %s", videoURL)
}

// fetchTranscripts fetches and processes the transcripts from YouTube
func (api *TranscriptAPI) fetchTranscripts(ctx context.Context, videoID string, languageCode string) (Transcript, error) {
	html, err := api.fetchVideoHTML(ctx, videoID)
	if err != nil {
		return nil, err
	}

	apiKey, err := api.extractInnertubeAPIKey(html, videoID)
	if err != nil {
		return nil, err
	}

	innertubeData, err := api.fetchInnertubeData(ctx, videoID, apiKey)
	if err != nil {
		return nil, err
	}

	captionsJSON, err := api.extractCaptionsJSONFromInnertube(innertubeData, videoID)
	if err != nil {
		return nil, err
	}

	captionTracks, ok := captionsJSON["captionTracks"].([]interface{})
	if !ok || len(captionTracks) == 0 {
		return nil, fmt.Errorf("no caption tracks found")
	}

	var targetTrack map[string]interface{}
	var availableLanguages []string
	if languageCode == "" {
		languageCode = "en"
	}

	for _, track := range captionTracks {
		trackMap, ok := track.(map[string]interface{})
		if !ok {
			continue
		}

		trackLangCode, ok := trackMap["languageCode"].(string)
		if !ok {
			continue
		}

		availableLanguages = append(availableLanguages, trackLangCode)
		if trackLangCode == languageCode {
			targetTrack = trackMap
			break
		}
	}

	if targetTrack == nil {
		return nil, fmt.Errorf("%w: %q (available: %s)", ErrLanguageUnavailable, languageCode, strings.Join(availableLanguages, ", "))
	}

	baseURL, ok := targetTrack["baseUrl"].(string)
	if !ok {
		return nil, fmt.Errorf("failed to extract caption track URL")
	}

	transcriptURL, err := addTranscriptFormat(baseURL, "srv1")
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, transcriptURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create transcript request: %w", err)
	}
	req.Header.Set("Accept-Language", "en-US")
	req.Header.Set("User-Agent", userAgent)

	resp, err := api.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch transcript XML: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch transcript (HTTP %d)", resp.StatusCode)
	}

	xmlData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read transcript data: %w", err)
	}
	if len(bytes.TrimSpace(xmlData)) == 0 {
		return nil, fmt.Errorf("empty transcript response")
	}

	return api.parseTranscriptXML(xmlData)
}

// fetchVideoHTML fetches the video page HTML
func (api *TranscriptAPI) fetchVideoHTML(ctx context.Context, videoID string) (string, error) {
	urlPath := fmt.Sprintf("https://www.youtube.com/watch?v=%s", videoID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, urlPath, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept-Language", "en-US")
	req.Header.Set("User-Agent", userAgent)

	resp, err := api.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to fetch video page: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("video not found or not accessible (HTTP %d)", resp.StatusCode)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read video page: %w", err)
	}

	return string(bodyBytes), nil
}

// extractInnertubeAPIKey extracts the API key required for YouTube's player endpoint.
func (api *TranscriptAPI) extractInnertubeAPIKey(html string, videoID string) (string, error) {
	matches := innertubeAPIKeyRegex.FindStringSubmatch(html)
	if len(matches) == 2 {
		return matches[1], nil
	}

	if strings.Contains(html, "class=\"g-recaptcha\"") {
		return "", fmt.Errorf("too many requests")
	}

	return "", fmt.Errorf("failed to extract Innertube API key for video: %s", videoID)
}

// fetchInnertubeData fetches current player metadata, including usable caption track URLs.
func (api *TranscriptAPI) fetchInnertubeData(ctx context.Context, videoID string, apiKey string) (map[string]interface{}, error) {
	payload := map[string]interface{}{
		"context": map[string]interface{}{
			"client": map[string]interface{}{
				"clientName":    innertubeClientName,
				"clientVersion": innertubeClientVersion,
			},
		},
		"videoId": videoID,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to build Innertube request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf(innertubeAPIURL, url.QueryEscape(apiKey)), bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create Innertube request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept-Language", "en-US")
	req.Header.Set("User-Agent", userAgent)

	resp, err := api.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch Innertube player data: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch Innertube player data (HTTP %d)", resp.StatusCode)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to parse Innertube player data: %w", err)
	}

	return result, nil
}

// extractCaptionsJSONFromInnertube extracts captions metadata from Innertube player data.
func (api *TranscriptAPI) extractCaptionsJSONFromInnertube(data map[string]interface{}, videoID string) (map[string]interface{}, error) {
	if playabilityStatus, ok := data["playabilityStatus"].(map[string]interface{}); ok {
		status, _ := playabilityStatus["status"].(string)
		if status != "" && status != "OK" {
			reason, _ := playabilityStatus["reason"].(string)
			if reason == "" {
				reason = status
			}
			return nil, fmt.Errorf("video unavailable: %s", reason)
		}
	}

	captions, ok := data["captions"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("transcripts disabled for this video")
	}

	captionsJSON, ok := captions["playerCaptionsTracklistRenderer"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("transcripts disabled for this video")
	}

	if _, ok = captionsJSON["captionTracks"]; !ok {
		return nil, fmt.Errorf("no transcript available for this video")
	}

	return captionsJSON, nil
}

func addTranscriptFormat(rawURL string, format string) (string, error) {
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("invalid caption track URL: %w", err)
	}

	q := parsedURL.Query()
	q.Set("fmt", format)
	parsedURL.RawQuery = q.Encode()
	return parsedURL.String(), nil
}

// parseTranscriptXML parses the transcript XML data into transcript structs
func (api *TranscriptAPI) parseTranscriptXML(xmlData []byte) (Transcript, error) {
	decoder := xml.NewDecoder(bytes.NewReader(xmlData))
	var transcript Transcript

	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to parse transcript XML: %w", err)
		}

		startElement, ok := token.(xml.StartElement)
		if !ok || (startElement.Name.Local != "text" && startElement.Name.Local != "p") {
			continue
		}

		startTime, duration, err := parseTranscriptTiming(startElement)
		if err != nil {
			return nil, fmt.Errorf("invalid transcript timing: %w", err)
		}

		text, err := readElementText(decoder)
		if err != nil {
			return nil, err
		}
		text = strings.TrimSpace(html.UnescapeString(text))
		if text == "" {
			continue
		}

		segment := Segment{
			Text:      text,
			Duration:  duration,
			Offset:    startTime,
			StartTime: startTime,
		}

		transcript = append(transcript, segment)
	}

	if len(transcript) == 0 {
		return nil, fmt.Errorf("failed to parse any transcript entries")
	}

	return transcript, nil
}

func parseTranscriptTiming(startElement xml.StartElement) (float64, float64, error) {
	startSeconds, found, err := parseTimingAttr(startElement.Attr, "start", 1)
	if err != nil {
		return 0, 0, err
	}
	if !found {
		startSeconds, found, err = parseTimingAttr(startElement.Attr, "t", 1000)
		if err != nil {
			return 0, 0, err
		}
	}
	if !found {
		return 0, 0, fmt.Errorf("missing start time")
	}

	durationSeconds, found, err := parseTimingAttr(startElement.Attr, "dur", 1)
	if err != nil {
		return 0, 0, err
	}
	if !found {
		durationSeconds, _, err = parseTimingAttr(startElement.Attr, "d", 1000)
		if err != nil {
			return 0, 0, err
		}
	}

	return startSeconds, durationSeconds, nil
}

func parseTimingAttr(attrs []xml.Attr, name string, divisor float64) (float64, bool, error) {
	for _, attr := range attrs {
		if attr.Name.Local != name {
			continue
		}

		value, err := strconv.ParseFloat(attr.Value, 64)
		if err != nil {
			return 0, true, fmt.Errorf("invalid %s attribute %q: %w", name, attr.Value, err)
		}
		if math.IsNaN(value) || math.IsInf(value, 0) || value < 0 {
			return 0, true, fmt.Errorf("%s must be a finite, nonnegative number: %q", name, attr.Value)
		}
		return value / divisor, true, nil
	}

	return 0, false, nil
}

func readElementText(decoder *xml.Decoder) (string, error) {
	var sb strings.Builder
	depth := 1

	for depth > 0 {
		token, err := decoder.Token()
		if err != nil {
			return "", fmt.Errorf("failed to read transcript text: %w", err)
		}

		switch typedToken := token.(type) {
		case xml.StartElement:
			depth++
			if typedToken.Name.Local == "br" {
				sb.WriteByte('\n')
			}
		case xml.EndElement:
			depth--
		case xml.CharData:
			if depth > 0 {
				sb.Write([]byte(typedToken))
			}
		}
	}

	return sb.String(), nil
}
