# report

Prints the end of `demo-triage`: how the test really failed, how the agent
sorted it, whether that was right, and what the flow did about it. It only
prints.

`demo-triage` hands it text an AI wrote. The script reads every value through
`{{quote ...}}`, which makes each one a single shell word: whatever the agent
answered, it is printed, never run.
