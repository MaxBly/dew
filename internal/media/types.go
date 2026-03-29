package media

// Genre matches the TMDB genre object.
type Genre struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// Movie is the metadata stored in movies.json.
// Slug format: {normalized-title}-{year}-{tmdb_id} (e.g. "the-fast-and-the-furious-2001-9799").
// Files holds absolute paths to all available versions (1080p, 4K, etc.).
type Movie struct {
	Slug          string   `json:"slug"`
	TMDBID        int      `json:"tmdb_id"`
	Title         string   `json:"title"`
	OriginalTitle string   `json:"original_title"`
	Overview      string   `json:"overview"`
	ReleaseDate   string   `json:"release_date"`
	PosterPath    string   `json:"poster_path"`
	BackdropPath  string   `json:"backdrop_path"`
	VoteAverage   float64  `json:"vote_average"`
	Genres        []Genre  `json:"genres"`
	Files         []string `json:"files"`
	AddedAt       string   `json:"added_at"`
}

// Series is the metadata stored in series.json.
// Folder is the filesystem directory name (e.g. "Mr.Robot") used for path resolution.
type Series struct {
	Slug           string  `json:"slug"`
	TMDBID         int     `json:"tmdb_id"`
	Name           string  `json:"name"`
	OriginalName   string  `json:"original_name"`
	Overview       string  `json:"overview"`
	FirstAirDate   string  `json:"first_air_date"`
	PosterPath     string  `json:"poster_path"`
	BackdropPath   string  `json:"backdrop_path"`
	VoteAverage    float64 `json:"vote_average"`
	Genres         []Genre `json:"genres"`
	NumberOfSeasons  int   `json:"number_of_seasons"`
	NumberOfEpisodes int   `json:"number_of_episodes"`
	Folder         string  `json:"folder"`
	AddedAt        string  `json:"added_at"`
}

// Episode is one entry in seasons.json. No embedded mediainfo — see cache/mediainfo.json.
type Episode struct {
	EpisodeNumber int    `json:"episode_number"`
	Name          string `json:"name"`
	File          string `json:"file"` // absolute path
	Overview      string `json:"overview"`
	AirDate       string `json:"air_date"`
	Runtime       int    `json:"runtime"`
	VoteAverage   float64 `json:"vote_average"`
	StillPath     string `json:"still_path"`
}

// AudioTrack describes one audio stream in a media file.
type AudioTrack struct {
	Index    int    `json:"index"`
	Lang     string `json:"lang"`
	Default  bool   `json:"default"`
	Title    string `json:"title"`
	Format   string `json:"format"`
	Channels string `json:"channels"`
}

// SubtitleTrack describes one subtitle stream in a media file.
type SubtitleTrack struct {
	Index   int    `json:"index"`
	Lang    string `json:"lang"`
	Default bool   `json:"default"`
	Forced  bool   `json:"forced"`
	Title   string `json:"title"`
	Format  string `json:"format"`
}

// MediaInfo is stored in cache/mediainfo.json, keyed by absolute file path.
type MediaInfo struct {
	Resolution     string          `json:"resolution"`
	VideoCodec     string          `json:"video_codec"`
	Duration       int             `json:"duration"` // seconds
	AudioTracks    []AudioTrack    `json:"audio_tracks"`
	SubtitleTracks []SubtitleTrack `json:"subtitle_tracks"`
	ScannedAt      string          `json:"scanned_at"`
}

// WatchEntry is one entry in watch_history.json.
type WatchEntry struct {
	File      string `json:"file"` // absolute path (which version was watched)
	Position  int    `json:"position"`
	Duration  int    `json:"duration"`
	Audio     int    `json:"audio"`
	Sub       int    `json:"sub"`
	UpdatedAt string `json:"updated_at"`
}

// Store types — top-level shapes of each JSON file.

// MovieStore is movies.json: slug → Movie.
type MovieStore map[string]Movie

// SeriesStore is series.json: slug → Series.
type SeriesStore map[string]Series

// SeasonStore is seasons.json: series_slug → season_number → episodes.
type SeasonStore map[string]map[string][]Episode

// MediaInfoStore is cache/mediainfo.json: absolute_path → MediaInfo.
type MediaInfoStore map[string]MediaInfo

// WatchHistory is watch_history.json: token → slug → WatchEntry.
type WatchHistory map[string]map[string]WatchEntry
