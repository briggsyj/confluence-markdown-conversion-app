# Confluence to Markdown

Converts a Confluence space to Markdown, from a static HTML space export or the live Confluence
Cloud REST API. Go, single binary, no runtime dependencies.

## Install / build

Requires Go 1.26+.

```
go build ./cmd/confluence-to-markdown
go build ./cmd/confluence-to-markdown-gui   # optional desktop GUI
```

## Usage

### From a Confluence HTML space export

Download an export from `https://<YOUR_DOMAIN>.atlassian.net/wiki/spaces/exportspacewelcome.action?key=<SPACE_KEY>`.
`--input` accepts either the `.zip` directly or an already-extracted folder.

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

`confluence-to-markdown-gui` is a drag-and-drop window for the export flow: drop a `.zip` or
extracted folder, pick an output directory, and convert.

### Options

Both subcommands share these post-processing flags:

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

`testdata/kitchen-sink-export/` is a real Confluence Cloud export used by the integration tests
in `cmd/confluence-to-markdown/`.

## Credits

Original CoffeeScript/Node tool: Eric White / Meridus,
[meridius/confluence-to-markdown](https://github.com/meridius/confluence-to-markdown).
Go rewrite and current maintenance: John Briggs, [briggsyj](https://github.com/briggsyj).
