# Changelog

What changes in each release of noct, for the people who use it. The section of a version is
the text of its release on GitHub and what noct shows in its updates popup (`prefix` + `c`).

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and versions follow
[semantic versioning](https://semver.org): `vMAJOR.MINOR.PATCH`. Keys are written as `prefix` +
a key and `shortcut` + a key, because both are set in the config: the prefix is `Ctrl+Space` and
the shortcut is `Alt` by default. How to write an entry is in
[CONTRIBUTING.md](CONTRIBUTING.md#5-releases).

## Unreleased

### Changed

- New terminals (`prefix` + `p`, `prefix` + `t`, `prefix` + `g`) open in the folder of the focused panel: where
  its shell is now, or where its script ran. Beside panels without a folder, such as scripts or
  logs, they open in your home folder.

## v0.1.2 - 2026-10-19

### Changed

- The text rows of the config (the scripts location and a custom editor or file manager command)
  open for typing only on `Enter`, so `j` and `k` move past them instead of typing into them.
  `Enter` again saves, and `Esc` puts back the text as it was.

### Fixed

- Closing a pane of a stack, with `shortcut` + `x` or when its program ends, keeps the focus in
  the stack, on the pane it shows next, instead of moving it to a pane in another column.
- Moving through the logs (`prefix` + `v`) no longer closes noct. A log of a full-screen program
  run in a taller pane made the preview write past its screen, and the whole workspace stopped.
- When noct stops unexpectedly, the terminal it leaves says why and where `daemon.log` is.
- The log preview (`prefix` + `v`) keeps what was printed before a `clear` in the log, so you
  can scroll up to it instead of finding only the last prompt.

## v0.1.1 - 2026-10-16

### Changed

- The updates popup remembers the last check: the changelog shows right away, even after noct
  restarts, with when it was checked. noct asks GitHub again only once a day, or when you press
  `c`.
- Checks ask GitHub only whether something changed, which does not count against its limit, and
  a limit reached is still respected after a restart.

## v0.1.0 - 2026-10-16

The first beta of noct: a terminal workspace for people who test. Expect changes before v1.0.0.

### Added

- Real terminals in unlimited tabs and panes, tiled like i3, with your own shell and config.
- Sessions that keep running when you detach, and open again as they were after a quit.
- Scripts: saved command lines with parameters, notes and the program's version.
- Flows: scripts chained in a visual editor with conditions, loops, waits, variables and prompts,
  started by hand, by cron, by file changes or by another flow. Every run is kept, with a
  calendar.
- AI through MCP: an agent reads your scripts, drafts flows and runs them, with your approval.
- Keys: every key can be changed, and `shortcut` + a mode key works without the prefix.
- Seven themes and mouse support.
- Updates: the tab bar shows noct's version and tells when a new one is out. `prefix` + `c` shows
  the changelog and installs the new version once you ask, and `noct update` does the same from
  the shell.

### Changed

- The config moved from `prefix` + `c` to `prefix` + `,`.
