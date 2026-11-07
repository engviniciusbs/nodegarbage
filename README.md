# NodeGarbage

**English** | [Português (BR)](README.pt-BR.md)

NodeGarbage is a free, open-source (MIT) disk-cleanup desktop app for Windows, built for developers and power users. It scans your drives, classifies the files and caches that take the most space, explains what can and cannot be deleted safely, and deletes only what you confirm.

<!-- screenshot: docs/screenshot.png (to be added) -->

The focus is on development caches and build artifacts (`node_modules`, package-manager caches, build output, Docker/WSL disks, IDE caches), without forgetting general Windows and browser caches.

## Key features

- **Fast directory scan** of any fixed volume: concurrent directory walk, long-path (`\\?\`) support, live progress, top 1000 largest files and an extension histogram.
- **Built-in rules** for Node, Python, Rust, Go, JVM, .NET, Android, containers, browsers, editors and Windows, each with a verdict (`safe`, `caution`, `keep`), an explanation and the command that regenerates the data.
- **Protected paths** (system folders, drive roots and similar) that can never be cleaned, with or without administrator rights.
- **Results view** with a sortable tree, a treemap of the current folder, category filters, "inactive project" detection (90 days) and a details panel.
- **Optional AI report** that explains the items the rules do not recognize (see [Privacy](#privacy)).
- **Safe cleaning**: confirmation dialog, Recycle Bin by default (permanent delete is opt-in and needs an extra confirmation), per-item results and a cleanup history.
- **Your own rules**: drop YAML files in `%LOCALAPPDATA%\NodeGarbage\rules.d\`.
- Works 100% without AI. Opens without admin and offers "restart as administrator" for a fuller scan.

## Only you delete: AI and rules only advise

Rules and the AI classify and explain; they never delete. The only code path that removes files is your explicit confirmation: the app builds a cleanup plan, shows it in a dialog, and only a single-use token (32 bytes, valid for 5 minutes) issued for that exact plan can execute it. Nothing is ever pre-selected by the AI, the AI has no tools or access to the app, and its answer is treated as untrusted data: paths outside what was sent are discarded, protected items never change, and when the AI and a rule disagree the more conservative verdict wins.

## Privacy

NodeGarbage has **zero telemetry**. The only network traffic is the AI analysis you trigger yourself, to the provider you configured.

What goes to the AI, only when you click send:

- Paths, sizes, file counts, modification dates and file extensions (up to 200 classified items, 150 unknown items, 30 extensions, 50 files, about 30,000 tokens), plus volume totals.
- **No file contents**, ever.
- Your Windows user name is anonymized (`C:\Users\<name>\` becomes `C:\Users\<user>\`). Known limitation: user names shorter than 4 characters are only replaced when they are a whole path component.
- You can add **redaction terms** (for example a client or project name); every occurrence is masked as `<redacted>`.
- **Exact preview**: the app shows the exact JSON payload and a token estimate before anything is sent. Nothing is sent without your click.
- API keys are stored in the **Windows Credential Manager**, never in files, the database or logs. The key is optional (Ollama, LM Studio).

## Supported AI providers

- **OpenAI-compatible** (any base URL): OpenAI, Ollama, LM Studio, OpenRouter, Azure OpenAI and others that expose `/chat/completions`.
- **Anthropic** (native Messages API).

## Install

Download the latest build from [Releases](https://github.com/engviniciusbs/nodegarbage/releases):

- `nodegarbage.exe`: portable executable.
- `nodegarbage-amd64-installer.exe`: NSIS installer (bootstraps WebView2 if needed).
- `SHA256SUMS`: checksums to verify the downloads.

The binaries are **not code-signed yet**, so Windows SmartScreen will warn "Windows protected your PC". Click "More info", then "Run anyway", after checking the SHA256 against `SHA256SUMS`. Requires Windows 10 or 11 with the WebView2 runtime.

## Build from source

Requirements: Go 1.27, Node.js (LTS), and the [Wails v2 CLI](https://wails.io/docs/gettingstarted/installation).

```powershell
go install github.com/wailsapp/wails/v2/cmd/wails@latest
npm ci --prefix frontend
wails build
```

The executable is written to `build/bin/`. Use `wails build -nsis` for the installer (requires [NSIS](https://nsis.sourceforge.io/)) and `wails dev` for live development.

## Running tests

```powershell
wails generate module            # generates frontend/wailsjs, needed by svelte-check
go vet ./...
go test ./...
go test -race ./...              # needs CGO and gcc
npm --prefix frontend run check
npm --prefix frontend test -- --run
```

Set `CI_NO_RECYCLE=1` to skip the tests that touch the real Recycle Bin.

## Known limitations

- The directory-walk engine **counts hardlinks twice** (notably in `WinSxS`), so some system folders look larger than they are.
- There is **no MFT engine yet** (planned for v0.2); scanning is a directory walk.
- Windows 10 visuals (solid background instead of Mica) are **untested**.
- The Recycle Bin is **unavailable on removable and network drives**.
- **Cloud placeholders** (OneDrive and similar, not downloaded) count as 0 bytes and are never read or downloaded.
- Interface text is in Brazilian Portuguese only (no i18n yet).

## Roadmap

MFT scan engine (v0.2); dark theme; scan history comparison; single-file search; running external commands (for example `docker system prune`); code signing (SignPath Foundation) and winget; internationalization; more native AI providers; other operating systems.

## Contributing and security

See [CONTRIBUTING.md](CONTRIBUTING.md), [SECURITY.md](SECURITY.md) and the [Code of Conduct](CODE_OF_CONDUCT.md).

## License

[MIT](LICENSE)
