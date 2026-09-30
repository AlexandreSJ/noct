<p align="center">
  <img src="docs/assets/logo.svg" alt="noct" width="260">
</p>

<p align="center">
  <strong>A terminal workspace for people who test.</strong><br>
  noct runs your shells, your AI agent and your repetitive test routines side by side, in one lightweight binary.
</p>

<p align="center">
  <a href="go.mod"><img src="https://img.shields.io/badge/go-1.24%2B-00ADD8?logo=go&logoColor=white" alt="Go 1.24+"></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/license-Apache%202.0-blue" alt="License: Apache 2.0"></a>
  <img src="https://img.shields.io/badge/platform-linux%20%7C%20macos-lightgrey" alt="Linux | macOS">
</p>

<p align="center">
  <strong>English</strong> · <a href="README.pt-br.md">Português</a>
</p>

<p align="center">
  <a href="#why-noct">Why noct</a> · <a href="#install">Install</a> · <a href="#getting-started">Getting started</a> · <a href="#scripts">Scripts</a> · <a href="#flows">Flows</a> · <a href="#ai">AI</a> · <a href="#sessions">Sessions</a> · <a href="#documentation">Docs</a>
</p>

<p align="center">
  <img src="docs/assets/hero.gif" alt="noct: a terminal tab, a script run from the scripts panel and a flow running" width="880">
</p>

> [!NOTE]
> **Early development.** Terminals, scripts, flows and the AI integration work today; expect changes before a first release.

## Why noct

Testing is repetitive, but never exactly the same: the environment changes, the user changes, the input changes. A fixed script doesn't fit, and retyping commands is slow and error-prone. The knowledge of *how* to test ends up scattered across shell history, loose files and memory.

noct keeps it all in one place:

- **Real terminals, your setup.** Every pane runs your own shell with your own config. Plugins, prompts and completions work as they always do.
- **Unlimited tabs and panes.** Keep a server running, follow a log, talk to an AI agent and still have a free shell, all in the same window.
- **Scripts with notes.** Save the command lines you use, with notes on how and when to run them, their version, and help one key away.
- **Flows without scripts.** Chain scripts in a visual editor with conditions, loops, delays, variables and prompts that ask you for a value mid-run; typed params are filled in a form (`p` in the inspect). Every run is kept, and a calendar shows when it ran and its full log.
- **AI that works with you.** Through MCP, an AI agent can read your scripts, draft flows, run them and analyze the results, and nothing new runs without your approval.
- **Lightweight.** A single Go binary with no runtime to install.

## Install

Requires Go 1.24 or later, on Linux or macOS.

```sh
go install github.com/AlexandreSJ/noct/cmd/noct@latest
```

Or build from source:

```sh
git clone https://github.com/AlexandreSJ/noct
cd noct
go build -o noct ./cmd/noct
```

## Getting started

Run `noct`. It opens on an empty home screen; press any key to see the prefix key that opens the commands.

Press the prefix key, `Ctrl+Space`, to enter **noct mode**. In noct mode, moving around keeps the mode on, so you can hop across panes and tabs with a single prefix:

| Key | Action |
|---|---|
| Arrows or `hjkl` | Move between panes; past the edge, into the next or previous tab |
| `Tab` / `Shift+Tab` | Change tab |
| `t` | New tab with a terminal |
| `p` | New terminal panel: splits the current pane, or opens a tab on the home screen |
| `s` | New scripts panel: your saved scripts, to run, edit and create |
| `i` | Inspect: your flows as flowcharts, to build step by step, run and review |
| `v` | Logs by day on a calendar colored by activity, with their size, lines and how each run ended |
| `Space` | Search scripts, actions and tabs |
| `?` | Help for the program running in the terminal |
| `a` | Move the pane with the arrows, even into another tab |
| `n` | Resize the pane with the arrows |
| `w` | Stack the panes of the focused group, as in zellij: the focused one takes the height, the others show as a title row each; again to split them |
| `f` | Fullscreen: show only the focused pane, or the layout again |
| `g` | New floating panel with a terminal, over the others; drag its border with the left button to move it, with the right one to resize it |
| `r` | Rename the tab |
| `e` | Rename the pane; an empty name gives back the program's |
| `[` | Scroll the terminal's history |
| `d` | Detach; programs keep running |
| `x` | Close the pane; a tab closes with its last pane |
| `c` | Config: prefix key, direct shortcuts, theme, background, mouse, editor, file manager, scripts folder, how long and how much of the logs to keep, how much step output flows keep |
| `q` | Quit, after asking: `Enter` or `q` again |
| `Esc` | Cancel |

A popup lists these keys while noct mode is on. Wherever noct moves something and you are not typing, `hjkl` work like the arrows.

Outside noct mode, every key goes straight to the program in the pane, except the **direct shortcuts**: `Alt` with a noct mode key runs that command without the prefix (`Alt+→` moves the focus, `Alt+T` opens a tab, `Alt+A` moves the pane), and `Alt+Shift` with the arrows moves the pane directly. This takes `Alt+F` and `Alt+D` from the shell's word editing; the modifier can be switched to `Ctrl+Alt` or turned off in the config.

Panes tile like i3 with autotiling: a new pane opens beside the focused one when it is wider than tall, below it otherwise. Moving a pane (`a`, or `Alt+Shift` with the arrows) follows i3's move: it trades places within its group, enters a neighbor group, or leaves its group to land beside it.

The mouse works too: click a tab or a pane to select it, click a command in the prefix popup to run it, drag the line between two panes to resize them, scroll a terminal's history with the wheel, and drag over text to select it; releasing copies it to the clipboard. Programs that use the mouse, like vim and htop, get the clicks.

## Scripts

A script is a command line with parameters, saved in `~/.local/share/noct/scripts/<name>/script.yaml` (or in a project's `.noct/scripts/`) with a `notes.md` beside it:

```yaml
description: Log in to the API
program: curl
run: ./login.sh {{.env}} {{.user}}
params:
  - name: env
    choices: [dev, staging, prod]
  - name: user
    remember: true
```

Scripts can be grouped in folders inside the scripts folder, and any other folder of scripts can be added by its path (`a` in the scripts panel); folders come first in the list, closed (`+`) until `Enter` opens them (`-`), and `n` asks whether to create a directory or a script. A script in a folder is `folder/name`.

In the scripts panel, `Enter` asks for the parameters and runs the script in a terminal beside it, which stays open with the exit code; `r` in the run pane runs it again. In the list, `e` opens a script's form, where `Enter` edits each part (command line, description, parameters, notes) alone in your editor (vim by default) and `Ctrl+S` saves; `n` creates one and `Ctrl+O` opens the scripts folder. The scripts folder, the editor and the file manager are set in the config. The full format is in [docs/scripts.md](docs/scripts.md) (Portuguese).

## Flows

A flow chains scripts into a test routine, saved in `~/.local/share/noct/flows/<name>.yaml`. `Ctrl+Space` `i` opens the inspect, where each flow is a flowchart you build step by step: run a script, `if`/`else`, `repeat` until a condition, wait, set a variable, or prompt for a value mid-run. Every run is kept with each step's output, and `Ctrl+Space` `v` shows them on a calendar.

Triggers start a flow on their own while the workspace runs: on a cron schedule, when files change, or after another flow ends. `noct enable <flow>` turns them on. Writing a flow by hand is covered in [docs/writing-flows.md](docs/writing-flows.md).

## AI

`noct mcp` serves noct to an AI client as an MCP server, giving it your scripts, flows, runs and panes. For Claude Code:

```sh
claude mcp add noct -- noct mcp
```

`Ctrl+Space` `y` shows the command for other clients (Codex, Gemini CLI, VS Code, opencode, Cursor) and the clients connected now. What an AI writes is a **draft** that doesn't run until you review it and promote it (`P` in the inspect or scripts panel, or `noct promote <name>`), and commands it asks to run wait for your approval in the same popup. See [docs/mcp.md](docs/mcp.md) (Portuguese).

## Command line

| Command | What it does |
|---|---|
| `noct` | Attach to the session used last, starting noct if needed |
| `noct attach <session>` (or `a`, `att`, `-a`) | Attach to a session by name |
| `noct new [session]` | Start a session, named or numbered, and attach to it |
| `noct sessions` | List the sessions: open or closed, tabs, last use |
| `noct kill <session>` | Delete a session: its programs stop and its layout is forgotten |
| `noct -s <script> [name=value ...]` | Run a script in this terminal |
| `noct ls` | List the scripts, their program version and folder |
| `noct flow run <flow> [name=value ...]` | Run a flow in this terminal, step by step |
| `noct flow ls` | List the flows, with their triggers on or off |
| `noct enable <flow>` / `disable <flow>` | Turn a flow's triggers on or off |
| `noct promote <flow or script>` | Let a draft an AI wrote run, once reviewed |
| `noct mcp` | Serve noct to an AI client over stdin and stdout |
| `noct check` | Report the scripts and flows that cannot be loaded |
| `noct quit` | Close every pane and stop the workspace; sessions open again as they were |

`noct -h` shows every option.

## Sessions

Your programs keep running when you close the terminal or detach with `Ctrl+Space` `d`: run `noct` again to get back to them.

Keep separate workspaces as named sessions: `noct new dev` starts one, `noct attach dev` comes back to it, and `Ctrl+Space` `o` lists them all with when each was last used, to switch, create, rename, close or delete them. Plain `noct` returns to the session you used last.

A closed session, or every session after `noct quit`, keeps its tabs and panes: opening it again starts its shells in the directories they were in and its noct panels where they were. Programs running inside the shells, and script runs, do not come back.

The daemon keeps running the version it started with. After updating or rebuilding noct, quit it (`Ctrl+Space` `q`) and run `noct` again; noct reminds you in the tab bar when it notices a newer binary.

## Documentation

The design docs are in Portuguese; [docs/README.md](docs/README.md) indexes them.

| Doc | About |
|---|---|
| [writing-flows.md](docs/writing-flows.md) | Writing flows and scripts in YAML by hand (English) |
| [scripts.md](docs/scripts.md) | Script format, parameters, notes and the CLI |
| [fluxos.md](docs/fluxos.md) | Flows, triggers and the calendar |
| [mcp.md](docs/mcp.md) | The tools given to an AI and the approval policy |
| [tui.md](docs/tui.md) | Tabs, panes, keys and look |
| [roadmap.md](docs/roadmap.md) | Phases and what is done |
| [CONTRIBUTING.md](CONTRIBUTING.md) | Code and scope rules for people and AI agents |

## License

[Apache 2.0](LICENSE) © [AlexandreSJ](https://github.com/AlexandreSJ)

noct's terminal emulation and rendering are built on [Ultraviolet](https://github.com/charmbracelet/ultraviolet) and [x/vt](https://github.com/charmbracelet/x/tree/main/vt) by [Charm](https://charm.land). Patched copies live in [third_party/](third_party/README.md) under their original MIT licenses; see [NOTICE](NOTICE).
