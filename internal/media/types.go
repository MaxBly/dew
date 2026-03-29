package media

// Genre matches the TMDB genre object.
type Genre struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// Movie is the metadata stored in movies.json.
// The map key is the filename (for ezserv compatibility).
type Movie struct {
	TMDBID        int     `json:"tmdb_id"`
	Title         string  `json:"title"`
	OriginalTitle string  `json:"original_title"`
	Overview      string  `json:"overview"`
	ReleaseDate   string  `json:"release_date"`
	PosterPath    string  `json:"poster_path"`
	BackdropPath  string  `json:"backdrop_path"`
	VoteAverage   float64 `json:"vote_average"`
	Genres        []Genre `json:"genres"`
	AddedAt       string  `json:"added_at"`
}

// MovieStore is the shape of movies.json: filename → metadata.
type MovieStore map[string]Movie
