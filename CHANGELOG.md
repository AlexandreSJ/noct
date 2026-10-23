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

- Eleven new themes in **Appearance → theme**: `tide`, `moss`, `pumpkin`, `neon`, `cobalt`,
  `sand`, `slate`, `amber`, `phosphor`, `paper` and `sakura`. Some give popups and inactive
  tabs a background of their own, a little lighter than the theme's.
- **Appearance → mode** switches every theme noct comes with between its dark and its light
  colors, so `blood` or any other works in a light terminal too. Your own themes keep their
  colors in both modes.

### Changed

- Scrolling a terminal's history moved to `prefix` + `g` (`shortcut` + `g`), and the floating
  panel to `prefix` + `u` (`shortcut` + `u`). Keys you changed in the binds stay as you set them.
- While scrolling, vim's keys work as well as the arrows: `ctrl+u`/`ctrl+d` half a page,
  `ctrl+y`/`ctrl+e` a line, `ctrl+b`/`ctrl+f` a page. Any other key goes back to live, and so
  does `shortcut` + `g` again.
- `⛶ fullscreen` and the scroll position (`↑ 0/18`) sit at the right of the panel's border, and
  a long panel name is cut with `…` to make room for them. The logs' preview does the same.

- The color picker of **Appearance → edit theme** and `prefix` + `P` groups the 256 colors:
  the terminal's 16, the color cube as a triangle with red, green and blue at its corners, and
  the grays. `,` and `.` make the triangle darker or lighter, keeping the color's mix of red,
  green and blue, and the selected color stays visible under its mark. In a terminal at
  least 25 lines tall the whole picker fits in its panel, without scrolling. Thanks to
  [Ryan](https://github.com/aishiteirai) for the triangle idea.

### Fixed

- A log in `prefix` + `v` that ended inside a full-screen program (a TUI, `nano`, `less` left
  open) could not be scrolled at all. It now scrolls up past the program's last screen to what
  the shell printed before it, and the title says `▣ ends in a full-screen program`.

## v0.2.0 - 2026-10-21

### Added

- Your own themes: **Appearance → new theme** and **edit theme** pick a color for each part
  (tabs, borders, titles, lists, messages) from the terminal's 256 or a `#rrggbb`, with a
  preview. Theme files in `~/.config/noct/themes/` work too, and reload when they change.
- Each session keeps its own theme, and moving through the sessions (`prefix` + `o`) or the
  search shows each one in its colors.
- `prefix` + `P` gives the focused panel and its tab their own colors, with a warning when text
  gets hard to read. They stay with the session.
- Tab templates: `prefix` + `R` saves the tab (layout, names, colors and each terminal's command,
  set to `run`, `ask` or `none`), and `prefix` + `T` opens one as a new tab.
- `shortcut` + `P`, `shortcut` + `R` and `shortcut` + `T` do the same without the prefix, when
  **Keys → alt** is on.
- Animations: an effect (`01`, `fade`, `jitter` or `none`) and a time for each moment, with a
  preview, in **Appearance → animations**. Moving the focus quickly leaves a trail; other
  animations end at the next key.

### Changed

- New terminals (`prefix` + `p`, `prefix` + `t`, `prefix` + `g`) open in the focused panel's
  folder, or in your home folder beside panels without one.
- A finished run shows `✓` and `✗` in the theme's success and error colors.
- A config note too long for the panel scrolls while its setting is selected.
- `shortcut` + `,` closes the config too.

### Fixed

- Narrowing the terminal no longer loses the end of lines: widening it shows them again, in the
  history too.
- An emoji such as `➡️` no longer moves the pane's right border.
- The cursor no longer jumps around while noct redraws, in terminals that do not draw frames at
  once.

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
