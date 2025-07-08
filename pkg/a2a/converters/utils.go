package converters

import "time"

// stringPtr creates a pointer to a string value.
func stringPtr(s string) *string {
	return &s
}

// Helper functions
func timePtr(t time.Time) *time.Time {
	return &t
}
