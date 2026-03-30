package tmdb

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/MaxBly/dew/internal/media"
)

const baseURL = "https://api.themoviedb.org/3"

// Client is a minimal TMDB API client.
// All methods return an error if the API key is empty.
type Client struct {
	apiKey string
	http   *http.Client
}

func New(apiKey string) *Client {
	return &Client{
		apiKey: apiKey,
		http:   &http.Client{Timeout: 10 * time.Second},
	}
}

// SearchMovie finds the best match for title+year and returns full Movie metadata.
// Makes two API calls: search → details (for full genre objects).
func (c *Client) SearchMovie(title string, year int) (media.Movie, error) {
	if c.apiKey == "" {
		return media.Movie{}, fmt.Errorf("tmdb: no API key configured")
	}

	// 1. Search
	params := url.Values{}
	params.Set("api_key", c.apiKey)
	params.Set("query", title)
	if year > 0 {
		params.Set("year", fmt.Sprintf("%d", year))
	}

	var searchResp struct {
		Results []struct {
			ID          int     `json:"id"`
			ReleaseDate string  `json:"release_date"`
			Popularity  float64 `json:"popularity"`
		} `json:"results"`
	}
	if err := c.get("/search/movie?"+params.Encode(), &searchResp); err != nil {
		return media.Movie{}, err
	}
	if len(searchResp.Results) == 0 {
		return media.Movie{}, fmt.Errorf("tmdb: no results for %q (%d)", title, year)
	}

	// Pick best match: prefer results where release year matches, then highest popularity.
	bestID := searchResp.Results[0].ID
	if year > 0 {
		var bestPop float64
		for _, r := range searchResp.Results {
			ry := 0
			if len(r.ReleaseDate) >= 4 {
				fmt.Sscanf(r.ReleaseDate[:4], "%d", &ry)
			}
			if ry == year && r.Popularity > bestPop {
				bestID = r.ID
				bestPop = r.Popularity
			}
		}
	}

	// 2. Details (includes full genre objects)
	return c.MovieDetails(bestID)
}

// MovieDetails fetches full metadata for a TMDB movie ID.
func (c *Client) MovieDetails(id int) (media.Movie, error) {
	if c.apiKey == "" {
		return media.Movie{}, fmt.Errorf("tmdb: no API key configured")
	}

	var details struct {
		ID            int          `json:"id"`
		Title         string       `json:"title"`
		OriginalTitle string       `json:"original_title"`
		Overview      string       `json:"overview"`
		ReleaseDate   string       `json:"release_date"`
		PosterPath    string       `json:"poster_path"`
		BackdropPath  string       `json:"backdrop_path"`
		VoteAverage   float64      `json:"vote_average"`
		Genres        []media.Genre `json:"genres"`
	}
	params := url.Values{}
	params.Set("api_key", c.apiKey)
	if err := c.get(fmt.Sprintf("/movie/%d?%s", id, params.Encode()), &details); err != nil {
		return media.Movie{}, err
	}

	return media.Movie{
		TMDBID:        details.ID,
		Title:         details.Title,
		OriginalTitle: details.OriginalTitle,
		Overview:      details.Overview,
		ReleaseDate:   details.ReleaseDate,
		PosterPath:    details.PosterPath,
		BackdropPath:  details.BackdropPath,
		VoteAverage:   details.VoteAverage,
		Genres:        details.Genres,
	}, nil
}

// SearchSeries finds the best match for a series name and returns full Series metadata.
func (c *Client) SearchSeries(name string) (media.Series, error) {
	if c.apiKey == "" {
		return media.Series{}, fmt.Errorf("tmdb: no API key configured")
	}

	params := url.Values{}
	params.Set("api_key", c.apiKey)
	params.Set("query", name)

	var searchResp struct {
		Results []struct {
			ID int `json:"id"`
		} `json:"results"`
	}
	if err := c.get("/search/tv?"+params.Encode(), &searchResp); err != nil {
		return media.Series{}, err
	}
	if len(searchResp.Results) == 0 {
		return media.Series{}, fmt.Errorf("tmdb: no results for series %q", name)
	}

	return c.SeriesDetails(searchResp.Results[0].ID)
}

// SeriesDetails fetches full metadata for a TMDB series ID.
func (c *Client) SeriesDetails(id int) (media.Series, error) {
	if c.apiKey == "" {
		return media.Series{}, fmt.Errorf("tmdb: no API key configured")
	}

	var details struct {
		ID               int          `json:"id"`
		Name             string       `json:"name"`
		OriginalName     string       `json:"original_name"`
		Overview         string       `json:"overview"`
		FirstAirDate     string       `json:"first_air_date"`
		PosterPath       string       `json:"poster_path"`
		BackdropPath     string       `json:"backdrop_path"`
		VoteAverage      float64      `json:"vote_average"`
		Genres           []media.Genre `json:"genres"`
		NumberOfSeasons  int          `json:"number_of_seasons"`
		NumberOfEpisodes int          `json:"number_of_episodes"`
	}
	params := url.Values{}
	params.Set("api_key", c.apiKey)
	if err := c.get(fmt.Sprintf("/tv/%d?%s", id, params.Encode()), &details); err != nil {
		return media.Series{}, err
	}

	return media.Series{
		TMDBID:           details.ID,
		Name:             details.Name,
		OriginalName:     details.OriginalName,
		Overview:         details.Overview,
		FirstAirDate:     details.FirstAirDate,
		PosterPath:       details.PosterPath,
		BackdropPath:     details.BackdropPath,
		VoteAverage:      details.VoteAverage,
		Genres:           details.Genres,
		NumberOfSeasons:  details.NumberOfSeasons,
		NumberOfEpisodes: details.NumberOfEpisodes,
	}, nil
}

func (c *Client) get(path string, out any) error {
	resp, err := c.http.Get(baseURL + path)
	if err != nil {
		return fmt.Errorf("tmdb: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("tmdb: HTTP %d for %s", resp.StatusCode, path)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}
