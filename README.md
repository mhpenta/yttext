# yttext

A command-line tool and library that extracts transcript text from YouTube videos.

> **yttext is a Go port of [youtube-transcript-api](https://github.com/jdepoix/youtube-transcript-api)**
> by Jonas Depoix. The approach to fetching caption tracks comes from that project;
> this port covers a subset of its features (see the
> [comparison](#comparison-with-youtube-transcript-api-python) below). If you work in
> Python, use the original.

## Installation

```bash
go install github.com/mhpenta/yttext/cmd/yttext@latest
```

Or clone and build manually:

```bash
git clone https://github.com/mhpenta/yttext.git
cd yttext
go build -o yttext ./cmd/yttext
./yttext -help
```

The module root is the library; the executable is in `cmd/yttext`. For library
use, run `go get github.com/mhpenta/yttext` in your Go project.

## Usage

```bash
yttext [options] "[youtube_url_or_video_id]"
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
```

Examples:

```bash
# Default text output with timestamps
yttext "https://www.youtube.com/watch?v=dQw4w9WgXcQ"

# A bare video ID also works
yttext dQw4w9WgXcQ

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

The requested language must have a matching caption track. English (`en`) is
the default. An unavailable language produces an error listing available tracks;
the tool does not silently select another language or translate captions.

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
	"context"
	"fmt"
	"os"
	"time"

	"github.com/mhpenta/yttext"
)

func main() {
	// Create a new API client
	ytAPI := yttext.New()

	// Get a transcript for a video
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	transcript, err := ytAPI.GetTranscriptByURLContext(ctx, "https://www.youtube.com/watch?v=dQw4w9WgXcQ", "en")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Format the transcript as reader-friendly paragraphs
	fmt.Print(transcript.Readable())
}
```

Transcript values can also be formatted directly:

```go
transcript.Text()                  // timestamped text
transcript.Readable()              // wrapped paragraphs
transcript.SRT()                   // SubRip subtitles
transcript.JSON()                  // pretty JSON
transcript.Format(yttext.FormatSRT) // generic format selector
```

`Readable()` merges segments into paragraphs. To keep captions on separate lines:

```go
transcript.ReadableWithOptions(yttext.ReadableOptions{
	MaxLineLength:    80,
	GroupByParagraph: false,
})
```

`New()` limits each HTTP request, including reading its response body, to 30
seconds. `GetTranscriptContext` and `GetTranscriptByURLContext` accept a context
for cancellation or a deadline across the entire fetch. `GetTranscript` and
`GetTranscriptByURL` use a background context.

Use `NewWithHTTPClient(client)` to supply your own `*http.Client` with a timeout,
transport, proxy, or cookie jar. A supplied client is used without modification,
including its timeout (zero disables the client timeout); passing `nil` uses the
default client settings. Underlying errors are wrapped, so `errors.Is` and
`errors.As` can identify cancellation, timeouts, and other transport errors.
Use `errors.Is(err, yttext.ErrLanguageUnavailable)` to detect a missing language.

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
**Proxy support** | Via a custom HTTP client | Yes
**Cookie support** | Via a custom HTTP client | Yes for age-restricted videos
**Library usage** | Can be imported and used in Go projects | Can be imported and used in Python projects
**HTML formatting** | No | Optional preservation of HTML formatting

## License

MIT. See [LICENSE](LICENSE), which also carries the copyright notice of
youtube-transcript-api (Copyright (c) 2018 Jonas Depoix), the MIT-licensed project
this port is based on.
