package formatters

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unicode"

	"github.com/mhpenta/yttext/pkg/api"
)

const defaultLineLength = 80

// Formatter is the interface that all transcript formatters must implement
type Formatter interface {
	Format(transcripts []api.Transcript) (string, error)
}

// TextFormatter formats transcripts as plain text with timestamps
type TextFormatter struct{}

// Format implements the Formatter interface for TextFormatter
func (f *TextFormatter) Format(transcripts []api.Transcript) (string, error) {
	var sb strings.Builder
	for _, t := range transcripts {
		timestamp := formatTime(t.StartTime)
		sb.WriteString(fmt.Sprintf("[%s] %s\n", timestamp, t.Text))
	}
	return sb.String(), nil
}

// ReadableFormatter formats transcripts in a paragraph-based, reader-friendly format
type ReadableFormatter struct {
	// MaxLineLength controls the maximum length of a line before wrapping
	MaxLineLength int
	// GroupByParagraph determines if text should be merged into paragraphs
	GroupByParagraph bool
}

// Format implements the Formatter interface for ReadableFormatter
func (f *ReadableFormatter) Format(transcripts []api.Transcript) (string, error) {
	if f.MaxLineLength <= 0 {
		f.MaxLineLength = defaultLineLength
	}

	var sb strings.Builder

	if f.GroupByParagraph {
		paragraphs := groupIntoParagraphs(transcripts)
		for i, paragraph := range paragraphs {
			if i > 0 {
				sb.WriteString("\n\n")
			}
			sb.WriteString(wrapText(paragraph, f.MaxLineLength))
		}
	} else {
		currentLine := ""
		for i, t := range transcripts {
			if i > 0 && shouldStartNewParagraph(transcripts[i-1].Text, t.Text) {
				sb.WriteString(wrapText(currentLine, f.MaxLineLength))
				sb.WriteString("\n\n")
				currentLine = t.Text
			} else if len(currentLine) > 0 {
				if !strings.HasSuffix(currentLine, " ") && !strings.HasPrefix(t.Text, " ") {
					currentLine += " "
				}
				currentLine += t.Text
			} else {
				currentLine = t.Text
			}
		}

		if len(currentLine) > 0 {
			sb.WriteString(wrapText(currentLine, f.MaxLineLength))
		}
	}

	return sb.String(), nil
}

// JSONFormatter formats transcripts as JSON
type JSONFormatter struct {
	Pretty bool
}

// Format implements the Formatter interface for JSONFormatter
func (f *JSONFormatter) Format(transcripts []api.Transcript) (string, error) {
	var jsonData []byte
	var err error
	if f.Pretty {
		jsonData, err = json.MarshalIndent(transcripts, "", "  ")
	} else {
		jsonData, err = json.Marshal(transcripts)
	}
	if err != nil {
		return "", fmt.Errorf("failed to format JSON: %v", err)
	}
	return string(jsonData), nil
}

// SRTFormatter formats transcripts in SubRip Text (SRT) format
type SRTFormatter struct{}

// Format implements the Formatter interface for SRTFormatter
func (f *SRTFormatter) Format(transcripts []api.Transcript) (string, error) {
	var sb strings.Builder
	for i, t := range transcripts {
		startTime := t.StartTime
		endTime := startTime + t.Duration

		startFormatted := formatSRTTime(startTime)
		endFormatted := formatSRTTime(endTime)

		sb.WriteString(fmt.Sprintf("%d\n", i+1))
		sb.WriteString(fmt.Sprintf("%s --> %s\n", startFormatted, endFormatted))
		sb.WriteString(fmt.Sprintf("%s\n\n", t.Text))
	}
	return sb.String(), nil
}

// formatTime formats time in MM:SS or HH:MM:SS format
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

// formatSRTTime formats time in SRT format (HH:MM:SS,mmm)
func formatSRTTime(seconds float64) string {
	duration := time.Duration(seconds * float64(time.Second))
	h := int(duration.Hours())
	m := int(duration.Minutes()) % 60
	s := int(duration.Seconds()) % 60
	ms := int((seconds - float64(int(seconds))) * 1000)

	return fmt.Sprintf("%02d:%02d:%02d,%03d", h, m, s, ms)
}

// shouldStartNewParagraph determines if we should start a new paragraph based on content
func shouldStartNewParagraph(prevText, currText string) bool {
	// Start a new paragraph if:
	// 1. Previous text ends with a sentence-ending punctuation
	// 2. Current text starts with a capital letter
	// 3. Or if music/sound notations are present (like [Music], [Applause])

	endsWithPunctuation := strings.HasSuffix(strings.TrimSpace(prevText), ".") ||
		strings.HasSuffix(strings.TrimSpace(prevText), "!") ||
		strings.HasSuffix(strings.TrimSpace(prevText), "?")

	startsWithCapital := len(currText) > 0 && unicode.IsUpper([]rune(currText)[0])

	containsNotation := strings.Contains(prevText, "[") || strings.Contains(currText, "[")

	return (endsWithPunctuation && startsWithCapital) || containsNotation
}

// groupIntoParagraphs groups transcript segments into coherent paragraphs
func groupIntoParagraphs(transcripts []api.Transcript) []string {
	if len(transcripts) == 0 {
		return []string{}
	}

	var paragraphs []string
	currentParagraph := transcripts[0].Text

	for i := 1; i < len(transcripts); i++ {
		if shouldStartNewParagraph(transcripts[i-1].Text, transcripts[i].Text) {
			paragraphs = append(paragraphs, currentParagraph)
			currentParagraph = transcripts[i].Text
		} else {
			if !strings.HasSuffix(currentParagraph, " ") && !strings.HasPrefix(transcripts[i].Text, " ") {
				currentParagraph += " "
			}
			currentParagraph += transcripts[i].Text
		}
	}

	if len(currentParagraph) > 0 {
		paragraphs = append(paragraphs, currentParagraph)
	}

	return paragraphs
}

// wrapText wraps text to the specified line length
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

// NewFormatter creates a formatter based on the specified format type
func NewFormatter(formatType string) (Formatter, error) {
	switch formatType {
	case "text":
		return &TextFormatter{}, nil
	case "json":
		return &JSONFormatter{Pretty: true}, nil
	case "srt":
		return &SRTFormatter{}, nil
	case "readable":
		return &ReadableFormatter{
			MaxLineLength:    80,
			GroupByParagraph: true,
		}, nil
	default:
		return nil, fmt.Errorf("unknown format type: %s", formatType)
	}
}
