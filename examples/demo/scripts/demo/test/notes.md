# test

Pretends to run the test suite. It always prints `42 passed, 0 failed`.

## How the flow reads the count

`demo-deploy` does not read this output as plain text. It pulls the number out
with a `capture` and a regular expression:

```yaml
capture:
  passed: '([0-9]+) passed'
```

The first group of the match, `42`, becomes `{{.steps.test.passed}}`, which the
flow drops into its final summary. Change the printed line and the capture has
to change with it.

## Trying it

Run the script on its own from the scripts panel, or test just this step inside
the flow with `t` in the inspect.
