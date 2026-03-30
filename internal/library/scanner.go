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
	cfg       *config.Config
	movies    *store.JsonStore[media.MovieStore]
	series    *store.JsonStore[media.SeriesStore]
	seasons   *store.JsonStore[media.SeasonStore]
	mediaInfo *store.JsonStore[media.MediaInfoStore] // nil if not configured
	tmdb      *tmdb.Client                           // nil if no API key configured
}

func NewScanner(
	cfg *config.Config,
	movies *store.JsonStore[media.MovieStore],
	series *store.JsonStore[media.SeriesStore],
	seasons *store.JsonStore[media.SeasonStore],
	mediaInfo *store.JsonStore[media.MediaInfoStore],
	tmdbClient *tmdb.Client,
) *Scanner {
	return &Scanner{
		cfg:       cfg,
		movies:    movies,
		series:    series,
		seasons:   seasons,
		mediaInfo: mediaInfo,
		tmdb:      tmdbClient,
	}
}

// miDurations returns a map of file path → duration in seconds from the MediaInfo cache.
// Returns nil if the store is not configured or empty.
func (s *Scanner) miDurations() map[string]int {
	if s.mediaInfo == nil {
		return nil
	}
	mi := s.mediaInfo.Get()
	if mi == nil {
		return nil
	}
	out := make(map[string]int, len(mi))
	for path, info := range mi {
		if info.Duration > 0 {
			out[path] = info.Duration
		}
	}
	return out
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

	miDurs := s.miDurations()

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

		// Walk series directory and collect new episodes grouped by season.
		newSeasons := make(map[string][]media.Episode)
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
				File:          path,
			}
			newSeasons[seasonKey] = append(newSeasons[seasonKey], ep)
			tracked[path] = true
			return nil
		})
		if err != nil {
			return err
		}

		if len(newSeasons) == 0 {
			continue
		}

		// Enrich new episodes with TMDB metadata.
		tmdbID := seriesStore[slug].TMDBID
		s.enrichSeries(newSeasons, tmdbID, folder, slug, miDurs)
		for seasonKey, eps := range newSeasons {
			seasonStore[slug][seasonKey] = append(seasonStore[slug][seasonKey], eps...)
		}
	}

	if err := s.series.Set(seriesStore); err != nil {
		return err
	}
	return s.seasons.Set(seasonStore)
}

// enrichSeries enriches all seasons of a series.
// If a TMDB episode group is configured for this slug, the entire series is enriched
// from the group. Otherwise, each season is enriched independently via the default API.
// After regular enrichment, specials are matched from TMDB S00 and episode overrides applied.
func (s *Scanner) enrichSeries(
	seasonStore map[string][]media.Episode,
	tmdbID int,
	folder, seriesSlug string,
	miDurs map[string]int,
) {
	if s.tmdb == nil || tmdbID == 0 {
		return
	}

	// Check for episode group override.
	var groupID string
	if s.cfg.SeriesOverrides != nil {
		for _, key := range []string{folder, seriesSlug} {
			if ov, ok := s.cfg.SeriesOverrides[key]; ok && ov.EpisodeGroup != "" {
				groupID = ov.EpisodeGroup
				break
			}
		}
	}

	if groupID != "" {
		s.enrichFromGroup(seasonStore, groupID, seriesSlug)
	} else {
		for seasonKey, eps := range seasonStore {
			seasonStore[seasonKey] = s.enrichSeasonDefault(eps, tmdbID, seasonKey, seriesSlug, miDurs)
		}
	}

	// After regular enrichment, try to place remaining unenriched episodes
	// by matching them against TMDB S00 specials via air-date proximity.
	if s.hasUnenriched(seasonStore) {
		s00Eps, err := s.tmdb.TVSeason(tmdbID, 0)
		if err != nil {
			log.Printf("library: %s — S00 fetch failed: %v", seriesSlug, err)
		} else if n := enrichFromSpecials(seasonStore, s00Eps, miDurs, seriesSlug); n > 0 {
			log.Printf("library: %s — matched %d special(s) from S00", seriesSlug, n)
		}
	}

	// Apply any manual episode overrides from dew.toml.
	s.applyEpisodeOverrides(seasonStore, tmdbID, folder, seriesSlug)
}

// hasUnenriched returns true if any episode in the season store has no name.
func (s *Scanner) hasUnenriched(seasonStore map[string][]media.Episode) bool {
	for _, eps := range seasonStore {
		for _, ep := range eps {
			if ep.Name == "" {
				return true
			}
		}
	}
	return false
}

// applyEpisodeOverrides applies manual overrides from [series_overrides."X".episode_overrides].
// Key format: "S<season>E<episode>" (e.g. "S11E11").
// For each override, fetches the specified TMDB episode and applies its metadata.
func (s *Scanner) applyEpisodeOverrides(
	seasonStore map[string][]media.Episode,
	tmdbID int,
	folder, seriesSlug string,
) {
	if s.cfg.SeriesOverrides == nil {
		return
	}
	var ov config.SeriesOverride
	var found bool
	for _, key := range []string{folder, seriesSlug} {
		if o, ok := s.cfg.SeriesOverrides[key]; ok && len(o.EpisodeOverrides) > 0 {
			ov = o
			found = true
			break
		}
	}
	if !found {
		return
	}

	// Cache fetched seasons to avoid repeated API calls.
	fetchedSeasons := make(map[int][]media.Episode)
	fetchSeason := func(sn int) []media.Episode {
		if eps, ok := fetchedSeasons[sn]; ok {
			return eps
		}
		eps, err := s.tmdb.TVSeason(tmdbID, sn)
		if err != nil {
			log.Printf("library: override fetch S%02d failed for %s: %v", sn, seriesSlug, err)
			fetchedSeasons[sn] = nil
		} else {
			fetchedSeasons[sn] = eps
		}
		return fetchedSeasons[sn]
	}

	for key, patch := range ov.EpisodeOverrides {
		var diskSeason, diskEp int
		if _, err := fmt.Sscanf(key, "S%dE%d", &diskSeason, &diskEp); err != nil {
			log.Printf("library: bad override key %q for %s (want S<N>E<N>)", key, seriesSlug)
			continue
		}
		seasonKey := fmt.Sprintf("%d", diskSeason)
		eps, ok := seasonStore[seasonKey]
		if !ok {
			continue
		}

		tmdbEps := fetchSeason(patch.TMDBSeason)
		var tmdbEp media.Episode
		var tmdbFound bool
		for _, te := range tmdbEps {
			if te.EpisodeNumber == patch.TMDBEpisode {
				tmdbEp = te
				tmdbFound = true
				break
			}
		}
		if !tmdbFound {
			log.Printf("library: override %s — TMDB S%02dE%02d not found", seriesSlug, patch.TMDBSeason, patch.TMDBEpisode)
			continue
		}

		for i, ep := range eps {
			if ep.EpisodeNumber == diskEp {
				applyTMDB(&eps[i], tmdbEp, 1, 1)
				seasonStore[seasonKey] = eps
				log.Printf("library: override %s S%02dE%02d → TMDB S%02dE%02d %q",
					seriesSlug, diskSeason, diskEp, patch.TMDBSeason, patch.TMDBEpisode, tmdbEp.Name)
				break
			}
		}
	}
}

// enrichFromGroup enriches all episodes using a TMDB episode group.
// Group order N → season N; episode at position K (1-based) → disk episode K.
func (s *Scanner) enrichFromGroup(seasonStore map[string][]media.Episode, groupID, seriesSlug string) {
	detail, err := s.tmdb.TVEpisodeGroupDetail(groupID)
	if err != nil {
		log.Printf("library: episode group fetch failed for %s (group %s): %v", seriesSlug, groupID, err)
		return
	}

	log.Printf("library: enriching %s via episode group %s (%d seasons)", seriesSlug, groupID, len(detail.Seasons))

	for seasonKey, eps := range seasonStore {
		var seasonNum int
		fmt.Sscanf(seasonKey, "%d", &seasonNum)

		groupEps, ok := detail.Seasons[seasonNum]
		if !ok {
			log.Printf("library: WARNING %s S%02d not found in episode group", seriesSlug, seasonNum)
			continue
		}

		// Build lookup by position (episode_number was set to position in TVEpisodeGroupDetail).
		byPos := make(map[int]media.Episode, len(groupEps))
		for _, ge := range groupEps {
			byPos[ge.EpisodeNumber] = ge
		}

		// Detect split-film case: disk has N × group episodes (e.g. Futurama S06:
		// 4 films × 4 TV parts = 16 disk episodes, 4 group entries).
		parts := 1
		if len(groupEps) > 0 && len(eps) > len(groupEps) && len(eps)%len(groupEps) == 0 {
			parts = len(eps) / len(groupEps)
			log.Printf("library: %s S%02d — %d files = %d films × %d parts",
				seriesSlug, seasonNum, len(eps), len(groupEps), parts)
		} else if len(eps) != len(groupEps) {
			log.Printf("library: WARNING %s S%02d — %d files on disk, %d in group",
				seriesSlug, seasonNum, len(eps), len(groupEps))
		}

		for i, ep := range eps {
			filmPos := (ep.EpisodeNumber-1)/parts + 1
			partNum := (ep.EpisodeNumber-1)%parts + 1

			if ge, ok := byPos[filmPos]; ok {
				eps[i].Overview = ge.Overview
				eps[i].AirDate = ge.AirDate
				eps[i].Runtime = ge.Runtime / parts
				eps[i].VoteAverage = ge.VoteAverage
				eps[i].StillPath = ge.StillPath
				if parts > 1 {
					eps[i].Name = fmt.Sprintf("%s (Part %d)", ge.Name, partNum)
				} else {
					eps[i].Name = ge.Name
				}
			} else {
				log.Printf("library: WARNING %s S%02d E%02d not found in episode group", seriesSlug, seasonNum, ep.EpisodeNumber)
			}
		}
		seasonStore[seasonKey] = eps
	}
}

// enrichSeasonDefault fetches TMDB season data and merges it into the scanned episodes.
//
// When disk count == TMDB count: match by episode_number.
// When disk count > TMDB count: attempt sequential matching using TMDB runtimes
// to detect split double/multi-part episodes (e.g. The Office 45min → 2×22min).
// Falls back to episode_number matching with mismatch warnings if sequential fails.
func (s *Scanner) enrichSeasonDefault(
	eps []media.Episode,
	tmdbID int,
	seasonKey, seriesSlug string,
	miDurs map[string]int,
) []media.Episode {
	var seasonNum int
	fmt.Sscanf(seasonKey, "%d", &seasonNum)

	tmdbEps, err := s.tmdb.TVSeason(tmdbID, seasonNum)
	if err != nil {
		log.Printf("library: TMDB season fetch failed for %s S%02d: %v", seriesSlug, seasonNum, err)
		return eps
	}

	if len(tmdbEps) > 0 && len(eps) != len(tmdbEps) {
		log.Printf("library: WARNING %s S%02d — %d files on disk, %d on TMDB",
			seriesSlug, seasonNum, len(eps), len(tmdbEps))
	}

	// When disk has MORE episodes than TMDB, try sequential duration-based matching.
	// This handles shows like The Office where a 45-min episode was split into 2×22-min files.
	if len(eps) > len(tmdbEps) {
		if sequentialEnrich(eps, tmdbEps, miDurs, seriesSlug, seasonNum) {
			return eps
		}
	}

	// Default: match by episode_number.
	byNum := make(map[int]media.Episode, len(tmdbEps))
	for _, te := range tmdbEps {
		byNum[te.EpisodeNumber] = te
	}
	for i, ep := range eps {
		if te, ok := byNum[ep.EpisodeNumber]; ok {
			eps[i].Name = te.Name
			eps[i].Overview = te.Overview
			eps[i].AirDate = te.AirDate
			eps[i].Runtime = te.Runtime
			eps[i].VoteAverage = te.VoteAverage
			eps[i].StillPath = te.StillPath
		} else {
			log.Printf("library: WARNING %s S%02d E%02d not in TMDB default order", seriesSlug, seasonNum, ep.EpisodeNumber)
		}
	}
	return eps
}

// resolveMovie returns a slug and Movie metadata for a given title+year.
// TMDB retry chain (each step only runs if the previous failed):
//  1. raw title + year
//  2. CleanTitle(title) + year  (strips brackets, prefixes, release tags)
//  3. CleanTitle(title) without year (handles wrong-year edge cases)
func (s *Scanner) resolveMovie(title string, year int) (string, media.Movie) {
	if s.tmdb != nil {
		clean := CleanTitle(title)
		attempts := []struct {
			t string
			y int
		}{
			{title, year},
			{clean, year},
			{clean, 0},
		}
		for _, a := range attempts {
			if a.t == "" {
				continue
			}
			m, err := s.tmdb.SearchMovie(a.t, a.y)
			if err == nil {
				releaseYear := year
				if len(m.ReleaseDate) >= 4 {
					fmt.Sscanf(m.ReleaseDate[:4], "%d", &releaseYear)
				}
				return Slug(m.Title, releaseYear, m.TMDBID), m
			}
		}
		log.Printf("library: TMDB lookup failed for %q (%d)", title, year)
	}
	return SlugNoTMDB(CleanTitle(title), year), media.Movie{
		Title:       CleanTitle(title),
		ReleaseDate: fmt.Sprintf("%d-01-01", year),
	}
}

// resolveSeries returns a slug and Series metadata for a given series name.
// Checks for a folder-level override in config before calling TMDB search.
// Handles folder names with trailing year (e.g. "Shameless 2011" → search "Shameless").
func (s *Scanner) resolveSeries(name, folder string) (string, media.Series) {
	// Check for a config override keyed by folder name.
	if s.cfg.SeriesOverrides != nil {
		if ov, ok := s.cfg.SeriesOverrides[folder]; ok && ov.TMDBID != 0 && s.tmdb != nil {
			ser, err := s.tmdb.SeriesDetails(ov.TMDBID)
			if err == nil {
				year := 0
				if len(ser.FirstAirDate) >= 4 {
					fmt.Sscanf(ser.FirstAirDate[:4], "%d", &year)
				}
				log.Printf("library: series %q resolved via config override → TMDB %d", folder, ov.TMDBID)
				return Slug(ser.Name, year, ser.TMDBID), ser
			}
			log.Printf("library: config override TMDB fetch failed for %q: %v", folder, err)
		}
	}

	// Strip trailing year from folder-derived names like "Shameless 2011" or "H 1998".
	stripped, _ := StripFolderYear(name)

	if s.tmdb != nil {
		for _, n := range []string{stripped, name} {
			if n == "" {
				continue
			}
			ser, err := s.tmdb.SearchSeries(n)
			if err == nil {
				year := 0
				if len(ser.FirstAirDate) >= 4 {
					fmt.Sscanf(ser.FirstAirDate[:4], "%d", &year)
				}
				return Slug(ser.Name, year, ser.TMDBID), ser
			}
		}
		log.Printf("library: TMDB lookup failed for series %q", name)
	}
	return normalizeTitle(stripped), media.Series{Name: stripped, Folder: folder}
}
