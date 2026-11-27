package app

import (
	"testing"

	"go.yaml.in/yaml/v3"

	"github.com/AlexandreSJ/noct/internal/flow"
)

func TestTheScopeMasksASecretsValue(t *testing.T) {
	var f flow.Flow
	err := yaml.Unmarshal([]byte(`variables:
  user: {default: me}
  password: {default: hunter2, secret: true}
steps:
  - {prompt: 'pin:', var: pin, secret: true}
`), &f)
	if err != nil {
		t.Fatal(err)
	}
	r := &flow.Result{Steps: map[string]flow.StepResult{"0": {Captures: map[string]string{"pin": "1234"}}}}
	for v, want := range map[flow.Variable]string{
		{Field: "user"}:            "me",
		{Field: "password"}:        "••••",
		{Path: "0", Field: "pin"}:  "••••",
		{Path: "1", Field: "none"}: "",
	} {
		if got := variableValue(f, v, r); got != want {
			t.Errorf("%s shows %q, want %q", v.Name(), got, want)
		}
	}
}
