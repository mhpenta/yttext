package yttext

import (
	"strings"
	"testing"
)

func TestExtractVideoID(t *testing.T) {
	api := New()

	tests := map[string]string{
		"Kj9kDBg7b-c": "Kj9kDBg7b-c",
		"https://www.youtube.com/watch?v=Kj9kDBg7b-c&t=1": "Kj9kDBg7b-c",
		"https://youtu.be/Kj9kDBg7b-c?si=abc123":          "Kj9kDBg7b-c",
		"https://www.youtube.com/embed/Kj9kDBg7b-c":       "Kj9kDBg7b-c",
		"https://www.youtube.com/shorts/Kj9kDBg7b-c":      "Kj9kDBg7b-c",
		"https://m.youtube.com/watch?v=Kj9kDBg7b-c":       "Kj9kDBg7b-c",
	}

	for input, want := range tests {
		got, err := api.extractVideoID(input)
		if err != nil {
			t.Fatalf("extractVideoID(%q) returned error: %v", input, err)
		}
		if got != want {
			t.Fatalf("extractVideoID(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestParseTranscriptXMLTextNodes(t *testing.T) {
	api := New()
	xmlData := []byte(`<?xml version="1.0" encoding="utf-8" ?><transcript><text start="0.24" dur="5.2">Bricks &amp; Mini Figs</text><text start="3.04" dur="5.519">created by Reckless Ben</text></transcript>`)

	transcripts, err := api.parseTranscriptXML(xmlData)
	if err != nil {
		t.Fatalf("parseTranscriptXML returned error: %v", err)
	}

	if len(transcripts) != 2 {
		t.Fatalf("len(transcripts) = %d, want 2", len(transcripts))
	}
	if transcripts[0].Text != "Bricks & Mini Figs" {
		t.Fatalf("first transcript text = %q", transcripts[0].Text)
	}
	if transcripts[0].StartTime != 0.24 || transcripts[0].Duration != 5.2 {
		t.Fatalf("first transcript timing = (%v, %v)", transcripts[0].StartTime, transcripts[0].Duration)
	}
}

func TestParseTranscriptXMLParagraphNodes(t *testing.T) {
	api := New()
	xmlData := []byte(`<?xml version="1.0" encoding="utf-8" ?><timedtext format="3"><body><p t="240" d="5200"><s>Bricks</s><s t="320"> and</s><s t="479"> Mini</s><s t="880"> Figs</s></p><p t="3030" d="2410"></p><p t="3040" d="5519"><s>GoFundMe</s><s> for Brian</s></p></body></timedtext>`)

	transcripts, err := api.parseTranscriptXML(xmlData)
	if err != nil {
		t.Fatalf("parseTranscriptXML returned error: %v", err)
	}

	if len(transcripts) != 2 {
		t.Fatalf("len(transcripts) = %d, want 2", len(transcripts))
	}
	if transcripts[0].Text != "Bricks and Mini Figs" {
		t.Fatalf("first transcript text = %q", transcripts[0].Text)
	}
	if transcripts[0].StartTime != 0.24 || transcripts[0].Duration != 5.2 {
		t.Fatalf("first transcript timing = (%v, %v)", transcripts[0].StartTime, transcripts[0].Duration)
	}
}

func TestTranscriptFormatMethods(t *testing.T) {
	transcript := Transcript{
		{
			Text:      "Hello world.",
			StartTime: 0.24,
			Offset:    0.24,
			Duration:  1.5,
		},
		{
			Text:      "Next sentence.",
			StartTime: 2.0,
			Offset:    2.0,
			Duration:  2.25,
		},
	}

	if got := transcript.Text(); got != "[0:00] Hello world.\n[0:02] Next sentence.\n" {
		t.Fatalf("Text() = %q", got)
	}

	if got := transcript.Readable(); got != "Hello world.\n\nNext sentence." {
		t.Fatalf("Readable() = %q", got)
	}

	if got := transcript.SRT(); got != "1\n00:00:00,240 --> 00:00:01,740\nHello world.\n\n2\n00:00:02,000 --> 00:00:04,250\nNext sentence.\n\n" {
		t.Fatalf("SRT() = %q", got)
	}

	jsonOutput, err := transcript.JSON()
	if err != nil {
		t.Fatalf("JSON() returned error: %v", err)
	}
	if want := `"text": "Hello world."`; !contains(jsonOutput, want) {
		t.Fatalf("JSON() = %q, want to contain %q", jsonOutput, want)
	}

	formatted, err := transcript.Format(FormatReadable)
	if err != nil {
		t.Fatalf("Format(FormatReadable) returned error: %v", err)
	}
	if formatted != transcript.Readable() {
		t.Fatalf("Format(FormatReadable) = %q, want %q", formatted, transcript.Readable())
	}
}

func contains(s string, substr string) bool {
	return strings.Contains(s, substr)
}
