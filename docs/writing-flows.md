# Writing flows

How an agent writes a flow (and the scripts it calls) by hand, as YAML files. The design and the
reasons behind it are in [fluxos.md](fluxos.md) and [scripts.md](scripts.md); the schema is
`internal/flow/flow.go` and `internal/script/script.go`. When they disagree, the code wins.

## Where the files go

| What | Path | Name used in YAML |
|---|---|---|
| Flow | `~/.local/share/noct/flows/<name>.yaml` | `<name>` (from the file name) |
| Function | `~/.local/share/noct/functions/<name>.yaml` | `<name>`, in `call:` |
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

- `run` is a shell text; `{{.param}}` is replaced before it runs. Write `{{quote .param}}` for a
  value that may hold spaces, quotes or `;`: it becomes one shell word and never runs as a command.
- Give every param a `default`, or have every flow step that calls the script set it in `with`.
  A name in `with` that is not one of the script's params fails validation.
  A step runs without asking, so a param with `choices` and no value fails.
- To pass another step's output as an argument, write `{{quote .steps.<id>.output}}`. To read a
  multi-line value line by line (a list of files), use a quoted heredoc so quotes and `$` in it are
  not interpreted:

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
| `script: <id>` | Runs a registered script | `with`, `capture`, `expect`, `on_fail`, `timeout`, `pane`, `background` |
| `run: <shell>` | Runs a shell line (a template) | `capture`, `expect`, `on_fail`, `timeout`, `pane`, `background` |
| `call: <function>` | Runs a function, its params from `with` | `with` |
| `agent: {ask: <template>}` | Asks an AI agent; the answer goes to `{{.answer}}` | `use`, `choices` in `agent`; `capture`, `expect`, `on_fail`, `timeout` |
| `if: <template>` | Runs `then` or `else` | `then`, `else`, `export` |
| `repeat: {...}` | Runs `steps` in passes | `steps`, `export` |
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
  token: {from: 'env:DEPLOY_TOKEN', secret: true}   # the variable's value when set; asked when not
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

- A param is its default alone, or a mapping with `default`, `type` (`text`, `number`, `bool`
  for true or false, `choice` with its `choices: [a, b]`, or `path`, which tab completes in the
  form),
  `prompt` (the label in the form), `required`, `from` and `secret`. `noct flow run` and the
  form refuse a `number` that is not one and a `required` param left empty.
- `from: env:NAME` takes the value from the variable `NAME` when it is set, and the form does not
  ask for it; unset, the param is asked like any other. A value given as `name=value` wins.
  Typed in the form, it goes to the run as `NAME`, never on its command line. `secret: true`
  masks it in the form and never remembers it. Scripts have the same `from` (ADR-063).
- Params, `set` and `prompt` share one namespace: `{{.name}}`. `steps` is not a valid name.
- Each script or run step also sets its results as variables of their own names: `{{.output}}`,
  `{{.exit}}`, `{{.ok}}`, its captures and its script's outputs, which the next such step sets
  again. `{{.steps.<id>.<field>}}` still reads one step's.
- Ifs and repeats scope variables as blocks do in a language: setting a variable that exists
  before the block changes it; one the block makes goes when it ends, unless the block exports it:
  `export: {count: passes}` keeps `count` after the block as `{{.passes}}` (`{count: ""}` keeps
  its name). `steps.<id>` results are not scoped. An if's export is surely set when both `then`
  and `else` make it; with only one of them, `noct check` warns that it may not be set.
- `set` renders all its values first, then sets them, so `set: {a: '{{.b}}', b: '{{.a}}'}` swaps.
- A `prompt` needs someone at the terminal. In noct's workspace it brings the run's pane to the
  screen before asking; a run nobody watches (a trigger) fails at the prompt instead of waiting.
- In the inspect, `p` opens the params form: enter checks the values, remembers them and runs.
  `r` runs with the values remembered, and opens the form when a required param is empty.

### Functions

A function is steps any flow can call, in a file of its own. It has `params` and `steps` as a flow
does, and `returns`, templates read at its end:

```yaml
# ~/.local/share/noct/functions/health.yaml
params:
  url: {required: true}
steps:
  - {id: probe, run: 'curl -s -o /dev/null -w "%{http_code}" {{quote .url}}'}
returns:
  status: '{{.output}}'
```

```yaml
# in a flow
- id: check
  call: health
  with: {url: 'https://{{.env}}.example.com'}
- if: '{{eq .status "200"}}'     # or {{.steps.check.status}}
```

- The function runs in a scope of its own: it reads its params, not the flow's variables, and
  what it makes goes when it ends. The returns become the call's captures and short variables.
- Changing a function changes every flow that calls it; the inspect lists them on the function's
  chart (`used by …`), and `noct check` reports a call to a missing function, a `with` for a
  param it has not, or a required param left out. Functions can call functions, 16 deep at most.

### Agent steps

```yaml
- id: review
  agent:
    ask: 'The deploy health check printed {{quote .output}}. Ship or roll back?'
    use: claude               # optional: the config's agent (claude by default)
    choices: [ship, rollback] # optional: the answer must say one of them
- if: '{{eq .answer "rollback"}}'
  then: [{script: demo/rollback}]
```

- The question goes to the agent's command as its input, or as one argument in place of
  `{prompt}`, never through a shell. noct knows `claude` (`claude -p --model haiku --tools ""`,
  with no tools), `ollama` (`ollama run llama3.2`) and `opencode` (`opencode run --agent plan {prompt}`, its
  read-only agent); the config's `[agents]` adds others or changes these, and `agent` picks the
  default. noct does not install them. An agent of your own should not be able to act either: a
  question carries text from programs and other people, which could ask it to.
- `{{.answer}}` is the answer trimmed; with `choices`, the first choice it says as a word, in any
  case, and an answer that says none fails the step (`on_fail` applies). `{{.output}}` is all it
  printed.
- An answer is text an AI wrote from what the question carried: put it in a `run:` only through
  `{{quote .answer}}`.

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
- Functions: Go's built-ins (`eq`, `ne`, `and`, `or`, `not`, `gt`, `lt`, `printf`, ...),
  `match`: `{{match .steps.mail.output "sent"}}`, and `quote`, which makes a value one shell word:
  `run: grep -r {{quote .pattern}} .`. Scripts have `quote` too.
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

### Timeouts

```yaml
timeout: 1h               # the whole run; absent is no limit
steps:
  - run: ./backup.sh
    timeout: 10m          # this step only; script and run steps
    on_fail: continue
```

- A Go duration: `30s`, `10m`, `1h30m`. Zero, negative or unparsable values fail validation.
- A step past its timeout is stopped (SIGTERM, then SIGKILL 2 s later) and fails with
  `timed out after 10m` and exit `124`; then its `on_fail` applies as for any failure.
- The flow's timeout covers everything, `wait` and `repeat` pauses included: the running step is
  stopped and the run fails with `flow timed out after 1h`, whatever `on_fail` says. A `prompt`
  or `ask` waiting on the terminal is not interrupted.
- `pane: new` steps are stopped by the workspace; `background: true` steps cannot have one.
- Give every flow with a `cron`, `watch` or `after` trigger a timeout: while a triggered run is
  still going, its triggers do not start it again. `noct check` warns when there is none.

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
- `cron` follows the local clock. When it goes back an hour, a flow does not run again on a minute
  the clock already showed; when it jumps forward, the minutes skipped do not run that day.
- A flow cannot follow itself, and flows following each other in a loop are an error.

## Drafts

A flow or script written through noct's MCP tools (`draft_flow`, `draft_script`) is saved with
`draft: true`: it does not run by hand, by trigger or as a step until the user reviews it and
promotes it (`P` in the inspect or scripts panel, or `noct promote <name>`). Tell the user what
you drafted and ask them to promote it; do not edit the file to drop the mark.

## Check and run

```sh
noct check                                  # schema, templates, scripts, max, scope and timeout warnings
noct flow ls                                # the flows, whether they load, their triggers on/off
noct flow run <flow> [name=value ...]       # runs it in this terminal
noct flow run <flow> --steps 2..3           # tests a range, reading the last run's values
noct enable <flow>                          # lets its triggers start it; noct disable turns them off
```

A flow is done when `noct check` reports no problem and no unintended warning for it, and
`noct flow run` ends as expected. The last run's result is in
`~/.local/state/noct/flows/<flow>.json`: status, exit, captures and output of each step. Each run
started from noct also keeps its result beside its log, which the logs view opens with `i`.
