// internal/assistant/core/constants.go
package core

import "time"

// Application constants
const (
	// Application info
	AppName    = "RemoteWorkAssistant"
	AppVersion = "1.0.0"

	// Default configurations
	DefaultTimeout        = 30 * time.Second
	DefaultRateLimit      = 10   // requests per minute
	DefaultMaxDepth       = 3    // for project scanning
	DefaultMinProjectSize = 1024 // bytes

	// File patterns
	ProjectIndexFileName = "projects-index.md"
	ConfigFileName       = "scanner-config.yaml"
	LogFileName          = "assistant.log"

	// Project indicators (files that indicate a project type)
	GoModFile           = "go.mod"
	PackageJSONFile     = "package.json"
	RequirementsTxtFile = "requirements.txt"
	ReadmeFile          = "README.md"
	GitDir              = ".git"
	DockerFile          = "Dockerfile"
	MakeFile            = "Makefile"

	// Excluded directories from scanning
	NodeModulesDir = "node_modules"
	GitDir2        = ".git"
	DistDir        = "dist"
	BuildDir       = "build"
	VendorDir      = "vendor"
	TargetDir      = "target"
	OutDir         = "out"
	TmpDir         = "tmp"
	TempDir        = "temp"

	// Message formats
	TelegramMaxMessageLength = 4096
	MaxFileContentDisplay    = 1000 // characters
	MaxProjectsToShow        = 20

	// Rate limiting
	RateLimitWindow = 1 * time.Minute
	RateLimitBurst  = 5

	// Command timeouts
	ShortCommandTimeout  = 10 * time.Second
	MediumCommandTimeout = 30 * time.Second
	LongCommandTimeout   = 60 * time.Second

	// Audio processing
	MaxAudioFileSize = 20 * 1024 * 1024 // 20MB
	MaxAudioDuration = 10 * time.Minute

	// File operation limits
	MaxFileSize         = 10 * 1024 * 1024 // 10MB
	MaxDirectoryListing = 100
	MaxFileReadChars    = 50000
)

// Command intentions
const (
	IntentProjectList   = "project_list"
	IntentProjectInfo   = "project_info"
	IntentFileOperation = "file_operation"
	IntentGitOperation  = "git_operation"
	IntentGeneralQuery  = "general_query"
	IntentAmbiguous     = "ambiguous"
	IntentHelp          = "help"
	IntentStatus        = "status"
	IntentRefresh       = "refresh"
)

// Supported file extensions for different project types
var (
	GoFileExtensions        = []string{".go", ".mod", ".sum"}
	JSFileExtensions        = []string{".js", ".ts", ".vue", ".jsx", ".tsx"}
	PythonFileExtensions    = []string{".py", ".pyx", ".pyi", ".pyc"}
	ConfigFileExtensions    = []string{".yaml", ".yml", ".json", ".toml", ".ini", ".env"}
	DocumentationExtensions = []string{".md", ".txt", ".rst", ".adoc"}
	WebExtensions           = []string{".html", ".css", ".scss", ".sass", ".less"}
)

// Technology stack detection patterns
var TechStackPatterns = map[string][]string{
	"go":         {"go.mod", "main.go", "*.go"},
	"nodejs":     {"package.json", "node_modules"},
	"vue":        {"vue.config.js", "src/App.vue", "src/main.js"},
	"react":      {"src/App.jsx", "src/App.tsx", "react"},
	"python":     {"requirements.txt", "setup.py", "pyproject.toml"},
	"docker":     {"Dockerfile", "docker-compose.yml"},
	"kubernetes": {"deployment.yaml", "service.yaml", "ingress.yaml"},
	"terraform":  {"*.tf", "terraform.tfstate"},
	"ansible":    {"playbook.yml", "inventory"},
}

// Safe git commands that are allowed
var SafeGitCommands = map[string]bool{
	"status":   true,
	"log":      true,
	"show":     true,
	"diff":     true,
	"branch":   true,
	"remote":   true,
	"fetch":    true,
	"pull":     true,
	"stash":    true,
	"tag":      true,
	"blame":    true,
	"shortlog": true,
}

// Dangerous commands that require explicit confirmation
var DangerousCommands = map[string]bool{
	"rm":     true,
	"delete": true,
	"format": true,
	"reset":  true,
	"clean":  true,
	"force":  true,
}

// File operation keywords
var FileOperationKeywords = []string{
	"read", "show", "cat", "view", "display",
	"list", "ls", "dir", "tree",
	"write", "create", "make", "touch",
	"edit", "modify", "update", "change",
	"delete", "remove", "rm",
	"copy", "cp", "move", "mv",
	"search", "find", "grep",
}

// Git operation keywords
var GitOperationKeywords = []string{
	"git", "commit", "push", "pull", "merge",
	"branch", "checkout", "status", "log",
	"diff", "stash", "remote", "fetch",
	"tag", "blame", "show", "reset",
}

// Help messages
const (
	WelcomeMessage = `👋 Welcome to Remote Work Assistant!

I can help you with:
📁 Project management and navigation
📝 File operations (read, write, list)
🔧 Git operations (status, log, diff)
🚀 Code execution and analysis

Try these commands:
• "list projects" - Show all projects
• "show taqwa main.go" - Read a file
• "git status all" - Git status for all projects
• "help" - Show detailed help`

	HelpMessage = `🤖 Remote Work Assistant Commands

**Project Operations:**
• list projects - Show all available projects
• show [project] - Show project information
• refresh projects - Update project index

**File Operations:**
• read/show/cat [file] - Display file content
• list/ls [directory] - List directory contents
• find [pattern] - Search for files

**Git Operations:**
• git status [project|all] - Show git status
• git log [project] - Show commit history
• git diff [project] - Show changes

**Shortcuts:**
• taqwa → TaqwaBoard
• car → CarLogbook
• jda → Junior-Dev-Acceleration

**Examples:**
• "show taqwa main.go"
• "git status all"
• "list car/src"
• "find *.vue in taqwa"`

	ErrorMessage = `❌ Something went wrong. Please try:
• Checking your command syntax
• Using "help" for available commands
• Refreshing projects with "refresh projects"

If the problem persists, contact the administrator.`
)

// Log levels
const (
	LogLevelDebug = "DEBUG"
	LogLevelInfo  = "INFO"
	LogLevelWarn  = "WARN"
	LogLevelError = "ERROR"
)

// Metrics periods
const (
	MetricsPeriodHour  = "hour"
	MetricsPeriodDay   = "day"
	MetricsPeriodWeek  = "week"
	MetricsPeriodMonth = "month"
)

// Audio formats
const (
	AudioFormatOGG = "ogg"
	AudioFormatMP3 = "mp3"
	AudioFormatWAV = "wav"
	AudioFormatM4A = "m4a"
)

// Telegram parse modes
const (
	ParseModeHTML       = "HTML"
	ParseModeMarkdown   = "Markdown"
	ParseModeMarkdownV2 = "MarkdownV2"
)

// Cache keys
const (
	CacheKeyProjectIndex    = "project_index"
	CacheKeyUserPermissions = "user_permissions"
	CacheKeyConfig          = "config"
)

// Default paths
const (
	DefaultDevelopmentPath = "/home/user/Development"
	DefaultConfigPath      = "/etc/remote-assistant"
	DefaultLogPath         = "/var/log/remote-assistant"
	DefaultCachePath       = "/var/cache/remote-assistant"
)
