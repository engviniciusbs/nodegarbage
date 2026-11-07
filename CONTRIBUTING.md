# Contributing to NodeGarbage

Thanks for helping! Portuguese speakers: the [Portuguese README](README.pt-BR.md) describes the project; issues and PRs may be written in English or Portuguese.

## Setup

Requirements: Windows, Go 1.27, Node.js (LTS), the [Wails v2 CLI](https://wails.io/docs/gettingstarted/installation) (a C compiler is only needed for `go test -race`).

```powershell
git clone https://github.com/engviniciusbs/nodegarbage
cd nodegarbage
go install github.com/wailsapp/wails/v2/cmd/wails@v2.17.1
wails generate module      # generates frontend/wailsjs (git-ignored); run before the frontend checks
npm ci --prefix frontend
wails dev                  # live development
```

## Tests and checks

Run these before opening a PR (CI runs the same on `windows-latest`):

```powershell
wails generate module
npm ci --prefix frontend
npm --prefix frontend run check
npm --prefix frontend test -- --run
go vet ./...
staticcheck ./...          # honnef.co/go/tools/cmd/staticcheck@v0.8.2
go test ./...
go test -race ./...
wails build -clean
```

Set `CI_NO_RECYCLE=1` to skip the tests that use the real Recycle Bin. Tests that delete files must always work inside `t.TempDir()`.

## Principles

- **Only the user deletes.** The only code path that may delete files is `cleaner.Planner.Redeem` followed by `cleaner.Execute`, triggered by `App.ExecuteClean(planID, token)`. Rules and AI output are advice, and AI output is untrusted data.
- Windows only; files using Win32 APIs carry `//go:build windows`. Long operations take a `context.Context`.
- UI text is Brazilian Portuguese and lives in `frontend/src/lib/strings.ts`. Components use CSS tokens, never raw hex colors, stay under 200 lines, and every screen has empty, loading and error states.
- Never log or store API keys outside the Windows Credential Manager.

## Commit convention

[Conventional Commits](https://www.conventionalcommits.org/): `feat(scope): ...`, `fix(scope): ...`, `docs: ...`, `test: ...`, `chore: ...`. Keep commits focused.

## How to add a rule

Built-in rules live in `internal/rules/builtin/*.yaml` (one file per ecosystem: `node.yaml`, `python.yaml`, ...). Users can also drop YAML files in `%LOCALAPPDATA%\NodeGarbage\rules.d\`. Each file is a list of rules:

```yaml
- id: node.node_modules          # unique, required
  name: node_modules             # shown in the UI
  category: build-artifact       # dev-cache | build-artifact | pkg-cache | temp | browser-cache | system | user-data
  match:                         # exactly ONE of path, dirname, glob
    dirname: node_modules
    sibling: package.json        # optional, only with dirname: a name (simple glob) that must exist in the parent dir
  verdict: safe                  # safe | caution | keep
  regenerable: npm install       # how to get the data back
  action: delete_dir             # delete_dir | delete_contents | manual
  explain: >-                    # required for built-in rules; friendly text for developers (existing rules are in Portuguese)
    ...
```

Schema fields (see `internal/rules/types.go`):

- `match.path`: absolute path, may use environment variables such as `%LOCALAPPDATA%`.
- `match.dirname` (+ optional `match.sibling`): any directory with that name. Use `sibling` to avoid false positives (for example `node_modules` next to `package.json`).
- `match.glob`: a glob pattern; `match.older_than_days` is only valid with `glob`.
- `action: manual` requires `command` (for example `pnpm store prune`); the app shows the command instead of deleting.

Steps:

1. Add the rule to the matching `internal/rules/builtin/*.yaml` file (or create a new file for a new ecosystem).
2. **Add a table case to `TestBuiltinTable` in `internal/rules/builtin_test.go`.** This is mandatory: the test fails if a built-in rule has no case.
3. Run `go test ./internal/rules/...`.
4. Open a PR using the template; fill in the source that confirms the path is a cache (official docs or similar).

**YAML can never unprotect a protected path.** Protection is evaluated in Go (`internal/rules/protect.go`) before and independently of rules; a rule that matches a protected path is classified `keep` (`TestBuiltinNeverUnprotects` enforces this). Do not try to work around it. When in doubt, choose `caution` over `safe`.

## Reporting bugs and security issues

Use the issue forms for bugs and new rules. For vulnerabilities, including anything that could delete the wrong thing, follow [SECURITY.md](SECURITY.md) and do not open a public issue.

By contributing you agree that your contributions are licensed under the [MIT License](LICENSE) and that you follow the [Code of Conduct](CODE_OF_CONDUCT.md).
