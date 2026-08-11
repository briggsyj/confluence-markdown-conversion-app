# confluence-to-markdown

Converts a Confluence space to Markdown, either from a static HTML space export or by pulling
live from the Confluence Cloud REST API. A Go rewrite of the original CoffeeScript/Node tool,
built for single-binary distribution and mature, actively-maintained dependencies.

## Requirements

Go 1.26+.

## Install / build

```
go build ./cmd/confluence-to-markdown
go build ./cmd/confluence-to-markdown-gui   # optional desktop GUI
```

## Usage

### From a Confluence HTML space export

Download an export via `https://<YOUR_DOMAIN>.atlassian.net/wiki/spaces/exportspacewelcome.action?key=<SPACE_KEY>`.
The `--input` flag accepts either the downloaded `.zip` directly or an already-extracted folder.

```
confluence-to-markdown export \
  --input path/to/export.zip \
  --output path/to/output \
  --confluence-url https://<YOUR_DOMAIN>.atlassian.net/wiki/spaces/
```

### From the live Confluence Cloud REST API

```
confluence-to-markdown api \
  --site https://<YOUR_DOMAIN>.atlassian.net \
  --email you@example.com \
  --api-token <your Atlassian API token> \
  --space <SPACE_KEY> \
  --output path/to/output
```

### GUI

`confluence-to-markdown-gui` gives a drag-and-drop window for the export flow: drop a `.zip` or
extracted folder, pick an output directory, and convert.

### Options

Both subcommands share post-processing flags:

| flag | description |
| --- | --- |
| `--post-process` | rewrite internal links and reorganize attachments after conversion (default on) |
| `--no-article-dir` | don't move each page into its own `<Title>/<Title>.md` directory |
| `--no-file-size-limit` | don't skip attachments over 10MB |

Run `confluence-to-markdown export --help` / `api --help` for the full flag list.

## Development

```
go build ./...
go vet ./...
go test ./...
```

`testdata/kitchen-sink-export/` holds a real Confluence Cloud export (zipped and pre-extracted)
used by the integration tests in `cmd/confluence-to-markdown/`.

## Recognition

### Original project and packages

Eric White / Meridus for the original [Confluence to Markdown](https://github.com/meridius/confluence-to-markdown)
(the CoffeeScript/Node tool this project began as a rewrite of).

### Current maintenance

John Briggs [briggsyj](https://github.com/briggsyj)
