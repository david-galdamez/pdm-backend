package config

import (
	"log"
	"os"
	"sync"

	"github.com/joho/godotenv"
)

type Config struct {
	PORT         string
	ENV          string
	DATABASE_URL string
	JWT_SECRET   string
}

func load() Config {

	env := os.Getenv("ENV")
	if env == "" {
		log.Println("ENV environment variable is not set, using default value 'development'")
		env = "development"
	}

	if os.Getenv("ENV") != "production" {
		err := godotenv.Load(".env")
		if err != nil {
			log.Println("Could not load .env (this is expected in production)")
		}
	}

	port := os.Getenv("PORT")
	if port == "" {
		log.Println("PORT environment variable is not set, using default port 8080")
		port = "8080"
	}

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Println("DATABASE_URL environment variable is not set, using default value 'postgres://user:password@localhost:5432/dbname'")
		databaseURL = "postgres://user:password@localhost:5432/dbname"
	}

	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		log.Fatal("JWT_SECRET environment variable is not set")
	}

	if len(secret) < 32 {
		log.Fatal("JWT_SECRET environment variable must be at least 32 characters long")
	}

	return Config{
		PORT:         port,
		ENV:          env,
		DATABASE_URL: databaseURL,
		JWT_SECRET:   secret,
	}
}

var (
	cfg  Config
	once sync.Once
)

// Get returns the process-wide configuration, loading it on first use.
func Get() Config {
	once.Do(func() {
		cfg = load()
	})

	return cfg
}
