package utils

func Copy[T comparable, T2 any](mp map[T]T2) map[T]T2 {
	cpy := make(map[T]T2)
	for k, v := range mp {
		cpy[k] = v
	}
	return cpy
}
