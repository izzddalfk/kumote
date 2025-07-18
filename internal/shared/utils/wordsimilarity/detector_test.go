package wordsimilarity_test

import (
	"fmt"
	"testing"

	"github.com/izzddalfk/kumote/internal/shared/utils/wordsimilarity"
	"github.com/stretchr/testify/assert"
)

func TestCalculateSimilarity(t *testing.T) {
	// Test cases
	testCases := []struct {
		word      string
		inputWord string
		threshold float64
		expected  bool
	}{
		{
			word:      "carlogbook",
			inputWord: "mycar-logbook",
			threshold: 0.7,
			expected:  true,
		},
		{
			word:      "personalweb",
			inputWord: "personal-website",
			threshold: 0.5,
			expected:  true,
		},
		{
			word:      "database",
			inputWord: "DB",
			threshold: 0.3,
			expected:  false, // DB is too short for current threshold
		},
		{
			word:      "javascript",
			inputWord: "JS",
			threshold: 0.3,
			expected:  false, // JS is too short for current threshold
		},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("test for word=%s", tc.word), func(t *testing.T) {
			result := wordsimilarity.CalculateSimilarity(tc.word, tc.inputWord)
			assert.Equal(t, tc.expected, result >= tc.threshold, "Expected similarity to be greater or equal than threshold")

			t.Logf("Similarity between '%s' and '%s': %f", tc.word, tc.inputWord, result)
		})
	}
}
