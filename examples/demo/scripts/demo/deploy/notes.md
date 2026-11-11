# deploy

Pretends to push the built `api` to an environment. Prints and sleeps only.

## Parameters

`env` and `version` have defaults. In `demo-deploy` the step fills them from the flow:

```yaml
with:
  env: '{{.env}}'
  version: '{{.version}}'
```

## A token from the environment

`token` has `from: env:DEMO_DEPLOY_TOKEN`. When that variable is set in your
shell, noct takes the token from it and does not ask:

```sh
export DEMO_DEPLOY_TOKEN=abc123
```

Unset, the form asks for it like any other parameter. `secret: true` masks it
while typed, never remembers it, and shows it as `••••` in the command line under
the form. The script only prints how long the token is.

## It runs in its own pane

In the flow, this step has `pane: new`, so it opens a pane beside the run with a
real terminal, like starting a long-running program. The flow waits for it and
uses its exit code. Because the output does not pass back through the flow, a
`pane: new` step cannot `capture` or `expect` anything.
