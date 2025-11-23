package database

import (
	"context"
	"fmt"
	"time"

	"github.com/enterprise-knowledge-base/ekb/pkg/config"
	"github.com/enterprise-knowledge-base/ekb/pkg/logger"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

type Database struct {
	*gorm.DB
}

var db *Database

func Init(cfg *config.DatabaseConfig) (*Database, error) {
	if db != nil {
		return db, nil
	}

	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		cfg.Username, cfg.Password, cfg.Host, cfg.Port, cfg.Database)

	gormConfig := &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Info),
	}

	connection, err := gorm.Open(mysql.Open(dsn), gormConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	sqlDB, err := connection.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get database instance: %w", err)
	}

	sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(cfg.MaxLifetime)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := sqlDB.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	db = &Database{connection}
	logger.Info("Database connection established successfully")
	
	return db, nil
}

func Get() *Database {
	if db == nil {
		cfg := config.Get()
		database, err := Init(&cfg.Database)
		if err != nil {
			logger.Fatalf("failed to initialize database: %v", err)
		}
		db = database
	}
	return db
}

func (d *Database) Close() error {
	sqlDB, err := d.DB.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

func (d *Database) Ping(ctx context.Context) error {
	sqlDB, err := d.DB.DB()
	if err != nil {
		return err
	}
	return sqlDB.PingContext(ctx)
}

func (d *Database) Transaction(ctx context.Context, fn func(*gorm.DB) error) error {
	return d.DB.WithContext(ctx).Transaction(fn)
}

func AutoMigrate(models ...interface{}) error {
	db := Get()
	if err := db.AutoMigrate(models...); err != nil {
		return fmt.Errorf("failed to auto migrate: %w", err)
	}
	logger.Info("Database migration completed successfully")
	return nil
}