package arrays

import (
	"reflect"
	"testing"
)

func TestRemoveElements(t *testing.T) {
	tests := []struct {
		allElements      []string
		elementsToRemove []string
		expected         []string
	}{
		{[]string{"apple", "banana", "cherry", "date", "fig", "grape"}, []string{"banana", "date", "grape"}, []string{"apple", "cherry", "fig"}},
		{[]string{"apple", "banana", "cherry"}, []string{"date", "fig", "grape"}, []string{"apple", "banana", "cherry"}},             // No elements to remove are present
		{[]string{"apple", "banana", "cherry", "date"}, []string{}, []string{"apple", "banana", "cherry", "date"}},                   // No elements to remove
		{[]string{}, []string{"apple", "banana", "cherry"}, []string{}},                                                              // Empty allElements slice
		{[]string{"apple", "banana", "banana", "cherry", "cherry", "date"}, []string{"banana", "cherry"}, []string{"apple", "date"}}, // Duplicate elements in allElements
		{[]string{"apple", "apple", "apple", "apple"}, []string{"apple"}, []string{}},                                                // All elements are removed
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			result := RemoveElements(tt.allElements, tt.elementsToRemove)
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("RemoveElements(%v, %v) = %v; want %v", tt.allElements, tt.elementsToRemove, result, tt.expected)
			}
		})
	}
}
