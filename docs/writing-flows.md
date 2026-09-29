# Writing flows

How an agent writes a flow (and the scripts it calls) by hand, as YAML files. The design and the
reasons behind it are in [fluxos.md](fluxos.md) and [scripts.md](scripts.md); the schema is
`internal/flow/flow.go` and `internal/script/script.go`. When they disagree, the code wins.

## Where the files go

| What | Path | Name used in YAML |
|---|---|---|
| Flow | `~/.local/share/noct/flows/<name>.yaml` | `<name>` (from the file name) |
| Script | `~/.local/share/noct/scripts/<name>/script.yaml` | `<name>` |
| Script in a folder | `~/.local/share/noct/scripts/<folder>/<name>/script.yaml` | `<folder>/<name>` |
| Script notes (optional) | `notes.md` beside `script.yaml` | — |

`$XDG_DATA_HOME` replaces `~/.local/share` when set. Flow names and ids use letters, digits, `.`,
`-` and `_`; step ids, params and capture names use letters, digits and `_` only.

## A script a flow can use

```yaml
description: Rolls a die, a random number from 1 to 6
run: |
  n=$(shuf -i 1-6 -n 1)
  echo "rolled a $n"
  echo "number=$n"
params:
  - name: sides        # read as {{.sides}} in run
    prompt: Sides
    default: "6"
outputs: [number]      # the first line number=... becomes .steps.<id>.number
```

- `run` is a shell text; `{{.param}}` is replaced before it runs.
- Give every param a `default`, or have every flow step that calls the script set it in `with`.
  A step runs without asking, so a param with `choices` and no value fails.
- To pass a multi-line value (a list of files, another step's output), read it through a quoted
  heredoc so quotes and `$` in it are not interpreted:

  ```sh
  cat <<'NOCT_ENTRIES' | while IFS= read -r entry; do ...; done
  {{.entries}}
  NOCT_ENTRIES
  ```

## A flow

```yaml
description: Rolls until a six, then deploys the version given
params:                 # defaults; `noct flow run <flow> version=2.0` overrides them
  version: "1.0.0"

steps:
  - id: roll
    repeat:
      until: '{{eq .steps.dice.number "6"}}'
      max: 10           # required with until: a loop never runs forever
      every: 1s         # optional pause between passes
    steps:
      - id: dice
        script: tests-noct/dice

  - id: build
    run: echo "building {{.version}}"; echo "artifact=app-{{.version}}.tar"
    capture:
      artifact: '^artifact=(\S+)$'   # regex per line; group 1 (or the whole match) is the value
    expect:
      exit: 0
      output_matches: 'building'

  - if: '{{.steps.build.ok}}'
    then:
      - id: ship
        script: tests-noct/deploy-sim
        with:
          env: staging
          version: '{{.version}}'
          token: dummy
        on_fail: continue            # stop (default), continue, or ask (asks retry/continue/stop)
    else:
      - run: echo "build failed"

  - wait: 2s

  - run: echo "done with {{.steps.build.artifact}}"   # not .steps.ship: it may not have run
```

### Step kinds

A step sets exactly one of these:

| Key | Does | Extra keys it may have |
|---|---|---|
| `script: <id>` | Runs a registered script | `with`, `capture`, `expect`, `on_fail`, `pane`, `background` |
| `run: <shell>` | Runs a shell line (a template) | `capture`, `expect`, `on_fail`, `pane`, `background` |
| `if: <template>` | Runs `then` or `else` | `then`, `else` |
| `repeat: {...}` | Runs `steps` in passes | `steps` |
| `wait: <duration>` | Sleeps (`5s`, `1m30s`) | — |
| `prompt: <template>` | Asks the user on the run's terminal; the answer goes to a variable | `var` (required), `default`, `type` |
| `set: {name: <template>}` | Gives variables, new or not, a value | — |
| `unset: [name, steps.<id>]` | Clears variables and steps' results | — |

`id` is optional on any step; give one to every step something later reads.

### Params and variables

```yaml
params:
  env: staging                    # a default alone
  a: {type: number, required: true, prompt: First number}
steps:
  - id: add
    run: awk 'BEGIN { print {{.a}} + 1 }'
  - set: {sum: '{{.steps.add.output}}'}   # {{.sum}} from here on
  - prompt: New a (enter keeps it)
    var: a                        # a new variable, or one to change (a param too)
    default: '{{.a}}'             # what an empty answer keeps
    type: number                  # asks again until the answer is a number, 3 times at most
  - unset: [sum, steps.add]       # reads as empty after this
```

- A param is its default alone, or a mapping with `default`, `type` (`text` or `number`),
  `prompt` (the label in the form) and `required`. `noct flow run` and the form refuse a
  `number` that is not one and a `required` param left empty.
- Params, `set` and `prompt` share one namespace: `{{.name}}`. `steps` is not a valid name.
- `set` renders all its values first, then sets them, so `set: {a: '{{.b}}', b: '{{.a}}'}` swaps.
- A `prompt` needs someone at the terminal. In noct's workspace it brings the run's pane to the
  screen before asking; a run nobody watches (a trigger) fails at the prompt instead of waiting.
- In the inspect, `p` opens the params form: enter checks the values, remembers them and runs.
  `r` runs with the values remembered, and opens the form when a required param is empty.

### Repeat

- `times: N` runs N passes; `until: <template>` stops when it gives `true` and then `max` is
  required. Reaching `max` without `until` becoming true is a failure.
- `until` is checked after each pass, so it can read steps inside the repeat.
- Each pass forgets the previous pass's inner steps: a step skipped in this pass reads as empty.

### Templates

Go `text/template`, rendered in `run`, `with` values, `if`, `until`, `prompt`, `default` and `set`
values.

| Reads | Value |
|---|---|
| `{{.<name>}}` | A flow param, or a variable a `set` or `prompt` gave |
| `{{.steps.<id>.output}}` | Output without colors, trailing newlines removed |
| `{{.steps.<id>.exit}}` | Exit code, as a string: `"0"` |
| `{{.steps.<id>.ok}}` | `"true"` or `"false"` |
| `{{.steps.<id>.<name>}}` | A capture, or an output the script declares |

- **Every value is a string.** Compare with strings: `{{eq .steps.x.exit "0"}}`, not `0`.
  `gt`/`lt` compare text, so `"10" < "9"`; do number comparisons inside a `run` step instead.
- `if` and `until` must render to `true`, `false` or empty (false); anything else fails the step.
- Functions: Go's built-ins (`eq`, `ne`, `and`, `or`, `not`, `gt`, `lt`, `printf`, ...) and
  `match`: `{{match .steps.mail.output "sent"}}`.
- A missing key renders empty, so a typo does not fail; `noct check` warns about it.

### Scope

A template sees the params, the variables set before it, and the steps with an `id` that ran
before it, in its own list and in the lists around it; an `unset` before it takes them away.
Steps inside an earlier `if` may not have run (`noct check` warns), and neither may what they set. Inside a
repeat, inner steps hold only the current pass. Write the flow so each read has a step that surely
ran before it, or accept the warning on purpose.

### When a step fails

`on_fail` is `stop` (default), `continue`, or `ask`. `ask` asks on the run's terminal: retry,
continue or stop. A run with no terminal to ask (started by a trigger) stops, so do not rely on
`ask` in flows that run by themselves.

### Steps in their own pane

```yaml
  - id: server
    run: npm run dev
    pane: new            # a pane beside the run, with a real terminal
    background: true     # the flow goes on without waiting; needs pane: new
```

A `pane: new` step without `background` waits for the program and uses its exit code. Its
output does not pass through the flow, so it cannot have `capture` or `expect`, and later steps
read no `.output` from it. Outside noct's workspace (`noct flow run` in a shell) the step runs in
that terminal instead.

### Triggers

```yaml
triggers:
  - manual                                        # also run by hand; starts nothing
  - cron: "0 9 * * 1-5"                           # minute hour day month weekday
  - watch: {paths: ["src/**/*.go"], debounce: 2s} # globs; ** is any folders
  - after: {flow: deploy, status: ok}             # status: ok, failed, or left out for either
steps: [...]
```

- Triggers do nothing until enabled on the machine: `noct enable <flow>` (or `o` in the inspect).
  Writing them in the file is not enough, on purpose; tell the user to enable the flow.
- The running workspace starts them, even with the TUI detached; nothing runs after `noct quit`.
- `watch` paths are relative to where the workspace started; prefer absolute or `~/` paths.
- A run started by a trigger has no terminal: no `ask`, and its output goes to a log.
- A flow cannot follow itself, and flows following each other in a loop are an error.

## Drafts

A flow or script written through noct's MCP tools (`draft_flow`, `draft_script`) is saved with
`draft: true`: it does not run by hand, by trigger or as a step until the user reviews it and
promotes it (`P` in the inspect or scripts panel, or `noct promote <name>`). Tell the user what
you drafted and ask them to promote it; do not edit the file to drop the mark.

## Check and run

```sh
noct check                                  # schema, templates, scripts, max, scope warnings
noct flow ls                                # the flows, whether they load, their triggers on/off
noct flow run <flow> [name=value ...]       # runs it in this terminal
noct flow run <flow> --steps 2..3           # tests a range, reading the last run's values
noct enable <flow>                          # lets its triggers start it; noct disable turns them off
```

A flow is done when `noct check` reports no problem and no unintended warning for it, and
`noct flow run` ends as expected. The last run's result is in
`~/.local/state/noct/flows/<flow>.json`: status, exit, captures and output of each step. Each run
started from noct also keeps its result beside its log, which the logs view opens with `i`.
