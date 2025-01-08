package array

type DiffResult[T comparable] struct {
	Added   []T
	Removed []T
}

func Diff[T comparable](old []T, new []T) DiffResult[T] {
	oldMap := make(map[T]struct{})
	newMap := make(map[T]struct{})

	for _, item := range old {
		oldMap[item] = struct{}{}
	}

	for _, item := range new {
		newMap[item] = struct{}{}
	}

	var result DiffResult[T]

	for item := range newMap {
		if _, found := oldMap[item]; !found {
			result.Added = append(result.Added, item)
		}
	}

	for item := range oldMap {
		if _, found := newMap[item]; !found {
			result.Removed = append(result.Removed, item)
		}
	}

	return result
}
