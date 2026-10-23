# health

Stands in for a `/health` endpoint. It prints `status: ok`, or
`status: degraded` when its `fail` parameter is `yes`.

## Driving it from the flow

`demo-deploy` has a `fail_health` parameter and passes it straight through:

```yaml
with:
  fail: '{{.fail_health}}'
```

Run the flow with `fail_health=yes` to watch the health check never recover and
the rollback path take over.

## Read inside a loop

The flow runs this script inside a `repeat` that retries it until the status is
`ok`. The status is pulled out with a capture:

```yaml
capture:
  status: 'status: (\w+)'
```

So `{{.steps.check.status}}` is `ok` or `degraded` after each pass.
