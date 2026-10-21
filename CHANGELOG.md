# Changelog

What changes in each release of noct, for the people who use it. The section of a version is
the text of its release on GitHub and what noct shows in its updates popup (`prefix` + `c`).

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and versions follow
[semantic versioning](https://semver.org): `vMAJOR.MINOR.PATCH`. Keys are written as `prefix` +
a key and `shortcut` + a key, because both are set in the config: the prefix is `Ctrl+Space` and
the shortcut is `Alt` by default. How to write an entry is in
[CONTRIBUTING.md](CONTRIBUTING.md#5-releases).

## Unreleased

### Added

- Animations. Panels and tabs fade in and out, the panel that gets the focus lights up, popups
  appear and go with a jitter, and a panel out of focus pulses when it prints. Each moment, its
  effect (`01`, `fade`, `jitter` or `none`) and its time in milliseconds are in
  **Appearance → animations**, with a preview beside the list. Moving the focus quickly leaves a
  trail of the panels it passed; any other animation ends at the next key, and programs in panels
  are never resized during one.
- Tab templates. `prefix` + `R` saves the tab's layout, names and colors as a template, and for
  each terminal whether its command runs when the template opens, asks first, or does not run.
  `prefix` + `T` opens a template as a new tab, picked from a list that shows its layout. With
  the direct shortcut on `alt`, `Alt+Shift+R` and `Alt+Shift+T` do the same. A template that
  cannot be read stays in the list with its problem, to edit or delete.
- `prefix` + `P` gives the focused panel and its tab colors of their own: the panel's borders,
  titles, background and text, and the tab's name and background. They stay with the session,
  and a warning tells when text gets hard to read. With the direct shortcut on `alt`,
  `Alt+Shift+P` does the same.
- **Appearance → new theme** and **edit theme** make and change themes in noct. Pick each part
  (tab names, borders, titles, lists, messages) and its color from the 256 the terminal shows,
  or type a `#rrggbb`, with a small workspace beside the list showing the result. On the
  backgrounds of tabs, lists and popups, `t` makes them follow the theme's background again.
- Each session keeps its own theme. **Appearance → theme** changes the session on screen, a new
  session starts with the theme of the one it was made from, and moving through the sessions
  (`prefix` + `o`) or the search shows each session in its colors before you open it.
- Your own themes: a theme file in `~/.config/noct/themes/` shows up in **Appearance → theme**,
  and changes to it show at once.

### Changed

- A finished run shows `✓` in the theme's success color and `✗` in its error color.

- New terminals (`prefix` + `p`, `prefix` + `t`, `prefix` + `g`) open in the folder of the focused panel: where
  its shell is now, or where its script ran. Beside panels without a folder, such as scripts or
  logs, they open in your home folder.

### Fixed

- Making the terminal narrower no longer loses the end of the lines in a pane: widening it again
  shows the text that did not fit, such as logs and values still to read, in the history too.
- A line with an emoji such as `➡️` keeps the pane's right border in place. In terminals that
  draw it one column wide, the rest of that line used to move one column left.

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
