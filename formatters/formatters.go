package formatters

import (
	"fmt"

	"github.com/mhpenta/yttext"
)

const defaultLineLength = 80

// Formatter is the interface that all transcript formatters implement.
type Formatter interface {
	Format(transcript yttext.Transcript) (string, error)
}

// TextFormatter formats transcripts as plain text with timestamps.
type TextFormatter struct{}

// Format implements the Formatter interface for TextFormatter.
func (f *TextFormatter) Format(transcript yttext.Transcript) (string, error) {
	return transcript.Text(), nil
}

// ReadableFormatter formats transcripts in a paragraph-based, reader-friendly format.
type ReadableFormatter struct {
	// MaxLineLength controls the maximum length of a line before wrapping.
	MaxLineLength int
	// GroupByParagraph determines if text should be merged into paragraphs.
	// When false, each segment is wrapped separately and separated by a newline.
	GroupByParagraph bool
}

// Format implements the Formatter interface for ReadableFormatter.
func (f *ReadableFormatter) Format(transcript yttext.Transcript) (string, error) {
	maxLineLength := f.MaxLineLength
	if maxLineLength <= 0 {
		maxLineLength = defaultLineLength
	}

	return transcript.ReadableWithOptions(yttext.ReadableOptions{
		MaxLineLength:    maxLineLength,
		GroupByParagraph: f.GroupByParagraph,
	}), nil
}

// JSONFormatter formats transcripts as JSON.
type JSONFormatter struct {
	Pretty bool
}

// Format implements the Formatter interface for JSONFormatter.
func (f *JSONFormatter) Format(transcript yttext.Transcript) (string, error) {
	if f.Pretty {
		return transcript.JSON()
	}
	return transcript.CompactJSON()
}

// SRTFormatter formats transcripts in SubRip Text (SRT) format.
type SRTFormatter struct{}

// Format implements the Formatter interface for SRTFormatter.
func (f *SRTFormatter) Format(transcript yttext.Transcript) (string, error) {
	return transcript.SRT(), nil
}

// NewFormatter creates a formatter based on the specified format type.
func NewFormatter(formatType string) (Formatter, error) {
	switch yttext.Format(formatType) {
	case yttext.FormatText:
		return &TextFormatter{}, nil
	case yttext.FormatJSON:
		return &JSONFormatter{Pretty: true}, nil
	case yttext.FormatSRT:
		return &SRTFormatter{}, nil
	case yttext.FormatReadable:
		return &ReadableFormatter{
			MaxLineLength:    defaultLineLength,
			GroupByParagraph: true,
		}, nil
	default:
		return nil, fmt.Errorf("unknown format type: %s", formatType)
	}
}
