package config

import (
	"log"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	DatabaseURL     string
	AnthropicAPIKey string
	Port            string
	Environment     string
	JWTSecret       string
	AllowedOrigins  []string
	RendererURL     string
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

	allowedOrigins := parseAllowedOrigins(os.Getenv("CORS_ALLOWED_ORIGINS"))
	if len(allowedOrigins) == 0 {
		if env == "development" {
			allowedOrigins = []string{"http://localhost:5173"}
		} else {
			log.Println("WARNING: CORS_ALLOWED_ORIGINS is not set; cross-origin requests will be rejected")
		}
	}

	rendererURL := os.Getenv("RENDERER_URL")
	if rendererURL == "" {
		if env == "development" {
			rendererURL = "http://localhost:8000"
		} else {
			log.Println("WARNING: RENDERER_URL is not set; resume rendering will fail")
		}
	}

	return Config{
		DatabaseURL:     os.Getenv("DATABASE_URL"),
		AnthropicAPIKey: os.Getenv("ANTHROPIC_API_KEY"),
		Port:            port,
		Environment:     env,
		JWTSecret:       os.Getenv("JWT_SECRET"),
		AllowedOrigins:  allowedOrigins,
		RendererURL:     rendererURL,
	}
}

func parseAllowedOrigins(raw string) []string {
	if raw == "" {
		return nil
	}
	var origins []string
	for _, origin := range strings.Split(raw, ",") {
		origin = strings.TrimSpace(origin)
		if origin != "" {
			origins = append(origins, origin)
		}
	}
	return origins
}
