package util

// LevenshteinDistance calculates the Levenshtein distance between two strings
// This implementation is Unicode-aware and works correctly with multi-byte characters
func LevenshteinDistance(s1, s2 string) int {
	// Convert strings to rune slices to handle Unicode properly
	r1 := []rune(s1)
	r2 := []rune(s2)

	if len(r1) == 0 {
		return len(r2)
	}
	if len(r2) == 0 {
		return len(r1)
	}

	// Create a 2D slice for dynamic programming
	dp := make([][]int, len(r1)+1)
	for i := range dp {
		dp[i] = make([]int, len(r2)+1)
	}

	// Initialize base cases
	for i := 0; i <= len(r1); i++ {
		dp[i][0] = i
	}
	for j := 0; j <= len(r2); j++ {
		dp[0][j] = j
	}

	// Fill the dp table
	for i := 1; i <= len(r1); i++ {
		for j := 1; j <= len(r2); j++ {
			if r1[i-1] == r2[j-1] {
				dp[i][j] = dp[i-1][j-1]
			} else {
				// MinOf should never fail here as we're passing 3 values
				minVal, _ := MinOf(dp[i-1][j], dp[i][j-1], dp[i-1][j-1])
				dp[i][j] = minVal + 1
			}
		}
	}

	return dp[len(r1)][len(r2)]
}

// Truncate truncates a string to the specified length
func Truncate(s string, length int) string {
	if len(s) <= length {
		return s
	}
	if length <= 3 {
		return "..."
	}
	return s[:length-3] + "..."
}

// Contains checks if a string slice contains a value
func Contains(slice []string, value string) bool {
	for _, v := range slice {
		if v == value {
			return true
		}
	}
	return false
}
