package formatters

import (
	"testing"

	"github.com/mhpenta/yttext"
)

func TestReadableFormatterParagraphOption(t *testing.T) {
	transcript := yttext.Transcript{{Text: "Hello"}, {Text: "world."}}
	for _, tc := range []struct {
		group bool
		want  string
	}{
		{group: true, want: "Hello world."},
		{group: false, want: "Hello\nworld."},
	} {
		formatter := &ReadableFormatter{GroupByParagraph: tc.group}
		got, err := formatter.Format(transcript)
		if err != nil || got != tc.want {
			t.Errorf("group=%v: got %q, %v; want %q", tc.group, got, err, tc.want)
		}
	}
}
