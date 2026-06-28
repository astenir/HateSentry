package database

import (
	"context"
	"fmt"
	"hatesentry/internal/config"
	"hatesentry/internal/errors"
	"hatesentry/internal/models"
	"log"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

// Initialize initializes the database connection
func Initialize(cfg *config.DatabaseConfig) error {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=%s&parseTime=%t&loc=%s",
		cfg.Username,
		cfg.Password,
		cfg.Host,
		cfg.Port,
		cfg.Database,
		cfg.Charset,
		cfg.ParseTime,
		cfg.Loc,
	)

	var gormLogger logger.Interface
	if cfg.ParseTime {
		gormLogger = logger.Default.LogMode(logger.Info)
	} else {
		gormLogger = logger.Default.LogMode(logger.Silent)
	}

	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{
		Logger: gormLogger,
	})
	if err != nil {
		return errors.DatabaseError(err, "failed to connect to database")
	}

	sqlDB, err := db.DB()
	if err != nil {
		return errors.DatabaseError(err, "failed to get database instance")
	}

	// Validate and set connection pool settings
	if cfg.MaxIdleConns > cfg.MaxOpenConns {
		cfg.MaxIdleConns = cfg.MaxOpenConns
	}
	if cfg.ConnMaxLifetime < 30*time.Minute {
		cfg.ConnMaxLifetime = 30 * time.Minute
	}

	sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	sqlDB.SetConnMaxLifetime(cfg.ConnMaxLifetime)

	// Test connection
	if err := sqlDB.Ping(); err != nil {
		return errors.DatabaseError(err, "failed to ping database")
	}

	DB = db

	// Auto migrate tables
	if err := AutoMigrate(); err != nil {
		return errors.DatabaseError(err, "failed to auto migrate")
	}

	log.Println("Database connection established successfully")
	return nil
}

// AutoMigrate auto migrates all models
func AutoMigrate() error {
	return DB.AutoMigrate(
		&models.User{},
		&models.DetectionRequest{},
		&models.DetectionResult{},
		&models.ModerationRequest{},
		&models.ModerationResult{},
		&models.ReviewCase{},
		&models.DetectionStats{},
		&models.AuditLog{},
	)
}

// Close closes the database connection
func Close() error {
	if DB == nil {
		return nil
	}

	sqlDB, err := DB.DB()
	if err != nil {
		return err
	}

	return sqlDB.Close()
}

// GetDB returns the database instance
func GetDB() *gorm.DB {
	return DB
}

// WithTransaction executes a function within a transaction
func WithTransaction(fn func(tx *gorm.DB) error) error {
	return DB.Transaction(fn)
}

// HealthCheck checks the database health
func HealthCheck() error {
	if DB == nil {
		return errors.ConfigurationError("database not initialized")
	}

	sqlDB, err := DB.DB()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := sqlDB.PingContext(ctx); err != nil {
		return errors.DatabaseError(err, "database health check failed")
	}

	return nil
}
