package domain

import "testing"

func TestNewModuleName(t *testing.T) {
	name, err := NewModuleName("  Customer Success  ")
	if err != nil {
		t.Fatal(err)
	}
	if got, want := name.String(), "Customer Success"; got != want {
		t.Fatalf("String() = %q, want %q", got, want)
	}
}

func TestNewModuleNameRejectsEmptyValue(t *testing.T) {
	if _, err := NewModuleName(" "); err == nil {
		t.Fatal("NewModuleName accepted an empty value")
	}
}
