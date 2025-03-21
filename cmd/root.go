package cmd

import (
	"context"
	"log"
	"log/slog"
	"os"
	"path"

	"github.com/joho/godotenv"
	"github.com/pyama86/YAS3/handler"
	"github.com/spf13/cobra"
)

var (
	configPath string
)

var rootCmd = &cobra.Command{
	Use:   "yas3",
	Short: "yas3 is a SlackBot for incident management",
	Run: func(cmd *cobra.Command, args []string) {
		if cmd.Name() == "version" {
			return
		}

		if err := run(); err != nil {
			slog.Error("Failed to run command", slog.Any("error", err))
			os.Exit(1)
		}
	},
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	// デフォルトはホームディレクトリのyas3.toml
	home, err := os.UserHomeDir()
	if err != nil {
		slog.Error("Failed to get user home directory", slog.Any("error", err))
		os.Exit(1)
	}
	rootCmd.Flags().StringVar(&configPath, "config", path.Join(home, "yas3.toml"), "config file path")
}

func run() error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if _, err := os.Stat(".env"); err == nil {
		err := godotenv.Load()
		if err != nil {
			log.Fatal("Error loading .env file")
		}
	}

	slog.Info("Server started")
	if err := handler.Handle(ctx, configPath); err != nil {
		return err
	}

	return nil
}
