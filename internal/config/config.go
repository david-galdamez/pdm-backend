package config

import (
	"log"
	"os"
	"strings"
	"sync"

	"github.com/joho/godotenv"
)

type Config struct {
	PORT         string
	ENV          string
	DATABASE_URL string
	JWT_SECRET   string
	// ALLOWED_ORIGINS is the CORS allowlist. Native mobile clients send no
	// Origin header and are unaffected by it; it only constrains browsers.
	ALLOWED_ORIGINS []string
}

// splitAndTrim turns "a, b ,c" into ["a","b","c"], dropping empty entries.
func splitAndTrim(value string) []string {
	var out []string

	for part := range strings.SplitSeq(value, ",") {
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, part)
		}
	}

	return out
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

	origins := splitAndTrim(os.Getenv("ALLOWED_ORIGINS"))
	if len(origins) == 0 {
		if env == "production" {
			log.Fatal("ALLOWED_ORIGINS environment variable must be set in production (comma-separated list of web origins)")
		}
		origins = []string{"http://localhost:3000", "http://localhost:5173"}
		log.Println("ALLOWED_ORIGINS environment variable is not set, using default development origins:", origins)
	}

	for _, origin := range origins {
		if origin == "*" {
			log.Fatal("ALLOWED_ORIGINS must not contain '*': list the exact web origins instead")
		}
	}

	return Config{
		PORT:         port,
		ENV:          env,
		DATABASE_URL: databaseURL,
		JWT_SECRET:   secret,

		ALLOWED_ORIGINS: origins,
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
