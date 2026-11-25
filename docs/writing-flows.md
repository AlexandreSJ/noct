# Writing flows

How an agent writes a flow (and the scripts it calls) by hand, as YAML files. The design and the
reasons behind it are in [fluxos.md](fluxos.md) and [scripts.md](scripts.md); the schema is
`internal/flow/flow.go` and `internal/script/script.go`. When they disagree, the code wins.

## Where the files go

| What | Path | Name used in YAML |
|---|---|---|
| Flow | `~/.local/share/noct/flows/<name>.yaml` | `<name>` (from the file name) |
| Flow in folders | `~/.local/share/noct/flows/<folder>/<name>.yaml` | `<folder>/<name>`, as `ops/deploy` |
| Function | `~/.local/share/noct/functions/<name>.yaml`, or in folders | `<name>` or `<folder>/<name>`, in `call:` |
| Script | `~/.local/share/noct/scripts/<name>/script.yaml` | `<name>` |
| Script in folders | `~/.local/share/noct/scripts/<folder>/<name>/script.yaml` | `<folder>/<name>`, as `ops/db/backup` |
| Script notes (optional) | `notes.md` beside `script.yaml` | — |

Folders nest at any depth. A flow, function or script inside them is named by its path,
`noct flow run ops/deploy`, and by its name alone while no other one has it (`call: greet`
finds `utils/greet`).

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
outputs: [number]      # the first line number=... becomes $number in the flow
```

- A script's `run` is a shell text; `{{.param}}` is replaced before it runs. Write
  `{{quote .param}}` for a value that may hold spaces, quotes or `;`: it becomes one shell word and
  never runs as a command. Scripts keep `{{ }}`; the `$name` of flows (see Templates) is not theirs.
- Give every param a `default`, or have every flow step that calls the script set it in `with`.
  A name in `with` that is not one of the script's params fails validation.
  A step runs without asking, so a param with `choices` and no value fails.
- In a flow's run line, `$output` passes the last step's output as one argument. To read a
  multi-line value line by line (a list of files), use a quoted heredoc so quotes and `$` in it are
  not interpreted, and write `{{.entries}}` there: a heredoc's lines are not shell words, so a
  `$entries` would come in quotes:

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
    run: echo "building $version"; echo "artifact=app-$version.tar"   # $version.tar: no field tar, so .tar is text
    capture:
      artifact: '^artifact=(\S+)$'   # regex per line; group 1 (or the whole match) is the value
    expect:
      exit: 0
      output_matches: 'building'

  - if: $build.ok
    then:
      - id: ship
        script: tests-noct/deploy-sim
        with:
          env: staging
          version: $version
          token: dummy
        on_fail: continue            # stop (default), continue, or ask (asks retry/continue/stop)
    else:
      - run: echo "build failed"

  - wait: 2s

  - run: echo "done with $build.artifact"   # not $ship: it may not have run
```

### Step kinds

A step sets exactly one of these:

| Key | Does | Extra keys it may have |
|---|---|---|
| `script: <id>` | Runs a registered script | `with`, `capture`, `expect`, `on_fail`, `retry`, `timeout`, `pane`, `background` |
| `run: <shell>` | Runs a shell line (a template) | `capture`, `expect`, `on_fail`, `retry`, `timeout`, `pane`, `background` |
| `call: <function>` | Runs a function, its params from `with` | `with` |
| `agent: {ask: <template>}` | Asks an AI agent; the answer goes to `$answer` | `use`, `choices` in `agent`; `capture`, `expect`, `on_fail`, `retry`, `timeout` |
| `if: <template>` | Runs `then` or `else` | `then`, `else`, `export` |
| `switch: <template>` | Runs the first case whose `is` holds the value, or `else` | `cases`, `else`, `export` |
| `repeat: {...}` | Runs `steps` in passes: `times`, `until`, or once per line of `each` | `steps`, `export` |
| `wait: <duration>` | Sleeps (`5s`, `1m30s`) | — |
| `prompt: <template>` | Asks the user on the run's terminal; the answer goes to a variable | `var` (required), `default`, `type` |
| `set: {name: <template>}` | Gives variables, new or not, a value | — |
| `unset: [name, steps.build]` | Clears variables and a step's results | — |

`id` is optional on any step; give one to every step something later reads.

### Params and variables

```yaml
params:
  env: staging                    # a default alone
  a: {type: number, required: true, prompt: First number}
  token: {from: 'env:DEPLOY_TOKEN', secret: true}   # the variable's value when set; asked when not
steps:
  - id: add
    run: echo $a | awk '{ print $1 + 1 }'   # $1 is awk's: noct knows no name 1
  - set: {sum: $add.output}       # $sum from here on
  - prompt: New a (enter keeps it)
    var: a                        # a new variable, or one to change (a param too)
    default: $a                   # what an empty answer keeps
    type: number                  # asks again until the answer is a number, 3 times at most
  - unset: [sum, steps.add]       # reads as empty after this
```

- A param is its default alone, or a mapping with `default`, `type` (`text`, `number`, `boolean`
  for true or false, `option` with its `choices: [a, b]`, `path`, which tab completes in the
  form, `list` or `object`), `prompt` (the label in the form), `required`, `from` and `secret`.
  `noct flow run` and the form refuse a value its type does not take and a `required` param left
  empty. Flows written with `bool` and `choice` still load, as `boolean` and `option`. Params
  keep the order they are written in, in the run form too; the inspect's params table moves them
  (`J`/`K`). The demo flow `demo-params` has one of each type.
- `list` and `object` hold JSON, lists and objects inside each other at any depth:
  `hosts: {type: list, default: '["a", "b"]'}`, `cfg: {type: object, default: '{"db": {"port": 5432}}'}`.
  Templates read inside them: `$cfg.db.port`, `{{index .hosts 0}}`, `{{range .hosts}}{{.}} {{end}}`;
  `$hosts` alone writes the compact JSON, one shell word in a run line. A
  repeat goes through a list with `each: '{{range .hosts}}{{println .}}{{end}}'`.
- A `default` is a template that reads the params written before it, so a param can be made of
  others, from the environment too:

  ```yaml
  params:
    user: {from: 'env:USER'}
    password: {secret: true}
    login:
      type: object
      default: '{"user": "$user", "pass": "$password", "again": $user}'
  steps:
    - run: echo $login.user
  ```

  In a `list` or `object` default, `$name` goes in as JSON: inside quotes with its quotes and `\`
  escaped, outside them as a JSON string, or a list or object as it is. `{{json .user}}` does the
  same inside `{{ }}`.
  Only the default renders: a value typed in the form or given as `name=value` is taken as it is.
- `from: env:NAME` takes the value from the variable `NAME` when it is set, and the form does not
  ask for it; unset, the param is asked like any other. A value given as `name=value` wins.
  Typed in the form, it goes to the run as `NAME`, never on its command line. `secret: true`
  masks it in the form and never remembers it. Scripts have the same `from` (ADR-063).
- Params, `set` and `prompt` share one namespace: `$name`. `steps` is not a valid name.
- Each script or run step also sets its results as variables of their own names: `$output`,
  `$exit`, `$ok`, its captures and its script's outputs, which the next such step sets
  again. `$build.output` reads the step whose `id` is `build`, for an earlier step's
  field a later one set again. When two steps give the same capture before a template that reads
  it short, `noct check` warns which one the short name reads (`scope: version is set by build and
  test; ...`); `output`, `exit`, `ok` and `answer` never warn, since the last one is the one meant.
- Ifs, switches and repeats scope variables as blocks do in a language: setting a variable that exists
  before the block changes it; one the block makes goes when it ends, unless the block exports it:
  `export: {count: passes}` keeps `count` after the block as `$passes` (`{count: ""}` keeps
  its name). `steps.build` results are not scoped. An if's export is surely set when both `then`
  and `else` make it, a switch's when every case and its `else` do; otherwise `noct check` warns
  that it may not be set.
- `set` renders all its values first, then sets them, so `set: {a: $b, b: $a}` swaps.
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
  - {id: probe, run: 'curl -s -o /dev/null -w "%{http_code}" $url'}
returns:
  status: $output
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
    ask: 'The deploy health check printed $output. Ship or roll back?'
    use: default              # the config's agent; or auto, or a name: claude, ollama...
                              # a template too, so $agent lets a param pick
    choices: [ship, rollback] # optional: the answer must say one of them
  timeout: 3m
- if: '{{eq .answer "rollback"}}'
  then: [{script: demo/rollback}]
```

- The question goes to the agent's command as one argument in place of `{prompt}`, or as its
  input when the command ends in `< {prompt}` (or has no `{prompt}`), never through a shell.
- `use: default`, or no `use` (or an empty one), asks the config's `agent`. Its default, `auto`,
  asks the first agent that is ready here: the config's `[agents]` first, then claude, codex,
  opencode, llm and ollama. `use: auto` asks that one whatever the config says. `noct agents`
  lists them, the ready ones and the default; with none ready, it says how to start with ollama,
  which runs on your machine for free. `noct agents default <name>` writes `agent = "<name>"`
  into the config. `default` and `auto` name no agent of `[agents]`.
- Write `use: default` for a flow others will run, so each person's own agent answers; a name
  fixes the agent, so the flow asks the same one everywhere.
- noct knows these, each set up so it cannot change anything:

  | Agent | Command |
  |---|---|
  | `claude` | `claude -p --model haiku --tools "" --strict-mcp-config < {prompt}` (no tools) |
  | `codex` | `codex exec --ephemeral --skip-git-repo-check --sandbox read-only - < {prompt}` |
  | `opencode` | `opencode run --agent plan {prompt}` (its read-only agent) |
  | `llm` | `llm < {prompt}` (only answers) |
  | `ollama` | `ollama run --hidethinking llama3.2 < {prompt}`; without llama3.2, the first model ollama has |

- `[agent_models]` in the config picks the model of each of these, and noct writes the command
  around it (`--model`, or `-m` for llm), so the flags stay noct's to keep current. The config's
  **default agent** (`prefix` + `,`, then `Enter` on it) picks it from a list, `m` on the agent:
  claude's names for its models, the ones ollama has and the ones opencode and llm list.
- `[agents]` adds an agent of your own by name, or replaces the command of one of these, which then
  runs as written. Check its flags in its own docs:

  ```toml
  agent = "ollama"                 # or auto, or any name below
  [agent_models]
  ollama = "qwen3.5:4b"
  claude = "sonnet"
  [agents]
  hermes = "hermes chat --oneshot -Q -t todo -q {prompt}"  # -t todo: no tool that acts
  ```

  An agent of your own should not be able to act: a question carries text from programs and
  other people, which could ask it to. That includes MCP servers in its own config, which can
  give it tools; noct's own tools that act still ask you first. noct does not install any of them.
- In the inspect, `a` then `a` adds an agent step from one form in place of the chart: the
  question (`$` completes a variable), the answers, the agent and the timeout; `enter` adds it
  from any row.
- `$answer` is the answer trimmed, without the thinking a model prints first (ollama's
  `Thinking...` and `<think>` tags); with `choices`, the first choice it says as a word, in any
  case, and an answer that says none fails the step (`on_fail` applies). `$output` is all it
  printed.
- An answer is text an AI wrote from what the question carried: put it in a `run:` only as
  `$answer`, which stays one word or inside its quotes, never inside `{{ }}` unquoted.

### Switch

```yaml
- switch: $answer
  cases:
    - is: flaky                  # one value alone
      steps: [{script: tests}]
    - is: [bug, regression]      # or several
      steps: [{call: explain}]
  else:                          # no case holds the value; may be left out
    - prompt: What is it?
      var: kind
```

- The template is rendered and trimmed; the first case with an equal value (trimmed too) runs, else
  `else`. Values compare as text, exactly: `Bug` is not `bug`.
- A value in two cases is refused, since only the first would ever run. `is: ""` matches an empty
  value.
- After an agent step with `choices`, a case for each choice reads its answer.
- In paths, a case's steps are `<switch>/case0/<n>`, `case1`, ...; the else's, `<switch>/else/<n>`.

### Repeat

- `times: N` runs N passes; `until: <template>` stops when it gives `true` and then `max` is
  required. Reaching `max` without `until` becoming true is a failure.
- `each: <template>` runs one pass for each line of the text, trimmed, skipping empty lines, with
  the line in `$item`, or in the variable `as` names: `{each: $ls.output, as:
  file}`. That variable is the repeat's own: one of the same name outside is the same after it.
  `until` may end it early and `max` caps the passes; no lines is no pass, not a failure.
- `until` is checked after each pass, so it can read steps inside the repeat.
- Each pass forgets the previous pass's inner steps: a step skipped in this pass reads as empty.

### Templates

Rendered in `run`, `with` values, `if`, `switch`, `until`, `each`, `prompt`, `default` and `set`
values, function `returns`, and in params' `default`. A template reads a value as `$name`, and
logic is Go's `text/template` inside `{{ }}`.

| Reads | Value |
|---|---|
| `$<name>` | A flow param, or a variable a `set` or `prompt` gave |
| `$output` | The last step's output without colors, trailing newlines removed |
| `$exit` | The last step's exit code, as a string: `"0"` |
| `$ok` | `"true"` or `"false"` |
| `$<capture>` | A capture, or an output the script declares |
| `$build.output` | Any of the above for the step whose `id` is `build` |
| `$cfg.db.port` | A field of an `object` param, at any depth |

- **Only the flow's names are its own.** `$name` is a template when the flow knows `name`: a
  param, a variable a step sets, prompts, captures or exports, `output`, `exit`, `ok`, `answer`,
  or a step's id. Anything else stays as it is, so the shell keeps `$HOME`, `$PATH` and `$1`.
  `${name}` and `$$name` (written `$name` to the shell) are always the shell's, and so is a name
  the run line sets itself: in `for f in *; do echo $f; done` and `n=3; echo $n`, `$f` and `$n`
  are the shell's even when the flow has an `f` or an `n`.
- **A name ends** at the first character that is not a letter, digit or `_`, or a `.` no name
  follows: `$version.tar` reads `version` and then the text `.tar`; `$obj.pass` reads a field.
  Where a name runs into text, write `{{.version}}x`.
- **In a run line each `$name` is safe:** outside quotes it is one shell word; inside `"..."`
  or `'...'` it is escaped to stay inside them, so `echo $msg` and `echo "msg: $msg"` never run
  what the value holds. A program that reads its own text (`sh -c '...'`, `jq '...'`) still reads
  the value as its own code: pass it as an argument instead. Inside a heredoc, write `{{.name}}`.
- A name that is both a variable and a step's id cannot be read with a field (`$build.output`):
  `noct check` says so; rename one, or write `{{.steps.build.output}}`.
- **Every value is a string**, but for `list` and `object` params. Compare with strings:
  `{{eq .exit "0"}}`, not `0`. `gt`/`lt` compare text, so `"10" < "9"`; do number comparisons
  inside a `run` step instead.
- `if` and `until` must render to `true`, `false` or empty (false); anything else fails the step:
  `if: $build.ok`, `until: '{{eq .status "ready"}}'`.
- Inside `{{ }}`, names are `.name` and `.steps.build.output`. Functions: Go's built-ins (`eq`,
  `ne`, `and`, `or`, `not`, `gt`, `lt`, `printf`, `len`, ...), `match`:
  `{{match .steps.mail.output "sent"}}`, `quote`, which makes a value one shell word, and `json`,
  which puts it in JSON.
- A missing key renders empty, so a typo does not fail; `noct check` warns about it.

### Scope

A template sees the params, the variables set before it, and the steps with an `id` that ran
before it, in its own list and in the lists around it; an `unset` before it takes them away.
Steps inside an earlier `if` may not have run (`noct check` warns), and neither may what they set. Inside a
repeat, inner steps hold only the current pass. Write the flow so each read has a step that surely
ran before it, or accept the warning on purpose.

### When a step fails

```yaml
- run: curl -fsS https://example.com/health
  retry: {max: 3, every: 10s}   # or retry: 3, which does not wait
  on_fail: continue
```

- `retry` runs a failed script, run or agent step again, at most `max` more times, waiting `every`
  before each try. Only after the last try fails does `on_fail` apply. The flow's timeout ends the
  tries too.
- `on_fail` is `stop` (default), `continue`, or `ask`. `ask` asks on the run's terminal: retry,
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
