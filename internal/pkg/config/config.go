package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
)

type Config struct {
	AppName               string
	AppEnv                string
	HTTPAddr              string
	DBDSN                 string
	MySQLDSN              string
	JWTSecret             string
	AccessTokenTTL        int
	RefreshTokenTTL       int
	VerificationTTL       int
	AppSecretKey          string
	DeviceModelSecretsRaw string
	DeviceModelSecrets    map[string]string
	DeviceModelsRaw       string
	DeviceModels          []DeviceModelConfig
	DeviceUpgradesRaw     string
	DeviceUpgrades        []DeviceUpgradeConfig
	SignTimestampSkew     int
	OSSEndpoint           string
	OSSBucketName         string
	OSSPublicBaseURL      string
	OSSAccessKeyID        string
	OSSAccessKeySecret    string
	OSSAvatarPrefix       string
	OSSUploadURLTTL       int
	OSSSignedReadURLTTL   int
	OSSSTSRoleARN         string
	OSSSTSSessionPrefix   string
	OSSSTSDuration        int
}

type DeviceModelConfig struct {
	ModelCode   string `json:"model_code"`
	Status      int    `json:"status"`
	ModelName   string `json:"model_name"`
	Category    string `json:"category"`
	ShowName    string `json:"show_name"`
	DefaultName string `json:"default_name"`
	Thumbnail   string `json:"thumbnail"`
}

type DeviceUpgradeConfig struct {
	ModelCode string `json:"model_code"`
	Flag      string `json:"flag"`
	Version   string `json:"version"`
}

func Load() Config {
	v := newViper()

	cfg := Config{
		AppName:               v.GetString("APP_NAME"),
		AppEnv:                v.GetString("APP_ENV"),
		HTTPAddr:              v.GetString("HTTP_ADDR"),
		DBDSN:                 v.GetString("DB_DSN"),
		MySQLDSN:              v.GetString("MYSQL_DSN"),
		JWTSecret:             v.GetString("JWT_SECRET"),
		AccessTokenTTL:        v.GetInt("ACCESS_TOKEN_TTL_SECONDS"),
		RefreshTokenTTL:       v.GetInt("REFRESH_TOKEN_TTL_SECONDS"),
		VerificationTTL:       v.GetInt("VERIFICATION_CODE_TTL_SECONDS"),
		AppSecretKey:          v.GetString("APP_SECRET_KEY"),
		DeviceModelSecretsRaw: v.GetString("DEVICE_MODEL_SECRETS"),
		DeviceModelsRaw:       v.GetString("DEVICE_MODELS"),
		DeviceUpgradesRaw:     v.GetString("DEVICE_UPGRADES"),
		SignTimestampSkew:     v.GetInt("SIGN_TIMESTAMP_SKEW_SECONDS"),
		OSSEndpoint:           v.GetString("OSS_ENDPOINT"),
		OSSBucketName:         v.GetString("OSS_BUCKET_NAME"),
		OSSPublicBaseURL:      v.GetString("OSS_PUBLIC_BASE_URL"),
		OSSAccessKeyID:        v.GetString("OSS_ACCESS_KEY_ID"),
		OSSAccessKeySecret:    v.GetString("OSS_ACCESS_KEY_SECRET"),
		OSSAvatarPrefix:       v.GetString("OSS_AVATAR_PREFIX"),
		OSSUploadURLTTL:       v.GetInt("OSS_UPLOAD_URL_TTL_SECONDS"),
		OSSSignedReadURLTTL:   v.GetInt("OSS_SIGNED_READ_URL_TTL_SECONDS"),
		OSSSTSRoleARN:         v.GetString("OSS_STS_ROLE_ARN"),
		OSSSTSSessionPrefix:   v.GetString("OSS_STS_SESSION_PREFIX"),
		OSSSTSDuration:        v.GetInt("OSS_STS_DURATION_SECONDS"),
	}

	if raw := strings.TrimSpace(cfg.DeviceModelSecretsRaw); raw != "" {
		modelSecrets := make(map[string]string)
		if err := json.Unmarshal([]byte(raw), &modelSecrets); err == nil {
			cfg.DeviceModelSecrets = modelSecrets
		}
	}
	if raw := strings.TrimSpace(cfg.DeviceModelsRaw); raw != "" {
		var models []DeviceModelConfig
		if err := json.Unmarshal([]byte(raw), &models); err == nil {
			cfg.DeviceModels = models
		}
	}
	if raw := strings.TrimSpace(cfg.DeviceUpgradesRaw); raw != "" {
		var upgrades []DeviceUpgradeConfig
		if err := json.Unmarshal([]byte(raw), &upgrades); err == nil {
			cfg.DeviceUpgrades = upgrades
		}
	}

	if cfg.MySQLDSN != "" {
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
	v.SetDefault("DB_DSN", "")
	v.SetDefault("MYSQL_DSN", "")
	v.SetDefault("JWT_SECRET", "dev-secret-change-me")
	v.SetDefault("ACCESS_TOKEN_TTL_SECONDS", 7200)
	v.SetDefault("REFRESH_TOKEN_TTL_SECONDS", 2592000)
	v.SetDefault("VERIFICATION_CODE_TTL_SECONDS", 300)
	v.SetDefault("APP_SECRET_KEY", "")
	v.SetDefault("DEVICE_MODEL_SECRETS", "")
	v.SetDefault("DEVICE_MODELS", "")
	v.SetDefault("DEVICE_UPGRADES", "")
	v.SetDefault("SIGN_TIMESTAMP_SKEW_SECONDS", 300)
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
