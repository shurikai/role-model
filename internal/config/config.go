package config

import (
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	DatabaseURL     string
	AnthropicAPIKey string
	Port            string
	Environment     string
}

func Load() Config {
	_ = godotenv.Load() // loads .env into the environment if present; silent if absent

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	env := os.Getenv("ENVIRONMENT")
	if env == "" {
		env = "development"
	}
	return Config{
		DatabaseURL:     os.Getenv("DATABASE_URL"),
		AnthropicAPIKey: os.Getenv("ANTHROPIC_API_KEY"),
		Port:            port,
		Environment:     env,
	}
}
