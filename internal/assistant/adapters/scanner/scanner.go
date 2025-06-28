package scanner

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/knightazura/kumote/internal/assistant/core"
)

type ProjectScanner struct {
	config    *core.ScanConfig
	indexPath string
	logger    *slog.Logger
}

func NewProjectScanner(config *core.ScanConfig, indexPath string, logger *slog.Logger) *ProjectScanner {
	return &ProjectScanner{
		config:    config,
		indexPath: indexPath,
		logger:    logger,
	}
}

// ScanProjects scans the base directory and returns discovered projects
func (ps *ProjectScanner) ScanProjects(ctx context.Context, config core.ScanConfig) (*core.ProjectIndex, error) {
	ps.logger.InfoContext(ctx, "Starting project scan",
		"base_path", config.BasePath,
		"max_depth", config.MaxDepth,
	)

	startTime := time.Now()
	projects := []core.Project{}

	err := ps.scanDirectory(ctx, config.BasePath, 0, config, &projects)
	if err != nil {
		return nil, fmt.Errorf("failed to scan directory: %w", err)
	}

	// Build shortcuts map
	shortcuts := make(map[string]string)
	for _, project := range projects {
		for _, shortcut := range project.Shortcuts {
			shortcuts[shortcut] = project.Name
		}
	}

	// Add configured shortcuts
	for shortcut, projectName := range config.Shortcuts {
		shortcuts[shortcut] = projectName
	}

	index := &core.ProjectIndex{
		Projects:   projects,
		UpdatedAt:  time.Now(),
		TotalCount: len(projects),
		ScanPath:   config.BasePath,
		Shortcuts:  shortcuts,
	}

	ps.logger.InfoContext(ctx, "Project scan completed",
		"projects_found", len(projects),
		"duration_ms", time.Since(startTime).Milliseconds(),
	)

	return index, nil
}

// UpdateIndex refreshes the project index
func (ps *ProjectScanner) UpdateIndex(ctx context.Context) (*core.ProjectIndex, error) {
	ps.logger.InfoContext(ctx, "Updating project index")

	index, err := ps.ScanProjects(ctx, *ps.config)
	if err != nil {
		return nil, fmt.Errorf("failed to scan projects: %w", err)
	}

	if err := ps.SaveProjectIndex(ctx, index); err != nil {
		return nil, fmt.Errorf("failed to save project index: %w", err)
	}

	return index, nil
}

// GetProjectIndex returns the current project index
func (ps *ProjectScanner) GetProjectIndex(ctx context.Context) (*core.ProjectIndex, error) {
	// Try to load from file first
	index, err := ps.LoadProjectIndex(ctx)
	if err != nil {
		ps.logger.WarnContext(ctx, "Failed to load project index, creating new one",
			"error", err.Error(),
		)

		// If loading fails, create a new index
		return ps.UpdateIndex(ctx)
	}

	// Check if index is stale (older than 24 hours)
	if time.Since(index.UpdatedAt) > 24*time.Hour {
		ps.logger.InfoContext(ctx, "Project index is stale, updating",
			"last_updated", index.UpdatedAt,
		)
		return ps.UpdateIndex(ctx)
	}

	return index, nil
}

// SaveProjectIndex persists the project index
func (ps *ProjectScanner) SaveProjectIndex(ctx context.Context, index *core.ProjectIndex) error {
	if err := core.ValidateProjectIndex(*index); err != nil {
		return fmt.Errorf("invalid project index: %w", err)
	}

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(ps.indexPath), 0755); err != nil {
		return fmt.Errorf("failed to create index directory: %w", err)
	}

	data, err := json.MarshalIndent(index, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal project index: %w", err)
	}

	if err := os.WriteFile(ps.indexPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write project index: %w", err)
	}

	ps.logger.DebugContext(ctx, "Project index saved",
		"index_path", ps.indexPath,
		"projects", len(index.Projects),
	)

	return nil
}

// LoadProjectIndex loads the persisted project index
func (ps *ProjectScanner) LoadProjectIndex(ctx context.Context) (*core.ProjectIndex, error) {
	data, err := os.ReadFile(ps.indexPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, core.ErrProjectIndexCorrupt
		}
		return nil, fmt.Errorf("failed to read project index: %w", err)
	}

	var index core.ProjectIndex
	if err := json.Unmarshal(data, &index); err != nil {
		return nil, fmt.Errorf("failed to unmarshal project index: %w", err)
	}

	if err := core.ValidateProjectIndex(index); err != nil {
		return nil, fmt.Errorf("loaded project index is invalid: %w", err)
	}

	ps.logger.DebugContext(ctx, "Project index loaded",
		"projects", len(index.Projects),
		"last_update", index.UpdatedAt,
	)

	return &index, nil
}

// scanDirectory recursively scans a directory for projects
func (ps *ProjectScanner) scanDirectory(ctx context.Context, dirPath string, currentDepth int, config core.ScanConfig, projects *[]core.Project) error {
	if currentDepth >= config.MaxDepth {
		return nil
	}

	entries, err := os.ReadDir(dirPath)
	if err != nil {
		// Log warning but continue scanning
		ps.logger.WarnContext(ctx, "Failed to read directory",
			"path", dirPath,
			"error", err.Error(),
		)
		return nil
	}

	// Check if current directory is a project
	if project := ps.detectProject(ctx, dirPath, config); project != nil {
		*projects = append(*projects, *project)
		// Don't scan subdirectories of detected projects
		return nil
	}

	// Scan subdirectories
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		dirName := entry.Name()

		// Skip excluded directories
		if ps.isExcludedDir(dirName, config.ExcludedDirs) {
			continue
		}

		// Skip hidden directories (starting with .)
		if strings.HasPrefix(dirName, ".") {
			continue
		}

		subPath := filepath.Join(dirPath, dirName)
		if err := ps.scanDirectory(ctx, subPath, currentDepth+1, config, projects); err != nil {
			ps.logger.WarnContext(ctx, "Error scanning subdirectory",
				"path", subPath,
				"error", err.Error(),
			)
		}
	}

	return nil
}

// detectProject checks if a directory contains a project
func (ps *ProjectScanner) detectProject(ctx context.Context, dirPath string, config core.ScanConfig) *core.Project {
	foundIndicators := []string{}
	keyFiles := []string{}

	// Check for project indicators
	for _, indicator := range config.Indicators {
		indicatorPath := filepath.Join(dirPath, indicator)
		if _, err := os.Stat(indicatorPath); err == nil {
			foundIndicators = append(foundIndicators, indicator)
			keyFiles = append(keyFiles, indicator)
		}
	}

	// Must have at least one indicator
	if len(foundIndicators) == 0 {
		return nil
	}

	// Check minimum project size
	if config.MinProjectSize > 0 {
		size := ps.getDirectorySize(dirPath)
		if size < config.MinProjectSize {
			return nil
		}
	}

	// Determine project type and tech stack
	projectType, techStack := ps.determineProjectType(foundIndicators, dirPath)

	// Get project name from directory
	projectName := filepath.Base(dirPath)

	// Get purpose from README if available
	purpose := ps.extractPurposeFromReadme(dirPath)

	// Get last commit time if it's a git repository
	var lastCommit *time.Time
	if ps.hasGitRepository(dirPath) {
		if commit := ps.getLastCommitTime(dirPath); commit != nil {
			lastCommit = commit
		}
	}

	// Generate shortcuts from configured mapping and project name
	shortcuts := ps.generateShortcuts(projectName, config.Shortcuts)

	// Determine project status
	status := ps.determineProjectStatus(dirPath, lastCommit)

	project := &core.Project{
		Name:       projectName,
		Path:       dirPath,
		Type:       projectType,
		TechStack:  techStack,
		Purpose:    purpose,
		KeyFiles:   keyFiles,
		Status:     status,
		LastCommit: lastCommit,
		Shortcuts:  shortcuts,
		Metadata:   make(map[string]string),
	}

	ps.logger.DebugContext(ctx, "Project detected",
		"name", project.Name,
		"path", project.Path,
		"type", project.Type,
		"tech_stack", project.TechStack,
	)

	return project
}

// isExcludedDir checks if directory should be excluded from scanning
func (ps *ProjectScanner) isExcludedDir(dirName string, excludedDirs []string) bool {
	for _, excluded := range excludedDirs {
		if dirName == excluded {
			return true
		}
	}
	return false
}

// getDirectorySize calculates approximate directory size
func (ps *ProjectScanner) getDirectorySize(dirPath string) int64 {
	var size int64

	err := filepath.WalkDir(dirPath, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // Skip errors
		}

		if !d.IsDir() {
			if info, err := d.Info(); err == nil {
				size += info.Size()
			}
		}

		// Stop if we've walked too deep or size is already large enough
		if size > 100*1024*1024 { // 100MB
			return filepath.SkipAll
		}

		return nil
	})

	if err != nil {
		return 0
	}

	return size
}

// determineProjectType determines project type based on indicators
func (ps *ProjectScanner) determineProjectType(indicators []string, dirPath string) (core.ProjectType, []string) {
	techStack := []string{}

	// Check for Go project
	if ps.containsAny(indicators, []string{"go.mod"}) {
		techStack = append(techStack, "go")

		// Check if it's also a web project with Vue
		if ps.hasVueFiles(dirPath) {
			techStack = append(techStack, "vue")
			return core.ProjectTypeGo, techStack
		}

		return core.ProjectTypeGo, techStack
	}

	// Check for Node.js/Vue project
	if ps.containsAny(indicators, []string{"package.json"}) {
		techStack = append(techStack, "nodejs")

		// Check if it's a Vue project
		if ps.hasVueFiles(dirPath) {
			techStack = append(techStack, "vue")
			return core.ProjectTypeVue, techStack
		}

		// Check if it's React
		if ps.hasReactFiles(dirPath) {
			techStack = append(techStack, "react")
			return core.ProjectTypeNodeJS, techStack
		}

		return core.ProjectTypeNodeJS, techStack
	}

	// Check for Python project
	if ps.containsAny(indicators, []string{"requirements.txt", "setup.py", "pyproject.toml"}) {
		techStack = append(techStack, "python")
		return core.ProjectTypePython, techStack
	}

	// Check for documentation project
	if ps.containsAny(indicators, []string{"README.md"}) && len(indicators) == 1 {
		techStack = append(techStack, "documentation")
		return core.ProjectTypeDocumentation, techStack
	}

	return core.ProjectTypeUnknown, techStack
}

// containsAny checks if slice contains any of the target values
func (ps *ProjectScanner) containsAny(slice []string, targets []string) bool {
	for _, item := range slice {
		for _, target := range targets {
			if item == target {
				return true
			}
		}
	}
	return false
}

// hasVueFiles checks if directory contains Vue.js files
func (ps *ProjectScanner) hasVueFiles(dirPath string) bool {
	vueFiles := []string{
		"vue.config.js",
		"src/App.vue",
		"src/main.js",
		"src/main.ts",
	}

	for _, file := range vueFiles {
		if _, err := os.Stat(filepath.Join(dirPath, file)); err == nil {
			return true
		}
	}

	return false
}

// hasReactFiles checks if directory contains React files
func (ps *ProjectScanner) hasReactFiles(dirPath string) bool {
	reactFiles := []string{
		"src/App.jsx",
		"src/App.tsx",
		"src/index.js",
		"src/index.tsx",
	}

	for _, file := range reactFiles {
		if _, err := os.Stat(filepath.Join(dirPath, file)); err == nil {
			return true
		}
	}

	return false
}

// extractPurposeFromReadme extracts purpose from README file
func (ps *ProjectScanner) extractPurposeFromReadme(dirPath string) string {
	readmePath := filepath.Join(dirPath, "README.md")
	data, err := os.ReadFile(readmePath)
	if err != nil {
		return ""
	}

	content := string(data)
	lines := strings.Split(content, "\n")

	// Look for the first meaningful line after the title
	for i, line := range lines {
		line = strings.TrimSpace(line)

		// Skip title lines (starting with #)
		if strings.HasPrefix(line, "#") {
			continue
		}

		// Skip empty lines
		if len(line) == 0 {
			continue
		}

		// Found a description line
		if len(line) > 10 && len(line) < 200 {
			return line
		}

		// Don't search too far
		if i > 10 {
			break
		}
	}

	return ""
}

// hasGitRepository checks if directory is a git repository
func (ps *ProjectScanner) hasGitRepository(dirPath string) bool {
	gitPath := filepath.Join(dirPath, ".git")
	if stat, err := os.Stat(gitPath); err == nil {
		return stat.IsDir()
	}
	return false
}

// getLastCommitTime gets the last commit time from git
func (ps *ProjectScanner) getLastCommitTime(dirPath string) *time.Time {
	// This is a simplified implementation
	// In a real implementation, you'd use git commands or go-git library
	gitPath := filepath.Join(dirPath, ".git", "logs", "HEAD")
	if data, err := os.ReadFile(gitPath); err == nil {
		lines := strings.Split(string(data), "\n")
		if len(lines) > 0 {
			lastLine := lines[len(lines)-2] // Last line is usually empty
			if parts := strings.Fields(lastLine); len(parts) >= 3 {
				if timestamp := parts[2]; len(timestamp) > 0 {
					if t, err := time.Parse("1136239445", timestamp); err == nil {
						return &t
					}
				}
			}
		}
	}
	return nil
}

// generateShortcuts generates shortcuts for a project
func (ps *ProjectScanner) generateShortcuts(projectName string, configShortcuts map[string]string) []string {
	shortcuts := []string{}

	// Check if project has configured shortcuts
	for shortcut, configProjectName := range configShortcuts {
		if strings.EqualFold(configProjectName, projectName) {
			shortcuts = append(shortcuts, shortcut)
		}
	}

	// Generate automatic shortcuts if none configured
	if len(shortcuts) == 0 {
		// Create shortcut from project name
		lowerName := strings.ToLower(projectName)

		// Simple shortcut generation rules
		if len(lowerName) <= 4 {
			shortcuts = append(shortcuts, lowerName)
		} else {
			// Use first few characters
			shortcuts = append(shortcuts, lowerName[:4])
		}
	}

	return shortcuts
}

// determineProjectStatus determines the current status of a project
func (ps *ProjectScanner) determineProjectStatus(dirPath string, lastCommit *time.Time) core.ProjectStatus {
	// If no git repository, consider it unknown
	if lastCommit == nil {
		return core.ProjectStatusUnknown
	}

	// If last commit is within 30 days, consider active
	if time.Since(*lastCommit) <= 30*24*time.Hour {
		return core.ProjectStatusActive
	}

	// If last commit is within 6 months, consider maintenance
	if time.Since(*lastCommit) <= 180*24*time.Hour {
		return core.ProjectStatusMaintenance
	}

	// Otherwise, consider archived
	return core.ProjectStatusArchived
}
