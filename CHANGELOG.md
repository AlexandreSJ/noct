# Changelog

What changes in each release of noct. The section of a version is the text of its release on
GitHub, and what noct shows in its updates popup (`prefix` + `c`), so write it for the people who
use noct. Versions follow [semantic versioning](https://semver.org): `vMAJOR.MINOR.PATCH`.

## v0.1.0

The first beta of noct: a terminal workspace for people who test. Expect changes before v1.0.0.

- Real terminals in unlimited tabs and panes, tiled like i3, with your own shell and config.
- Sessions that keep running when you detach, and open again as they were after a quit.
- Scripts: saved command lines with parameters, notes and the program's version.
- Flows: scripts chained in a visual editor with conditions, loops, waits, variables and prompts,
  started by hand or by cron, file changes or another flow; every run is kept, with a calendar.
- AI through MCP: an agent reads your scripts, drafts flows and runs them, with your approval.
- Every key can be changed, with direct shortcuts (`Alt` + a noct mode key), seven themes and
  the mouse.
- Updates: noct shows its version in the tab bar and tells when a new one is out; `prefix` + `c`
  shows the changelog and installs it once you ask, and `noct update` does it from the shell.
- The config moves from `prefix` + `c` to `prefix` + `,` (`Alt+,`).
