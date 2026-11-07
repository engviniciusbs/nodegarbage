## What and why

<!-- Summary of the change and the motivation. Link issues with "Fixes #123". -->

## Checklist

- [ ] `go vet ./...`, `go test ./...` and `go test -race ./...` pass
- [ ] `npm --prefix frontend run check` and `npm --prefix frontend test -- --run` pass
- [ ] New or changed behavior has tests
- [ ] New rule: added a table case in `internal/rules/builtin_test.go` and cited a source
- [ ] No rule or code path can unprotect a protected path or delete without the user's confirmation
- [ ] UI text is in `frontend/src/lib/strings.ts` and uses CSS tokens (no raw hex)
- [ ] No secrets, personal paths or telemetry added
