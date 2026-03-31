package db

import (
	"fmt"

	"gorm.io/driver/mysql"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"has-smartlock-service/internal/pkg/config"
	"has-smartlock-service/internal/user/model"
)

func Open(cfg config.Config) (*gorm.DB, error) {
	var dialector gorm.Dialector

	switch cfg.DBDriver {
	case "mysql":
		dialector = mysql.Open(cfg.DBDSN)
	case "sqlite":
		dialector = sqlite.Open(cfg.DBDSN)
	default:
		return nil, fmt.Errorf("unsupported db driver: %s", cfg.DBDriver)
	}

	db, err := gorm.Open(dialector, &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return nil, err
	}

	if cfg.AutoMigrate {
		if err := db.AutoMigrate(
			&model.User{},
			&model.VerificationCode{},
			&model.RefreshToken{},
			&model.UserClient{},
		); err != nil {
			return nil, err
		}
	}

	return db, nil
}
