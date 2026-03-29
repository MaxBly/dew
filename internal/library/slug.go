package library

import (
	"fmt"
	"regexp"
	"strings"
)

var nonAlphanumRe = regexp.MustCompile(`[^a-z0-9]+`)

// Slug returns "{normalized-title}-{year}-{tmdbID}".
// This is the standard slug format for all media in Dew.
func Slug(title string, year, tmdbID int) string {
	return fmt.Sprintf("%s-%d-%d", normalizeTitle(title), year, tmdbID)
}

// SlugNoTMDB returns "{normalized-title}-{year}" for entries without a TMDB ID.
func SlugNoTMDB(title string, year int) string {
	return fmt.Sprintf("%s-%d", normalizeTitle(title), year)
}

func normalizeTitle(title string) string {
	s := strings.ToLower(title)
	s = nonAlphanumRe.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	return s
}
