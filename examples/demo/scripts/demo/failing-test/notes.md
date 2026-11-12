# failing-test

Pretends to run a test that always fails. It only prints and sleeps. The
`failure` parameter picks how: `flaky` (a network timeout), `bug` (a wrong
total) or `env` (a missing setting).

The output never says which one it is. That is the point: `demo-triage` hands
it to an AI agent, which has to tell them apart by reading it, as you would.
