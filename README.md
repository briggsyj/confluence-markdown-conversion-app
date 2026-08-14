# Confluence Markdown Conversion App

[![Release](https://github.com/briggsyj/confluence-markdown-conversion-app/actions/workflows/release.yml/badge.svg)](https://github.com/briggsyj/confluence-markdown-conversion-app/actions/workflows/release.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

A simple desktop app that turns an exported Confluence space into a folder of Markdown files —
drop in your export, pick where to save it, and convert.

## Download

Grab the latest version for your operating system from the
[Releases page](https://github.com/briggsyj/confluence-markdown-conversion-app/releases/latest).
No installation needed — just download and run it.

## How to use it

1. **Export your Confluence space.** In Confluence, go to your space, open **Space settings**,
   and choose **Export space** (or visit
   `https://<YOUR_DOMAIN>.atlassian.net/wiki/spaces/exportspacewelcome.action?key=<SPACE_KEY>`
   directly). This downloads a `.zip` file.
2. **Open the app** and either drag that `.zip` file onto the window or click **Browse** to
   select it (an already-unzipped export folder works too).
3. **Choose an output folder** — this is where the converted Markdown files will be saved.
4. Click **Convert**. Progress and any warnings appear in the log at the bottom of the window.

### Optional settings

| Setting | What it does |
| --- | --- |
| Confluence space URL | If your pages link to each other using full Confluence URLs (e.g. `https://acme.atlassian.net/wiki/spaces/TEAM/pages/12345/Some+Page`), set this to your space's URL prefix so those links get rewritten to point at the converted Markdown files instead — for that example it would be `https://acme.atlassian.net/wiki/spaces/`. Leave it blank if you're not sure. |
| Rewrite internal links and reorganize attachments | Turns Confluence page links into working links between your new Markdown files, and tidies up attachments. On by default — recommended. |
| Move each page to its own `/Title/` directory | Puts every page in its own folder alongside its attachments, instead of one flat pile of files. |
| Skip attachments over 10MB | Avoids pulling in oversized files. On by default. |

## Development

Requires Go 1.26+.

```
go build ./...
go vet ./...
go test ./...
```

`testdata/kitchen-sink-export/` is a real Confluence Cloud export used by the integration tests
in `cmd/confluence-to-markdown-gui/`.

## Credits

Original CoffeeScript/Node tool: Eric White / Meridus,
[meridius/confluence-to-markdown](https://github.com/meridius/confluence-to-markdown).
Go rewrite and current maintenance: John Briggs, [briggsyj](https://github.com/briggsyj).
