package vdir

import (
	"testing"
)

func TestHashFilename(t *testing.T) {
	testCases := []struct {
		input    string
		expected string
	}{
		// Test case 1: Empty string
		{
			input:    "",
			expected: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		},
		// Test case 2: Regular UID
		{
			input:    "example-event-uid@example.com",
			expected: "975837bf2870e3a0d5541605f3085b09b005ecb1002683b8f4d3d121865398b6",
		},
		// Test case 3: Already looks like a hash
		{
			input:    "90ddb49d3d7b7722b82c21f7197e4aa1",
			expected: "66dfc12f7542e29222b18048d439a05821a39202259016c1d44937f1844e8033",
		},
	}

	for i, tc := range testCases {
		result := HashFilename(tc.input)
		if result != tc.expected {
			t.Errorf("Test case %d failed: got %s, want %s", i+1, result, tc.expected)
		}
	}
}