package filesystem_test

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/knightazura/kumote/internal/assistant/adapters/filesystem"
	"github.com/knightazura/kumote/internal/assistant/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func getTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelWarn, // Reduce noise in tests
	}))
}

func setupFileSystem(t *testing.T) (*filesystem.FileSystem, string, func()) {
	tmpDir := t.TempDir()
	logger := getTestLogger()

	fs, err := filesystem.NewFileSystem(tmpDir, []string{tmpDir}, logger)
	require.NoError(t, err)

	cleanup := func() {
		fs.Close()
	}

	return fs, tmpDir, cleanup
}

func TestFileSystem_NewFileSystem_Success(t *testing.T) {
	tmpDir := t.TempDir()
	logger := getTestLogger()

	fs, err := filesystem.NewFileSystem(tmpDir, []string{tmpDir}, logger)
	require.NoError(t, err)
	assert.NotNil(t, fs)

	fs.Close()
}

func TestFileSystem_NewFileSystem_EmptyBasePath(t *testing.T) {
	logger := getTestLogger()

	fs, err := filesystem.NewFileSystem("", []string{}, logger)
	assert.Error(t, err)
	assert.Nil(t, fs)
	assert.Contains(t, err.Error(), "base path cannot be empty")
}

func TestFileSystem_NewFileSystem_InvalidAllowedDir(t *testing.T) {
	tmpDir := t.TempDir()
	logger := getTestLogger()

	// Try with non-existent directory
	fs, err := filesystem.NewFileSystem(tmpDir, []string{"/invalid/path/that/does/not/exist"}, logger)
	// Should still work as it just resolves the path
	require.NoError(t, err)
	assert.NotNil(t, fs)

	fs.Close()
}

func TestFileSystem_Exists_FileExists(t *testing.T) {
	fs, tmpDir, cleanup := setupFileSystem(t)
	defer cleanup()

	ctx := context.Background()

	// Create test file
	testFile := filepath.Join(tmpDir, "test.txt")
	err := os.WriteFile(testFile, []byte("test content"), 0644)
	require.NoError(t, err)

	// Test with absolute path
	assert.True(t, fs.Exists(ctx, testFile))

	// Test with relative path
	assert.True(t, fs.Exists(ctx, "test.txt"))
}

func TestFileSystem_Exists_FileNotExists(t *testing.T) {
	fs, _, cleanup := setupFileSystem(t)
	defer cleanup()

	ctx := context.Background()

	assert.False(t, fs.Exists(ctx, "nonexistent.txt"))
}

func TestFileSystem_Exists_Directory(t *testing.T) {
	fs, tmpDir, cleanup := setupFileSystem(t)
	defer cleanup()

	ctx := context.Background()

	// Create test directory
	testDir := filepath.Join(tmpDir, "testdir")
	err := os.MkdirAll(testDir, 0755)
	require.NoError(t, err)

	assert.True(t, fs.Exists(ctx, "testdir"))
}

func TestFileSystem_Exists_InvalidPath(t *testing.T) {
	fs, _, cleanup := setupFileSystem(t)
	defer cleanup()

	ctx := context.Background()

	// Test with path traversal
	assert.False(t, fs.Exists(ctx, "../../../etc/passwd"))
}

func TestFileSystem_ReadFile_Success(t *testing.T) {
	fs, tmpDir, cleanup := setupFileSystem(t)
	defer cleanup()

	ctx := context.Background()
	testContent := "Hello, World! This is test content."

	// Create test file
	testFile := filepath.Join(tmpDir, "test.txt")
	err := os.WriteFile(testFile, []byte(testContent), 0644)
	require.NoError(t, err)

	// Read file
	content, err := fs.ReadFile(ctx, "test.txt")
	require.NoError(t, err)
	assert.Equal(t, testContent, string(content))
}

func TestFileSystem_ReadFile_NotFound(t *testing.T) {
	fs, _, cleanup := setupFileSystem(t)
	defer cleanup()

	ctx := context.Background()

	content, err := fs.ReadFile(ctx, "nonexistent.txt")
	assert.Error(t, err)
	assert.Nil(t, content)
	assert.Equal(t, core.ErrFileNotFound, err)
}

func TestFileSystem_ReadFile_Directory(t *testing.T) {
	fs, tmpDir, cleanup := setupFileSystem(t)
	defer cleanup()

	ctx := context.Background()

	// Create test directory
	testDir := filepath.Join(tmpDir, "testdir")
	err := os.MkdirAll(testDir, 0755)
	require.NoError(t, err)

	// Try to read directory as file
	content, err := fs.ReadFile(ctx, "testdir")
	assert.Error(t, err)
	assert.Nil(t, content)
	assert.Contains(t, err.Error(), "path is a directory")
}

func TestFileSystem_ReadFile_TooLarge(t *testing.T) {
	fs, tmpDir, cleanup := setupFileSystem(t)
	defer cleanup()

	ctx := context.Background()

	// Set small file size limit
	config := filesystem.FileSystemConfig{
		MaxFileSize:       100, // 100 bytes limit
		MaxDirDepth:       10,
		AllowedExtensions: []string{},
		ReadOnly:          false,
		FollowSymlinks:    false,
		Timeout:           30 * time.Second,
	}
	fs.SetConfig(config)

	// Create large file
	largeContent := strings.Repeat("a", 200)
	testFile := filepath.Join(tmpDir, "large.txt")
	err := os.WriteFile(testFile, []byte(largeContent), 0644)
	require.NoError(t, err)

	// Try to read large file
	content, err := fs.ReadFile(ctx, "large.txt")
	assert.Error(t, err)
	assert.Nil(t, content)
	assert.Contains(t, err.Error(), "exceeds maximum allowed size")
}

func TestFileSystem_ReadFile_RestrictedExtension(t *testing.T) {
	fs, tmpDir, cleanup := setupFileSystem(t)
	defer cleanup()

	ctx := context.Background()

	// Set extension restrictions
	config := filesystem.FileSystemConfig{
		MaxFileSize:       1000,
		MaxDirDepth:       10,
		AllowedExtensions: []string{".txt", ".md"},
		ReadOnly:          false,
		FollowSymlinks:    false,
		Timeout:           30 * time.Second,
	}
	fs.SetConfig(config)

	// Create file with restricted extension
	testFile := filepath.Join(tmpDir, "test.exe")
	err := os.WriteFile(testFile, []byte("content"), 0644)
	require.NoError(t, err)

	// Try to read file with restricted extension
	content, err := fs.ReadFile(ctx, "test.exe")
	assert.Error(t, err)
	assert.Nil(t, content)
	assert.Contains(t, err.Error(), "not allowed")
}

func TestFileSystem_WriteFile_Success(t *testing.T) {
	fs, _, cleanup := setupFileSystem(t)
	defer cleanup()

	ctx := context.Background()
	testContent := "Hello, World!"

	// Write file
	err := fs.WriteFile(ctx, "test.txt", []byte(testContent))
	require.NoError(t, err)

	// Verify file was written
	content, err := fs.ReadFile(ctx, "test.txt")
	require.NoError(t, err)
	assert.Equal(t, testContent, string(content))
}

func TestFileSystem_WriteFile_ReadOnly(t *testing.T) {
	fs, _, cleanup := setupFileSystem(t)
	defer cleanup()

	ctx := context.Background()

	// Set read-only mode
	config := filesystem.FileSystemConfig{
		MaxFileSize:       1000,
		MaxDirDepth:       10,
		AllowedExtensions: []string{},
		ReadOnly:          true,
		FollowSymlinks:    false,
		Timeout:           30 * time.Second,
	}
	fs.SetConfig(config)

	// Try to write file in read-only mode
	err := fs.WriteFile(ctx, "test.txt", []byte("content"))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "read-only mode")
}

func TestFileSystem_WriteFile_TooLarge(t *testing.T) {
	fs, _, cleanup := setupFileSystem(t)
	defer cleanup()

	ctx := context.Background()

	// Set small file size limit
	config := filesystem.FileSystemConfig{
		MaxFileSize:       50,
		MaxDirDepth:       10,
		AllowedExtensions: []string{},
		ReadOnly:          false,
		FollowSymlinks:    false,
		Timeout:           30 * time.Second,
	}
	fs.SetConfig(config)

	// Try to write large content
	largeContent := strings.Repeat("a", 100)
	err := fs.WriteFile(ctx, "large.txt", []byte(largeContent))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds maximum allowed size")
}

func TestFileSystem_WriteFile_RestrictedExtension(t *testing.T) {
	fs, _, cleanup := setupFileSystem(t)
	defer cleanup()

	ctx := context.Background()

	// Set extension restrictions
	config := filesystem.FileSystemConfig{
		MaxFileSize:       1000,
		MaxDirDepth:       10,
		AllowedExtensions: []string{".txt"},
		ReadOnly:          false,
		FollowSymlinks:    false,
		Timeout:           30 * time.Second,
	}
	fs.SetConfig(config)

	// Try to write file with restricted extension
	err := fs.WriteFile(ctx, "test.exe", []byte("content"))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not allowed")
}

func TestFileSystem_WriteFile_CreateDirectory(t *testing.T) {
	fs, _, cleanup := setupFileSystem(t)
	defer cleanup()

	ctx := context.Background()

	// Write file in non-existent subdirectory
	err := fs.WriteFile(ctx, "subdir/test.txt", []byte("content"))
	require.NoError(t, err)

	// Verify directory and file were created
	assert.True(t, fs.Exists(ctx, "subdir"))
	assert.True(t, fs.Exists(ctx, "subdir/test.txt"))

	// Verify content
	content, err := fs.ReadFile(ctx, "subdir/test.txt")
	require.NoError(t, err)
	assert.Equal(t, "content", string(content))
}

func TestFileSystem_ListDir_Success(t *testing.T) {
	fs, tmpDir, cleanup := setupFileSystem(t)
	defer cleanup()

	ctx := context.Background()

	// Create test files and directories
	testFiles := []string{"file1.txt", "file2.go", "README.md"}
	for _, file := range testFiles {
		err := os.WriteFile(filepath.Join(tmpDir, file), []byte("content"), 0644)
		require.NoError(t, err)
	}

	// Create test directory
	err := os.MkdirAll(filepath.Join(tmpDir, "subdir"), 0755)
	require.NoError(t, err)

	// Create hidden file (should be excluded)
	err = os.WriteFile(filepath.Join(tmpDir, ".hidden"), []byte("hidden"), 0644)
	require.NoError(t, err)

	// List directory
	entries, err := fs.ListDir(ctx, ".")
	require.NoError(t, err)

	// Should include files and subdirectory but not hidden file
	assert.Len(t, entries, 4) // 3 files + 1 subdirectory
	assert.Contains(t, entries, "file1.txt")
	assert.Contains(t, entries, "file2.go")
	assert.Contains(t, entries, "README.md")
	assert.Contains(t, entries, "subdir")
	assert.NotContains(t, entries, ".hidden")
}

func TestFileSystem_ListDir_NotFound(t *testing.T) {
	fs, _, cleanup := setupFileSystem(t)
	defer cleanup()

	ctx := context.Background()

	entries, err := fs.ListDir(ctx, "nonexistent")
	assert.Error(t, err)
	assert.Nil(t, entries)
	assert.Equal(t, core.ErrFileNotFound, err)
}

func TestFileSystem_ListDir_NotDirectory(t *testing.T) {
	fs, tmpDir, cleanup := setupFileSystem(t)
	defer cleanup()

	ctx := context.Background()

	// Create test file
	testFile := filepath.Join(tmpDir, "test.txt")
	err := os.WriteFile(testFile, []byte("content"), 0644)
	require.NoError(t, err)

	// Try to list file as directory
	entries, err := fs.ListDir(ctx, "test.txt")
	assert.Error(t, err)
	assert.Nil(t, entries)
	assert.Contains(t, err.Error(), "not a directory")
}

func TestFileSystem_GetFileInfo_File(t *testing.T) {
	fs, tmpDir, cleanup := setupFileSystem(t)
	defer cleanup()

	ctx := context.Background()
	testContent := "Hello, World!"

	// Create test file
	testFile := filepath.Join(tmpDir, "test.go")
	err := os.WriteFile(testFile, []byte(testContent), 0644)
	require.NoError(t, err)

	// Get file info
	info, err := fs.GetFileInfo(ctx, "test.go")
	require.NoError(t, err)
	assert.NotNil(t, info)

	assert.Equal(t, "test.go", info.Name)
	assert.Equal(t, int64(len(testContent)), info.Size)
	assert.False(t, info.IsDirectory)
	assert.Equal(t, "go", info.Language)
	assert.Empty(t, info.Content) // Content should be empty for info-only request
}

func TestFileSystem_GetFileInfo_Directory(t *testing.T) {
	fs, tmpDir, cleanup := setupFileSystem(t)
	defer cleanup()

	ctx := context.Background()

	// Create test directory
	testDir := filepath.Join(tmpDir, "testdir")
	err := os.MkdirAll(testDir, 0755)
	require.NoError(t, err)

	// Get directory info
	info, err := fs.GetFileInfo(ctx, "testdir")
	require.NoError(t, err)
	assert.NotNil(t, info)

	assert.Equal(t, "testdir", info.Name)
	assert.True(t, info.IsDirectory)
}

func TestFileSystem_GetFileInfo_NotFound(t *testing.T) {
	fs, _, cleanup := setupFileSystem(t)
	defer cleanup()

	ctx := context.Background()

	info, err := fs.GetFileInfo(ctx, "nonexistent.txt")
	assert.Error(t, err)
	assert.Nil(t, info)
	assert.Equal(t, core.ErrFileNotFound, err)
}

func TestFileSystem_CreateDir_Success(t *testing.T) {
	fs, _, cleanup := setupFileSystem(t)
	defer cleanup()

	ctx := context.Background()

	// Create directory
	err := fs.CreateDir(ctx, "newdir")
	require.NoError(t, err)

	// Verify directory exists
	assert.True(t, fs.Exists(ctx, "newdir"))

	// Verify it's actually a directory
	info, err := fs.GetFileInfo(ctx, "newdir")
	require.NoError(t, err)
	assert.True(t, info.IsDirectory)
}

func TestFileSystem_CreateDir_Nested(t *testing.T) {
	fs, _, cleanup := setupFileSystem(t)
	defer cleanup()

	ctx := context.Background()

	// Create nested directory
	err := fs.CreateDir(ctx, "level1/level2/level3")
	require.NoError(t, err)

	// Verify all levels exist
	assert.True(t, fs.Exists(ctx, "level1"))
	assert.True(t, fs.Exists(ctx, "level1/level2"))
	assert.True(t, fs.Exists(ctx, "level1/level2/level3"))
}

func TestFileSystem_CreateDir_ReadOnly(t *testing.T) {
	fs, _, cleanup := setupFileSystem(t)
	defer cleanup()

	ctx := context.Background()

	// Set read-only mode
	config := filesystem.FileSystemConfig{
		MaxFileSize:       1000,
		MaxDirDepth:       10,
		AllowedExtensions: []string{},
		ReadOnly:          true,
		FollowSymlinks:    false,
		Timeout:           30 * time.Second,
	}
	fs.SetConfig(config)

	// Try to create directory in read-only mode
	err := fs.CreateDir(ctx, "newdir")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "read-only mode")
}

func TestFileSystem_CreateDir_ExistsAsFile(t *testing.T) {
	fs, tmpDir, cleanup := setupFileSystem(t)
	defer cleanup()

	ctx := context.Background()

	// Create file first
	testFile := filepath.Join(tmpDir, "existing")
	err := os.WriteFile(testFile, []byte("content"), 0644)
	require.NoError(t, err)

	// Try to create directory with same name
	err = fs.CreateDir(ctx, "existing")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not a directory")
}

func TestFileSystem_LanguageDetection(t *testing.T) {
	fs, tmpDir, cleanup := setupFileSystem(t)
	defer cleanup()

	ctx := context.Background()

	testCases := []struct {
		filename string
		expected string
	}{
		{"test.go", "go"},
		{"script.js", "javascript"},
		{"component.vue", "vue"},
		{"app.py", "python"},
		{"style.css", "css"},
		{"config.yaml", "yaml"},
		{"readme.md", "markdown"},
		{"Dockerfile", "text"},  // No extension, falls back to text
		{"unknown.xyz", "text"}, // Unknown extension
	}

	for _, tc := range testCases {
		t.Run(tc.filename, func(t *testing.T) {
			// Create test file
			testFile := filepath.Join(tmpDir, tc.filename)
			err := os.WriteFile(testFile, []byte("content"), 0644)
			require.NoError(t, err)

			// Get file info and check language
			info, err := fs.GetFileInfo(ctx, tc.filename)
			require.NoError(t, err)
			assert.Equal(t, tc.expected, info.Language)
		})
	}
}

func TestFileSystem_PathTraversalAttacks(t *testing.T) {
	fs, _, cleanup := setupFileSystem(t)
	defer cleanup()

	ctx := context.Background()

	maliciousPaths := []string{
		"../../../etc/passwd",
		"..\\..\\..\\windows\\system32\\config\\sam",
		"./../../secret.txt",
		"../outside.txt",
		"dir/../../../etc/hosts",
	}

	for _, path := range maliciousPaths {
		t.Run(path, func(t *testing.T) {
			// All these operations should fail
			assert.False(t, fs.Exists(ctx, path))

			_, err := fs.ReadFile(ctx, path)
			assert.Error(t, err)

			err = fs.WriteFile(ctx, path, []byte("content"))
			assert.Error(t, err)

			_, err = fs.ListDir(ctx, path)
			assert.Error(t, err)

			_, err = fs.GetFileInfo(ctx, path)
			assert.Error(t, err)

			err = fs.CreateDir(ctx, path)
			assert.Error(t, err)
		})
	}
}

func TestFileSystem_ContextCancellation(t *testing.T) {
	fs, _, cleanup := setupFileSystem(t)
	defer cleanup()

	// Create context that cancels immediately
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Operations should respect context cancellation
	_, err := fs.ReadFile(ctx, "test.txt")
	assert.Error(t, err)
	assert.Equal(t, context.Canceled, err)

	err = fs.WriteFile(ctx, "test.txt", []byte("content"))
	assert.Error(t, err)
	assert.Equal(t, context.Canceled, err)
}

func TestFileSystem_TimeoutBehavior(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping timeout test in short mode")
	}

	fs, _, cleanup := setupFileSystem(t)
	defer cleanup()

	ctx := context.Background()

	// Set very short timeout
	config := filesystem.FileSystemConfig{
		MaxFileSize:       1000,
		MaxDirDepth:       10,
		AllowedExtensions: []string{},
		ReadOnly:          false,
		FollowSymlinks:    false,
		Timeout:           1 * time.Nanosecond, // Extremely short timeout
	}
	fs.SetConfig(config)

	// Operations should timeout (though they might complete too fast to actually timeout)
	_, err := fs.ReadFile(ctx, "nonexistent.txt")
	// Could be timeout or file not found error
	assert.Error(t, err)
}

func TestFileSystem_GetStats(t *testing.T) {
	fs, tmpDir, cleanup := setupFileSystem(t)
	defer cleanup()

	ctx := context.Background()

	stats := fs.GetStats(ctx)
	require.NotNil(t, stats)

	assert.Equal(t, tmpDir, stats["base_path"])
	assert.Equal(t, 1, stats["allowed_dirs"])
	assert.Contains(t, stats, "max_file_size")
	assert.Contains(t, stats, "read_only")
	assert.Contains(t, stats, "timeout")
}

func TestFileSystem_SetConfig(t *testing.T) {
	fs, _, cleanup := setupFileSystem(t)
	defer cleanup()

	ctx := context.Background()

	// Set new configuration
	newConfig := filesystem.FileSystemConfig{
		MaxFileSize:       500,
		MaxDirDepth:       5,
		AllowedExtensions: []string{".txt", ".md"},
		ReadOnly:          true,
		FollowSymlinks:    true,
		Timeout:           10 * time.Second,
	}
	fs.SetConfig(newConfig)

	// Verify configuration took effect
	stats := fs.GetStats(ctx)
	assert.Equal(t, int64(500), stats["max_file_size"])
	assert.Equal(t, 5, stats["max_dir_depth"])
	assert.Equal(t, true, stats["read_only"])
	assert.Equal(t, true, stats["follow_symlinks"])
	assert.Equal(t, []string{".txt", ".md"}, stats["allowed_extensions"])
}

func TestFileSystem_ConcurrentOperations(t *testing.T) {
	fs, _, cleanup := setupFileSystem(t)
	defer cleanup()

	ctx := context.Background()
	const numGoroutines = 10

	// Test concurrent file operations
	errChan := make(chan error, numGoroutines*3)

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			filename := fmt.Sprintf("concurrent_%d.txt", id)
			content := fmt.Sprintf("Content for file %d", id)

			// Write file
			err := fs.WriteFile(ctx, filename, []byte(content))
			errChan <- err

			// Read file
			_, err = fs.ReadFile(ctx, filename)
			errChan <- err

			// Check existence
			fs.Exists(ctx, filename)
			errChan <- nil // Exists doesn't return error
		}(i)
	}

	// Collect results
	for i := 0; i < numGoroutines*3; i++ {
		err := <-errChan
		if err != nil {
			t.Errorf("Concurrent operation failed: %v", err)
		}
	}
}

func TestFileSystem_LargeDirectoryListing(t *testing.T) {
	fs, tmpDir, cleanup := setupFileSystem(t)
	defer cleanup()

	ctx := context.Background()

	// Create many files
	const numFiles = 150 // More than MaxDirectoryListing (100)
	for i := 0; i < numFiles; i++ {
		filename := filepath.Join(tmpDir, fmt.Sprintf("file_%03d.txt", i))
		err := os.WriteFile(filename, []byte("content"), 0644)
		require.NoError(t, err)
	}

	// List directory
	entries, err := fs.ListDir(ctx, ".")
	require.NoError(t, err)

	// Should be limited to MaxDirectoryListing
	assert.LessOrEqual(t, len(entries), core.MaxDirectoryListing)
}

func TestFileSystem_Close(t *testing.T) {
	fs, _, cleanup := setupFileSystem(t)
	defer cleanup()

	// Close should work without error
	err := fs.Close()
	assert.NoError(t, err)
}
