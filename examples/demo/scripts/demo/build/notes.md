# build

Pretends to compile the `api` service. It only prints and sleeps.

## The version parameter

Change the default version where it says `default: "1.4.2"` in `script.yaml`
(open it with `e` in the scripts panel). When `demo-deploy` runs this script it
passes its own `version` down with `with: {version: ...}`, so the flow's value
wins over this default.

## How the flow uses it

The last line, `artifact=...`, is declared in `outputs`. That turns it into
`{{.steps.build.artifact}}` for later steps in a flow: a script hands a value
back by printing one `name=value` line and listing the name under `outputs`.
