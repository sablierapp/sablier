package arrays

// RemoveElements returns a new slice containing all elements from `allElements` that are not in `elementsToRemove`
func RemoveElements(allElements, elementsToRemove []string) []string {
	// Create a map to store elements to remove for quick lookup
	removeMap := make(map[string]struct{}, len(elementsToRemove))
	for _, elem := range elementsToRemove {
		removeMap[elem] = struct{}{}
	}

	// Create a slice to store the result
	result := make([]string, 0, len(allElements)) // Preallocate memory based on the size of allElements
	for _, elem := range allElements {
		// Check if the element is not in the removeMap
		if _, found := removeMap[elem]; !found {
			result = append(result, elem)
		}
	}

	return result
}
