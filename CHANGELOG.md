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

- `Space` on **Flows → tell when runs end** in the config sends a test notification, the way a
  run's end is told.
- `g` or `Home` in a flow's chart in the inspect selects its start, and `G` or `End` its last
  step, as in a log.
- `a` in a flow's chart picks the kind of step in a list centered over the chart, in the panel,
  so noct's prefix and the other panes stay a key away: typing narrows the list, the arrows and
  the wheel move through it, `Enter` or a click adds, and `Tab` adds again the step added last,
  down to its script or function. A script or a function is picked in the same list.
- A flow's `set` gives its variables types with `types: {count: number}`: `var`, the default,
  is what its value is, a list or an object when it is one in JSON; `text`, `number`, `boolean`,
  `list` and `object` refuse a value of another type. Its form has a type column, starts typing
  the first variable's name, and `Tab` completes the name of a variable in scope, which brings
  its type.
- What `Tab` would put in shows dim after the cursor in the inspect's forms and tables.
- **Flows → when leaving a step's form** in the config says what `esc` does with what changed in
  a step's form: `ask` first (the default), `save`, or `discard`. A save that is refused keeps the
  form with why, and `esc` again leaves the step as it was.
- Dragging over the text of a popup selects it, and letting go copies it, as in a pane.
- Clickable rows light under the mouse with the hover animation in popups too: the config, the
  commands, the agents, choices and the search, which a click now picks from, and the cards of
  a flow's chart.
- A flow variable's type is `var` unless it says another, and new ones start as `var`: its value
  is what it is, and JSON of a list or an object is read inside, as `cfg.port`.
- `g` and `G` go to the first and last setting in the config, as `Home` and `End` do.
- `Space` opens an option's choices in the inspect's forms and tables, as `Enter` does.
- A step form's text can take several lines: while typing, `Enter` starts a new line, and `esc`,
  `Ctrl+Enter` or `Alt+Enter` leave the cell. A condition shows laid out by its logic in the
  form too, its groups in parentheses on lines of their own.
- A set's or a variable's JSON closes its braces, brackets and quotes as it is typed, `Enter`
  adds the comma, and it shows a member a line, indented, in the colors of conditions. A list
  or an object that reads itself is refused.
- An object's fields complete after its name: `obj` shows `.isOnline` dim, and `Tab` puts it in.
- A flow's `prompt` reads the escapes a print does, as `\n` and `\t`; its form shows them in
  the primary color, dimmed.
- Letters work on the rows of the inspect's tables and of row forms, as the set's, while nothing
  is typed: `Shift+↑↓` mark rows and `Ctrl`+click marks one, `esc` lets the marks go; `d`
  doubles, `c` copies, `x` cuts and `v` pastes the marked rows, or the focused one; `Delete`
  removes them; `z` and `y` undo and redo.
- The help beside an inspect editor hides with `i` (it was `v`, which now pastes rows), and
  `Shift+↑↓` mark rows instead of moving them; `J` and `K` still move a row.
- `v` in a flow's chart in the inspect hides the details under it, giving their rows to the
  chart, and shows them again.
- Flow conditions take `nand`, `nor` and `xor`. The inspect's details show a condition with
  them, or with `and` and `or`, a part a line indented by its level, as `[a and b] or [c]`: each
  level in a shade of the primary color, the operators in the color itself.
- A flow's `prompt` can be `secret: true`: each character typed shows as `•`, the answer stays
  out of the run's result, and the run shows it as `••••`, as in a run line that reads it. The
  prompt's form has a `secret` toggle.
- A flow's `if`, `until`, `switch` and `each` are expressions, with names alone:
  `if: login == expectedLogin and not ok`. They have `== != < <= > >=`, `and`/`&&`,
  `or`/`||`, `not`/`!`, `+ - * / %`, `in`, `??`, lambdas such as `h => startsWith(h, "web")`
  and functions: `len`, `lower`, `join`, `lines`, `match`, `map`, `filter`, `any`, `fromJSON` and
  more, which the `if`'s help lists. A text that reads as a number compares as one, so
  `exit == 0` works. A repeat's `each` goes through a list's items or a text's lines.
- Every other text of a flow (a run line, `with` and `set` values, a prompt, a question)
  reads a value as `$name`: `$version`, `$build.output` for a step's field, `$cfg.db.port` for an
  object's, `$hosts[0]`, a function's call such as `$join(hosts, ", ")`, or any expression in
  braces, `${count + 1}`. Only names the flow knows are its own, so `$HOME`, `${HOME}` and `$1`
  stay the shell's, and so do `$$name` and a name the run line sets itself. In a run line each
  value is escaped for where it lands: one shell word outside quotes, kept inside `"..."`,
  `'...'`, `$(( ))` or a heredoc, so a value never runs as a command. Typing a name in a step
  form completes the variables and the functions.
- Flow variables can be a `list` or an `object`: JSON that holds lists and objects at any depth.
  Steps read inside them, as `hosts[0]` and `cfg.db.port`, and `$hosts` alone writes the JSON
  again.
- A scope warning tells when two steps give the same capture before a step reads it by its short
  name: `version` reads the last one, and `build.version` picks another.
- The demo installs `demo-params`, a flow with one variable of each type that its steps read.
- A flow variable's `default` can read the variables written before it, from the environment
  too: `default: '{"user": "$user"}'` makes an object of them, each `$name` quoted and escaped as
  JSON. Only the default renders: a value typed in the form is taken as it is.
- Scripts, flows and functions can be kept in folders inside folders, at any depth. A flow in
  `flows/ops/deploy.yaml` is `ops/deploy` (`noct flow run ops/deploy`, or `deploy` while no
  other flow has that name), and `call: utils/greet` runs a function in a folder. The inspect's
  list shows flows in their folders like the scripts panel does, with the functions under
  `ƒ functions`; `a` in a folder starts the new flow's name with its path, and a `/` makes
  folders.
- In the inspect's list, `N` makes a folder, `m` moves or renames a flow, a function or a folder
  (a `/` puts it in folders, none takes it out), `c` copies a flow or function, `delete` removes
  a folder, and dragging a flow onto a folder moves it there. Remembered values, the last result
  and triggers go with a moved flow, and the calls of a moved function follow it.
- The keys popup (`?`) and other text popups filter their lines with `f` or `/`, scroll with the
  mouse wheel and tell which lines show when the text is taller than the popup.
- The demo installs its flows and functions in a folder `demo`, as its scripts, and moves the
  ones an older noct installed outside it there; they still run by their names alone, as
  `noct flow run demo-deploy`.
- The inspect opens on the flow selected last, and a saved session remembers the flow of each
  inspect and the script of each scripts panel. One that is gone leaves the panel on the first
  of its list.
- Dragging the mouse over a cell's text in the inspect's params table and step forms selects it
  and copies it; `Ctrl+C` while typing copies the selection or the whole cell.

### Changed

- A flow's chart starts at `‹ start ›` and ends at `‹ end ›`, centered on its line, in place of
  `● start` and `◉ end`.
- The hints under a flow's chart show only the main keys; `?` lists all of them.
- A flow's `prompt` is shown as written, without a `:` after it (`login:` no longer shows
  `login::`); what an empty answer keeps shows beside the step's name.
- A flow's `params:` are now `variables:`, and the inspect calls them variables. A flow or
  function with `params:` does not load until the key is renamed.
- `{{ }}` is no longer a template in a flow: conditions are expressions and texts read `$name`.
  A condition with `{{ }}`, or a `{{ }}` in a text that reads one of the flow's names, is
  refused with the new way to write it; any other `{{ }}` is text, as
  `docker ps --format '{{.Names}}'`. Scripts keep `{{quote .name}}` in their `run`.
- `echo key=$key` in a flow's run line reads the flow's `key`: only `name=` where a command
  starts, or after `export` or `local`, makes a name the shell's.
- A flow's run form no longer remembers a value that is the param's default, so a default changed
  in the flow is the one its next run takes.
- The updates popup (`prefix` + `c`) draws each release's changelog as formatted markdown in
  the theme's colors: headings, `•` lists wrapped under their text, `code`, **bold**, *italic*,
  quotes and links.
- **Default agent** in the config (`prefix` + `,`) opens a list on `Enter` instead of stepping
  through the agents with the arrows. It shows only the agents your machine has, each with its
  model; `Enter` makes one the default, `m` picks its model from the ones it lists (claude's
  models, the ones ollama has, opencode's and llm's) or one you type, `e` changes its command and
  `Del` removes an agent you added. The model is saved under `[agent_models]` and noct writes the
  command around it, so its flags stay current with noct. The row shows the agent and its model,
  and the popup the command it runs, ending in `< {prompt}` when the question goes as its input.
- The agent step's form in the inspect and `noct agents` leave out the agents your machine lacks.
- While you type in a list's filter, the footer keeps the list's keys; the filter's line shows
  `enter`, which keeps it, and every list counts the same way: `3 of 12`, or `no match`.
- Shorter notes and hints in the config, the binds' `?` list and the agent step's form.
- `Enter` on a flow's variables card in the inspect (`prefix` + `i`) opens its variables as a
  table in place of the chart, a variable a row like declarations in code: type, name, value, environment
  variable, label, required and secret in columns. `hjkl` or the arrows move from cell to cell
  (with `Ctrl`, to the first or last), and `Enter` or a click edits one: it types in a text, or
  opens the choices of a type or a boolean right under the cell. An option's value is the list
  of its options, shown with the one it holds and how many there are, as `"dev" · 3 options`:
  add one, mark the one it holds with `Space`, filter with `f`. A list's value is a list of its
  items too. `+ add variable`, the last row, adds a variable;
  `J`/`K` move one, `Ctrl+Delete` removes it and `Ctrl+Z`/`Ctrl+Y` undo and redo. The table is
  saved as it changes; a cell that keeps its row from being saved turns red and says why. What a
  click would work lights under the mouse, as a list's rows do. `v`
  hides the help beside it, and `esc` closes it.
- Variables keep the order they are written in, in the run form and the variables card too.
- Adding a step (`a`) or changing one (`e`) in the inspect fills one form in place of the chart
  instead of a chain of prompts. The variables the step can read are listed beside it, with
  their last values and examples written with them, and typing a name completes them.
  Changing a step keeps what its form does not ask, such as its captures and the steps inside.
  `Enter` or a click edits a row, and `Ctrl+S` or the last row adds or saves the step. A
  switch's cases, an agent's answers and the names an unset clears are lists, an item a row.
  A block's exports (`x`), a step's retry (`R`) and a function's returns open the same way.
- Neither editor locks noct: the prefix still reaches every panel and tab while you edit, and
  the editor is there when you come back. Their hints take one row, and `?` lists their keys,
  which you can change in the config.
- Variable types are named `boolean` and `option`, for what were `bool` and `choice`; flows that
  use the old names still work. Secret and environment variable are settings of any type.
- The inspect names variables by their short names, as `$output` and `$version`;
  `$build.output` shows only for an earlier step's value a later step replaced.

### Removed

- `ctrl+t` in the inspect's prompts and forms, which many terminals take to open a tab: the
  variables are listed beside the step's form now.

### Fixed

- The mouse wheel scrolls the updates popup (`prefix` + `c`).
- A flow's chart in the inspect no longer moves up and down as the selection moves: the details
  under it keep one height for every block, and the hints keep the rows of the longest.
- Going to the next tab and back with the arrows keeps the pane you left, the top one of two
  stacked panes too, instead of taking the bottom one.

## v0.3.1 - 2026-11-15

### Added

- `f` (or `/`) filters long lists as you type: the config (`prefix` + `,`, then `f` and any part
  of a setting, such as `agent` or `logs size`), the keys of every screen, the scripts panel and
  the inspect's list of flows. Only the rows with every word stay, with their sections; the arrows
  move among them, `Enter` keeps the filter and `Esc` clears it. The inspect's list now scrolls
  when the flows do not fit.
- The config's **Flows** section has **default agent**, the agent a step with `use: default` asks:
  `auto`, the agents your machine has, the ones it lacks, and **custom agent**, where `Enter` asks
  for a name and a command and makes it the default. Its note tells whether your machine has the
  agent shown, and its command.
- **Fullscreen hint** under **Appearance** turns off the dashes that, in fullscreen, mark the sides
  with other panes beyond them. It is on by default.
- `?` lists every key of the binds page and of the scripts panel, whose footers now show the main
  ones.

### Fixed

- Hints at the bottom of the config, the binds, the theme editor, the panel colors, MCP, updates,
  templates, the script form and setup wrap onto more lines instead of being cut, at any terminal
  size. Long notes in the config wrap too, instead of scrolling slowly, and a long question, such
  as deleting a function its flows call, widens its popup.
- The updates popup reads each changelog paragraph as one, instead of breaking it where the file
  does, and the logs list's heading wraps in a narrow pane.
- Adding a custom agent showed `prompt` where its hint meant `{prompt}`.
- A drag no longer sticks when the mouse button goes up outside where it started: off the
  inspect's chart, over another pane, or outside the terminal's window. Leaving the window lets go
  of the drag right there, and a card let go off the chart stays where it was.

## v0.3.0 - 2026-11-13

Flows grow up. A step can ask an AI agent you already have (claude, codex, opencode, llm, ollama
or your own) and the flow acts on its answer; `switch`, `repeat each` and `retry` join `if` and
`repeat`; functions are steps any flow can call; and variables are scoped as in a language. The
inspect draws all of it as round cards, tells about the selected one below the chart, and moves
steps with `J`, `K` or the mouse. The demo's new `demo-triage` shows an agent sorting a failing
test. Animations got smoother too: panes, tabs and popups now go away as gently as they appear,
with new effects, and hovering lights tabs, titles and rows.

### Added

- The demo has `demo-triage`: it tells what is about to happen, asks how a made-up test should
  fail (enter picks at random), and an AI agent reads only the test's output and decides whether
  to run it again, explain it through the function `demo-explain`, or ask you for a missing
  setting. It asks ollama, which runs a model on your machine, or the agent its `agent` param
  names (`agent=default` for the config's), and ends with a report of how the test really failed
  and whether the agent got it right. `noct setup` installs it beside the others.
- The MCP server lists functions (with who calls each) and the agents an agent step can ask, reads
  a function with `get_item`, and drafts one with `draft_function`. A draft function does not run,
  and the flows that call it wait for its promotion.
- Agent steps: `agent: {ask: …}` asks an AI agent you have (`claude`, `codex`, `opencode`, `llm`,
  `ollama`, or a command under `[agents]` in the config) and keeps its answer in `{{.answer}}`,
  without the thinking a model prints first; with `choices`, the answer is one of them, so an if
  can branch on it. The question never goes through a shell, no preset can change anything, and
  the `claude` one does not load your MCP servers.
  `use: default`, or no `use`, asks the config's agent; by default (`agent = "auto"`) the first
  agent this machine has. `use:` can come from a param. `noct agents` lists them and, when there
  is none, says how to start with ollama; `noct agents default <name>` sets the config's agent.
  In the inspect, `a` then `a` sets one up in a single form (the question, its answers, the agent
  and a timeout) that `enter` adds from any field. The form always writes `use:`: `default`,
  `auto`, an agent, the ones this machine has first, or `custom agent`, which asks a name and a
  command and adds them under `[agents]`.
- Flows have a `switch`: `switch: '{{.answer}}'` runs the first of its `cases` whose `is` holds the
  value (`is: bug` or `is: [bug, regression]`), or its `else`. The inspect draws a column for each
  case with its values on top; in **add step**, `o` asks for the value and the cases.
- `repeat: {each: '{{.steps.ls.output}}', as: file}` runs its steps once for each line, the line in
  `{{.file}}` (`{{.item}}` without `as`); in **add step**, `e` (for each line).
- `retry: 3`, or `retry: {max: 3, every: 10s}`, runs a failed script, run or agent step again
  before its `on_fail`. In the inspect, `R` sets it, and the card shows `retry 3× every 10s`.
- The inspect tells about the selected block below the chart: what its kind of step does, its
  whole text and how it went in the run shown. Select it with the keys or a click; the wheel
  scrolls it. `?` lists every key of the inspect.
- Functions: steps any flow can call, in a file of their own with params and returns. A `call:` step
  runs one in a scope of its own and gets back what it returns. The inspect lists functions after
  the flows (`F` makes one), shows which flows call each, and tells what a change reaches.
- Template prompts in the inspect complete the variables in scope as `{{.` is typed: up and down
  pick one, tab puts it in.
- Flow params can be `bool`, `choice` (with its `choices`) or `path`, besides `text` and `number`.
  The inspect's params card, above the start, adds one offering every type and asks what the
  type needs, and changes or removes one.
- While a flow runs, the inspect lights the step running, marked `▶`, and three dots of different
  sizes and colors keep going down the arrow out of it toward the next step.
- In the inspect, `J` and `K` move a step through the flow as a line moves through code, into and
  out of ifs and repeats; `m` moves it with the arrows, and dragging a card with the mouse drops
  it below another. `z` undoes the last change to the flow and `y` does it again.
- Animation effects for any moment in **Appearance → animations**: `dissolve` shows or hides
  each cell at once in a random order, `wipe` sweeps across from left to right, `grow` spreads
  out from the center and shrinks back into it, `slide-right` and `slide-left` bring the content
  in from the left or the right edge like a drawer and take it back out, and `shine` sweeps a
  bright band across the text, darker on light themes.
- `prefix` + `D` duplicates the focused terminal: a new one opens beside it in the same folder and,
  when its shell runs a program, offers to run the same command.
- In a log in the logs view, `/` searches up from the end, `n` and `N` go to the next match up and
  down, and `f` shows only the lines with a text. Matches show in reverse, lower case matches any
  case, and `esc` clears the search or filter before going back to the list.
- A script or flow that ends out of sight, in a pane off screen or started by a trigger, is told
  by a system notification, or by the terminal's bell, as **Flows → tell when runs end** sets.
  Over ssh, or without `notify-send` or `osascript`, the notification goes through the terminal
  (OSC 9).
- A line of a script's notes that is only an image, `![diagram](diagram.png)`, shows the image in
  the scripts panel: in full resolution in kitty and Ghostty, and drawn with half blocks in other
  terminals, so any terminal with colors shows it.
- A repository can keep tab templates in `.noct/templates/`. They show in `prefix` + `T` marked
  `project`, and every command they hold is asked before it runs.
- `noct new [session] --template <name>` starts the session with the template's tab.
- While scrolling a terminal (`prefix` + `g`), `f` filters it: the pane shows only the lines
  that match a regular expression, live as the program goes on, until `f` with nothing typed.
- `Ctrl` + click on a pane opens what is under the mouse: a link the program set, a URL in the
  text, even one a long line wrapped, or the path of a file or folder that exists. Over ssh it is
  copied instead.
- `noct export theme`, `noct export config` and `noct export keys` print a theme, the settings
  you changed, or your keys and binds as one `noct import …` line, with a QR code of it on a
  terminal, to run on another machine. `noct import` shows what it changes, each bind's command
  included, and asks before applying it.
- `noct import theme <file>` turns a kitty, Ghostty or base16 theme into a noct theme, with its
  16 colors.
- A theme file can give the terminal's 16 colors in an `[ansi]` table (`color0` to `color15`).
  With **Appearance → background** on `theme`, programs in panes draw with them, so the whole
  workspace follows the theme.
- In a log in the logs view, `t` shows when each line started (`2026-11-11 09-30-00`) before it.
  Logs written before this version show the lines without times.
- Flow params can come from the environment, as script params already could: with
  `from: env:NAME`, the variable's value is used without asking. `secret: true` masks a flow
  param in the form and never remembers it. The demo's `demo-deploy` reads its `token` from
  `DEMO_DEPLOY_TOKEN`.
- A shell's pane title tells the folder it is in, as `zsh · ~/proj`, until a program sets a
  title of its own.
- Script and flow templates have `quote`: `{{quote .pattern}}` puts a value in single quotes as one
  shell word, so spaces, quotes or `;` in a param or in another step's output never run as a
  command.

### Changed

- Flow variables are scoped as in a language: an if or a repeat changes a variable made before it,
  and one it makes goes when it ends unless the block exports it (`export: {count: passes}`, or
  `x` in the inspect). Script and run steps also set `{{.output}}`, `{{.exit}}`, `{{.ok}}` and
  their captures as short variables. A flow that read after an if a variable made inside it now
  gets a warning; export the variable to keep it. An if whose `then` and `else` both make the
  variable exports it without a warning.
- The inspect's flowchart has round cards joined by longer arrows drawn with box lines (`│`,
  `↓`), no longer `▼`. The `+` gaps between cards are gone: `a` adds below the selected step,
  and the head of a list (the start, an if's yes or no, a repeat's way in) adds above its first
  step.
- Each card in the inspect starts its title with a mark of its kind (`▸` script, `$` command, `◆`
  AI agent, `ƒ` function, `◇` if, `↻` repeat, `◷` wait, `?` asks you, `=` set, `∅` unset) and
  names the kind on its right; text that does not fit ends in `…`.
- An empty list in the inspect's flowchart, as an if's `no` with no steps, shows only `∅` on its
  line instead of a `∅ a adds` card. Its head (the start, `yes` or `no`, a repeat's way in) is
  selectable even when the list is empty, and `a` there adds its first step.
- A card moves with the mouse only with `alt` (or `ctrl`) held; dragged without either, the chart
  pans. While it is carried, a dashed outline shows exactly where it lands: before or after any
  card, at the top of an if's branch or a repeat, or in an empty list.
- Hints at the bottom of panels and popups break into more lines instead of being cut; popups
  grow to fit them.
- Typing in a field no longer paints the whole row: the text shows in the selected color and the
  cursor is the cell under it on the selected color's background, so no mark shifts the text
  while the cursor moves.
- A script or flow param with `from: env:NAME` whose variable is not set is asked in the form,
  instead of refusing to run. Typed for a flow, it reaches the run as that variable, never on its
  command line, and it is never remembered.
- Closing a pane, a tab or a popup no longer flashes away on the last frame. With `fade`, its
  text fades out and what is below fades in; with `01` and `jitter`, its cells go away one by one.
  Fades change only text colors, so a terminal's own background stays as it is.
- Hovering a tab's name shows it in the active tab's color and a pane's title in the focused
  pane's, on their own background, also from an empty tab; with `fade`, `dissolve`, `wipe` or
  `grow` the color blends there and back. Only the title's text lights up, never the border around it.
- Rows of the scripts, logs and flows lists light up under the mouse in the selection's color,
  text only, and blend back as it leaves; the selected row stays as it is.
- Config choices show `<` and `>` instead of `◂` and `▸`, and only the arrows, with a cell on
  each side, change a choice when clicked; clicking its text only selects the row. With the mouse
  where a click changes something, that shows in reverse: the arrow, a value without arrows
  whole, or a text row's field.
- The logs view lists a log on every day it had output, not only the day it started and the
  day it was last written to, so a terminal used across several days shows on each of them;
  filtering by a date finds it there too.

### Fixed

- Saving a script, its notes or a draft from an AI no longer leaves a half-written file when noct
  stops in the middle of the write.

## v0.2.4 - 2026-11-08

### Added

- Flows and their `script` and `run` steps take an optional `timeout` (`30s`, `10m`, `1h30m`). A
  step that runs longer is stopped and fails, then follows its `on_fail`; a flow that runs longer
  stops as failed. A hung step no longer keeps a scheduled flow from starting again until noct
  restarts.
- `noct check` and the AI's `check` warn about a flow started by a schedule, file changes or
  another flow that has no timeout.

### Changed

- `noct run <script>` is listed in `noct -h` and the README as the way to run a script; `-s` and
  `--script` still work.

### Fixed

- Runs, remembered parameter values and enabled triggers are no longer lost or damaged when the
  workspace and the `noct` command save them at the same time.
- `noct mcp` exits as soon as the AI client closes, even while `get_run` waits for a run to end,
  instead of staying open for up to 10 minutes.
- `get_run` with `wait_seconds` follows a flow started with `noct flow run` in a shell to its end,
  instead of answering at once that the flow has no run.
- Panes whose programs show many images no longer keep growing in memory, and large images sent
  in many small pieces no longer slow the pane down.
- A scheduled flow no longer runs twice when clocks go back an hour for daylight saving time.
  Times skipped when clocks go forward do not run that day, as in cron.
- `noct quit` returns once the workspace has stopped, with the sessions saved and the logs of its
  programs closed, so a `noct` started right after it no longer races the old workspace.

## v0.2.3 - 2026-11-04

### Added

- In fullscreen, the middle of each side of the border is dashed (`─ ─ ─`) where another panel
  lies that way, so you can tell where `prefix` + an arrow or `shortcut` + an arrow will take you.

### Changed

- Scripts in a project's `.noct/scripts` show no program version in the scripts panel and in
  `noct ls`: finding it ran a command from the project, so opening noct inside a cloned
  repository could run that repository's code.
- The config panel no longer saves over a `config.toml` that has an error: it says where the
  error is, so a typo cannot cost every other setting in the file.

### Fixed

- noct no longer crashes when you turn the mouse wheel while selecting text in a panel that
  closed or is in another tab, or when a selection is copied from a panel too small to show a
  line.
- Running a script from the home screen no longer crashes noct when the script cannot start.
- A program that keeps asking the terminal questions without reading the answers no longer
  freezes noct.
- A terminal that stops reading, such as one suspended, no longer freezes noct for every
  other terminal attached to it.
- A flow step whose panel cannot open now fails with the reason instead of waiting forever.
- A program printing a crafted image command can no longer crash noct or make it use unbounded
  memory.
- `5/15` in a flow's cron now runs at :05, :20, :35 and :50, as in cron, not only at :05.
- A watch trigger on one file reads only that file every 2 seconds, and a pattern only the
  folders it can reach, instead of walking everything below them, such as the whole home folder
  for `~/notes.md`.
- `noct check` now finds invalid `capture` and `output_matches` patterns on script steps.
- A flow whose script step gives `with:` a name the script has no parameter for is refused,
  naming the script's parameters, instead of running the script with its default.
- Logs of flow steps that run a script in a folder now show in the logs view.
- Two panels with the same log name started in the same second, such as two flow steps with
  `pane: new`, keep a log each instead of writing over one.
- The MCP audit log keeps the calls of earlier AI sessions instead of starting over each time.
- Saving the config or a theme that is a link, such as one kept in a dotfiles repository, writes
  to the file it points to instead of replacing the link with a plain file.
- Tab completion of a path no longer inserts half a character when names share only part of an
  accented letter.
- A script killed by a signal makes `noct -s` exit with 128 plus the signal, as shells do.

### Security

- noct's AI tools can read only noct's own logs: a log name with `..` could reach any file you
  can read.
- An AI's request waiting for approval shows a command with several lines on one line, so lines
  below what the popup fits cannot hide.
- Without `XDG_RUNTIME_DIR`, noct refuses a socket folder in `/tmp` that another user created,
  where they could listen in noct's place and read what you type.
- Two noct daemons starting at the same moment no longer both run, which left one with panels
  nobody could reach and triggers firing twice.
- The command shown to update a binary in a folder you cannot write extracts into a new private
  folder instead of `/tmp/noct`, which another user could create first.
- An AI running a script or flow with values of its own (`params`) now waits for your approval:
  the values go into the command, so they could add commands of their own. `run = "allow"` in
  `[mcp.policy]` brings back running without asking.
- An AI's request waiting for approval is withdrawn when the AI client exits, so approving it
  later no longer runs what nobody waits for.

## v0.2.2 - 2026-10-31

### Added

- The logs list has titled columns: when each log was last written, when it started, how its
  run ended, its name, the session it belongs to, its weight, size and lines. **Logs → list
  columns** in the config chooses which show and in what order; on a narrow list the less
  important ones step aside so the name keeps room.
- The selected log can sit below, right, above or left of the calendar and the list, taking 20%
  to 80% of the logs view: `p` moves it and `+`/`-` resize it, and both are kept in the config
  as **Logs → log** and **Logs → log size**. `z` shows the log alone, and `v` hides it.
- `?` in the logs view lists all its keys and what each does, as your binds have them. Like
  every other key there, it can be changed in **Keys → binds of every screen → logs**.

### Changed

- A log shows on the day it was last written as well as the day it started, so a terminal left
  open since yesterday is in today's list, and the list is sorted by the last write.
  **Logs → keep** counts from the last write too, so a log still being written is never
  deleted for its age.
- Terminal logs are numbered within each day: the first terminal of the day is `pane1`, instead
  of a number that kept growing for as long as noct ran.
- The hints under the logs view show only the keys to move through the calendar, the list or
  the log, whichever has the focus, and how to reach the next one; `?` has the rest.
- `Space` works as `Enter` in the logs view: from the calendar to the list, to the log, and
  back to the list.

### Fixed

- Every line of a long log can be reached in the logs view. It shows 5,000 lines at a time,
  scrolling past either end goes on into the lines before or after, and its title says which
  lines are in view. Before, only the lines the terminal had kept could be scrolled to.
- A log with hundreds of thousands of lines no longer pushes its weight bar out of line with
  the others; counts read `300k` or `1.2M`.
- The scroll position of the log says `rows`, since it counts rows on screen, which long lines
  and full-screen programs make differ from the log's lines.
- On the 31st, `]` in the logs calendar no longer skips the next month.
- The logs list no longer draws over the day's summary when the view is short.

## v0.2.1 - 2026-10-25

### Added

- Eleven new themes in **Appearance → theme**: `tide`, `moss`, `pumpkin`, `neon`, `cobalt`,
  `sand`, `slate`, `amber`, `phosphor`, `paper` and `sakura`. Some give popups and inactive
  tabs a background of their own, a little lighter than the theme's.
- **Appearance → mode** switches every theme noct comes with between its dark and its light
  colors, so `blood` or any other works in a light terminal too. Your own themes keep their
  colors in both modes.
- **noct setup**, which also opens the first time noct runs: the prefix and shortcut, which
  themes the config offers and which one is on (with its mode and background, shown on the
  whole screen), the animations (`all`, `few` or `none`), a demo of flows, adding noct to the
  AI clients you have, and **create new shortcut** for keys of your own. `s` skips a step;
  nothing is saved until the summary. `make install` runs it at the end. If you used noct
  before, the tab bar tells you about it once.
- Your own binds: a key that runs a command, a script or a flow, made in **Keys → binds of
  every screen → custom** (`n` adds one) or as `[[custom_binds]]` in the config. A direct key
  needs `ctrl` or `alt`; `prefix m` puts it after the prefix. A command's `{{.file}}` is asked
  before it runs, with `Tab` completing the path, and its pane closes when it ends well. The
  search lists the binds.
- A script param with `path: true` completes with `Tab` in the form, as in a shell.
- The demo: scripts `demo/*` and the flows `demo-deploy` and `demo-loops`, made up with
  `printf` and `sleep`, to learn flows by running them. Start with `demo/start-here`.
- `'` opens the search too: `prefix` + `'` and `shortcut` + `'`.

### Changed

- Animations are on by default in most moments: hover fade, panels and tabs jitter, move fade,
  fullscreen and popups `01`, activity fade. Times you set in the config stay as they are.
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
