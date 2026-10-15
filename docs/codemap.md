# Code map

Where each responsibility lives. Package-level design and phases: [arquitetura.md](arquitetura.md).

Keep this table current: add a row when you add a package or a main file, and fix it when a responsibility moves.

| Path | Responsibility |
|---|---|
| `cmd/noct/main.go` | Entry point: `noct` (attach to the last session, starting the daemon), dispatch of the session commands, `noct quit`, `noct daemon`, dispatch of the script commands |
| `cmd/noct/sessions.go` | `noct attach`/`a`/`att`/`-a`, `noct new`, `noct sessions`, `noct kill`: sessions by name, asking the running workspace or, without one, reading `sessions.json` |
| `cmd/noct/sessions_test.go` | End-to-end tests of sessions: two sessions apart, a missing one refused, a session opened again after quit in its shell's directory, kill without noct running |
| `cmd/noct/mcp.go` | `noct mcp`: the MCP server on stdin and stdout, with the audit log and the daemon as its workspace |
| `cmd/noct/mcp_test.go` | End-to-end test of `noct mcp`: an AI runs a flow in a real workspace and reads its result and pane |
| `cmd/noct/scripts.go` | `noct ls`, `noct check`, `noct -s`: scripts in the current terminal, without the daemon |
| `cmd/noct/flows.go` | `noct flow run` (whole, or `--steps` to test a range; asks on the terminal for `on_fail: ask`, asks the workspace for step panes), `noct flow ls`, `noct enable`/`disable`, flows and their warnings in `noct check`: runs a flow in the current terminal and keeps its result |
| `cmd/noct/main_test.go` | End-to-end tests: the real binary in a pane, each test with its own daemon |
| `cmd/noct/flows_test.go` | End-to-end tests of the flow commands: warnings, kept output, testing a range, enabling triggers, an after trigger and a step's pane in a real workspace |
| `internal/ipc/ipc.go` | Client↔daemon protocol: framed messages, socket path, listen and dial; attach, quit and pane requests |
| `internal/client/client.go` | The terminal side: raw mode, forwarding keys and size, writing frames (and tracing them with `NOCT_TRACE`) |
| `internal/client/request.go` | Asking the running workspace to do one thing for the MCP server, and reading its answer |
| `internal/client/pane.go` | Asking the running workspace for a step's pane and waiting for its exit code |
| `internal/daemon/daemon.go` | Run the daemon; start it in the background when needed |
| `internal/logstore/logstore.go` | State directory and size-capped, rotating pane logs |
| `internal/logstore/entries.go` | Logs by day, each with a flow run's result beside it: listing, line counts, removing, pruning by age and total size, readable sizes |
| `internal/pty/pty.go` | Start programs in pseudo-terminals, resize, hang up, exit code, foreground program, current directory |
| `internal/pane/pane.go` | One terminal: process + VT emulator, input/output goroutines, drawing, mouse modes and forwarding |
| `internal/pane/sync.go` | Holds back a synchronized-output frame (mode 2026) until it ends, so it never shows half drawn |
| `internal/pane/images.go` | Writes image placeholder cells at the cursor and moves it as kitty does |
| `internal/pane/keys.go` | Key encodings the emulator lacks (modified navigation keys) |
| `internal/session/session.go` | Saved sessions (`sessions.json`): tabs as layout shapes and panes that can open again, names, what the command line lists |
| `internal/layout/shape.go` | A tree as plain data (`Shape`) and building it again under new pane IDs |
| `internal/layout/layout.go` | A tab's i3-like container tree: split (autotiling), stacks (one pane shown, the others folded to a row), move, remove, rects, directional focus, draggable dividers |
| `internal/app/app.go` | App state, event loop, frame pacing, attach (to the session asked for)/detach, config loading, shutdown |
| `internal/app/sessions.go` | Sessions: create, switch, close (keeping the layout), delete, rename; saving them and opening a closed one again; the command line's list and delete |
| `internal/app/sessions_popup.go` | Sessions popup (prefix+o): the sessions by last use, keys to open, create, rename, close and delete |
| `internal/app/client.go` | Daemon side of an attached terminal: decoding its keys, drawing to it, greeting connections |
| `internal/app/scroll.go` | Scroll mode through a terminal's history |
| `internal/app/workspace.go` | Starting programs in panes (shells remembered for their session), tabs and panes: open panels and tabs, focus, stacking, move across tabs, close, detach (in any session) |
| `internal/app/view.go` | The `view` interface a pane shows: a terminal or a noct panel |
| `internal/app/scripts_panel.go` | Scripts panel: list with versions, details and notes, keys to run, edit, create, delete |
| `internal/app/scripts.go` | Script actions: reload the library, run (form or not) as a tracked run with its log and exit code, create, edit in the configured editor, rename, delete |
| `internal/app/form.go` | Parameters form shown before a script runs, or a flow (`p` in the inspect) |
| `internal/app/run_view.go` | A script's run: its terminal plus a status line, held open with the exit code |
| `internal/app/finder.go` | Fuzzy search over scripts, actions and tabs (prefix+space), or over given items (a flow step's script) |
| `internal/app/help.go` | Help for the focused terminal's foreground program (prefix+?) |
| `internal/app/home.go` | Home screen (no tabs or an empty tab): title, raven, intro and when it plays (animate intro), per-character flicker on hover, shortcuts reveal and timer |
| `internal/app/raven2.ascii` | The raven logo embedded in the binary |
| `internal/app/keys.go` | Key routing by mode, command and move modes, the shortcuts table and popup |
| `internal/app/prompt.go` | Text prompt (tab and panel rename, new folder), yes/no confirmation (quit, delete), a choice of a few options (new directory or script) and an info popup (what a flow's step does) |
| `internal/app/textfield.go` | The one-line text field every prompt, form, search and config row types into: cursor, shell editing keys, paste (bracketed or OSC 52), cursor view |
| `internal/app/logs_panel.go` | Logs panel (prefix+v): month calendar marked by runs, the day's logs with weight bars, sorting, opening, clearing; log pruning |
| `internal/app/library_watch.go` | Reading scripts and flows again while a panel shows them and their files changed on disk |
| `internal/app/inspect_panel.go` | Inspect panel (prefix+i): framed list of flows (tab hides it) and the selected flowchart, keys, step ranges and tests, panning, zoom, clicks and double clicks |
| `internal/app/flowchart.go` | Laying out and drawing a flow as a flowchart on its own canvas, and which block is where: steps, empty lists and the gaps where steps are added; ranges and warnings marked |
| `internal/app/triggers.go` | The daemon's trigger ticks, flows started by triggers (headless, logged, recorded), adding triggers from the inspect |
| `internal/app/requests.go` | What the MCP server asks of the workspace: list and read panes, type into one, run a command, script or flow where the user sees it |
| `internal/app/mcp_panel.go` | MCP popup (prefix+y): approvals waiting for the user and their timeout, AI clients connected now (the `client` request), setups to copy |
| `internal/app/mcp_setup.go` | The command or config JSON that adds noct to each known AI client, with this binary's path |
| `internal/app/pane_request.go` | Opening the pane a flow's step asks for, beside its run, and answering with its exit code; bringing a run to the screen for its prompt (`attention`) |
| `internal/app/logs_heatmap.go` | The logs calendar's activity colors: each day's step by its logs, the style over it, and the legend |
| `internal/app/logs_filter.go` | The logs filter line: words over name, date, time and status; size and line comparisons |
| `internal/app/scope.go` | The inspect's scope pane (variables of the selected block with their last values) and the variable picker of template prompts (ctrl+t) |
| `internal/app/flows.go` | Flow actions: create, run and test steps (as tracked runs), add, edit in the editor, move and remove steps, what a step does and its kept output |
| `internal/app/keymap.go` | Changeable keys: the contexts and their actions (`keyContexts`), the keymap built from the config, turning a pressed key into the default one a handler knows, hints written with the keys as they are |
| `internal/app/binds.go` | The config's binds page: a row per action, taking a key with conflicts, moving keys between actions, reset |
| `internal/app/shortcut.go` | Direct shortcuts: modifier + noct mode key without the prefix, modifier + shift + arrows to move |
| `internal/app/mouse.go` | Mouse: tab and pane clicks, dragging dividers, wheel scrolling (over the tab bar, changing tab), forwarding to views that take it |
| `internal/app/editor.go` | Program settings (editor, file manager): choices found on the system or a custom command; opening a folder |
| `internal/app/script_form.go` | Script form: name and program typed in place, command line, description, params and notes each edited alone in the editor |
| `internal/app/selection.go` | Selecting text with the mouse in one pane, across scrolling, highlighting and copying it (OSC 52) |
| `internal/app/rebuild.go` | Notices on attach that the noct binary was rebuilt after the daemon started |
| `internal/app/prefix.go` | The configurable prefix key: parsing, matching, key names; capturing a new one and what it would take from shortcuts and programs |
| `internal/app/config_panel.go` | Config panel: rows, key handling, clicks and hover, applying and saving, reset to defaults, drawing |
| `internal/app/config_numbers.go` | Log limits typed by hand in the config: enter or a digit opens them, days and sizes in KB/MB/GB parsed, enter saves and esc gives up |
| `internal/app/panel_kinds.go` | The kinds of panel a key opens (`panelKinds`), which `runAction` and the new tab setting read; opening a new tab |
| `internal/app/panel.go` | Shared panel component: framed box, rows, list scrolling with a margin around the selection, centered and corner placement |
| `internal/app/tab.go` | A tab: its views, pane names, focus history, floating panes, fullscreen, visible frames, size rules |
| `internal/app/floating.go` | Floating panes: opening, keyboard moves and resizes, mouse drags, focus navigation over both layers |
| `internal/app/resize.go` | Resize mode (prefix+n): arrows grow or shrink the focused tile or floating pane |
| `internal/app/draw.go` | Frame drawing: panes (folded ones as a title row), the tab bar's status, mode overlays, cursor placement |
| `internal/app/tabbar.go` | Tab bar: names cut to fit, scrolling that keeps the active tab in view, marks for hidden tabs, which tab a click or the wheel selects |
| `internal/app/images.go` | Image support detection, outer image ids, cell size, forwarding to the terminal |
| `internal/app/environment.go` | Shell and environment given to pane programs |
| `internal/app/theme.go` | Predefined themes and the palette of styles derived from them |
| `internal/kittygfx/command.go` | Parse and encode kitty graphics commands |
| `internal/kittygfx/stream.go` | Split graphics commands out of program output, across reads |
| `internal/kittygfx/session.go` | Per-pane image state: id translation, uploads, sizing, replies |
| `internal/kittygfx/placeholder.go` | Placeholder cell text and color for an image cell |
| `internal/kittygfx/diacritics.go` | Kitty's fixed row/column diacritics table |
| `internal/config/config.go` | Settings file: defaults, validation, atomic load and save |
| `third_party/ultraviolet`, `third_party/vt` | Copies of charmbracelet/ultraviolet and x/vt with throughput and rendering fixes (`noct:` comments); see `third_party/README.md`. `noct_render_test.go` replays the renderer's output into the emulator |
| `docs/demo/record.sh`, `docs/demo/*.tape` | The README's GIFs (`make demo`): VHS tapes run noct in a home of its own, with the demo scripts and flows of `docs/demo/scripts` and `docs/demo/flows` |
| `docs/assets/` | The README's logo (`logo.svg`, the home's art as rectangles) and GIFs |
| `internal/script/script.go` | A script: load, validate (params and outputs), render its command line, create and delete |
| `internal/script/library.go` | The library: personal scripts, folders of scripts (in the scripts folder, linked, project), IDs, lookups |
| `internal/script/edit.go` | Creating a script (with optional hints) and updating it from the form's fields, keeping the rest of the YAML; params as YAML text |
| `internal/script/values.go` | Starting values of parameters, checking them, remembered values file |
| `internal/script/history.go` | The list of recent runs of scripts and flows (the last 1000), with exit code and log, and sorting scripts by name, edit or last run |
| `internal/flow/flow.go` | A flow: steps (script, run, if, repeat, wait, prompt, set, unset), loading, saving, creating, validation, template functions |
| `internal/flow/run.go` | Running a flow step by step in a terminal: commands, if, repeat, wait, captures (and scripts' outputs), expect; its last result with each step's output |
| `internal/flow/try.go` | Testing a range of sibling steps over the last result (its steps' fields and variables), and merging what they did into it |
| `internal/flow/vars.go` | Params with a type (text, number), required and a label, and checking values; running prompt, set and unset steps |
| `internal/flow/trigger.go` | Triggers (manual, cron, watch, after) in YAML, their validation, globs with `**`, loops of after triggers, which flows have them enabled |
| `internal/flow/cron.go` | Five-field cron schedules: parsing and matching a minute |
| `internal/flow/pane.go` | Steps in a pane of their own (`pane: new`, `background`): validation, asking noct's workspace, running inline outside it |
| `internal/mcp/server.go` | The MCP protocol: JSON-RPC over stdio, initialize, tools/list, tools/call, answers as text and structured content |
| `internal/mcp/tools.go` | Tools, their input schemas, the policy (allow, ask, deny) and the audit log around every call |
| `internal/mcp/library_tools.go` | Tools over scripts and flows: list, read, check, scope, drafts, triggers |
| `internal/mcp/run_tools.go` | Tools over runs and logs: list, the structured result, search, paged reading |
| `internal/mcp/workspace_tools.go` | Tools the running workspace does: panes, runs the user sees, command lines, typing |
| `internal/schedule/schedule.go` | Deciding when enabled flows start: cron minutes, watched files settling, another flow's run ending |
| `internal/flow/scope.go` | What templates can read at each point of a flow (parameters, variables set and unset, earlier steps' fields, how sure), and warnings for what they read that may not be there |
| `internal/flow/edit.go` | Editing a flow by step path: insert, replace, remove, move, unique ids |
| `internal/script/version.go` | Program version detection, in the background and cached |
