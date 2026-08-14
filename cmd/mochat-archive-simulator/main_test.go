package main

import "testing"

func TestSimulationCLIRequiresExplicitEnableSwitch(t *testing.T) {
	if err := requireSimulationEnabled(false); err == nil {
		t.Fatal("simulation command unexpectedly enabled without explicit switch")
	}
	if err := requireSimulationEnabled(true); err != nil {
		t.Fatal(err)
	}
}
