package library

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/MaxBly/dew/internal/config"
	"github.com/MaxBly/dew/internal/media"
	"github.com/MaxBly/dew/internal/store"
	"github.com/MaxBly/dew/internal/tmdb"
)

var videoExts = map[string]bool{
	".mkv": true, ".mp4": true, ".avi": true,
	".mov": true, ".m4v": true, ".ts": true,
}

// Scanner walks the library paths and populates the JSON stores.
type Scanner struct {
	cfg     *config.Config
	movies  *store.JsonStore[media.MovieStore]
	series  *store.JsonStore[media.SeriesStore]
	seasons *store.JsonStore[media.SeasonStore]
	tmdb    *tmdb.Client // nil if no API key configured
}

func NewScanner(
	cfg *config.Config,
	movies *store.JsonStore[media.MovieStore],
	series *store.JsonStore[media.SeriesStore],
	seasons *store.JsonStore[media.SeasonStore],
	tmdbClient *tmdb.Client,
) *Scanner {
	return &Scanner{
		cfg:     cfg,
		movies:  movies,
		series:  series,
		seasons: seasons,
		tmdb:    tmdbClient,
	}
}

// ScanFilms walks cfg.Library.FilmsPath and updates movies.json.
// Existing entries are preserved; only new files are processed.
// Multiple files resolving to the same slug are grouped under files[].
func (s *Scanner) ScanFilms() error {
	root := s.cfg.Library.FilmsPath
	if root == "" {
		return fmt.Errorf("library: films_path not configured")
	}

	store := s.movies.Get()
	if store == nil {
		store = make(media.MovieStore)
	}

	// Build a set of already-tracked paths for fast lookup.
	tracked := make(map[string]bool)
	for _, m := range store {
		for _, f := range m.Files {
			tracked[f] = true
		}
	}

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !videoExts[strings.ToLower(filepath.Ext(path))] {
			return nil
		}
		if tracked[path] {
			return nil
		}

		title, year, ok := ParseMovieFilename(info.Name())
		if !ok {
			log.Printf("library: skipping unparseable film: %s", info.Name())
			return nil
		}

		slug, movie := s.resolveMovie(title, year)

		if existing, exists := store[slug]; exists {
			existing.Files = append(existing.Files, path)
			store[slug] = existing
		} else {
			movie.Slug = slug
			movie.Files = []string{path}
			movie.AddedAt = time.Now().UTC().Format(time.RFC3339)
			store[slug] = movie
		}

		log.Printf("library: added film %q → %s", title, slug)
		return nil
	})
	if err != nil {
		return err
	}

	return s.movies.Set(store)
}

// ScanSeries walks cfg.Library.SeriesPath and updates series.json + seasons.json.
// Series are detected by directory name; episodes by SxxExx pattern in filenames.
func (s *Scanner) ScanSeries() error {
	root := s.cfg.Library.SeriesPath
	if root == "" {
		return fmt.Errorf("library: series_path not configured")
	}

	seriesStore := s.series.Get()
	if seriesStore == nil {
		seriesStore = make(media.SeriesStore)
	}
	seasonStore := s.seasons.Get()
	if seasonStore == nil {
		seasonStore = make(media.SeasonStore)
	}

	// Build tracked set.
	tracked := make(map[string]bool)
	for _, seasons := range seasonStore {
		for _, eps := range seasons {
			for _, ep := range eps {
				tracked[ep.File] = true
			}
		}
	}

	// Top-level directories = one series each.
	entries, err := os.ReadDir(root)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		folder := entry.Name()
		seriesName := strings.NewReplacer(".", " ", "_", " ").Replace(folder)
		seriesName = strings.TrimSpace(seriesName)

		slug, ser := s.resolveSeries(seriesName, folder)

		if _, exists := seriesStore[slug]; !exists {
			ser.Slug = slug
			ser.Folder = folder
			ser.AddedAt = time.Now().UTC().Format(time.RFC3339)
			seriesStore[slug] = ser
			log.Printf("library: added series %q → %s", seriesName, slug)
		}

		if seasonStore[slug] == nil {
			seasonStore[slug] = make(map[string][]media.Episode)
		}

		seriesRoot := filepath.Join(root, folder)
		err := filepath.Walk(seriesRoot, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() || !videoExts[strings.ToLower(filepath.Ext(path))] {
				return nil
			}
			if tracked[path] {
				return nil
			}

			_, season, episode, ok := ParseEpisodeFilename(info.Name())
			if !ok {
				log.Printf("library: skipping unparseable episode: %s", info.Name())
				return nil
			}

			seasonKey := fmt.Sprintf("%d", season)
			ep := media.Episode{
				EpisodeNumber: episode,
				Name:          info.Name(),
				File:          path,
			}
			seasonStore[slug][seasonKey] = append(seasonStore[slug][seasonKey], ep)
			tracked[path] = true
			return nil
		})
		if err != nil {
			return err
		}
	}

	if err := s.series.Set(seriesStore); err != nil {
		return err
	}
	return s.seasons.Set(seasonStore)
}

// resolveMovie returns a slug and Movie metadata for a given title+year.
// Uses TMDB if available, falls back to parsed filename data.
func (s *Scanner) resolveMovie(title string, year int) (string, media.Movie) {
	if s.tmdb != nil {
		m, err := s.tmdb.SearchMovie(title, year)
		if err == nil {
			return Slug(m.Title, year, m.TMDBID), m
		}
		log.Printf("library: TMDB lookup failed for %q: %v", title, err)
	}
	return SlugNoTMDB(title, year), media.Movie{
		Title:       title,
		ReleaseDate: fmt.Sprintf("%d-01-01", year),
	}
}

// resolveSeries returns a slug and Series metadata for a given series name.
func (s *Scanner) resolveSeries(name, folder string) (string, media.Series) {
	if s.tmdb != nil {
		ser, err := s.tmdb.SearchSeries(name)
		if err == nil {
			year := 0
			if len(ser.FirstAirDate) >= 4 {
				fmt.Sscanf(ser.FirstAirDate[:4], "%d", &year)
			}
			return Slug(ser.Name, year, ser.TMDBID), ser
		}
		log.Printf("library: TMDB lookup failed for series %q: %v", name, err)
	}
	return normalizeTitle(name), media.Series{Name: name, Folder: folder}
}
