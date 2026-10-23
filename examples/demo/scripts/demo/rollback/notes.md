# rollback

Pretends to undo a bad deploy. Prints and sleeps only.

## When the flow runs it

`demo-deploy` only reaches this script when the health check never returns to
`ok`. An `if` after the health loop decides:

```yaml
- if: '{{ne .health_status "ok"}}'
  then:
    - id: rollback
      script: demo/rollback
      ...
```

After rolling back, the flow exits with a non-zero code so the run is marked
failed. Run `demo-deploy` with `fail_health=yes` to see this whole branch.

## Changing the message

The `env` and `version` it prints come from the flow through `with`. Edit the
text in `run` with `e` in the scripts panel.
