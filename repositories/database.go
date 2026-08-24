package repositories

import (
	"log"
	"os"
	"sync"

	"github.com/joho/godotenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var (
	db   *gorm.DB
	once sync.Once
)

func GetDB() *gorm.DB {
	once.Do(func() {
		if os.Getenv("ENV") != "production" {
			err := godotenv.Load(".env")
			if err != nil {
				log.Println("Could not load .env (this is expected in production)")
			}
		}

		dsn := os.Getenv("POSTGRES_URL")
		if dsn == "" {
			log.Fatal("POSTGRES_URL is not set")
		}

		DB, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
		if err != nil {
			log.Fatal("Error connecting to the database: ", err)
		}

		db = DB
	})

	return db
}
