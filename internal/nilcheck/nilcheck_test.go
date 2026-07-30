package nilcheck

import "testing"

func TestIsNilHandlesEveryNilCapableKindWithoutPanicking(t *testing.T) {
	var (
		pointer  *int
		function func()
		channel  chan int
		mapping  map[string]int
		slice    []int
	)
	for name, value := range map[string]any{
		"nil interface": nil,
		"pointer":       pointer,
		"function":      function,
		"channel":       channel,
		"map":           mapping,
		"slice":         slice,
	} {
		t.Run(name, func(t *testing.T) {
			if !IsNil(value) {
				t.Fatalf("IsNil(%T) = false", value)
			}
		})
	}
}

func TestIsNilRejectsNonNilValuesWithoutReflectPanics(t *testing.T) {
	value := 41
	for name, candidate := range map[string]any{
		"integer":         41,
		"struct":          struct{}{},
		"pointer":         &value,
		"function":        func() {},
		"non-empty slice": []int{41},
	} {
		t.Run(name, func(t *testing.T) {
			if IsNil(candidate) {
				t.Fatalf("IsNil(%T) = true", candidate)
			}
		})
	}
}
