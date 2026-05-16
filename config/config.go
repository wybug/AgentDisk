package config

import (
	"bufio"
	"log"
	"os"
	"strings"

	"github.com/spf13/viper"
)

// Config represents a configuration.
type Config struct {
	Server        ServerConfig        `mapstructure:"server"`
	Database      DatabaseConfig      `mapstructure:"database"`
	OSS           OSSConfig           `mapstructure:"oss"`
	Redis         RedisConfig         `mapstructure:"redis"`
	JWT           JWTConfig           `mapstructure:"jwt"`
	Log           LogConfig           `mapstructure:"log"`
	OAuth2        OAuth2Config        `mapstructure:"oauth2"`
	DownloadToken DownloadTokenConfig `mapstructure:"download_token"`
}

// ServerConfig represents a serverconfiguration.
type ServerConfig struct {
	Port string `mapstructure:"port"`
	Mode string `mapstructure:"mode"`
}

// DatabaseConfig represents a databaseconfiguration.
type DatabaseConfig struct {
	Driver       string `mapstructure:"driver"`
	Host         string `mapstructure:"host"`
	Port         int    `mapstructure:"port"`
	Name         string `mapstructure:"name"`
	User         string `mapstructure:"user"`
	Password     string `mapstructure:"password"`
	MaxIdleConns int    `mapstructure:"max_idle_conns"`
	MaxOpenConns int    `mapstructure:"max_open_conns"`
	LogLevel     string `mapstructure:"log_level"`
}

// OSSConfig represents a ossconfiguration.
type OSSConfig struct {
	Endpoint  string `mapstructure:"endpoint"`
	AccessKey string `mapstructure:"access_key"`
	SecretKey string `mapstructure:"secret_key"`
	Bucket    string `mapstructure:"bucket"`
	UseSSL    bool   `mapstructure:"use_ssl"`
	Region    string `mapstructure:"region"`
}

// RedisConfig represents a redisconfiguration.
type RedisConfig struct {
	Enabled  bool   `mapstructure:"enabled"`
	Addr     string `mapstructure:"addr"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
}

// JWTConfig represents a jwtconfiguration.
type JWTConfig struct {
	Secret      string `mapstructure:"secret"`
	ExpireHours int    `mapstructure:"expire_hours"`
}

// LogConfig represents a logconfiguration.
type LogConfig struct {
	Level    string `mapstructure:"level"`
	Output   string `mapstructure:"output"`
	FilePath string `mapstructure:"file_path"`
}

// OAuth2Config represents a oauth2configuration.
type OAuth2Config struct {
	Enabled      bool     `mapstructure:"enabled"`
	ClientID     string   `mapstructure:"client_id"`
	ClientSecret string   `mapstructure:"client_secret"`
	AuthURL      string   `mapstructure:"auth_url"`
	TokenURL     string   `mapstructure:"token_url"`
	UserInfoURL  string   `mapstructure:"userinfo_url"`
	RedirectURL  string   `mapstructure:"redirect_url"`
	FrontendURL  string   `mapstructure:"frontend_url"`
	Scopes       []string `mapstructure:"scopes"`
}

// DownloadTokenConfig represents a downloadtokenconfiguration.
type DownloadTokenConfig struct {
	Secret        string `mapstructure:"secret"`
	ExpireSeconds int    `mapstructure:"expire_seconds"`
}

// Load handles HTTP requests.
func Load(path string) (*Config, error) {
	loadDotEnv()

	viper.SetConfigFile(path)
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		return nil, err
	}

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, err
	}

	// Sensitive fields: environment variables override config file
	overrideFromEnv(&cfg)

	return &cfg, nil
}

// loadDotEnv reads .env file and sets environment variables (no external dependency).
func loadDotEnv() {
	f, err := os.Open(".env")
	if err != nil {
		return // .env is optional
	}
	defer func() { _ = f.Close() }()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		k = strings.TrimSpace(k)
		v = strings.TrimSpace(v)
		if os.Getenv(k) == "" { // real env vars take precedence over .env
			_ = os.Setenv(k, v)
		}
	}
	if err := scanner.Err(); err != nil {
		log.Printf("warning: error reading .env: %v", err)
	}
}

// overrideFromEnv replaces sensitive fields with environment variables when set.
func overrideFromEnv(cfg *Config) {
	if v := os.Getenv("DB_PASSWORD"); v != "" {
		cfg.Database.Password = v
	}
	if v := os.Getenv("OSS_ACCESS_KEY"); v != "" {
		cfg.OSS.AccessKey = v
	}
	if v := os.Getenv("OSS_SECRET_KEY"); v != "" {
		cfg.OSS.SecretKey = v
	}
	if v := os.Getenv("REDIS_PASSWORD"); v != "" {
		cfg.Redis.Password = v
	}
	if v := os.Getenv("JWT_SECRET"); v != "" {
		cfg.JWT.Secret = v
	}
	if v := os.Getenv("OAUTH2_CLIENT_ID"); v != "" {
		cfg.OAuth2.ClientID = v
	}
	if v := os.Getenv("OAUTH2_CLIENT_SECRET"); v != "" {
		cfg.OAuth2.ClientSecret = v
	}
	if v := os.Getenv("DL_TOKEN_SECRET"); v != "" {
		cfg.DownloadToken.Secret = v
	}
}
