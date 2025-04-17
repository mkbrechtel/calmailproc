package processor

// containsAny checks if the string contains any of the substrings
func containsAny(s string, substrings []string) bool {
	for _, sub := range substrings {
		if contains(s, sub) {
			return true
		}
	}
	return false
}

// contains checks if s contains substring
func contains(s, substring string) bool {
	for i := 0; i < len(s); i++ {
		if i+len(substring) <= len(s) && s[i:i+len(substring)] == substring {
			return true
		}
	}
	return false
}