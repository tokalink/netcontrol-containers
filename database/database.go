package database

import (
	"log"
	"os"
	"path/filepath"

	"netcontrol-containers/config"
	"netcontrol-containers/models"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

func Init() error {
	cfg := config.Get()

	// Ensure data directory exists
	dir := filepath.Dir(cfg.DBPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	// Configure logger
	logLevel := logger.Silent
	if cfg.DebugMode {
		logLevel = logger.Info
	}

	// Open database
	db, err := gorm.Open(sqlite.Open(cfg.DBPath), &gorm.Config{
		Logger: logger.Default.LogMode(logLevel),
	})
	if err != nil {
		return err
	}

	// Auto migrate
	if err := db.AutoMigrate(&models.User{}, &models.Settings{}); err != nil {
		return err
	}

	// Seed default admin user
	var count int64
	db.Model(&models.User{}).Count(&count)
	if count == 0 {
		admin := &models.User{Username: "admin"}
		if err := admin.SetPassword("admin123"); err != nil {
			return err
		}
		if err := db.Create(admin).Error; err != nil {
			return err
		}
		log.Println("Default admin user created (admin:admin123)")
	}

	DB = db
	return nil
}

func Get() *gorm.DB {
	return DB
}
