package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

var textRegex = regexp.MustCompile(`<text start="([0-9.]+)" dur="([0-9.]+)".*?>(.*?)</text>`)

// Transcript represents a single caption/subtitle entry
type Transcript struct {
	Text      string  `json:"text"`
	Duration  float64 `json:"duration"`
	Offset    float64 `json:"offset"`
	StartTime float64 `json:"start"`
}

// TranscriptAPI provides methods to interact with YouTube transcript APIs
type TranscriptAPI struct {
	httpClient *http.Client
}

// New creates a new TranscriptAPI instance
func New() *TranscriptAPI {
	return &TranscriptAPI{
		httpClient: &http.Client{},
	}
}

// GetTranscripts fetches transcripts for a YouTube video
func (api *TranscriptAPI) GetTranscripts(videoID string, languageCode string) ([]Transcript, error) {
	return api.fetchTranscripts(videoID, languageCode)
}

// GetTranscriptsByURL fetches transcripts for a YouTube video URL
func (api *TranscriptAPI) GetTranscriptsByURL(videoURL string, languageCode string) ([]Transcript, error) {
	videoID, err := api.extractVideoID(videoURL)
	if err != nil {
		return nil, err
	}
	return api.fetchTranscripts(videoID, languageCode)
}

// extractVideoID extracts the video ID from a YouTube URL
func (api *TranscriptAPI) extractVideoID(videoURL string) (string, error) {
	if strings.Contains(videoURL, "youtu.be") {
		parts := strings.Split(videoURL, "/")
		return parts[len(parts)-1], nil
	}

	u, err := url.Parse(videoURL)
	if err != nil {
		return "", fmt.Errorf("invalid URL: %v", err)
	}

	if u.Host == "www.youtube.com" || u.Host == "youtube.com" {
		q := u.Query()
		if v := q.Get("v"); v != "" {
			return v, nil
		}
	}

	return "", fmt.Errorf("could not extract video ID from URL: %s", videoURL)
}

// fetchTranscripts fetches and processes the transcripts from YouTube
func (api *TranscriptAPI) fetchTranscripts(videoID string, languageCode string) ([]Transcript, error) {
	html, err := api.fetchVideoHTML(videoID)
	if err != nil {
		return nil, err
	}

	captionsJSON, err := api.extractCaptionsJSON(html, videoID)
	if err != nil {
		return nil, err
	}

	captionTracks, ok := captionsJSON["captionTracks"].([]interface{})
	if !ok || len(captionTracks) == 0 {
		return nil, fmt.Errorf("no caption tracks found")
	}

	var targetTrack map[string]interface{}
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

		if trackLangCode == languageCode {
			targetTrack = trackMap
			break
		}
	}

	if targetTrack == nil && len(captionTracks) > 0 {
		targetTrack, _ = captionTracks[0].(map[string]interface{})
	}

	if targetTrack == nil {
		return nil, fmt.Errorf("no suitable caption track found")
	}

	baseURL, ok := targetTrack["baseUrl"].(string)
	if !ok {
		return nil, fmt.Errorf("failed to extract caption track URL")
	}

	resp, err := api.httpClient.Get(baseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch transcript XML: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch transcript (HTTP %d)", resp.StatusCode)
	}

	xmlData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read transcript data: %v", err)
	}

	return api.parseTranscriptXML(xmlData)
}

// fetchVideoHTML fetches the video page HTML
func (api *TranscriptAPI) fetchVideoHTML(videoID string) (string, error) {
	urlPath := fmt.Sprintf("https://www.youtube.com/watch?v=%s", videoID)
	req, err := http.NewRequest("GET", urlPath, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("Accept-Language", "en-US")

	resp, err := api.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to fetch video page: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("video not found or not accessible (HTTP %d)", resp.StatusCode)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read video page: %v", err)
	}

	return string(bodyBytes), nil
}

// extractCaptionsJSON extracts captions JSON from the video HTML
func (api *TranscriptAPI) extractCaptionsJSON(html string, videoID string) (map[string]interface{}, error) {
	parts := strings.Split(html, `"captions":`)
	if len(parts) <= 1 {
		if strings.Contains(html, "class=\"g-recaptcha\"") {
			return nil, fmt.Errorf("too many requests")
		}
		if !strings.Contains(html, "\"playabilityStatus\":") {
			return nil, fmt.Errorf("video unavailable")
		}
		return nil, fmt.Errorf("transcripts disabled for this video")
	}

	jsonPart := parts[1]
	endIndex := strings.Index(jsonPart, `,"videoDetails`)
	if endIndex == -1 {
		return nil, fmt.Errorf("failed to extract captions JSON")
	}

	jsonStr := jsonPart[:endIndex]
	jsonStr = strings.Replace(jsonStr, "\n", "", -1)

	var result map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		return nil, fmt.Errorf("failed to parse captions JSON: %v", err)
	}

	captionsJSON, ok := result["playerCaptionsTracklistRenderer"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("transcripts disabled for this video")
	}

	if _, ok = captionsJSON["captionTracks"]; !ok {
		return nil, fmt.Errorf("no transcript available for this video")
	}

	return captionsJSON, nil
}

// parseTranscriptXML parses the transcript XML data into transcript structs
func (api *TranscriptAPI) parseTranscriptXML(xmlData []byte) ([]Transcript, error) {

	matches := textRegex.FindAllStringSubmatch(string(xmlData), -1)

	if len(matches) == 0 {
		return nil, fmt.Errorf("no transcript text found in XML")
	}

	var transcripts []Transcript
	for _, match := range matches {
		if len(match) < 4 {
			continue
		}

		startTime, err := strconv.ParseFloat(match[1], 64)
		if err != nil {
			continue
		}

		duration, err := strconv.ParseFloat(match[2], 64)
		if err != nil {
			continue
		}

		// Unescape HTML entities in the text
		text := match[3]
		text = strings.ReplaceAll(text, "&amp;", "&")
		text = strings.ReplaceAll(text, "&lt;", "<")
		text = strings.ReplaceAll(text, "&gt;", ">")
		text = strings.ReplaceAll(text, "&quot;", "\"")
		text = strings.ReplaceAll(text, "&#39;", "'")

		transcript := Transcript{
			Text:      text,
			Duration:  duration,
			Offset:    startTime,
			StartTime: startTime,
		}

		transcripts = append(transcripts, transcript)
	}

	if len(transcripts) == 0 {
		return nil, fmt.Errorf("failed to parse any transcript entries")
	}

	return transcripts, nil
}
