# AGENTS.md

## Read before changing anything

1. `CONTRIBUTING.md`: code rules, scope rules, self-review checklist.
2. `docs/roadmap.md`: only work from the current phase is in scope.
3. `docs/decisoes.md`: ADRs; changing a decision means updating its ADR in the same change.
4. The design doc for the area you touch (below).

## Key conventions

- Design docs are in Portuguese; code, comments, commits and all UI text are in English.
- User-facing docs (`README.md`, `CONTRIBUTING.md`) have a `.pt-br.md` twin; update both.
- A new `go.mod` dependency needs a justification in an ADR.
- Keep `docs/codemap.md` current when adding a file or moving a responsibility.
- A change users notice adds its entry under `## Unreleased` in `CHANGELOG.md` in the same commit, following CONTRIBUTING.md §5: Keep a Changelog groups, written for users, keys as `prefix` + `x` / `shortcut` + `x`.
- Tests that start processes use `sh -c`/`printf`, never the user's shell config.
- App tests replace `App.launch` with fake views instead of starting real programs.
- `third_party/` holds patched copies of two Charm libraries; change them only as `third_party/README.md` says.

## Commands

| Task | Command |
|---|---|
| One package's tests | `go test -race ./internal/<pkg>/` |
| One test | `go test -race ./internal/app/ -run TestName` |
| End-to-end (builds the binary, own daemon per test) | `go test ./cmd/noct/` |
| Build / run | `make build` / `make dev` |
| Done gate: gofmt + vet + race tests | `make check` |

## External references

| Need | File |
|---|---|
| Where each file's code lives | `docs/codemap.md` |
| Problem, principles, non-goals | `docs/visao.md` |
| Daemon, packages, storage, errors, tests | `docs/arquitetura.md` |
| Views, tabs, panes, keys, look and feel | `docs/tui.md` |
| Scripts: format, panel, CLI (phase 1) | `docs/scripts.md` |
| Flows, editor, calendar (phase 2) | `docs/fluxos.md` |
| Writing a flow or script YAML by hand | `docs/writing-flows.md` |
| MCP and AI policy (phase 3) | `docs/mcp.md` |
| Package installation (phase 4) | `docs/catalogo.md` |
| Library bugs and workarounds | ADR-011 in `docs/decisoes.md` |
| Images from pane programs | ADR-014 in `docs/decisoes.md` |
