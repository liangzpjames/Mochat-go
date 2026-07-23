package domain

import (
	"errors"
	"strings"
)

type ModuleName struct {
	value string
}

func NewModuleName(value string) (ModuleName, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return ModuleName{}, errors.New("module name is required")
	}
	return ModuleName{value: value}, nil
}

func (n ModuleName) String() string {
	return n.value
}

type Module struct {
	ID   string
	Name ModuleName
}
