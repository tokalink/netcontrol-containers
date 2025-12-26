package models

import (
	"time"

	"gorm.io/gorm"
)

type Settings struct {
	ID        uint           `gorm:"primarykey" json:"id"`
	Key       string         `gorm:"uniqueIndex;size:100" json:"key"`
	Value     string         `gorm:"type:text" json:"value"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

func GetSetting(db *gorm.DB, key string) string {
	var setting Settings
	db.Where("key = ?", key).First(&setting)
	return setting.Value
}

func SetSetting(db *gorm.DB, key, value string) error {
	var setting Settings
	result := db.Where("key = ?", key).First(&setting)
	if result.Error != nil {
		setting = Settings{Key: key, Value: value}
		return db.Create(&setting).Error
	}
	setting.Value = value
	return db.Save(&setting).Error
}
