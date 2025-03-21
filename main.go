package main

import (
	"fmt"
	"log"
	"log/slog"
	"os"

	"github.com/joho/godotenv"
	"github.com/pyama86/YAS3/cmd"
)

func validateEnv() error {
	requiredEnv := []string{
		"SLACK_BOT_TOKEN",
		"SLACK_APP_TOKEN",
	}
	for _, env := range requiredEnv {
		if os.Getenv(env) == "" {
			return fmt.Errorf("environment variable %s is required but not set", env)
		}
	}
	return nil
}

func main() {
	if _, err := os.Stat(".env"); err == nil {
		err := godotenv.Load()
		if err != nil {
			log.Fatal("Error loading .env file")
		}
	}
	if err := validateEnv(); err != nil {
		slog.Error("failed to validate environment", slog.Any("error", err))
		os.Exit(1)
	}

	if err := cmd.Execute(); err != nil {
		slog.Error("failed to execute command", slog.Any("error", err))
		os.Exit(1)
	}
}
