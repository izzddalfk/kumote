// internal/assistant/adapters/scanner/config.go
package scanner

import (
	"os"
	"path/filepath"

	"github.com/knightazura/kumote/internal/assistant/core"
)

// DefaultScanConfig returns a default scan configuration
func DefaultScanConfig() *core.ScanConfig {
	homeDir, _ := os.UserHomeDir()
	developmentPath := filepath.Join(homeDir, "Development")

	return &core.ScanConfig{
		BasePath: developmentPath,
		Indicators: []string{
			core.GoModFile,
			core.PackageJSONFile,
			core.RequirementsTxtFile,
			core.ReadmeFile,
			core.GitDir,
			core.DockerFile,
			core.MakeFile,
		},
		ExcludedDirs: []string{
			core.NodeModulesDir,
			core.GitDir2,
			core.DistDir,
			core.BuildDir,
			core.VendorDir,
			core.TargetDir,
			core.OutDir,
			core.TmpDir,
			core.TempDir,
		},
		MaxDepth:       3,
		MinProjectSize: 1024, // 1KB minimum
		Shortcuts: map[string]string{
			"taqwa": "TaqwaBoard",
			"car":   "CarLogbook",
			"jda":   "Junior-Dev-Acceleration",
		},
		UpdateSchedule: "0 9 * * *", // Daily at 9 AM
	}
}

// GetIndexPath returns the path where project index should be stored
func GetIndexPath() string {
	homeDir, _ := os.UserHomeDir()
	return filepath.Join(homeDir, ".config", "remote-assistant", "projects-index.json")
}
