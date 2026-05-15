package sql

import (
	"fmt"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

type Config struct {
	Host     string
	Port     int
	User     string
	Password string
	DbName   string
}

func OpenPgSql(config *Config) (*gorm.DB, error) {
	return OpenPgSqlWithLogger(config, nil)
}

func OpenPgSqlWithLogger(config *Config, logger gormlogger.Interface) (*gorm.DB, error) {
	dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		config.Host, config.Port, config.User, config.Password, config.DbName)
	if logger == nil {
		logger = NewSlogGormLogger(nil)
	}
	gormConfig := &gorm.Config{
		Logger:      logger,
		QueryFields: true,
	}
	db, err := gorm.Open(postgres.Open(dsn), gormConfig)
	if err != nil {
		return nil, err
	}
	return db, nil
}
