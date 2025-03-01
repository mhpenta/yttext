# yttext

A command-line tool and library that extracts transcript text from YouTube videos. This is a rough Go port of the [youtube-transcript-api](https://github.com/jdepoix/youtube-transcript-api) Python package.

## Installation

```bash
go install github.com/mhpenta/yttext@latest
```

Or clone and build manually:

```bash
git clone https://github.com/mhpenta/yttext.git
cd yttext
go build
```

## Usage

```bash
yttext [options] "[youtube_url]"
```

Options:
```
  -copy
        Copy output to clipboard in addition to stdout
  -format string
        Output format (text, json, srt, readable) (default "text")
  -lang string
        Language code for transcript (e.g., 'en', 'es', 'fr')
  -readable
        Use readable format (same as --format=readable)
  -debug
        [Deprecated] No longer has any effect
  -log-request
        [Deprecated] No longer has any effect
```

Examples:

```bash
# Default text output with timestamps
yttext "https://www.youtube.com/watch?v=dQw4w9WgXcQ"

# JSON output
yttext -format json "https://www.youtube.com/watch?v=dQw4w9WgXcQ"

# SRT format (for subtitle files)
yttext -format srt "https://www.youtube.com/watch?v=dQw4w9WgXcQ" > subtitles.srt

# Get transcript in readable paragraph format
yttext -readable "https://www.youtube.com/watch?v=dQw4w9WgXcQ"

# Copy transcript to clipboard while also printing to screen
yttext -copy "https://www.youtube.com/watch?v=dQw4w9WgXcQ"

# Combine options - get readable format and copy to clipboard
yttext -readable -copy "https://www.youtube.com/watch?v=dQw4w9WgXcQ"
```

Note: Always put the YouTube URL in quotes to avoid shell interpretation issues with special characters.

Text output format (default):
```
[0:00] Never gonna give you up
[0:03] Never gonna let you down
...
```

JSON output format:
```json
[
  {
    "text": "Never gonna give you up",
    "duration": 3.05,
    "offset": 0.0,
    "start": 0.0
  },
  {
    "text": "Never gonna let you down",
    "duration": 3.79,
    "offset": 3.05,
    "start": 3.05
  },
  ...
]
```

## Features

- Extracts transcripts from YouTube videos
- Displays text with timestamps
- Works with any public YouTube video that has captions
- Supports both youtube.com and youtu.be URLs
- Can be used both as a command-line tool and as a library in Go projects

## Library Usage

You can use yttext as a library in your Go projects:

```go
package main

import (
	"fmt"
	"os"

	"github.com/mhpenta/yttext/pkg/api"
	"github.com/mhpenta/yttext/pkg/formatters"
)

func main() {
	// Create a new API client
	ytAPI := api.New()

	// Get transcripts for a video
	transcripts, err := ytAPI.GetTranscriptsByURL("https://www.youtube.com/watch?v=dQw4w9WgXcQ", "en")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Format the transcripts in different ways
	
	// 1. Basic text with timestamps
	textFormatter, _ := formatters.NewFormatter("text")
	textOutput, _ := textFormatter.Format(transcripts)
	
	// 2. JSON format
	jsonFormatter, _ := formatters.NewFormatter("json")
	jsonOutput, _ := jsonFormatter.Format(transcripts)
	
	// 3. SRT subtitle format
	srtFormatter, _ := formatters.NewFormatter("srt")
	srtOutput, _ := srtFormatter.Format(transcripts)
	
	// 4. Human-readable paragraphs
	readableFormatter, _ := formatters.NewFormatter("readable")
	readableOutput, _ := readableFormatter.Format(transcripts)
	
	// Print the readable format
	fmt.Print(readableOutput)
}
```

## Comparison with youtube-transcript-api (Python)

Feature | yttext (Go) | youtube-transcript-api (Python)
------- | --------- | -------------------------
**Language** | Go | Python 
**Interface** | Library + CLI | Library + CLI
**Multiple languages** | Basic language selection | Comprehensive language selection with priority
**Output format** | Terminal text, JSON, SRT, Readable | Multiple formats (JSON, Text, SRT, WebVTT, Pretty Print)
**Transcript types** | No distinction | Can filter manually created vs auto-generated
**Translation** | Not supported | Supports translating transcripts
**Error handling** | Basic | Comprehensive with specific exceptions
**Proxy support** | No | Yes
**Cookie support** | No | Yes for age-restricted videos
**Library usage** | Can be imported and used in Go projects | Can be imported and used in Python projects
**HTML formatting** | No | Optional preservation of HTML formatting

## License

MIT