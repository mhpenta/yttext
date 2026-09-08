package yttext

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func response(body string) *http.Response {
	return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}
}

// Every response is local; these tests never contact YouTube.
func fixtureResponse(t *testing.T, r *http.Request) *http.Response {
	t.Helper()
	if r.Header.Get("User-Agent") == "" || r.Header.Get("Accept-Language") == "" {
		t.Fatal("missing request headers")
	}
	switch r.URL.Path {
	case "/watch":
		if r.Method != http.MethodGet || r.URL.Query().Get("v") != "dQw4w9WgXcQ" {
			t.Fatalf("unexpected video request: %s %s", r.Method, r.URL)
		}
		return response(`{"INNERTUBE_API_KEY":"test-key"}`)
	case "/youtubei/v1/player":
		if r.Method != http.MethodPost || r.URL.Query().Get("key") != "test-key" {
			t.Fatalf("unexpected player request: %s %s", r.Method, r.URL)
		}
		var payload struct {
			VideoID string `json:"videoId"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil || payload.VideoID != "dQw4w9WgXcQ" {
			t.Fatalf("invalid player payload: %+v, error: %v", payload, err)
		}
		return response(`{"playabilityStatus":{"status":"OK"},"captions":{"playerCaptionsTracklistRenderer":{"captionTracks":[{"languageCode":"fr","baseUrl":"https://www.youtube.com/api/timedtext?lang=fr"},{"languageCode":"en","baseUrl":"https://www.youtube.com/api/timedtext?lang=en&fmt=json3&signature=a%2Bb"}]}}}`)
	case "/api/timedtext":
		q := r.URL.Query()
		if r.Method != http.MethodGet || q.Get("lang") != "en" || q.Get("fmt") != "srv1" || q.Get("signature") != "a+b" {
			t.Fatalf("unexpected caption request: %s %s", r.Method, r.URL)
		}
		return response(`<transcript><text start="0.25" dur="1.5">Hello<br/>world &amp; friends.</text></transcript>`)
	default:
		t.Fatalf("unexpected request: %s", r.URL)
		return nil
	}
}

func TestGetTranscriptFetchPipeline(t *testing.T) {
	api := NewWithHTTPClient(&http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return fixtureResponse(t, r), nil
	})})
	cases := map[string]func() (Transcript, error){
		"ID":               func() (Transcript, error) { return api.GetTranscript("dQw4w9WgXcQ", "en") },
		"default language": func() (Transcript, error) { return api.GetTranscript("dQw4w9WgXcQ", "") },
		"URL":              func() (Transcript, error) { return api.GetTranscriptByURL("https://youtu.be/dQw4w9WgXcQ?t=1", "en") },
		"context":          func() (Transcript, error) { return api.GetTranscriptContext(context.Background(), "dQw4w9WgXcQ", "en") },
		"URL context": func() (Transcript, error) {
			return api.GetTranscriptByURLContext(context.Background(), "https://www.youtube.com/watch?v=dQw4w9WgXcQ", "en")
		},
		"deprecated ID":  func() (Transcript, error) { return api.GetTranscripts("dQw4w9WgXcQ", "en") },
		"deprecated URL": func() (Transcript, error) { return api.GetTranscriptsByURL("https://youtu.be/dQw4w9WgXcQ", "en") },
	}
	want := Transcript{{Text: "Hello\nworld & friends.", StartTime: 0.25, Offset: 0.25, Duration: 1.5}}
	for name, fetch := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := fetch()
			if err != nil || !reflect.DeepEqual(got, want) {
				t.Fatalf("got %+v, %v; want %+v", got, err, want)
			}
		})
	}
}

func TestGetTranscriptUnavailableLanguage(t *testing.T) {
	for _, language := range []string{"en", "", "de"} {
		t.Run("language="+language, func(t *testing.T) {
			api := NewWithHTTPClient(&http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				switch r.URL.Path {
				case "/youtubei/v1/player":
					return response(`{"captions":{"playerCaptionsTracklistRenderer":{"captionTracks":[{"languageCode":"fr","baseUrl":"https://www.youtube.com/api/timedtext?lang=fr"}]}}}`), nil
				case "/api/timedtext":
					t.Fatal("must not fetch a different language")
				}
				return fixtureResponse(t, r), nil
			})})
			got, err := api.GetTranscript("dQw4w9WgXcQ", language)
			if got != nil || !errors.Is(err, ErrLanguageUnavailable) || !strings.Contains(err.Error(), "fr") {
				t.Fatalf("got %+v, %v; want unavailable-language error listing fr", got, err)
			}
		})
	}
}

func TestDefaultClientAppliesRequestDeadline(t *testing.T) {
	for name, api := range map[string]*TranscriptAPI{"New": New(), "nil HTTP client": NewWithHTTPClient(nil)} {
		t.Run(name, func(t *testing.T) {
			api.httpClient.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
				deadline, ok := r.Context().Deadline()
				if remaining := time.Until(deadline); !ok || remaining <= 0 || remaining > 30*time.Second {
					t.Fatalf("default request %s has no deadline within 30 seconds", r.URL.Path)
				}
				return fixtureResponse(t, r), nil
			})
			if _, err := api.GetTranscript("dQw4w9WgXcQ", "en"); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestGetTranscriptRejectsInvalidInputBeforeHTTP(t *testing.T) {
	api := NewWithHTTPClient(&http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		t.Fatalf("invalid input reached HTTP: %s", r.URL)
		return nil, nil
	})})
	for _, id := range []string{"", "not-an-id", "dQw4w9WgXcQ&other=value", "https://youtu.be/dQw4w9WgXcQ"} {
		if _, err := api.GetTranscript(id, "en"); err == nil {
			t.Errorf("GetTranscript(%q) accepted invalid ID", id)
		}
	}
	for _, input := range []string{"", "not-an-id", "https://example.com/watch?v=dQw4w9WgXcQ", "https://youtu.be/short"} {
		if _, err := api.GetTranscriptByURL(input, "en"); err == nil {
			t.Errorf("GetTranscriptByURL(%q) accepted invalid URL", input)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := api.GetTranscriptContext(ctx, "dQw4w9WgXcQ", "en"); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled request returned %v", err)
	}
}

func TestGetTranscriptHTTPFailures(t *testing.T) {
	cases := []struct {
		name, path, body, want string
		status                 int
	}{
		{name: "video HTTP error", path: "/watch", status: 429, want: "HTTP 429"},
		{name: "missing API key", path: "/watch", body: "<html></html>", want: "API key"},
		{name: "player HTTP error", path: "/youtubei/v1/player", status: 503, want: "HTTP 503"},
		{name: "invalid player JSON", path: "/youtubei/v1/player", body: "{", want: "parse Innertube"},
		{name: "unavailable video", path: "/youtubei/v1/player", body: `{"playabilityStatus":{"status":"ERROR","reason":"Video removed"}}`, want: "Video removed"},
		{name: "disabled captions", path: "/youtubei/v1/player", body: `{}`, want: "transcripts disabled"},
		{name: "no tracks", path: "/youtubei/v1/player", body: `{"captions":{"playerCaptionsTracklistRenderer":{"captionTracks":[]}}}`, want: "no caption tracks"},
		{name: "caption HTTP error", path: "/api/timedtext", status: 403, want: "HTTP 403"},
		{name: "empty captions", path: "/api/timedtext", body: " \n", want: "empty transcript"},
		{name: "invalid caption XML", path: "/api/timedtext", body: `<transcript><text start="0">oops`, want: "read transcript text"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			api := NewWithHTTPClient(&http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				if r.URL.Path != tc.path {
					return fixtureResponse(t, r), nil
				}
				resp := response(tc.body)
				if tc.status != 0 {
					resp.StatusCode = tc.status
				}
				return resp, nil
			})})
			got, err := api.GetTranscript("dQw4w9WgXcQ", "en")
			if got != nil || err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("got %+v, %v; want error containing %q", got, err, tc.want)
			}
		})
	}
}

func TestGetTranscriptPreservesTransportErrors(t *testing.T) {
	wantErr := errors.New("transport failure")
	for _, path := range []string{"/watch", "/youtubei/v1/player", "/api/timedtext"} {
		t.Run(path, func(t *testing.T) {
			api := NewWithHTTPClient(&http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				if r.URL.Path == path {
					return nil, wantErr
				}
				return fixtureResponse(t, r), nil
			})})
			if _, err := api.GetTranscript("dQw4w9WgXcQ", "en"); !errors.Is(err, wantErr) {
				t.Fatalf("transport error lost: %v", err)
			}
		})
	}
}

func TestGetTranscriptCancellationAtEveryStage(t *testing.T) {
	for _, path := range []string{"/watch", "/youtubei/v1/player", "/api/timedtext"} {
		t.Run(path, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			api := NewWithHTTPClient(&http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				if r.URL.Path == path {
					cancel()
					select {
					case <-r.Context().Done():
						return nil, r.Context().Err()
					case <-time.After(time.Second):
						return nil, errors.New("request did not receive cancellation")
					}
				}
				return fixtureResponse(t, r), nil
			})})
			if _, err := api.GetTranscriptByURLContext(ctx, "https://youtu.be/dQw4w9WgXcQ", "en"); !errors.Is(err, context.Canceled) {
				t.Fatalf("cancellation lost: %v", err)
			}
		})
	}
}

type stalledBody struct{ ctx context.Context }

func (b stalledBody) Read([]byte) (int, error) {
	select {
	case <-b.ctx.Done():
		return 0, b.ctx.Err()
	case <-time.After(time.Second):
		return 0, errors.New("body read did not receive deadline")
	}
}

func (stalledBody) Close() error { return nil }

func TestGetTranscriptDeadlinesIncludeResponseBodies(t *testing.T) {
	for _, useClientTimeout := range []bool{false, true} {
		for _, path := range []string{"/watch", "/youtubei/v1/player", "/api/timedtext"} {
			name := "context deadline" + path
			if useClientTimeout {
				name = "client timeout" + path
			}
			t.Run(name, func(t *testing.T) {
				client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
					if r.URL.Path == path {
						resp := response("")
						resp.Body = stalledBody{ctx: r.Context()}
						return resp, nil
					}
					return fixtureResponse(t, r), nil
				})}
				ctx := context.Background()
				if useClientTimeout {
					client.Timeout = 20 * time.Millisecond
				} else {
					var cancel context.CancelFunc
					ctx, cancel = context.WithTimeout(ctx, 20*time.Millisecond)
					defer cancel()
				}
				api := NewWithHTTPClient(client)
				if _, err := api.GetTranscriptContext(ctx, "dQw4w9WgXcQ", "en"); !errors.Is(err, context.DeadlineExceeded) {
					t.Fatalf("deadline lost: %v", err)
				}
			})
		}
	}
}
