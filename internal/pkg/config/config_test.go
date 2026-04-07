package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadUsesFallbackWithoutDotEnv(t *testing.T) {
	unsetEnv(t,
		"CONFIG_FILE",
		"APP_NAME",
		"APP_ENV",
		"HTTP_ADDR",
		"DB_DSN",
		"MYSQL_DSN",
		"JWT_SECRET",
		"ACCESS_TOKEN_TTL_SECONDS",
		"REFRESH_TOKEN_TTL_SECONDS",
		"VERIFICATION_CODE_TTL_SECONDS",
		"OSS_ENDPOINT",
		"OSS_BUCKET_NAME",
		"OSS_PUBLIC_BASE_URL",
		"OSS_ACCESS_KEY_ID",
		"OSS_ACCESS_KEY_SECRET",
		"OSS_AVATAR_PREFIX",
		"OSS_UPLOAD_URL_TTL_SECONDS",
		"OSS_SIGNED_READ_URL_TTL_SECONDS",
		"OSS_STS_ROLE_ARN",
		"OSS_STS_SESSION_PREFIX",
		"OSS_STS_DURATION_SECONDS",
	)

	inTempDirWithoutRepoRoot(t)

	cfg := Load()
	if cfg.AppName != "has-smartlock-service" {
		t.Fatalf("AppName = %q, want has-smartlock-service", cfg.AppName)
	}
	if cfg.HTTPAddr != ":8080" {
		t.Fatalf("HTTPAddr = %q, want :8080", cfg.HTTPAddr)
	}
	if cfg.OSSAvatarPrefix != "avatar" {
		t.Fatalf("OSSAvatarPrefix = %q, want avatar", cfg.OSSAvatarPrefix)
	}
	if cfg.OSSSignedReadURLTTL != 900 {
		t.Fatalf("OSSSignedReadURLTTL = %d, want 900", cfg.OSSSignedReadURLTTL)
	}
	if cfg.OSSSTSSessionPrefix != "has-smartlock-avatar" {
		t.Fatalf("OSSSTSSessionPrefix = %q, want has-smartlock-avatar", cfg.OSSSTSSessionPrefix)
	}
}

func TestLoadUsesDotEnvWhenShellEnvMissing(t *testing.T) {
	unsetEnv(t, "CONFIG_FILE", "APP_NAME", "HTTP_ADDR", "OSS_BUCKET_NAME", "OSS_PUBLIC_BASE_URL")

	repoRoot := setupFakeRepoRoot(t, map[string]string{
		".env": "APP_NAME=from-dotenv\nHTTP_ADDR=:9090\nOSS_BUCKET_NAME=test-bucket\nOSS_PUBLIC_BASE_URL=https://cdn.example.com\n",
	})
	subDir := filepath.Join(repoRoot, "internal", "pkg", "config")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatalf("mkdir subdir: %v", err)
	}
	changeDir(t, subDir)

	cfg := Load()
	if cfg.AppName != "from-dotenv" {
		t.Fatalf("AppName = %q, want from-dotenv", cfg.AppName)
	}
	if cfg.HTTPAddr != ":9090" {
		t.Fatalf("HTTPAddr = %q, want :9090", cfg.HTTPAddr)
	}
	if cfg.OSSBucketName != "test-bucket" {
		t.Fatalf("OSSBucketName = %q, want test-bucket", cfg.OSSBucketName)
	}
	if cfg.OSSPublicBaseURL != "https://cdn.example.com" {
		t.Fatalf("OSSPublicBaseURL = %q, want https://cdn.example.com", cfg.OSSPublicBaseURL)
	}
}

func TestLoadPrefersShellEnvOverDotEnv(t *testing.T) {
	unsetEnv(t, "CONFIG_FILE", "APP_NAME", "JWT_SECRET")
	t.Setenv("APP_NAME", "from-shell")
	t.Setenv("JWT_SECRET", "shell-secret")

	repoRoot := setupFakeRepoRoot(t, map[string]string{
		".env": "APP_NAME=from-dotenv\nJWT_SECRET=dotenv-secret\n",
	})
	changeDir(t, repoRoot)

	cfg := Load()
	if cfg.AppName != "from-shell" {
		t.Fatalf("AppName = %q, want from-shell", cfg.AppName)
	}
	if cfg.JWTSecret != "shell-secret" {
		t.Fatalf("JWTSecret = %q, want shell-secret", cfg.JWTSecret)
	}
}

func TestLoadUsesConfigFileWhenProvided(t *testing.T) {
	unsetEnv(t, "CONFIG_FILE", "APP_NAME", "HTTP_ADDR")

	repoRoot := setupFakeRepoRoot(t, map[string]string{
		".env": "APP_NAME=from-repo-dotenv\nHTTP_ADDR=:8080\n",
	})
	customConfig := filepath.Join(t.TempDir(), "custom.env")
	content := "APP_NAME=from-config-file\nHTTP_ADDR=:9999\n"
	if err := os.WriteFile(customConfig, []byte(content), 0o600); err != nil {
		t.Fatalf("write custom env: %v", err)
	}
	t.Setenv("CONFIG_FILE", customConfig)
	changeDir(t, repoRoot)

	cfg := Load()
	if cfg.AppName != "from-config-file" {
		t.Fatalf("AppName = %q, want from-config-file", cfg.AppName)
	}
	if cfg.HTTPAddr != ":9999" {
		t.Fatalf("HTTPAddr = %q, want :9999", cfg.HTTPAddr)
	}
}

func TestEnvExampleContainsCurrentConfigKeys(t *testing.T) {
	content, err := os.ReadFile(filepath.Join("..", "..", "..", ".env.example"))
	if err != nil {
		t.Fatalf("read .env.example: %v", err)
	}

	text := string(content)
	requiredKeys := []string{
		"APP_NAME=",
		"APP_ENV=",
		"HTTP_ADDR=",
		"DB_DSN=",
		"MYSQL_DSN=",
		"JWT_SECRET=",
		"ACCESS_TOKEN_TTL_SECONDS=",
		"REFRESH_TOKEN_TTL_SECONDS=",
		"VERIFICATION_CODE_TTL_SECONDS=",
		"OSS_ENDPOINT=",
		"OSS_BUCKET_NAME=",
		"OSS_PUBLIC_BASE_URL=",
		"OSS_ACCESS_KEY_ID=",
		"OSS_ACCESS_KEY_SECRET=",
		"OSS_AVATAR_PREFIX=",
		"OSS_UPLOAD_URL_TTL_SECONDS=",
		"OSS_SIGNED_READ_URL_TTL_SECONDS=",
		"OSS_STS_ROLE_ARN=",
		"OSS_STS_SESSION_PREFIX=",
		"OSS_STS_DURATION_SECONDS=",
	}

	for _, key := range requiredKeys {
		if !strings.Contains(text, key) {
			t.Fatalf(".env.example missing key %s", key)
		}
	}
}

func unsetEnv(t *testing.T, keys ...string) {
	t.Helper()

	for _, key := range keys {
		original, exists := os.LookupEnv(key)
		if err := os.Unsetenv(key); err != nil {
			t.Fatalf("unset %s: %v", key, err)
		}
		t.Cleanup(func() {
			if exists {
				_ = os.Setenv(key, original)
				return
			}
			_ = os.Unsetenv(key)
		})
	}
}

func changeDir(t *testing.T, dir string) {
	t.Helper()

	originalWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir %s: %v", dir, err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(originalWD)
	})
}

func setupFakeRepoRoot(t *testing.T, files map[string]string) string {
	t.Helper()

	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module test\n"), 0o600); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}
	for name, content := range files {
		path := filepath.Join(root, name)
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	return root
}

func inTempDirWithoutRepoRoot(t *testing.T) {
	t.Helper()
	changeDir(t, t.TempDir())
}
