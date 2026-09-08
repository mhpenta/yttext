package cli

import (
	"flag"
	"fmt"
	"os"

	"github.com/mhpenta/yttext"

	"github.com/atotto/clipboard"
)

// CLI represents the command-line interface
type CLI struct {
	// Deprecated: Debug is retained for compatibility and has no effect.
	Debug bool
	// Deprecated: LogRequest is retained for compatibility and has no effect.
	LogRequest   bool
	LanguageCode string
	FormatType   string
	Readable     bool
	Copy         bool
	VideoURL     string
}

// NewCLI creates a new CLI instance with parsed command-line arguments
func NewCLI() *CLI {
	cli := &CLI{}

	flag.BoolVar(&cli.Debug, "debug", false, "[Deprecated] No longer has any effect")
	flag.BoolVar(&cli.LogRequest, "log-request", false, "[Deprecated] No longer has any effect")
	flag.StringVar(&cli.LanguageCode, "lang", "en", "Language code for transcript (e.g., 'en', 'es', 'fr')")
	flag.StringVar(&cli.FormatType, "format", "text", "Output format (text, json, srt, readable)")
	flag.BoolVar(&cli.Readable, "readable", false, "Use readable format (same as --format=readable)")
	flag.BoolVar(&cli.Copy, "copy", false, "Copy output to clipboard in addition to stdout")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [options] \"youtube_url_or_video_id\"\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Get transcript from YouTube video and print it to stdout\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nNote: Remember to put quotes around YouTube URLs to avoid shell interpretation issues\n")
	}

	flag.Parse()

	if flag.NArg() != 1 {
		flag.Usage()
		os.Exit(1)
	}

	cli.VideoURL = flag.Arg(0)

	return cli
}

// Run executes the CLI application
func (c *CLI) Run() int {
	if c.Readable {
		c.FormatType = "readable"
	}

	ytAPI := yttext.New()

	transcript, err := ytAPI.GetTranscriptByURL(c.VideoURL, c.LanguageCode)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)

		fmt.Fprintf(os.Stderr, "\nPossible solutions:\n")
		fmt.Fprintf(os.Stderr, "1. Make sure the video exists and is publicly accessible\n")
		fmt.Fprintf(os.Stderr, "2. Verify that the video has captions/transcripts available\n")
		fmt.Fprintf(os.Stderr, "3. Try a different video to confirm the tool is working\n")

		return 1
	}

	if len(transcript) == 0 {
		fmt.Fprintf(os.Stderr, "Error: No transcript content found\n")
		return 1
	}

	output, err := transcript.Format(yttext.Format(c.FormatType))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}

	if c.Copy {
		if err := clipboard.WriteAll(output); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: Failed to copy to clipboard: %v\n", err)
		} else {
			fmt.Fprintf(os.Stderr, "Copied to clipboard!\n")
		}
	}

	fmt.Print(output)
	return 0
}
