package config

import (
	"os"

	"github.com/BurntSushi/toml"
)

type Config struct {
	Server          ServerConfig              `toml:"server"`
	Library         LibraryConfig             `toml:"library"`
	Auth            AuthConfig                `toml:"auth"`
	TMDB            TMDBConfig                `toml:"tmdb"`
	Data            DataConfig                `toml:"data"`
	Addons          AddonsConfig              `toml:"addons"`
	SeriesOverrides map[string]SeriesOverride `toml:"series_overrides"`
}

// SeriesOverride is keyed by the filesystem folder name (e.g. "Futurama", "H.1998").
// It allows forcing a specific TMDB ID when auto-detection fails, and/or selecting
// an alternative episode group ordering.
type SeriesOverride struct {
	// TMDBID forces a specific TMDB series ID, skipping the search step.
	// Use this when the folder name yields a wrong TMDB match.
	TMDBID int `toml:"tmdb_id"`

	// EpisodeGroup forces enrichment via a specific TMDB episode group ID.
	// When set, the entire series is enriched using the group's ordering:
	// group order N → season N, episode position K → episode K.
	EpisodeGroup string `toml:"episode_group"`

	// EpisodeOverrides maps "S<N>E<N>" (disk position, e.g. "S11E11") to a specific
	// TMDB episode reference. Applied after automatic enrichment to fix edge cases.
	// Example: "S11E11" = { tmdb_season = 0, tmdb_episode = 14 }
	EpisodeOverrides map[string]EpisodePatch `toml:"episode_overrides"`
}

// EpisodePatch overrides the TMDB source for a specific disk episode.
// Key format in dew.toml: "S<season>E<episode>" (e.g. "S11E11").
type EpisodePatch struct {
	TMDBSeason  int `toml:"tmdb_season"`
	TMDBEpisode int `toml:"tmdb_episode"`
}

type ServerConfig struct {
	Host string `toml:"host"`
	Port int    `toml:"port"`
}

type LibraryConfig struct {
	FilmsPath  string `toml:"films_path"`
	SeriesPath string `toml:"series_path"`
}

type AuthConfig struct {
	Provider string `toml:"provider"` // "token" | "password" | "both"
	Secret   string `toml:"secret"`   // cookie signing secret
}

type TMDBConfig struct {
	APIKey string `toml:"api_key"`
}

type DataConfig struct {
	Dir string `toml:"dir"`
}

type AddonsConfig struct {
	Enabled    []string           `toml:"enabled"`
	Streams    StreamsAddonConfig `toml:"streams"`
	I18n       I18nAddonConfig    `toml:"i18n"`
	Federation FederationConfig   `toml:"federation"`
}

type StreamsAddonConfig struct {
	MaxPerIP   int `toml:"max_per_ip"`
	MaxPerUser int `toml:"max_per_user"`
}

type I18nAddonConfig struct {
	Locale string `toml:"locale"`
}

type FederationConfig struct {
	Mode      string       `toml:"mode"` // "node" | "hub"
	NodeName  string       `toml:"node_name"`
	PublicURL string       `toml:"public_url"`
	Peers     []PeerConfig `toml:"peers"`
}

type PeerConfig struct {
	Name   string `toml:"name"`
	URL    string `toml:"url"`
	Secret string `toml:"secret"`
}

func defaults() *Config {
	return &Config{
		Server: ServerConfig{Host: "0.0.0.0", Port: 7777},
		Auth:   AuthConfig{Provider: "token"},
		Data:   DataConfig{Dir: "data"},
	}
}

// Load reads a TOML config file. Missing fields keep their default values.
func Load(path string) (*Config, error) {
	cfg := defaults()
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	if _, err := toml.NewDecoder(f).Decode(cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}
