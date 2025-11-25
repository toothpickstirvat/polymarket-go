package polymarket

// Helper functions for pointer types

// BoolPtr returns a pointer to the given bool value
func BoolPtr(b bool) *bool {
	return &b
}

// IntPtr returns a pointer to the given int value
func IntPtr(i int) *int {
	return &i
}

// Float64Ptr returns a pointer to the given float64 value
func Float64Ptr(f float64) *float64 {
	return &f
}

// StringPtr returns a pointer to the given string value
func StringPtr(s string) *string {
	return &s
}
