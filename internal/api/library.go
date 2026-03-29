package api

import (
	"net/http"

	"github.com/MaxBly/dew/internal/media"
	"github.com/MaxBly/dew/internal/store"
	"github.com/labstack/echo/v4"
)

// FilmItem is the API response shape for a single film.
type FilmItem struct {
	Filename string `json:"filename"`
	media.Movie
}

// RegisterLibrary attaches library routes to the Echo instance.
func RegisterLibrary(e *echo.Echo, movies *store.JsonStore[media.MovieStore]) {
	e.GET("/api/films", listFilms(movies))
}

func listFilms(movies *store.JsonStore[media.MovieStore]) echo.HandlerFunc {
	return func(c echo.Context) error {
		data := movies.Get()
		items := make([]FilmItem, 0, len(data))
		for filename, movie := range data {
			items = append(items, FilmItem{Filename: filename, Movie: movie})
		}
		return c.JSON(http.StatusOK, items)
	}
}
