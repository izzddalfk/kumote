package wordsimilarity

import (
	"math"
	"strings"
)

// CalculateSimilarity uses a combination of techniques for better matching
func CalculateSimilarity(word1, word2 string) float64 {
	// Exact match
	if word1 == word2 {
		return 1.0
	}

	// Substring match (if one contains the other)
	if strings.Contains(word1, word2) || strings.Contains(word2, word1) {
		shorter := math.Min(float64(len(word1)), float64(len(word2)))
		longer := math.Max(float64(len(word1)), float64(len(word2)))
		return shorter / longer
	}

	// Levenshtein distance for fuzzy matching
	distance := levenshteinDistance(word1, word2)
	maxLen := math.Max(float64(len(word1)), float64(len(word2)))

	if maxLen == 0 {
		return 1.0
	}

	return 1.0 - (float64(distance) / maxLen)
}

// levenshteinDistance calculates the edit distance between two strings
func levenshteinDistance(s1, s2 string) int {
	if len(s1) == 0 {
		return len(s2)
	}
	if len(s2) == 0 {
		return len(s1)
	}

	matrix := make([][]int, len(s1)+1)
	for i := range matrix {
		matrix[i] = make([]int, len(s2)+1)
	}

	// Initialize first row and column
	for i := 0; i <= len(s1); i++ {
		matrix[i][0] = i
	}
	for j := 0; j <= len(s2); j++ {
		matrix[0][j] = j
	}

	// Fill the matrix
	for i := 1; i <= len(s1); i++ {
		for j := 1; j <= len(s2); j++ {
			cost := 0
			if s1[i-1] != s2[j-1] {
				cost = 1
			}

			matrix[i][j] = min(
				matrix[i-1][j]+1,      // deletion
				matrix[i][j-1]+1,      // insertion
				matrix[i-1][j-1]+cost, // substitution
			)
		}
	}

	return matrix[len(s1)][len(s2)]
}

// min returns the minimum of three integers
func min(a, b, c int) int {
	if a < b {
		if a < c {
			return a
		}
		return c
	}
	if b < c {
		return b
	}
	return c
}
