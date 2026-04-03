package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
)

type Config struct {
	AppName             string
	AppEnv              string
	HTTPAddr            string
	DBDriver            string
	DBDSN               string
	MySQLDSN            string
	AutoMigrate         bool
	JWTSecret           string
	AccessTokenTTL      int
	RefreshTokenTTL     int
	VerificationTTL     int
	OSSEndpoint         string
	OSSBucketName       string
	OSSPublicBaseURL    string
	OSSAccessKeyID      string
	OSSAccessKeySecret  string
	OSSAvatarPrefix     string
	OSSUploadURLTTL     int
	OSSSignedReadURLTTL int
	OSSSTSRoleARN       string
	OSSSTSSessionPrefix string
	OSSSTSDuration      int
}

func Load() Config {
	v := newViper()

	cfg := Config{
		AppName:             v.GetString("APP_NAME"),
		AppEnv:              v.GetString("APP_ENV"),
		HTTPAddr:            v.GetString("HTTP_ADDR"),
		DBDriver:            v.GetString("DB_DRIVER"),
		DBDSN:               v.GetString("DB_DSN"),
		MySQLDSN:            v.GetString("MYSQL_DSN"),
		AutoMigrate:         v.GetBool("AUTO_MIGRATE"),
		JWTSecret:           v.GetString("JWT_SECRET"),
		AccessTokenTTL:      v.GetInt("ACCESS_TOKEN_TTL_SECONDS"),
		RefreshTokenTTL:     v.GetInt("REFRESH_TOKEN_TTL_SECONDS"),
		VerificationTTL:     v.GetInt("VERIFICATION_CODE_TTL_SECONDS"),
		OSSEndpoint:         v.GetString("OSS_ENDPOINT"),
		OSSBucketName:       v.GetString("OSS_BUCKET_NAME"),
		OSSPublicBaseURL:    v.GetString("OSS_PUBLIC_BASE_URL"),
		OSSAccessKeyID:      v.GetString("OSS_ACCESS_KEY_ID"),
		OSSAccessKeySecret:  v.GetString("OSS_ACCESS_KEY_SECRET"),
		OSSAvatarPrefix:     v.GetString("OSS_AVATAR_PREFIX"),
		OSSUploadURLTTL:     v.GetInt("OSS_UPLOAD_URL_TTL_SECONDS"),
		OSSSignedReadURLTTL: v.GetInt("OSS_SIGNED_READ_URL_TTL_SECONDS"),
		OSSSTSRoleARN:       v.GetString("OSS_STS_ROLE_ARN"),
		OSSSTSSessionPrefix: v.GetString("OSS_STS_SESSION_PREFIX"),
		OSSSTSDuration:      v.GetInt("OSS_STS_DURATION_SECONDS"),
	}

	if cfg.MySQLDSN != "" {
		cfg.DBDriver = "mysql"
		cfg.DBDSN = cfg.MySQLDSN
	}

	if cfg.OSSPublicBaseURL == "" && cfg.OSSEndpoint != "" && cfg.OSSBucketName != "" {
		cfg.OSSPublicBaseURL = fmt.Sprintf("https://%s.%s", cfg.OSSBucketName, cfg.OSSEndpoint)
	}

	return cfg
}

func newViper() *viper.Viper {
	v := viper.New()
	v.SetConfigType("env")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	setDefaults(v)

	if configFile := strings.TrimSpace(os.Getenv("CONFIG_FILE")); configFile != "" {
		v.SetConfigFile(configFile)
		_ = v.ReadInConfig()
		return v
	}

	if repoRoot, ok := findRepoRoot(); ok {
		v.SetConfigFile(filepath.Join(repoRoot, ".env"))
		_ = v.ReadInConfig()
	}

	return v
}

func setDefaults(v *viper.Viper) {
	v.SetDefault("APP_NAME", "has-smartlock-service")
	v.SetDefault("APP_ENV", "development")
	v.SetDefault("HTTP_ADDR", ":8080")
	v.SetDefault("DB_DRIVER", "sqlite")
	v.SetDefault("DB_DSN", "file:has_smartlock_service.db?_foreign_keys=on")
	v.SetDefault("MYSQL_DSN", "")
	v.SetDefault("AUTO_MIGRATE", true)
	v.SetDefault("JWT_SECRET", "dev-secret-change-me")
	v.SetDefault("ACCESS_TOKEN_TTL_SECONDS", 7200)
	v.SetDefault("REFRESH_TOKEN_TTL_SECONDS", 2592000)
	v.SetDefault("VERIFICATION_CODE_TTL_SECONDS", 300)
	v.SetDefault("OSS_ENDPOINT", "")
	v.SetDefault("OSS_BUCKET_NAME", "")
	v.SetDefault("OSS_PUBLIC_BASE_URL", "")
	v.SetDefault("OSS_ACCESS_KEY_ID", "")
	v.SetDefault("OSS_ACCESS_KEY_SECRET", "")
	v.SetDefault("OSS_AVATAR_PREFIX", "avatar")
	v.SetDefault("OSS_UPLOAD_URL_TTL_SECONDS", 900)
	v.SetDefault("OSS_SIGNED_READ_URL_TTL_SECONDS", 900)
	v.SetDefault("OSS_STS_ROLE_ARN", "")
	v.SetDefault("OSS_STS_SESSION_PREFIX", "has-smartlock-avatar")
	v.SetDefault("OSS_STS_DURATION_SECONDS", 900)
}

func findRepoRoot() (string, bool) {
	wd, err := os.Getwd()
	if err != nil {
		return "", false
	}

	for {
		if _, err := os.Stat(filepath.Join(wd, "go.mod")); err == nil {
			return wd, true
		}

		parent := filepath.Dir(wd)
		if parent == wd {
			return "", false
		}
		wd = parent
	}
}
