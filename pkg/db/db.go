package db

import (
	"bs-notify-hub/internal/conf"
	"bs-notify-hub/internal/model"
	"sync"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var (
	db   *gorm.DB
	once sync.Once
)

func InitDB(cfg conf.PostgresConfig) error {
	var err error

	once.Do(func() {
		db, err = gorm.Open(postgres.Open(cfg.DSN()), &gorm.Config{})
		if err != nil {
			return
		}

		sqlDB, _ := db.DB()
		sqlDB.SetMaxIdleConns(10)
		sqlDB.SetMaxOpenConns(100)

		err = db.AutoMigrate(
			&model.NotifyRecord{},
			&model.NotifyStatus{},
			&model.NotifyWatermark{},
		)
		if err != nil {
			return
		}
	})

	return err
}

func GetDB() *gorm.DB {
	if db == nil {
		panic("db 未初始化")
	}
	return db
}
