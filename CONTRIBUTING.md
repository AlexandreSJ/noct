# Contributing to noct

[Português](CONTRIBUTING.pt-br.md)

These rules apply to everyone who changes this repository, human or AI agent. Read them before writing code.

## 1. Code rules

### Clarity first

- Code must be easy to read by someone who has never seen it. If a reviewer needs you to explain it, rewrite it.
- Names carry the context. A reader should understand what a function, type or variable is for without opening its body: `focusedPane`, `splitDirectionFor`, `ptyOutputBuffer`, not `fp`, `dir`, `buf2`.
- Prefer boring, direct code over clever code. No reflection, generics or channels where a plain function call does the job.
- One function, one goal. If a function name needs "and", split it.

### Complexity limits

- Keep cognitive complexity per function at or below 15 (SonarQube's default). Deep nesting, long `if/else` chains and loops inside loops are the usual causes; use early returns and small helpers.
- A function longer than ~40 lines is a signal to split, not a hard rule.
- Files should have one clear topic. A file that keeps growing is doing too much.

### Comments

- The best code needs no comments: simple functions and types have none.
- When a comment is needed, write a short single-line comment about the **goal** or the **reason**, not about what each line does.
  - Good: `// Redraw at most once per frame so heavy output cannot starve input.`
  - Bad: `// Loop over the panes` above a loop over panes.
- Functions that orchestrate a lot (many calls, branches and loops, close to a "brain function") may get one or two short comments explaining the goal and the non-obvious decisions.
- Every package has a one-line package comment stating its purpose; it is how agents and people find their way around the code.
- No commented-out code, no `TODO` without an issue or a roadmap phase next to it.

### Go practices

- `gofmt` and `go vet` clean. `go test -race ./...` passes.
- Wrap errors with context: `fmt.Errorf("starting shell: %w", err)`. Never ignore an error silently; if it is safe to ignore, say why in one line.
- No `panic` outside `main` for recoverable situations.
- Every goroutine has an owner and a clear way to stop. No goroutine leaks.
- No package-level mutable state.
- Interfaces are defined where they are consumed, and only when there are two implementations or a real test seam (for example, around the PTY).
- Use the standard library first.

### Tests

- Test behavior, not internals. Table-driven tests for pure logic.
- Every bug fix comes with a test that fails without the fix.
- Tests that start processes use short, deterministic commands (`printf`, `sh -c`), never the user's shell configuration.

### Review your own work

After implementing, before calling it done, re-read the whole change and check:

1. Does it do exactly what was asked, nothing more?
2. Are there logic holes: empty inputs, zero sizes, closed processes, concurrent access, errors on every path?
3. Would a new contributor understand every name without explanation?
4. Is any comment explaining *what* instead of *why*? Remove it or rename things instead.
5. Did it add a dependency, option or abstraction that is not needed yet? Remove it.
6. Do `gofmt`, `go vet` and `go test -race ./...` pass?
7. If users notice the change, does it have its entry under `## Unreleased` in [CHANGELOG.md](CHANGELOG.md) (see [Releases](#5-releases))?

## 2. Scope rules

noct stays small and focused by following these rules:

- **Every change maps to the roadmap.** Work belongs to the current phase in [docs/roadmap.md](docs/roadmap.md). Nothing from a later phase lands early, even if it is "just a small thing".
- **Delegate, don't reimplement.** noct orchestrates the user's shell and programs. It does not reimplement shell features, completions, package managers or editors.
- **No speculative code.** No abstractions, options or extension points for needs that do not exist yet. Add them when the second use case shows up.
- **Every dependency is justified.** A new module in `go.mod` needs a reason in the pull request: what it does that the standard library or an existing dependency cannot.
- **No configuration without a user need.** Defaults should be good enough; each option is a maintenance cost.
- **Files are the source of truth.** Scripts, flows and config are human-readable files. The TUI, the CLI and the AI all read and write the same format.
- **Keyboard first.** Every feature is fully usable from the keyboard.
- **Delete dead code.** Unused code is removed, not kept "just in case".
- **Decisions are recorded.** A change that alters a design decision updates [docs/decisoes.md](docs/decisoes.md) in the same change.

## 3. Language

- Code, identifiers, comments, commit messages and user-facing docs (README, this file) are in English, with a pt-BR translation for user-facing docs.
- Design docs in [docs/](docs/) are currently written in Portuguese.

## 4. Commits

- One logical change per commit, with a short imperative subject (`Add pane resize`, `Fix focus after closing a pane`).
- Keep the build and the tests green on every commit.

## 5. Releases

- Versions follow semantic versioning: `vMAJOR.MINOR.PATCH`.
- [CHANGELOG.md](CHANGELOG.md) follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/). Its sections are the text of the GitHub releases and of noct's updates popup, so they are written for the people who use noct.
- A change that users notice adds its entry under `## Unreleased` in the same commit as the change. Changes they do not notice (refactors, tests, CI, design docs) get no entry.
- Entries go under `### Added`, `### Changed`, `### Fixed`, `### Removed` or `### Security`, in that order, and only the groups that have entries appear. A fix of something no release has shipped yet is not a `Fixed` entry: edit the entry it fixes instead.
- Each entry is one bullet, in English like the UI, in the present tense, saying what the user sees or can do now, and why when that is not obvious. Name keys and settings as noct shows them: `prefix` + `c` and `shortcut` + `c`, never `Ctrl+Space` or `Alt`, which the user may have changed, and settings by their config path (**Updates → check for new versions**). No file names, function names or ADR numbers. A change that breaks a habit, such as a key that moved, says what to do now.
- To release, rename `## Unreleased` to the version and the date (`## v0.2.0 - 2026-11-03`), add an empty `## Unreleased` above it, commit, then tag and push: `git tag v0.2.0 && git push origin v0.2.0`. The release workflow runs the checks, builds the archives (`make dist`) and publishes the release with that section as its text; `make dist VERSION=v0.2.0` and `make release-notes VERSION=v0.2.0` show locally what it will publish.
