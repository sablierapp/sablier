package array

func Map[T, V any](arr []T, fn func(T) V) []V {
	result := make([]V, len(arr))
	for i, t := range arr {
		result[i] = fn(t)
	}
	return result
}
