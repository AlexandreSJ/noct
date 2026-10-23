# deploy

Pretends to push the built `api` to an environment. Prints and sleeps only.

## Two parameters

`env` and `version` both have defaults, so the script runs on its own without
asking. In `demo-deploy` the step fills them from the flow:

```yaml
with:
  env: '{{.env}}'
  version: '{{.version}}'
```

## It runs in its own pane

In the flow, this step has `pane: new`, so it opens a pane beside the run with a
real terminal, like starting a long-running program. The flow waits for it and
uses its exit code. Because the output does not pass back through the flow, a
`pane: new` step cannot `capture` or `expect` anything.
