package config

import (
	"fmt"
	"os"
)

type Config struct {
	AppName         string
	AppEnv          string
	HTTPAddr        string
	DBDriver        string
	DBDSN           string
	MySQLDSN        string
	AutoMigrate     bool
	JWTSecret       string
	AccessTokenTTL  int
	RefreshTokenTTL int
	VerificationTTL int
}

func Load() Config {
	dbDriver := getEnv("DB_DRIVER", "sqlite")
	dbDSN := getEnv("DB_DSN", "file:has_smartlock_service.db?_foreign_keys=on")
	if mysqlDSN := getEnv("MYSQL_DSN", ""); mysqlDSN != "" {
		dbDriver = "mysql"
		dbDSN = mysqlDSN
	}

	return Config{
		AppName:         getEnv("APP_NAME", "has-smartlock-service"),
		AppEnv:          getEnv("APP_ENV", "development"),
		HTTPAddr:        getEnv("HTTP_ADDR", ":8080"),
		DBDriver:        dbDriver,
		DBDSN:           dbDSN,
		MySQLDSN:        getEnv("MYSQL_DSN", ""),
		AutoMigrate:     getEnv("AUTO_MIGRATE", "true") == "true",
		JWTSecret:       getEnv("JWT_SECRET", "dev-secret-change-me"),
		AccessTokenTTL:  getEnvInt("ACCESS_TOKEN_TTL_SECONDS", 7200),
		RefreshTokenTTL: getEnvInt("REFRESH_TOKEN_TTL_SECONDS", 2592000),
		VerificationTTL: getEnvInt("VERIFICATION_CODE_TTL_SECONDS", 300),
	}
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}

	return fallback
}

func getEnvInt(key string, fallback int) int {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	var parsed int
	_, err := fmt.Sscanf(value, "%d", &parsed)
	if err != nil {
		return fallback
	}

	return parsed
}
