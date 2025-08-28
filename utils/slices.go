package utils

func MapSlice[T any, R any](slice []T, fn func(T) R) []R {
	var result []R
	for _, item := range slice {
		result = append(result, fn(item))
	}
	return result
}
