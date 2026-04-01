package util

// DamerauLevenshteinDistance calculates the restricted Damerau-Levenshtein distance
// (Optimal String Alignment) between two strings.
// Supports insertion, deletion, substitution, and transposition of adjacent characters.
// This implementation is Unicode-aware and works correctly with multi-byte characters.
func DamerauLevenshteinDistance(s1, s2 string) int {
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
			cost := 1
			if r1[i-1] == r2[j-1] {
				cost = 0
			}

			// MinOf should never fail here as we're passing 3 values
			minVal, _ := MinOf(dp[i-1][j]+1, dp[i][j-1]+1, dp[i-1][j-1]+cost)
			dp[i][j] = minVal

			// Transposition of adjacent characters
			if i > 1 && j > 1 && r1[i-1] == r2[j-2] && r1[i-2] == r2[j-1] {
				if dp[i-2][j-2]+cost < dp[i][j] {
					dp[i][j] = dp[i-2][j-2] + cost
				}
			}
		}
	}

	return dp[len(r1)][len(r2)]
}

// LevenshteinDistance is a deprecated alias for DamerauLevenshteinDistance.
// Deprecated: Use DamerauLevenshteinDistance instead.
func LevenshteinDistance(s1, s2 string) int {
	return DamerauLevenshteinDistance(s1, s2)
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
