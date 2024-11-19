package array

func Filter[T any](arr []T, fn func(T) bool) (result []T) {
	for _, v := range arr {
		if fn(v) {
			result = append(result, v)
		}
	}
	return result
}
