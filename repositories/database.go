package repositories

import (
	"log"
	"pdm-backend/internal/config"
	"sync"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var (
	db   *gorm.DB
	once sync.Once
)

func GetDB() *gorm.DB {
	once.Do(func() {

		DB, err := gorm.Open(postgres.Open(config.Get().DATABASE_URL), &gorm.Config{})
		if err != nil {
			log.Fatal("Error connecting to the database: ", err)
		}

		db = DB
	})

	return db
}
