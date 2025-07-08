package ptr

// Ptr returns a pointer to the given value.
func Ptr[T any](v T) *T {
	return &v
}

func Float32(v float32) *float32 {
	return &v
}
