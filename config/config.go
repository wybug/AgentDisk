package config

import (
	"bufio"
	"log"
	"os"
	"path/filepath"
	"strconv"
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
	DownloadToken DownloadTokenConfig `mapstructure:"download_token"`
	WebAuthn      WebAuthnConfig      `mapstructure:"webauthn"`
	Okf           OkfConfig           `mapstructure:"okf"`
	Features      FeaturesConfig      `mapstructure:"features"`
}

// ServerConfig represents a serverconfiguration.
type ServerConfig struct {
	Port        string `mapstructure:"port"`
	Mode        string `mapstructure:"mode"`
	FrontendURL string `mapstructure:"frontend_url"`
}

// DatabaseConfig represents a databaseconfiguration.
type DatabaseConfig struct {
	Driver       string `mapstructure:"driver"`
	Path         string `mapstructure:"path"`
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
	Driver    string `mapstructure:"driver"`
	Path      string `mapstructure:"path"`
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

// DownloadTokenConfig represents a downloadtokenconfiguration.
type DownloadTokenConfig struct {
	Secret        string `mapstructure:"secret"`
	ExpireSeconds int    `mapstructure:"expire_seconds"`
}

// WebAuthnConfig represents WebAuthn/FIDO2 passkey configuration.
type WebAuthnConfig struct {
	Enabled       bool   `mapstructure:"enabled"`
	RPDisplayName string `mapstructure:"rp_display_name"`
	RPID          string `mapstructure:"rp_id"`
	RPOrigins     string `mapstructure:"rp_origins"`
	Timeout       int    `mapstructure:"timeout"`
}

// OkfConfig configures the OKF v0.1 bundle APIs. When Enabled is false the OKF
// reader routes are not registered and the public-directory content writer
// skips OKF materialization (the original pdWrite text behavior is unchanged).
type OkfConfig struct {
	Enabled         bool `mapstructure:"enabled"`
	AutoIndexUpdate bool `mapstructure:"autoIndexUpdate"` // default true via ApplyDefaults
	LockTTLSeconds  int  `mapstructure:"lockTTLSeconds"`  // default 5 via ApplyDefaults
	// RedisAddr is the optional Redis address used by the OKF bundle writer
	// lock. When empty, the writer falls back to a no-op lock (last-writer-
	// wins). Operators that want strict serialization must point this at the
	// same Redis the rest of the process uses.
	RedisAddr string `mapstructure:"redisAddr"`
}

// FeaturesConfig carries runtime-toggleable feature flags. Unlike Okf.Enabled
// (which is read once at boot and gates route registration), these flags are
// hot-reloaded from config.yaml and can be flipped at runtime via the
// /v1/disk/admin/features admin API. All flags default to true; operators opt
// out by setting `features.<name>: false` in config.yaml or by PATCHing the
// admin endpoint.
type FeaturesConfig struct {
	// OkfReader gates the OKF reader routes (list/get bundles, nodes, search,
	// graph queries). When flipped to false at runtime, the routes return 403
	// until re-enabled. Master switch remains cfg.Okf.Enabled at boot.
	OkfReader bool `mapstructure:"okfReader"`
	// OkfWriter gates the OKF writer routes (write_markdown, refresh bundle,
	// register/unregister). Reader routes remain available when writer is off.
	OkfWriter bool `mapstructure:"okfWriter"`
	// OkfGraphBFS gates the BFS-heavy routes (reachable, shortest-path,
	// subgraph, neighbors). When off, the lighter reader routes still work,
	// letting operators shed BFS load without disabling the whole reader.
	OkfGraphBFS bool `mapstructure:"okfGraphBFS"`
}

// Load handles HTTP requests.
//
// viper's package-level functions (SetConfigFile, ReadInConfig, Unmarshal, …)
// all operate on a single global default instance. Concurrent Load calls —
// e.g. the fsnotify-reload goroutine inside feature.Registry racing a test —
// therefore race on that shared state. We sidestep the global by constructing
// a fresh viper.New() instance per call and threading it into applyDefaults
// for its IsSet checks.
func Load(path string) (*Config, error) {
	loadDotEnv()

	v := viper.New()
	v.SetConfigFile(path)
	v.AutomaticEnv()

	if err := v.ReadInConfig(); err != nil {
		return nil, err
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, err
	}

	// Sensitive fields: environment variables override config file
	overrideFromEnv(&cfg)

	// Expand ~ in paths to home directory
	expandPaths(&cfg)

	// Apply defaults for optional toggles. v.IsSet distinguishes "key
	// absent" from "key explicitly false", so the default is only applied
	// when the operator did not write the key at all.
	applyDefaults(v, &cfg)

	return &cfg, nil
}

// applyDefaults fills in defaults for fields that should be "on" unless the
// operator explicitly disables them. This keeps existing config files forward
// compatible when a new opt-in feature is added. The viper instance is passed
// in so IsSet reads from the same source Load just parsed, without touching
// the package-level global default.
func applyDefaults(v *viper.Viper, cfg *Config) {
	// OKF is enabled by default. Operators opt out by setting
	// `okf.enabled: false` in config.yaml or OKF_ENABLED=false in the env.
	if envVal := os.Getenv("OKF_ENABLED"); envVal == "true" || envVal == "false" {
		cfg.Okf.Enabled = envVal == "true"
	} else if !v.IsSet("okf.enabled") {
		cfg.Okf.Enabled = true
	}
	// AutoIndexUpdate is on by default. v.IsSet distinguishes "key
	// absent" from "key explicitly false", so the default is only applied
	// when the operator did not write the key at all.
	if envVal := os.Getenv("OKF_AUTO_INDEX_UPDATE"); envVal == "true" || envVal == "false" {
		cfg.Okf.AutoIndexUpdate = envVal == "true"
	} else if !v.IsSet("okf.autoIndexUpdate") {
		cfg.Okf.AutoIndexUpdate = true
	}
	// LockTTLSeconds defaults to 5s, the OKF spec's recommended ceiling for
	// a markdown write. Operators that take longer writes can bump it.
	if cfg.Okf.LockTTLSeconds <= 0 {
		if envVal := os.Getenv("OKF_LOCK_TTL_SECONDS"); envVal != "" {
			if n, err := strconv.Atoi(envVal); err == nil && n > 0 {
				cfg.Okf.LockTTLSeconds = n
			}
		}
	}
	if cfg.Okf.LockTTLSeconds <= 0 {
		cfg.Okf.LockTTLSeconds = 5
	}
	if envVal := os.Getenv("OKF_REDIS_ADDR"); envVal != "" {
		cfg.Okf.RedisAddr = envVal
	}
	// Features default to ON. An operator who wants one off must explicitly
	// set it false in config.yaml or patch it at runtime via the admin API.
	// v.IsSet distinguishes "key absent" from "key explicitly false".
	if !v.IsSet("features.okfReader") {
		cfg.Features.OkfReader = true
	}
	if !v.IsSet("features.okfWriter") {
		cfg.Features.OkfWriter = true
	}
	if !v.IsSet("features.okfGraphBFS") {
		cfg.Features.OkfGraphBFS = true
	}
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
	if v := os.Getenv("DB_DRIVER"); v != "" {
		cfg.Database.Driver = v
	}
	if v := os.Getenv("DB_PATH"); v != "" {
		cfg.Database.Path = v
	}
	if v := os.Getenv("DB_PASSWORD"); v != "" {
		cfg.Database.Password = v
	}
	if v := os.Getenv("STORAGE_DRIVER"); v != "" {
		cfg.OSS.Driver = v
	}
	if v := os.Getenv("STORAGE_PATH"); v != "" {
		cfg.OSS.Path = v
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
	if v := os.Getenv("FRONTEND_URL"); v != "" {
		cfg.Server.FrontendURL = v
	}
	if v := os.Getenv("DL_TOKEN_SECRET"); v != "" {
		cfg.DownloadToken.Secret = v
	}
}

// expandPaths expands ~ to the user's home directory in path fields.
func expandPaths(cfg *Config) {
	cfg.Database.Path = expandHome(cfg.Database.Path)
	cfg.OSS.Path = expandHome(cfg.OSS.Path)
}

func expandHome(path string) string {
	if path == "" {
		return path
	}
	if path[0] == '~' {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return filepath.Join(homeDir, path[1:])
	}
	return path
}
