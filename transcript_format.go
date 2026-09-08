package yttext

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"
	"unicode"
)

const defaultLineLength = 80

// Format identifies an output format for a transcript.
type Format string

const (
	FormatText     Format = "text"
	FormatJSON     Format = "json"
	FormatSRT      Format = "srt"
	FormatReadable Format = "readable"
)

// ReadableOptions controls readable paragraph formatting.
type ReadableOptions struct {
	// MaxLineLength controls the maximum length of a line before wrapping.
	MaxLineLength int
	// GroupByParagraph determines if text should be merged into paragraphs.
	// When false, each segment is wrapped separately and separated by a newline.
	GroupByParagraph bool
}

// Text formats the transcript as timestamped plain text.
func (t Transcript) Text() string {
	var sb strings.Builder
	for _, segment := range t {
		timestamp := formatTime(segment.StartTime)
		sb.WriteString(fmt.Sprintf("[%s] %s\n", timestamp, segment.Text))
	}
	return sb.String()
}

// Readable formats the transcript as reader-friendly paragraphs.
func (t Transcript) Readable() string {
	return t.ReadableWithOptions(ReadableOptions{
		MaxLineLength:    defaultLineLength,
		GroupByParagraph: true,
	})
}

// ReadableWithOptions formats the transcript as reader-friendly paragraphs.
func (t Transcript) ReadableWithOptions(options ReadableOptions) string {
	if options.MaxLineLength <= 0 {
		options.MaxLineLength = defaultLineLength
	}

	var sb strings.Builder

	if options.GroupByParagraph {
		paragraphs := groupIntoParagraphs(t)
		for i, paragraph := range paragraphs {
			if i > 0 {
				sb.WriteString("\n\n")
			}
			sb.WriteString(wrapText(paragraph, options.MaxLineLength))
		}
		return sb.String()
	}

	for _, segment := range t {
		line := wrapText(segment.Text, options.MaxLineLength)
		if line == "" {
			continue
		}
		if sb.Len() > 0 {
			sb.WriteByte('\n')
		}
		sb.WriteString(line)
	}

	return sb.String()
}

// JSON formats the transcript as pretty-printed JSON.
func (t Transcript) JSON() (string, error) {
	return t.JSONIndent("", "  ")
}

// CompactJSON formats the transcript as compact JSON.
func (t Transcript) CompactJSON() (string, error) {
	jsonData, err := json.Marshal(t)
	if err != nil {
		return "", fmt.Errorf("failed to format JSON: %w", err)
	}
	return string(jsonData), nil
}

// JSONIndent formats the transcript as indented JSON.
func (t Transcript) JSONIndent(prefix string, indent string) (string, error) {
	jsonData, err := json.MarshalIndent(t, prefix, indent)
	if err != nil {
		return "", fmt.Errorf("failed to format JSON: %w", err)
	}
	return string(jsonData), nil
}

// SRT formats the transcript as SubRip Text.
func (t Transcript) SRT() string {
	var sb strings.Builder
	for i, segment := range t {
		startTime := segment.StartTime
		endTime := startTime + segment.Duration

		startFormatted := formatSRTTime(startTime)
		endFormatted := formatSRTTime(endTime)

		sb.WriteString(fmt.Sprintf("%d\n", i+1))
		sb.WriteString(fmt.Sprintf("%s --> %s\n", startFormatted, endFormatted))
		sb.WriteString(fmt.Sprintf("%s\n\n", segment.Text))
	}
	return sb.String()
}

// Format formats the transcript using one of the built-in formats.
func (t Transcript) Format(format Format) (string, error) {
	switch Format(strings.ToLower(string(format))) {
	case FormatText:
		return t.Text(), nil
	case FormatJSON:
		return t.JSON()
	case FormatSRT:
		return t.SRT(), nil
	case FormatReadable:
		return t.Readable(), nil
	default:
		return "", fmt.Errorf("unknown format type: %s", format)
	}
}

// formatTime formats time in MM:SS or HH:MM:SS format.
func formatTime(seconds float64) string {
	duration := time.Duration(seconds * float64(time.Second))
	h := int(duration.Hours())
	m := int(duration.Minutes()) % 60
	s := int(duration.Seconds()) % 60

	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%d:%02d", m, s)
}

// formatSRTTime formats time in SRT format (HH:MM:SS,mmm).
func formatSRTTime(seconds float64) string {
	totalMs := int(math.Round(seconds * 1000))
	h := totalMs / (60 * 60 * 1000)
	m := (totalMs / (60 * 1000)) % 60
	s := (totalMs / 1000) % 60
	ms := totalMs % 1000

	return fmt.Sprintf("%02d:%02d:%02d,%03d", h, m, s, ms)
}

func shouldStartNewParagraph(prevText, currText string) bool {
	endsWithPunctuation := strings.HasSuffix(strings.TrimSpace(prevText), ".") ||
		strings.HasSuffix(strings.TrimSpace(prevText), "!") ||
		strings.HasSuffix(strings.TrimSpace(prevText), "?")

	startsWithCapital := len(currText) > 0 && unicode.IsUpper([]rune(currText)[0])
	containsNotation := strings.Contains(prevText, "[") || strings.Contains(currText, "[")

	return (endsWithPunctuation && startsWithCapital) || containsNotation
}

func groupIntoParagraphs(transcript Transcript) []string {
	if len(transcript) == 0 {
		return []string{}
	}

	var paragraphs []string
	currentParagraph := transcript[0].Text

	for i := 1; i < len(transcript); i++ {
		if shouldStartNewParagraph(transcript[i-1].Text, transcript[i].Text) {
			paragraphs = append(paragraphs, currentParagraph)
			currentParagraph = transcript[i].Text
		} else {
			if !strings.HasSuffix(currentParagraph, " ") && !strings.HasPrefix(transcript[i].Text, " ") {
				currentParagraph += " "
			}
			currentParagraph += transcript[i].Text
		}
	}

	if len(currentParagraph) > 0 {
		paragraphs = append(paragraphs, currentParagraph)
	}

	return paragraphs
}

func wrapText(text string, lineLength int) string {
	if lineLength <= 0 {
		return text
	}

	var sb strings.Builder
	words := strings.Fields(text)

	if len(words) == 0 {
		return ""
	}

	currentLine := words[0]

	for _, word := range words[1:] {
		if len(currentLine)+1+len(word) > lineLength {
			sb.WriteString(currentLine + "\n")
			currentLine = word
		} else {
			currentLine += " " + word
		}
	}

	sb.WriteString(currentLine)

	return sb.String()
}
