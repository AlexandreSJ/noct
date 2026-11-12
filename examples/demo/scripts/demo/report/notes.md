# report

Prints the end of `demo-triage`: how the agent sorted the failure and what the
flow did about it. It only prints.

`demo-triage` hands it text an AI wrote. The script reads both values through
`{{quote ...}}`, which makes each one a single shell word: whatever the agent
answered, it is printed, never run.
