package config

import (
	"fmt"
	"time"

	"github.com/gosidekick/goconfig"
)

type Configs struct {
	ApplicationConfig ApplicationConfig
	ServerConfig      ServerConfig
}

type ApplicationConfig struct {
	LogLevel               string `cfg:"log_level" cfgDefault:"debug"`
	ProjectsPath           string `cfg:"projects_path" cfgRequired:"true"`
	ClaudeCodePath         string `cfg:"claude_code_path" cfgRequired:"true"`
	ProjectIndexPath       string `cfg:"project_index_path"`
	TelegramBaseURL        string `cfg:"telegram_base_url" cfgDefault:"https://api.telegram.org"`
	TelegramBotToken       string `cfg:"kumote_telegram_bot_token" cfgRequired:"true"`
	TelegramAllowedUserIDs string `cfg:"telegram_allowed_user_ids" cfgRequired:"true"` // TODO: It's not used by now and should be []int64
}

// ServerConfig holds server configuration
type ServerConfig struct {
	Port         int           `cfg:"port" cfgDefault:"3377"`
	ReadTimeout  time.Duration `cfg:"read_timeout"`
	WriteTimeout time.Duration `cfg:"write_timeout"`
}

// LoadConfig loads configuration from environment variables
// and do validations to them
func LoadConfig() (*Configs, error) {
	var (
		appCfg    ApplicationConfig
		serverCfg ServerConfig
	)
	err := goconfig.Parse(&appCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to parse application config: %w", err)
	}
	err = goconfig.Parse(&serverCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to parse server config: %w", err)
	}

	return &Configs{
		ApplicationConfig: appCfg,
		ServerConfig:      serverCfg,
	}, nil
}
