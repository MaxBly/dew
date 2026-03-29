package library

import (
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

var (
	// matches a 4-digit year surrounded by separators or parentheses
	yearRe = regexp.MustCompile(`[\.\s\(]((?:19|20)\d{2})[\.\s\)]`)
	// matches SxxExx patterns (case-insensitive)
	episodeRe = regexp.MustCompile(`(?i)[Ss](\d{1,2})[Ee](\d{1,2})`)
)

// ParseMovieFilename extracts title and year from a movie filename.
// Supports:
//
//	"The.Dark.Knight.2008.1080p.BluRay.mkv"
//	"The Dark Knight (2008) 1080p BluRay.mkv"
func ParseMovieFilename(filename string) (title string, year int, ok bool) {
	name := strings.TrimSuffix(filename, filepath.Ext(filename))

	m := yearRe.FindStringSubmatchIndex(name)
	if m == nil {
		return "", 0, false
	}

	raw := name[:m[0]]
	raw = strings.NewReplacer(".", " ", "_", " ").Replace(raw)
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", 0, false
	}

	sub := yearRe.FindStringSubmatch(name)
	y, _ := strconv.Atoi(sub[1])

	return raw, y, true
}

// ParseEpisodeFilename extracts series name, season and episode number from a filename.
// Supports:
//
//	"Mr.Robot.S01E01.720p.mkv"
//	"Family.Guy.S09E05.MULTi.1080p.WEB.H264-FW.mkv"
func ParseEpisodeFilename(filename string) (series string, season, episode int, ok bool) {
	name := strings.TrimSuffix(filename, filepath.Ext(filename))

	loc := episodeRe.FindStringSubmatchIndex(name)
	if loc == nil {
		return "", 0, 0, false
	}

	raw := name[:loc[0]]
	raw = strings.NewReplacer(".", " ", "_", " ").Replace(raw)
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", 0, 0, false
	}

	sub := episodeRe.FindStringSubmatch(name)
	s, _ := strconv.Atoi(sub[1])
	e, _ := strconv.Atoi(sub[2])

	return raw, s, e, true
}
