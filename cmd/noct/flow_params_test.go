package main

import (
	"testing"

	"github.com/AlexandreSJ/noct/internal/flow"
)

func TestARequiredFlowParamCanComeFromTheEnvironment(t *testing.T) {
	f := flow.Flow{Name: "ship", Params: map[string]flow.Param{"token": {From: "env:NOCT_TEST_TOKEN", Required: true}}}
	t.Setenv("NOCT_TEST_TOKEN", "")
	if _, err := flowParams(f, "ship", nil); err == nil {
		t.Error("ran with the required token neither given nor in the environment")
	}
	t.Setenv("NOCT_TEST_TOKEN", "s3cret")
	if params, err := flowParams(f, "ship", nil); err != nil || len(params) != 0 {
		t.Errorf("with the token in the environment: params %v, err %v", params, err)
	}
}
