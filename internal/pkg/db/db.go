package db

import (
	"fmt"
	"time"

	gosqlmysql "github.com/go-sql-driver/mysql"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"has-smartlock-service/internal/pkg/config"
)

func Open(cfg config.Config) (*gorm.DB, error) {
	if cfg.DBDSN == "" {
		return nil, fmt.Errorf("mysql dsn required")
	}

	dsn, err := normalizeMySQLDSN(cfg.DBDSN)
	if err != nil {
		return nil, err
	}

	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return nil, err
	}

	return db, nil
}

func normalizeMySQLDSN(dsn string) (string, error) {
	cfg, err := gosqlmysql.ParseDSN(dsn)
	if err != nil {
		return "", err
	}

	cfg.ParseTime = true
	cfg.MultiStatements = true
	if cfg.Loc == nil {
		cfg.Loc = time.Local
	}

	return cfg.FormatDSN(), nil
}
