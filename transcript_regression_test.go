package yttext

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"testing"
)

func TestParseTranscriptXMLPreservesLineBreaks(t *testing.T) {
	for name, input := range map[string]string{
		"text":              `<transcript><text start="0" dur="2">hello<br/>world</text></transcript>`,
		"paragraph":         `<timedtext><body><p t="0" d="2000"><s>hello<br/>world</s></p></body></timedtext>`,
		"nested formatting": `<transcript><text start="0" dur="2"><b>hello</b><br></br><i>world</i></text></transcript>`,
	} {
		t.Run(name, func(t *testing.T) {
			got, err := New().parseTranscriptXML([]byte(input))
			if err != nil {
				t.Fatal(err)
			}
			if len(got) != 1 || got[0].Text != "hello\nworld" {
				t.Fatalf("line break lost: %+v", got)
			}
			if !strings.Contains(got.SRT(), "hello\nworld\n\n") {
				t.Fatalf("SRT lost line break: %q", got.SRT())
			}
		})
	}
}

func TestParseTranscriptXMLRejectsInvalidTiming(t *testing.T) {
	for name, template := range map[string]string{
		"start":                          `<text start=%q dur="1">bad</text>`,
		"duration":                       `<text start="0" dur=%q>bad</text>`,
		"milliseconds start":             `<p t=%q d="1000">bad</p>`,
		"milliseconds duration":          `<p t="0" d=%q>bad</p>`,
		"invalid start with fallback":    `<text start=%q t="0" dur="1">bad</text>`,
		"invalid duration with fallback": `<text start="0" dur=%q d="1000">bad</text>`,
	} {
		for _, value := range []string{"NaN", "+Inf", "-Inf", "-1", "oops", "1e999"} {
			t.Run(name+"/"+value, func(t *testing.T) {
				// A valid sibling must not hide the malformed entry.
				input := `<transcript><text start="0" dur="1">valid</text>` + fmt.Sprintf(template, value) + `</transcript>`
				got, err := New().parseTranscriptXML([]byte(input))
				if got != nil || err == nil || !strings.Contains(err.Error(), "timing") {
					t.Fatalf("got %+v, %v; want timing error", got, err)
				}
			})
		}
	}
}

func TestParseTranscriptXMLMissingTiming(t *testing.T) {
	api := New()
	if _, err := api.parseTranscriptXML([]byte(`<transcript><text dur="1">hello</text></transcript>`)); err == nil {
		t.Fatal("accepted caption with missing start time")
	}
	// Missing duration is valid and defaults to zero in both supported formats.
	for _, input := range []string{
		`<transcript><text start="0">hello</text></transcript>`,
		`<timedtext><p t="0">hello</p></timedtext>`,
	} {
		got, err := api.parseTranscriptXML([]byte(input))
		if err != nil || len(got) != 1 || got[0].StartTime != 0 || got[0].Duration != 0 {
			t.Fatalf("missing duration: got %+v, %v", got, err)
		}
	}
}

func TestReadableParagraphGrouping(t *testing.T) {
	transcript := Transcript{{Text: "Hello"}, {Text: "world."}, {Text: "Next sentence."}}
	if got := transcript.Readable(); got != "Hello world.\n\nNext sentence." {
		t.Fatalf("grouped text = %q", got)
	}
	if got := transcript.ReadableWithOptions(ReadableOptions{GroupByParagraph: false}); got != "Hello\nworld.\nNext sentence." {
		t.Fatalf("ungrouped text = %q", got)
	}
	if got := transcript.ReadableWithOptions(ReadableOptions{GroupByParagraph: false, MaxLineLength: 8}); got != "Hello\nworld.\nNext\nsentence." {
		t.Fatalf("wrapped ungrouped text = %q", got)
	}
	withEmptySegments := Transcript{{Text: " "}, {Text: "hello"}, {}, {Text: "world"}, {Text: "\n"}}
	if got := withEmptySegments.ReadableWithOptions(ReadableOptions{}); got != "hello\nworld" {
		t.Fatalf("empty segments introduced blank lines: %q", got)
	}
	for _, group := range []bool{false, true} {
		if got := (Transcript(nil)).ReadableWithOptions(ReadableOptions{GroupByParagraph: group}); got != "" {
			t.Fatalf("nil transcript = %q", got)
		}
	}
}

func TestJSONPreservesEncodingErrors(t *testing.T) {
	transcript := Transcript{{Text: "hello", StartTime: math.NaN()}}
	for _, format := range []func() (string, error){transcript.JSON, transcript.CompactJSON} {
		_, err := format()
		var unsupported *json.UnsupportedValueError
		if !errors.As(err, &unsupported) {
			t.Fatalf("underlying JSON error lost: %v", err)
		}
	}
}
