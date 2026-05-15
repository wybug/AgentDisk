package config

import (
	"os"
	"testing"
)

func TestOverrideFromEnv(t *testing.T) {
	cfg := &Config{}
	cfg.Database.Password = "file-value"
	cfg.OSS.AccessKey = "file-ak"
	cfg.OSS.SecretKey = "file-sk"
	cfg.Redis.Password = "file-redis"
	cfg.JWT.Secret = "file-jwt"

	os.Setenv("DB_PASSWORD", "env-db-pass")
	os.Setenv("OSS_ACCESS_KEY", "env-ak")
	os.Setenv("OSS_SECRET_KEY", "env-sk")
	os.Setenv("REDIS_PASSWORD", "env-redis")
	os.Setenv("JWT_SECRET", "env-jwt")
	defer func() {
		os.Unsetenv("DB_PASSWORD")
		os.Unsetenv("OSS_ACCESS_KEY")
		os.Unsetenv("OSS_SECRET_KEY")
		os.Unsetenv("REDIS_PASSWORD")
		os.Unsetenv("JWT_SECRET")
	}()

	overrideFromEnv(cfg)

	if cfg.Database.Password != "env-db-pass" {
		t.Errorf("DB_PASSWORD not overridden, got %s", cfg.Database.Password)
	}
	if cfg.OSS.AccessKey != "env-ak" {
		t.Errorf("OSS_ACCESS_KEY not overridden, got %s", cfg.OSS.AccessKey)
	}
	if cfg.OSS.SecretKey != "env-sk" {
		t.Errorf("OSS_SECRET_KEY not overridden, got %s", cfg.OSS.SecretKey)
	}
	if cfg.Redis.Password != "env-redis" {
		t.Errorf("REDIS_PASSWORD not overridden, got %s", cfg.Redis.Password)
	}
	if cfg.JWT.Secret != "env-jwt" {
		t.Errorf("JWT_SECRET not overridden, got %s", cfg.JWT.Secret)
	}
}

func TestOverrideFromEnv_NotSet(t *testing.T) {
	cfg := &Config{}
	cfg.Database.Password = "file-value"

	os.Unsetenv("DB_PASSWORD")
	overrideFromEnv(cfg)

	if cfg.Database.Password != "file-value" {
		t.Error("should keep config value when env not set")
	}
}

func TestOverrideFromEnv_EmptyEnv(t *testing.T) {
	cfg := &Config{}
	cfg.Database.Password = "file-value"

	os.Setenv("DB_PASSWORD", "")
	overrideFromEnv(cfg)
	os.Unsetenv("DB_PASSWORD")

	if cfg.Database.Password != "file-value" {
		t.Error("empty env should not override config value")
	}
}

func TestLoadDotEnv(t *testing.T) {
	tmpDir := t.TempDir()
	envFile := tmpDir + "/.env"
	os.WriteFile(envFile, []byte("TEST_ABC=123\n# comment\nTEST_DEF=456\n\n"), 0644)

	oldWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldWd)

	os.Unsetenv("TEST_ABC")
	os.Unsetenv("TEST_DEF")

	loadDotEnv()

	if os.Getenv("TEST_ABC") != "123" {
		t.Errorf("expected 123, got %s", os.Getenv("TEST_ABC"))
	}
	if os.Getenv("TEST_DEF") != "456" {
		t.Errorf("expected 456, got %s", os.Getenv("TEST_DEF"))
	}
}

func TestLoadDotEnv_RealEnvPrecedence(t *testing.T) {
	tmpDir := t.TempDir()
	envFile := tmpDir + "/.env"
	os.WriteFile(envFile, []byte("TEST_PREC=from_dotenv\n"), 0644)

	os.Setenv("TEST_PREC", "from_real_env")
	defer os.Unsetenv("TEST_PREC")

	oldWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldWd)

	loadDotEnv()

	if os.Getenv("TEST_PREC") != "from_real_env" {
		t.Error("real env should take precedence over .env file")
	}
}
