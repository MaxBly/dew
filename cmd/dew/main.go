package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/MaxBly/dew/internal/api"
	"github.com/MaxBly/dew/internal/config"
	"github.com/MaxBly/dew/internal/media"
	"github.com/MaxBly/dew/internal/store"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/spf13/cobra"
)

var cfgPath string

var rootCmd = &cobra.Command{
	Use:   "dew",
	Short: "Self-hosted media streaming server",
}

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the Dew server",
	RunE:  runServe,
}

func init() {
	serveCmd.Flags().StringVarP(&cfgPath, "config", "c", "dew.toml", "path to config file")
	rootCmd.AddCommand(serveCmd)
}

func runServe(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}

	moviesPath := filepath.Join(cfg.Data.Dir, "movies.json")
	movies, err := store.New[media.MovieStore](moviesPath)
	if err != nil {
		return fmt.Errorf("store: %w", err)
	}

	e := echo.New()
	e.HideBanner = true
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	api.RegisterLibrary(e, movies)

	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	fmt.Printf("dew listening on %s\n", addr)
	return e.Start(addr)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
