# Code map

Where each responsibility lives. Package-level design and phases: [arquitetura.md](arquitetura.md).

Keep this table current: add a row when you add a package or a main file, and fix it when a responsibility moves.

| Path | Responsibility |
|---|---|
| `cmd/noct/main.go` | Entry point: `noct` (attach to the last session, starting the daemon, and starting the new binary after an update's restart), dispatch of the session commands, `noct quit`, `noct daemon`, `noct version`, dispatch of the script and update commands |
| `cmd/noct/version.go` | The version the binary was built as (`-X main.version`, or go install's, or dev) and `noct update`: the newer releases' changelog, then installing the newest once the user agrees |
| `cmd/noct/version_test.go` | Tests of `noct update` against a local release server: install on yes, nothing without, newest, a read-only folder, a dev build |
| `cmd/noct/sessions.go` | `noct attach`/`a`/`att`/`-a` (listing the sessions without a name), `noct new`, `noct sessions`, `noct kill`: sessions by name, asking the running workspace or, without one, reading `sessions.json` |
| `cmd/noct/sessions_test.go` | End-to-end tests of sessions: two sessions apart, a missing one refused, a session opened again after quit in its shell's directory, a program offered to run again after quit, kill without noct running |
| `cmd/noct/share.go` | `noct export` (a theme, the changed settings, the keys and binds as a line and a QR code) and `noct import` (a code, showing what changes and asking; a theme file of kitty, Ghostty or base16) |
| `cmd/noct/log_times.go` | `noct log-times`, which the logs view runs: a log's lines, a range of them or following its end, after when each started |
| `cmd/noct/mcp.go` | `noct mcp`: the MCP server on stdin and stdout, with the audit log and the daemon as its workspace |
| `cmd/noct/mcp_test.go` | End-to-end test of `noct mcp`: an AI runs a flow in a real workspace and reads its result and pane |
| `cmd/noct/scripts.go` | `noct ls`, `noct check`, `noct -s`: scripts in the current terminal, without the daemon |
| `cmd/noct/flows.go` | `noct flow run` (whole, or `--steps` to test a range; asks on the terminal for `on_fail: ask`, asks the workspace for step panes), `noct flow ls`, `noct enable`/`disable`, flows and their warnings in `noct check`: runs a flow in the current terminal and keeps its result |
| `cmd/noct/main_test.go` | End-to-end tests: the real binary in a pane, each test with its own daemon |
| `cmd/noct/flows_test.go` | End-to-end tests of the flow commands: warnings, kept output, testing a range, enabling triggers, an after trigger and a step's pane in a real workspace |
| `internal/ipc/ipc.go` | Client↔daemon protocol: framed messages, socket path, listen and dial; attach, quit and pane requests |
| `internal/client/client.go` | The terminal side: raw mode, forwarding keys and size, writing frames (and tracing them with `NOCT_TRACE`); `Quit`, which returns once the daemon has stopped |
| `internal/client/request.go` | Asking the running workspace to do one thing for the MCP server, and reading its answer |
| `internal/client/pane.go` | Asking the running workspace for a step's pane and waiting for its exit code |
| `internal/daemon/daemon.go` | Run the daemon at its version; start it in the background when needed; why it stopped, from the panic in `daemon.log` |
| `internal/daemon/daemon_test.go` | Tests of telling why the daemon stopped: the last panic of a recent log, or only where the log is |
| `internal/update/version.go` | Semantic versions: reading and ordering them, telling a release from a dev build |
| `internal/update/release.go` | Releases on GitHub: listing them with their changelogs, the ones newer than a version |
| `internal/update/cache.go` | `updates.json`: the last check (releases, when, ETag, how long GitHub asked to wait), read at start and written after each check |
| `internal/update/install.go` | Installing a release: download, checksum, replace the binary atomically; whether its folder is writable; the command that installs it by hand |
| `internal/update/update_test.go` | Tests of versions, listing releases and installing them against a local server |
| `internal/logstore/logstore.go` | State directory and size-capped, rotating pane logs; appending to one and reading its end; noting each new day a log writes on |
| `internal/logstore/times.go` | When each line of a log started: the `.times` file beside it (a record per second with output), and printing lines after their times, following the last file (`noct log-times`) |
| `internal/logstore/entries.go` | Logs by day, each with when it started and was last written, the days it had output on and a flow run's result beside it: listing, line counts, removing, pruning by last write and total size, terminal logs numbered within the day, readable sizes |
| `internal/pty/pty.go` | Start programs in pseudo-terminals, resize, hang up, stop (SIGTERM then SIGKILL to the group), exit code, foreground program and its command line, current directory |
| `internal/pane/pane.go` | One terminal: process + VT emulator, input/output goroutines, drawing and its history (the main screen under a full-screen program, for the logs preview), its lines as text and the lines matching a filter drawn alone |
| `internal/pane/mouse.go` | Mouse modes a program asked for, and forwarding mouse events to it |
| `internal/pane/history.go` | An earlier program's output drawn on a screen of its own and replayed as text and colors above a new one |
| `internal/pane/sync.go` | Holds back a synchronized-output frame (mode 2026) until it ends, so it never shows half drawn |
| `internal/pane/images.go` | Writes image placeholder cells at the cursor and moves it as kitty does |
| `internal/pane/keys.go` | Key encodings the emulator lacks (modified navigation keys) |
| `internal/session/session.go` | Saved sessions (`sessions.json`): tabs as layout shapes and panes that can open again (a terminal with its command and log), names, what the command line lists |
| `internal/theme/theme.go` | Theme components, colors (hex, terminal indexes, following the background) and the themes noct comes with, each with dark and light colors |
| `internal/theme/foreign.go` | Themes of other terminals (kitty, Ghostty, base16) read as noct themes: background, text, the 16 colors and the components from them |
| `internal/theme/files.go` | User themes: the themes folder, reading theme files built on a base (with their `[ansi]` colors), and writing only what differs from it |
| `internal/share/share.go` | The codes `noct export` prints and `noct import` reads: kind, name and TOML, compressed into one word after `noct1.` |
| `internal/share/qr.go` | A code's line drawn as a QR code with half blocks, dark on light (rsc.io/qr, ADR-064) |
| `internal/templates/project.go` | A project's templates (`.noct/templates`, found as git finds a repository), with every command asked |
| `internal/templates/templates.go` | Tab templates: the YAML file (panes, groups, colors, how each command starts), layouts to and from it, and the templates folder |
| `internal/layout/shape.go` | A tree as plain data (`Shape`) and building it again under new pane IDs |
| `internal/layout/layout.go` | A tab's i3-like container tree: split (autotiling), stacks of a pane and those below it (one pane shown, the others folded to a row), move by columns, remove, rects, directional focus, draggable dividers |
| `internal/layout/geometry.go` | Each pane's rect from the tree and the area it is given: shares, stacks, boundaries |
| `internal/layout/divider.go` | Draggable dividers between panes and resizing by keys |
| `internal/layout/neighbor.go` | Directional focus: the nearest pane in a direction, preferring the one a stack shows and the one used last |
| `internal/app/app.go` | App state, event loop, frame pacing, attach (to the session asked for)/detach, config loading, shutdown |
| `internal/app/sessions.go` | Sessions: create, switch, close (keeping the layout), delete, rename; the command line's list and delete |
| `internal/app/sessions_saved.go` | Saving sessions to `sessions.json` and opening a closed one again: tabs, panes, views and their directories; the session of each terminal's log |
| `internal/app/rerun.go` | The box over a reopened terminal offering to run again what it ran (enter types it, esc forgets it); typing text into a view |
| `internal/app/sessions_popup.go` | Sessions popup (prefix+o): the sessions by last use, keys to open, create, rename, close and delete |
| `internal/app/updates.go` | Updates: the cache of the last check, checking once it is a day old or on demand (with the ETag and a cooldown), installing the newest in the background, the command to install by hand, restarting into it |
| `internal/app/updates_popup.go` | Updates popup (prefix+c): what is new, the changelog of every release, keys to check, update, copy the command and restart |
| `internal/app/markdown.go` | Markdown drawn in the theme's colors, for the changelog: headings, lists, quotes, code, rules and inline marks, wrapped to a width with lists hanging under their text |
| `internal/app/updates_test.go` | Tests of the updates popup, the tab bar's version, installing, restarting and the read-only case |
| `internal/app/client.go` | Daemon side of an attached terminal: decoding its keys, drawing to it (synchronized updates only when the terminal has mode 2026), greeting connections |
| `internal/app/scroll.go` | Scroll mode through a terminal's history: vim's keys and the arrows, shared with the logs preview; any other key leaves |
| `internal/app/workspace.go` | Starting programs in panes (shells remembered for their session), the folder a new terminal opens in (`terminalDir`), tabs and panes: open panels and tabs, focus, stacking, move across tabs, close, detach (in any session) |
| `internal/app/view.go` | The `view` interface a pane shows: a terminal or a noct panel |
| `internal/app/scripts_panel.go` | Scripts panel: list with versions, details and notes (with their images), its filter, keys to run, edit, create, delete |
| `internal/app/scripts.go` | Script actions: reload the library, run (form or not) as a tracked run with its log and exit code, create, edit in the configured editor, rename, delete |
| `internal/app/form.go` | Parameters form shown before a script runs, or a flow (`p` in the inspect) |
| `internal/app/run_view.go` | A script's run: its terminal plus a status line, held open with the exit code |
| `internal/app/template_complete.go` | Completing the variables in scope as `{{.` (or `.` after a function, `{{eq .`) is typed in a step form's template row |
| `internal/app/finder.go` | Fuzzy search over scripts, actions and tabs (prefix+space), or over given items (a flow step's script) |
| `internal/app/help.go` | Help for the focused terminal's foreground program (prefix+?) |
| `internal/app/home.go` | Home screen (no tabs or an empty tab): title, raven, intro and when it plays (animate intro), per-character flicker on hover, shortcuts reveal and timer |
| `internal/app/raven2.ascii` | The raven logo embedded in the binary |
| `internal/app/keys.go` | Key routing by mode, command and move modes, the shortcuts table and popup |
| `internal/app/lock.go` | The lock (prefix+b): which commands it holds back and the hint on how to unlock |
| `internal/app/prompt.go` | Text prompt (tab and panel rename, new folder) and yes/no confirmation (quit, delete) |
| `internal/app/choice.go` | A choice of a few options (new directory or script) |
| `internal/app/info_popup.go` | An info popup (what a flow's step does, a screen's keys): its filter, scrolling by keys and wheel, back to the screen it was opened from |
| `internal/app/textfield.go` | The one-line text field every prompt, form, search and config row types into: cursor, shell editing keys, paste (bracketed or OSC 52), the text a drag marks, cursor view |
| `internal/app/logs_panel.go` | Logs panel (prefix+v): month calendar marked by runs, the logs of a day (started or last written on it), sorting, clearing, where the log goes and whether it or the lists show, the hints of the part with the focus and the popup of every key; log pruning |
| `internal/app/logs_columns.go` | The logs list's columns from the config: their widths over the whole list, dropping them on a narrow list, each cell (times, session, size, compact line counts), the titles |
| `internal/app/logs_preview.go` | The selected log beside the logs panel's lists: a terminal that prints a part of 5000 lines of it, the last following its end, restarted as the selection or the part changes, and scrolling its history on into the parts before and after |
| `internal/app/logs_search.go` | Searching the log shown (`/`, `n`, `N`, its matches lit) and filtering its lines (`f`) |
| `internal/app/library_watch.go` | Reading scripts and flows again while a panel shows them and their files changed on disk |
| `internal/app/tree_list.go` | The rows of a list of items in folders inside folders (`treeList`), for the scripts panel and the inspect: open folders, the filter, counts, the bars of each level |
| `internal/app/inspect_folders.go` | Organizing the inspect's list: new folders (N), moving or renaming flows, functions and folders (m, dragging), copies (c), deleting folders |
| `internal/app/inspect_list.go` | The inspect's list in folders: flows by their folders, functions under ƒ functions, the selected row (a folder or the flow), its keys and new flows in a folder |
| `internal/app/inspect_panel.go` | Inspect panel (prefix+i): framed list of flows (tab hides it, f filters it) and the selected flowchart, keys, step ranges and tests, panning, zoom, clicks and double clicks, alt+drag carrying a card to its place |
| `internal/app/inspect_details.go` | The details under the inspect's chart: the selected block's kind, whole text and run, scrolled by the wheel |
| `internal/app/flowchart.go` | Laying out and drawing a flow as a flowchart on its own canvas, and which block is where: steps with the mark and name of their kind, empty lists, the places a carried card drops at and its dashed outline; ranges and warnings marked |
| `internal/app/triggers.go` | The daemon's trigger ticks, flows started by triggers (headless, logged, recorded), adding triggers from the inspect |
| `internal/app/notify.go` | Telling a run that ended out of sight: the system's notification (`notify-send`, `osascript`), OSC 9 to the terminal, or its bell |
| `internal/app/requests.go` | What the MCP server asks of the workspace: list and read panes, type into one, run a command, script or flow where the user sees it; reading a request's connection until it ends (`receiveUntilEnd`) |
| `internal/app/mcp_panel.go` | MCP popup (prefix+y): approvals waiting for the user and their timeout, AI clients connected now (the `client` request), setups to copy |
| `internal/app/mcp_setup.go` | The command or config JSON that adds noct to each known AI client, with this binary's path |
| `internal/app/pane_request.go` | Opening the pane a flow's step asks for, beside its run, and answering with its exit code; stopping it at the step's timeout; bringing a run to the screen for its prompt (`attention`) |
| `internal/app/logs_heatmap.go` | The logs calendar's activity colors: each day's step by its logs, the style over it, and the legend |
| `internal/app/logs_filter.go` | The logs filter line: words over name, date, time and status; size and line comparisons |
| `internal/app/scope.go` | The inspect's scope pane: the variables of the selected block by their short names, with their last values |
| `internal/app/flow_agents.go` | The agents an agent step's form offers (default, auto, the installed ones, a custom one), what each note says, and adding a custom agent to the config |
| `internal/app/flow_functions.go` | Functions in the inspect: making one, picking a call step's function, returns on the params card, which flows a change reaches |
| `internal/app/flow_params.go` | A flow's params table in the inspect, a param a row in columns (type, name, default, from env, label, required, secret): the default's cell for each type and changing it with the type, checking each row, saving the table as it changes and in its order, the help of each column, how the card writes a param |
| `internal/app/edit_grid.go` | `editGrid`: rows of cells (text, option, toggle, list) moved through with hjkl, the arrows and tab, worked with enter or a click; typing in a text cell and completing variables in template cells; button rows; moving and removing rows; undo and redo; clicks and the wheel |
| `internal/app/grid_lists.go` | What opens under a grid's cell: the menu of an option cell's options and the list of a list cell's items, a grid of its own with the default marked, both filtered with f or / |
| `internal/app/inspect_editor.go` | `inspectEditor`, what the inspect shows in place of the chart while params or a step are edited: its frame, the help beside it, hints kept to one row |
| `internal/app/flow_step_form.go` | A step's form in the inspect: its rows, checking them, adding or saving the step, the variables beside it; opening the params table, adding and editing steps from the inspect |
| `internal/app/flow_setting_forms.go` | The forms for a block's exports, a step's retry and a function's returns, opened from the inspect like a step's form |
| `internal/app/flow_step_kinds.go` | What each kind of step's form asks (command, if, switch, repeat, for each, wait, prompt, set, unset, call, agent) and how its answers become the step, keeping what it does not ask |
| `internal/app/flowchart_live.go` | A flow's run on its chart as it goes: the dots going down the arrow out of the step running, and the frames that move them |
| `internal/app/flow_history.go` | Undo and redo of the inspect's changes to a flow: its file before each change, kept while noct runs |
| `internal/app/flows.go` | Flow actions: create, run and test steps (as tracked runs), add, edit in the editor, move and remove steps, what a step does and its kept output |
| `internal/app/keymap.go` | Changeable keys: the contexts and their actions (`keyContexts`), the keymap built from the config, turning a pressed key into the default one a handler knows, hints written with the keys as they are |
| `internal/app/binds.go` | The config's binds page: a row per action, taking a key with conflicts, moving keys between actions, reset, ? listing its keys |
| `internal/app/custom_binds.go` | The user's own binds: which key each takes and why one cannot be used, running a command, a script or a flow on it |
| `internal/app/custom_binds_page.go` | The custom section of the binds page: adding a bind (key, kind, what it runs, name), editing and removing one |
| `internal/app/complete_path.go` | Tab completion of a path param in the form, to the common prefix of the matches in its folder |
| `internal/app/setup.go` | noct setup (ADR-056): when it opens (or, once, tells it is there), its steps and keys, the draft config and saving it, the demo and the AI clients at the end |
| `internal/app/setup_draw.go` | Drawing the setup: the steps' bar, each step's lines, the theme swatches beside the stage, the summary |
| `internal/app/shortcut.go` | Direct shortcuts: modifier + noct mode key without the prefix, modifier + shift + arrows to move |
| `internal/app/mouse.go` | Mouse: tab and pane clicks, dragging dividers, wheel scrolling (over the tab bar, changing tab), forwarding to views that take it; the pane a button is held on keeps the drag, and a release noct never saw (outside the window) ends it |
| `internal/app/editor.go` | Program settings (editor, file manager): choices found on the system or a custom command; opening a folder |
| `internal/app/note_images.go` | Images in a script's notes (a line `![what](file.png)`): reading PNG, JPEG and GIF within size limits, and drawing them with half blocks, kept per size |
| `internal/app/note_images_kitty.go` | Note images in full resolution where kitty's placeholders show: sending the PNG once per attached terminal, its virtual placement, the placeholder cells |
| `internal/app/script_form.go` | Script form: name and program typed in place, command line, description, params and notes each edited alone in the editor |
| `internal/app/pane_filter.go` | A terminal filtered by a pattern (`f` while scrolling): the last lines that match, drawn live in place of its screen |
| `internal/app/links.go` | `Ctrl` + click on a pane: the OSC 8 link, URL (joined over wrapped rows) or existing path under the mouse, opened with `xdg-open`/`open`, or copied over ssh |
| `internal/app/selection.go` | Selecting text with the mouse in one pane, across scrolling, highlighting and copying it (OSC 52) |
| `internal/app/rebuild.go` | Notices on attach that the noct binary was rebuilt after the daemon started |
| `internal/app/prefix.go` | The configurable prefix key: parsing, matching, key names; capturing a new one and what it would take from shortcuts and programs |
| `internal/app/config_panel.go` | Config panel: rows built section by section, key handling, how each row steps its setting, clicks and hover, applying and saving, reset to defaults, drawing (notes and hints wrapped) |
| `internal/app/config_filter.go` | The filter of the config's settings and binds pages: which rows and sections it keeps, its keys, keeping the selection on a row shown |
| `internal/app/config_agent.go` | The config's default agent row and its popup: auto, the agents this machine has with their models, a custom agent; the model picker (models listed in the background), and changing or removing an agent's command |
| `internal/app/list_filter.go` | `listFilter`, the live filter of long lists (config, binds, scripts, flows): f or / opens it, matching every word, its keys and its line |
| `internal/app/config_numbers.go` | Log limits typed by hand in the config: enter or a digit opens them, days and sizes in KB/MB/GB parsed, enter saves and esc gives up |
| `internal/app/panel_kinds.go` | The kinds of panel a key opens (`panelKinds`), which `runAction` and the new tab setting read; opening a new tab |
| `internal/app/panel.go` | Shared panel component: framed box, rows, list scrolling with a margin around the selection, the list keys shared by panels (`listJump`, `listMove`: arrows, j/k, g/G, home/end, pages), centered and corner placement |
| `internal/app/tab.go` | A tab: its views, pane names, focus history, floating panes, fullscreen, visible frames, size rules |
| `internal/app/floating.go` | Floating panes: opening, keyboard moves and resizes, mouse drags, focus navigation over both layers |
| `internal/app/resize.go` | Resize mode (prefix+n): arrows grow or shrink the focused tile or floating pane |
| `internal/app/draw.go` | Frame drawing: panes (folded ones as a title row) with their status (fullscreen, scroll position) at the right of the border and, in fullscreen, the dashed sides that have a pane beyond them, the tab bar's status, mode overlays, cursor placement |
| `internal/app/tabbar.go` | Tab bar: names cut to fit, scrolling that keeps the active tab in view, marks for hidden tabs, which tab a click or the wheel selects |
| `internal/app/images.go` | Image support detection, outer image ids, cell size, forwarding to the terminal |
| `internal/app/environment.go` | Shell and environment given to pane programs |
| `internal/app/theme.go` | The themes on offer (built-in, then the user's, read again when their folder changes) and the palette of styles drawn from the theme in use |
| `internal/app/theme_editor.go` | Making and editing themes in the config: the new and edit rows, the name and base, and the page of components beside the stage |
| `internal/app/color_picker.go` | The color picker: the terminal's 16 colors, one slice of the cube as a triangle (`,` and `.` change it), and the grays, drawn with Lip Gloss; moves by position; a typed #rrggbb |
| `internal/app/colorset.go` | A pane's or a tab's own colors over the session's theme: saved by the theme file's keys, and the palette a pane or a tab draws with |
| `internal/app/colors_popup.go` | The popup of prefix + P: the focused pane's and its tab's own colors, with the stage and the contrast warning |
| `internal/app/template_save.go` | The popup of prefix + R: the tab as a template, with how each terminal's command starts |
| `internal/app/templates_popup.go` | The popup of prefix + T: the user's and the project's templates with the layout of the selected one, opening one as a tab, and edit, rename and delete; the template a new session opens with (`noct new --template`) |
| `internal/app/animation.go` | Animations: what changed since the last frame, the ones running, and drawing them over the frame without touching the programs |
| `internal/app/hover.go` | Hover: what is under the mouse (a tab's name, a pane's title, a list's row), its animation there and back, and lighting list rows |
| `internal/app/effects_sweep.go` | The effects that move or sweep: `slide-left` and `slide-right` in and back out, the `shine` band toward the color that stands out |
| `internal/app/effects.go` | The animation effects as functions of time: easing, 01 and jitter |
| `internal/app/stage_preview.go` | The config's rows of the animations (each moment's effect and time) and the stage under them playing the selected moment's effect |
| `internal/app/stage.go` | The stage: a small made-up workspace with a bit of everything a theme colors, to preview themes in place |
| `internal/kittygfx/command.go` | Parse and encode kitty graphics commands |
| `internal/kittygfx/stream.go` | Split graphics commands out of program output, across reads |
| `internal/kittygfx/session.go` | Per-pane image state: id translation, uploads, sizing, replies |
| `internal/kittygfx/placeholder.go` | Placeholder cell text and color for an image cell |
| `internal/kittygfx/diacritics.go` | Kitty's fixed row/column diacritics table |
| `internal/atomicfile/atomicfile.go` | Replacing files whole through a temp file of their own (`Write`) and changing shared state files under a `flock` on `<file>.lock` (`Update`) |
| `internal/naming/naming.go` | The rules for names of scripts, flows, templates and themes (`IsValid`), for names inside folders (`IsValidPath`) and for template identifiers (`IsIdentifier`) |
| `internal/tree/tree.go` | Folders inside folders, read in name order to `MaxDepth`: the scripts', flows' and functions' (`Walk`), and a path's `Parent` and `Base` |
| `internal/config/config.go` | Settings file: defaults, validation, atomic load and save |
| `third_party/ultraviolet`, `third_party/vt` | Copies of charmbracelet/ultraviolet and x/vt with throughput and rendering fixes (`noct:` comments); see `third_party/README.md`. `noct_render_test.go` replays the renderer's output into the emulator |
| `docs/demo/record.sh`, `docs/demo/*.tape` | The README's GIFs (`make demo`): VHS tapes run noct in a home of its own, with the demo scripts and flows of `docs/demo/scripts` and `docs/demo/flows` |
| `examples/examples.go`, `examples/demo/` | The demo the setup installs, all in folders `demo`, moving a demo flow or function an older noct left outside it: scripts `demo/*` with tutorial notes the flows `demo-deploy` (a deploy with health check and rollback), `demo-loops` (each step kind alone) and `demo-triage` (an AI agent sorts a failing test), and the function `demo-explain` (scripts include `demo/triage-intro`, `demo/pick-failure`, `demo/failing-test` and `demo/report`), embedded and copied without overwriting |
| `docs/assets/` | The README's logo (`logo.svg`, the home's art as rectangles) and GIFs |
| `.github/workflows/ci.yml`, `.githooks/pre-push`, `Makefile` (`hooks`) | Checks before a release: `make check` on GitHub for every pushed branch and pull request, and locally on each pushed commit, which also stops a version tag off the tip of `main` or without its CHANGELOG section |
| `.github/workflows/release.yml`, `Makefile` (`dist`, `release-notes`), `CHANGELOG.md` | Releases: a pushed `vX.Y.Z` tag runs the checks, builds the archives and checksums, and publishes the GitHub release with the tag's CHANGELOG section, which noct's updates read (ADR-047) |
| `internal/script/script.go` | A script: load, validate (params and outputs), render its command line, create and delete; `WalkFields`, the template field walker flows also use |
| `internal/script/library.go` | The library: personal scripts, folders of scripts (in the scripts folder, linked, project), IDs, lookups |
| `internal/script/edit.go` | Creating a script (with optional hints) and updating it from the form's fields, keeping the rest of the YAML; params as YAML text |
| `internal/script/values.go` | Starting values of parameters, checking them, remembered values file |
| `internal/script/history.go` | The list of recent runs of scripts and flows (the last 1000), with exit code, log and session, and sorting scripts by name, edit or last run |
| `internal/flow/flow.go` | A flow: steps (script, run, call, agent, if, switch, repeat with each, wait, prompt, set, unset, retry), the lists inside a step (`Branches`), loading, saving, creating, template functions |
| `internal/flow/timeout.go` | Flow and step timeouts: parsing and validation, the `noct check` warning, run and step deadlines, stopping a step's processes |
| `internal/flow/validate.go` | Checking a flow and its steps: ids, fields that belong to each kind, patterns |
| `internal/flow/agent.go` | Agent steps: a command split into words with the question in place of `{prompt}`, or as its input after `< {prompt}` or with no `{prompt}`, the answer without the model's thinking, and its choices |
| `internal/flow/agents.go` | The agents noct knows (presets, each a command with `{model}` and its default model), the models the config picks for them and the ones each program lists, the config's own commands over them, which are ready on this machine, `auto` (the first ready one), `default` (the config's, for a step's use) and the ollama preset falling back to a model the machine has |
| `cmd/noct/agents.go` | `noct agents`: the agents, which are ready, the default, and one to start from when none is; `noct agents default <name>` saves the config's agent |
| `internal/flow/functions.go` | Functions: their folder, loading them with the flows (`LoadWithFunctions`), checking calls, who calls each (`Callers`), running a call in a scope of its own |
| `internal/flow/progress.go` | The step a run is on, kept beside the flow's last result while it runs, for the inspect's live view |
| `internal/flow/run.go` | Running a flow step by step in a terminal: commands, if, repeat, wait, captures (and scripts' outputs), expect |
| `internal/flow/result.go` | A run's result with each step's output and status; saving and loading the last one |
| `internal/flow/try.go` | Testing a range of sibling steps over the last result (its steps' fields and variables), and merging what they did into it |
| `internal/flow/vars.go` | Params with a type (text, number, boolean, option, path, list, object; `bool` and `choice` read as before), required and a label, and checking values; running prompt, set and unset steps |
| `internal/flow/json.go` | List and object params: decoding their JSON, lists and objects inside each other, for templates to read inside and print whole as JSON; a list's items as the inspect edits them (`ListItems`, `ItemsJSON`) |
| `internal/flow/dollar.go` | `$name` in flow templates: the names a flow knows (`knownNames`) and `expandDollars`, which rewrites them as `{{ }}` escaped for a run line's quotes or a JSON default |
| `internal/flow/move.go` | Flows and functions as files in folders: `Folders`, `CreateFolder`, `DeleteFolder`, `Copy`, `Rename` and `MoveFolder` (with the last result and triggers), `RenameCalls` |
| `internal/flow/param_defaults.go` | Params' defaults as templates over the params before them (`withDefaults`, `DefaultProblem`), and the `json` template function |
| `internal/flow/param_order.go` | The order of a flow's params: read and written as the YAML has them (`ParamOrder`), and `ParamNames` in that order |
| `internal/flow/trigger.go` | Triggers (manual, cron, watch, after) in YAML, their validation, globs with `**`, loops of after triggers, which flows have them enabled |
| `internal/flow/cron.go` | Five-field cron schedules: parsing and matching a minute |
| `internal/flow/pane.go` | Steps in a pane of their own (`pane: new`, `background`): validation, asking noct's workspace, running inline outside it |
| `internal/mcp/server.go` | The MCP protocol: JSON-RPC over stdio, initialize, tools/list, tools/call, answers as text and structured content |
| `internal/mcp/tools.go` | Tools, their input schemas, the policy (allow, ask, deny) and the audit log around every call, whose context ends when the AI client leaves |
| `internal/mcp/library_tools.go` | Tools over scripts and flows: list, read, check, scope, drafts, triggers |
| `internal/mcp/run_tools.go` | Tools over runs and logs: list, the structured result, search, paged reading |
| `internal/mcp/workspace_tools.go` | Tools the running workspace does: panes, runs the user sees, command lines, typing |
| `internal/schedule/schedule.go` | Deciding when enabled flows start: cron minutes (each wall-clock minute once, across daylight saving), watched files settling, another flow's run ending |
| `internal/flow/scope.go` | What templates can read at each point of a flow (parameters, variables set and unset, earlier steps' fields, how sure), and the shortest name each is read by (`Readable`) |
| `internal/flow/warnings.go` | Warnings for what templates read that may not be there, and the scope warning for a short name two steps set |
| `internal/flow/edit.go` | Editing a flow by step path: insert, replace, remove, move (through any block's lists), unique ids |
| `internal/script/version.go` | Program version detection, in the background and cached |
