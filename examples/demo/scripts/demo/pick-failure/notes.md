# pick-failure

Picks how `demo-triage`'s test fails: `flaky`, `bug` or `env` as given, or one
of them at random. It prints the pick, and `demo-triage` captures it as
`{{.steps.pick.kind}}` to check the agent's answer at the end.

The agent never reads this step's output, only the test's: it has to tell the
failure from the log alone.
