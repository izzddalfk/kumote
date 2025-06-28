package scanner_test

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/knightazura/kumote/internal/assistant/adapters/scanner"
	"github.com/knightazura/kumote/internal/assistant/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func getTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelWarn, // Reduce noise in tests
	}))
}

func TestProjectScanner_ScanProjects(t *testing.T) {
	// Create temporary test directory structure
	tmpDir := t.TempDir()

	// Create test projects
	createTestProject(t, tmpDir, "go-project", []string{"go.mod", "main.go"})
	createTestProject(t, tmpDir, "vue-project", []string{"package.json", "src/App.vue"})
	createTestProject(t, tmpDir, "python-project", []string{"requirements.txt", "main.py"})
	createTestProject(t, tmpDir, "docs-project", []string{"README.md"})

	// Create non-project directory
	nonProjectDir := filepath.Join(tmpDir, "not-a-project")
	require.NoError(t, os.MkdirAll(nonProjectDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(nonProjectDir, "random.txt"), []byte("content"), 0644))

	config := &core.ScanConfig{
		BasePath: tmpDir,
		Indicators: []string{
			"go.mod", "package.json", "requirements.txt", "README.md",
		},
		ExcludedDirs:   []string{"node_modules", ".git"},
		MaxDepth:       2,
		MinProjectSize: 0,
		Shortcuts:      map[string]string{},
		UpdateSchedule: "",
	}

	indexPath := filepath.Join(tmpDir, "index.json")
	logger := getTestLogger()
	scanner := scanner.NewProjectScanner(config, indexPath, logger)

	ctx := context.Background()
	index, err := scanner.ScanProjects(ctx, *config)

	require.NoError(t, err)
	assert.NotNil(t, index)
	assert.Equal(t, tmpDir, index.ScanPath)
	assert.Equal(t, 4, index.TotalCount) // Should find 4 projects
	assert.Len(t, index.Projects, 4)

	// Verify project types are detected correctly
	projectTypes := make(map[string]core.ProjectType)
	for _, project := range index.Projects {
		projectTypes[project.Name] = project.Type
	}

	assert.Equal(t, core.ProjectTypeGo, projectTypes["go-project"])
	assert.Equal(t, core.ProjectTypeVue, projectTypes["vue-project"])
	assert.Equal(t, core.ProjectTypePython, projectTypes["python-project"])
	assert.Equal(t, core.ProjectTypeDocumentation, projectTypes["docs-project"])
}

func TestProjectScanner_SaveAndLoadIndex(t *testing.T) {
	tmpDir := t.TempDir()
	indexPath := filepath.Join(tmpDir, "test-index.json")

	config := scanner.DefaultScanConfig()
	logger := getTestLogger()
	scanner := scanner.NewProjectScanner(config, indexPath, logger)

	ctx := context.Background()

	// Create test index
	originalIndex := &core.ProjectIndex{
		Projects: []core.Project{
			{
				Name:      "TestProject",
				Path:      "/test/path",
				Type:      core.ProjectTypeGo,
				TechStack: []string{"go"},
				Purpose:   "Test project",
				KeyFiles:  []string{"go.mod"},
				Status:    core.ProjectStatusActive,
				Shortcuts: []string{"test"},
				Metadata:  map[string]string{},
			},
		},
		UpdatedAt:  time.Now(),
		TotalCount: 1,
		ScanPath:   "/test",
		Shortcuts: map[string]string{
			"test": "TestProject",
		},
	}

	// Save index
	err := scanner.SaveProjectIndex(ctx, originalIndex)
	require.NoError(t, err)

	// Verify file exists
	assert.FileExists(t, indexPath)

	// Load index
	loadedIndex, err := scanner.LoadProjectIndex(ctx)
	require.NoError(t, err)
	assert.NotNil(t, loadedIndex)

	// Verify loaded index matches original
	assert.Equal(t, originalIndex.TotalCount, loadedIndex.TotalCount)
	assert.Equal(t, originalIndex.ScanPath, loadedIndex.ScanPath)
	assert.Len(t, loadedIndex.Projects, 1)
	assert.Equal(t, "TestProject", loadedIndex.Projects[0].Name)
	assert.Equal(t, core.ProjectTypeGo, loadedIndex.Projects[0].Type)
}

func TestProjectScanner_GetProjectIndex_CreatesNewWhenNotExists(t *testing.T) {
	tmpDir := t.TempDir()
	indexPath := filepath.Join(tmpDir, "nonexistent-index.json")

	// Create a test project to scan
	createTestProject(t, tmpDir, "test-project", []string{"go.mod"})

	config := &core.ScanConfig{
		BasePath:       tmpDir,
		Indicators:     []string{"go.mod"},
		ExcludedDirs:   []string{},
		MaxDepth:       2,
		MinProjectSize: 0,
		Shortcuts:      map[string]string{},
	}

	logger := getTestLogger()
	scanner := scanner.NewProjectScanner(config, indexPath, logger)

	ctx := context.Background()

	// Should create new index since file doesn't exist
	index, err := scanner.GetProjectIndex(ctx)
	require.NoError(t, err)
	assert.NotNil(t, index)
	assert.Len(t, index.Projects, 1)
	assert.Equal(t, "test-project", index.Projects[0].Name)

	// Verify index was saved
	assert.FileExists(t, indexPath)
}

func TestProjectScanner_UpdateIndex(t *testing.T) {
	tmpDir := t.TempDir()
	indexPath := filepath.Join(tmpDir, "update-test-index.json")

	// Initially create one project
	createTestProject(t, tmpDir, "initial-project", []string{"go.mod"})

	config := &core.ScanConfig{
		BasePath:       tmpDir,
		Indicators:     []string{"go.mod", "package.json"},
		ExcludedDirs:   []string{},
		MaxDepth:       2,
		MinProjectSize: 0,
		Shortcuts:      map[string]string{},
		UpdateSchedule: "",
	}

	logger := getTestLogger()
	scanner := scanner.NewProjectScanner(config, indexPath, logger)

	ctx := context.Background()

	// Create initial index
	index1, err := scanner.UpdateIndex(ctx)
	require.NoError(t, err)
	assert.Len(t, index1.Projects, 1)

	// Add another project
	createTestProject(t, tmpDir, "new_project", []string{"package.json"})

	// Update index
	index2, err := scanner.UpdateIndex(ctx)
	require.NoError(t, err)
	assert.Len(t, index2.Projects, 2)

	// Verify both projects are found
	projectNames := make([]string, len(index2.Projects))
	for i, project := range index2.Projects {
		projectNames[i] = project.Name
	}
	assert.Contains(t, projectNames, "initial-project")
	assert.Contains(t, projectNames, "new_project")
}

// Helper function to create test project directory structure
func createTestProject(t *testing.T, baseDir, projectName string, files []string) {
	projectDir := filepath.Join(baseDir, projectName)
	require.NoError(t, os.MkdirAll(projectDir, 0755))

	for _, file := range files {
		filePath := filepath.Join(projectDir, file)

		// Create subdirectories if needed
		if dir := filepath.Dir(filePath); dir != projectDir {
			require.NoError(t, os.MkdirAll(dir, 0755))
		}

		// Create file with some content
		content := "# " + file + "\nTest content for " + projectName
		if file == "go.mod" {
			content = "module " + projectName + "\n\ngo 1.24.1"
		} else if file == "package.json" {
			content = `{"name": "` + projectName + `", "version": "1.0.0"}`
		} else if file == "requirements.txt" {
			content = "# Requirements for " + projectName
		}

		require.NoError(t, os.WriteFile(filePath, []byte(content), 0644))
	}
}
