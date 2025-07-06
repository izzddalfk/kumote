package scanner

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"

	"github.com/knightazura/kumote/internal/shared/utils/wordsimilarity"
	"gopkg.in/validator.v2"
)

const defaultWordProximityThreshold = 0.7

type FileSystemScanner struct {
	projectIndexPath       string
	wordProximityThreshold float64
}

type FileSystemScannerConfig struct {
	ProjectIndexPath       string `validate:"nonzero"`
	WordProximityThreshold float64
}

func NewFileSystemScanner(config FileSystemScannerConfig) (*FileSystemScanner, error) {
	if err := validator.Validate(config); err != nil {
		return nil, fmt.Errorf("invalid filesystem scanner configuration: %w", err)
	}

	// set word proximity threshold
	wpl := defaultWordProximityThreshold
	if config.WordProximityThreshold > 0 {
		wpl = config.WordProximityThreshold
	}

	return &FileSystemScanner{
		projectIndexPath:       config.ProjectIndexPath,
		wordProximityThreshold: wpl,
	}, nil
}

func (s *FileSystemScanner) GetProjectDirectory(query string) (string, error) {
	projects, err := s.loadProjectIndex()
	if err != nil {
		return "", fmt.Errorf("failed to load project index: %w", err)
	}

	// iterate over the projects and check if the query matches any project name
	for _, project := range projects {
		if s.detectWord(project.Name, query) {
			// return the project path if a match is found
			return project.Path, nil
		}
	}

	return "", nil
}

func (s *FileSystemScanner) loadProjectIndex() ([]projectEntry, error) {
	// Read the project index from a file.
	// Currently it only support JSON format.
	fileBytes, err := os.ReadFile(s.projectIndexPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read project index file: %w", err)
	}
	var projectIndex struct {
		Projects []projectEntry `json:"projects"`
	}
	if err := json.Unmarshal(fileBytes, &projectIndex); err != nil {
		return nil, fmt.Errorf("failed to unmarshal project index: %w", err)
	}
	if len(projectIndex.Projects) == 0 {
		return nil, fmt.Errorf("no projects found in index file: %s", s.projectIndexPath)
	}

	return projectIndex.Projects, nil
}

func (s *FileSystemScanner) detectWord(targetWord, sentence string) bool {
	// Clean inputs
	targetWord = strings.ToLower(strings.TrimSpace(targetWord))
	sentence = strings.ToLower(sentence)

	// Extract words from sentence (including hyphenated words)
	words := extractWords(sentence)

	log.Printf("Extracted words %v\n", words)

	// Check each word for similarity
	for _, word := range words {
		if wordsimilarity.CalculateSimilarity(targetWord, word) >= s.wordProximityThreshold {
			return true
		}
	}

	return false
}

// extractWords extracts individual words and handles compound words
func extractWords(text string) []string {
	var words []string

	// Split by common delimiters
	re := regexp.MustCompile(`[^\w-]+`)
	parts := re.Split(text, -1)

	for _, part := range parts {
		if part == "" {
			continue
		}

		// Add the whole part
		words = append(words, part)

		// Also split hyphenated words and add individual components
		if strings.Contains(part, "-") {
			hyphenParts := strings.Split(part, "-")
			for _, hyphenPart := range hyphenParts {
				if hyphenPart != "" {
					words = append(words, hyphenPart)
				}
			}
		}
	}

	return words
}
