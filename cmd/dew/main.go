package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/MaxBly/dew/internal/api"
	"github.com/MaxBly/dew/internal/config"
	"github.com/MaxBly/dew/internal/library"
	"github.com/MaxBly/dew/internal/media"
	"github.com/MaxBly/dew/internal/store"
	"github.com/MaxBly/dew/internal/tmdb"
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

var libraryCmd = &cobra.Command{
	Use:   "library",
	Short: "Library management",
}

var libraryScanCmd = &cobra.Command{
	Use:   "scan",
	Short: "Scan the library and update movies.json / series.json / seasons.json",
	RunE:  runLibraryScan,
}

var libraryMediainfoCmd = &cobra.Command{
	Use:   "mediainfo",
	Short: "Run ffprobe on all tracked files and update cache/mediainfo.json",
	RunE:  runLibraryMediainfo,
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&cfgPath, "config", "c", "dew.toml", "path to config file")
	libraryCmd.AddCommand(libraryScanCmd, libraryMediainfoCmd)
	rootCmd.AddCommand(serveCmd, libraryCmd)
}

// loadStores opens all JSON stores from cfg.Data.Dir.
func loadStores(cfg *config.Config) (
	movies *store.JsonStore[media.MovieStore],
	series *store.JsonStore[media.SeriesStore],
	seasons *store.JsonStore[media.SeasonStore],
	mediaInfo *store.JsonStore[media.MediaInfoStore],
	err error,
) {
	open := func(name string) string { return filepath.Join(cfg.Data.Dir, name) }
	cacheOpen := func(name string) string { return filepath.Join(cfg.Data.Dir, "cache", name) }

	if err = os.MkdirAll(filepath.Join(cfg.Data.Dir, "cache"), 0755); err != nil {
		return nil, nil, nil, nil, fmt.Errorf("store cache dir: %w", err)
	}

	movies, err = store.New[media.MovieStore](open("movies.json"))
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("store movies: %w", err)
	}
	series, err = store.New[media.SeriesStore](open("series.json"))
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("store series: %w", err)
	}
	seasons, err = store.New[media.SeasonStore](open("seasons.json"))
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("store seasons: %w", err)
	}
	mediaInfo, err = store.New[media.MediaInfoStore](cacheOpen("mediainfo.json"))
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("store mediainfo: %w", err)
	}
	return movies, series, seasons, mediaInfo, nil
}

func runServe(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}

	movies, _, _, _, err := loadStores(cfg)
	if err != nil {
		return err
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

func runLibraryScan(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}

	movies, series, seasons, mediaInfo, err := loadStores(cfg)
	if err != nil {
		return err
	}

	var tmdbClient *tmdb.Client
	if cfg.TMDB.APIKey != "" {
		tmdbClient = tmdb.New(cfg.TMDB.APIKey)
	}

	scanner := library.NewScanner(cfg, movies, series, seasons, mediaInfo, tmdbClient)

	fmt.Println("scanning films...")
	if err := scanner.ScanFilms(); err != nil {
		fmt.Fprintf(os.Stderr, "films scan error: %v\n", err)
	}

	fmt.Println("scanning series...")
	if err := scanner.ScanSeries(); err != nil {
		fmt.Fprintf(os.Stderr, "series scan error: %v\n", err)
	}

	fmt.Println("scan complete.")
	return nil
}

func runLibraryMediainfo(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}

	movies, _, seasons, mediaInfo, err := loadStores(cfg)
	if err != nil {
		return err
	}

	fmt.Println("probing files with ffprobe...")
	if err := library.ScanMediaInfo(movies, seasons, mediaInfo); err != nil {
		return fmt.Errorf("mediainfo scan: %w", err)
	}
	fmt.Println("mediainfo scan complete.")
	return nil
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
