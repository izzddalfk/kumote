package scanner_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/knightazura/kumote/internal/assistant/adapters/scanner"
	"github.com/stretchr/testify/assert"
)

func TestGetProjectDirectory(t *testing.T) {
	// setup temporary test file
	// after test finished, the file will be removed automatically
	testFileContent := `{
  "projects": [
    {
      "name": "personal-website",
      "path": "/home/users/projects/personal-website"
    },
    {
      "name": "mycar-logbook",
      "path": "/home/users/projects/mycar-logbook"
    },
    {
      "name": "personal-assistant",
      "path": "/home/users/projects/kumote"
    }
  ]
}
`
	// Create a temporary directory for our test files
	tempDir, err := os.MkdirTemp("", "scanner-test")
	assert.NoError(t, err, "failed to create temp directory")
	defer os.RemoveAll(tempDir) // Clean up when test finishes

	// Create the temporary index file
	tempIndexFile := filepath.Join(tempDir, "projects-index.json")
	err = os.WriteFile(tempIndexFile, []byte(testFileContent), 0644)
	assert.NoError(t, err, "failed to write temp index file")

	// Create a non-existent file path for testing error cases
	invalidFilePath := filepath.Join(tempDir, "invalid-projects-index.json")

	testCases := []struct {
		name         string
		query        string
		inputPath    string // Path to the project index file
		expectedPath string
		expectError  bool
	}{
		{
			name:         "Exact match",
			query:        "Give me the tech stack that used in my personal-website",
			inputPath:    tempIndexFile,
			expectedPath: "/home/users/projects/personal-website",
			expectError:  false,
		},
		{
			name:         "Proximity match",
			query:        "How do the receipt read in carlogbook project?",
			inputPath:    tempIndexFile,
			expectedPath: "/home/users/projects/mycar-logbook",
			expectError:  false,
		},
		{
			name:         "No match",
			query:        "What is the weather like today?",
			inputPath:    tempIndexFile,
			expectedPath: "",
			expectError:  false,
		},
		{
			name:         "Invalid project index path",
			query:        "What is the weather like today?",
			inputPath:    invalidFilePath, // Non-existent file
			expectedPath: "",
			expectError:  true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			scanner, err := scanner.NewFileSystemScanner(scanner.FileSystemScannerConfig{
				ProjectIndexPath: tc.inputPath,
			})
			assert.NoError(t, err, "failed to create FileSystemScanner")

			path, err := scanner.GetProjectDirectory(tc.query)
			if tc.expectError {
				assert.Error(t, err, "Expected an error but got none")
			} else {
				assert.NoError(t, err, "Did not expect an error")
			}
			assert.Equal(t, tc.expectedPath, path, "Expected project directory path does not match")
		})
	}
}
