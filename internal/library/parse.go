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
	// matches SxxExx patterns (case-insensitive), episode number up to 3 digits (e.g. Kaamelott S01E001)
	episodeRe = regexp.MustCompile(`(?i)[Ss](\d{1,2})[Ee](\d{1,3})`)
	// matches content in square brackets
	bracketRe = regexp.MustCompile(`\[.*?\]`)
	// matches content in parentheses
	parenRe = regexp.MustCompile(`\(.*?\)`)
	// matches leading collection number prefix like "10. " or "10 " or "11. "
	leadingNumRe = regexp.MustCompile(`^\d+[\. ]+`)
	// trailing 4-digit year at end of series folder name
	trailingYearRe = regexp.MustCompile(`\s+((?:19|20)\d{2})$`)
	// common release tags to strip from titles (word-boundary match)
	// Also strips resolution strings like 1080p, 2160p, 4K, 4KLight.
	releaseTagRe = regexp.MustCompile(`(?i)\b(MULTi|VFF|VF2|VF|VO[A-Z]*|UNRATED|EXTENDED|THEATRICAL|REMASTERED|DIRECTORS?.?CUT|HDLight|BluRay|WEB|x264|x265|H264|H265|AAC|AC3|DDP|DTS|FRENCH|TRUEHDL?|4KLight|4K|\d{3,4}p)\b.*`)
	// "Version anything" suffix
	versionRe = regexp.MustCompile(`(?i)\bVersion\b.*`)
	// multiple spaces
	multiSpaceRe = regexp.MustCompile(`\s{2,}`)
)

// ParseMovieFilename extracts title and year from a movie filename.
// Uses the LAST year match so "Blade Runner 2049 (2017)" correctly yields year=2017.
// Supports:
//
//	"The.Dark.Knight.2008.1080p.BluRay.mkv"
//	"The Dark Knight (2008) 1080p BluRay.mkv"
//	"Blade Runner 2049 (2017) MULTi.mkv"
//	"10.F9 The Fast Saga (2021) Directors Cut MULTi.mkv"
func ParseMovieFilename(filename string) (title string, year int, ok bool) {
	name := strings.TrimSuffix(filename, filepath.Ext(filename))
	// Replace dots and underscores used as separators before searching.
	name = strings.NewReplacer(".", " ", "_", " ").Replace(name)

	// Find all year matches and use the LAST one.
	// This handles titles like "Blade Runner 2049 (2017)".
	matches := yearRe.FindAllStringSubmatchIndex(name, -1)
	if len(matches) == 0 {
		return "", 0, false
	}
	last := matches[len(matches)-1]

	raw := strings.TrimSpace(name[:last[0]])
	if raw == "" {
		return "", 0, false
	}

	y, _ := strconv.Atoi(name[last[2]:last[3]])
	return raw, y, true
}

// CleanTitle strips release tags, brackets, parentheticals, and collection
// prefixes from a raw parsed title to improve TMDB search accuracy.
//
//	"Star Wars I [1080p] MULTi"          → "Star Wars I"
//	"10 F9 The Fast Saga"                → "F9 The Fast Saga"
//	"A History of Violence [1080p] MULTi"→ "A History of Violence"
//	"Enter the Void Version cinéma"      → "Enter the Void"
//	"Lady Vengeance (Fade to Black)"     → "Lady Vengeance"
func CleanTitle(title string) string {
	s := leadingNumRe.ReplaceAllString(title, "")
	// If stripping the leading number left an empty string or another digit
	// (e.g. "11 6" → "6"), the number was part of the title — keep original.
	if t := strings.TrimSpace(s); t == "" || (len(t) > 0 && t[0] >= '0' && t[0] <= '9') {
		s = title
	}
	s = bracketRe.ReplaceAllString(s, "")
	s = parenRe.ReplaceAllString(s, "")
	s = versionRe.ReplaceAllString(s, "")
	s = releaseTagRe.ReplaceAllString(s, "")
	s = multiSpaceRe.ReplaceAllString(s, " ")
	return strings.TrimSpace(s)
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

// StripFolderYear removes a trailing 4-digit year from a series folder name.
// "Shameless 2011" → ("Shameless", 2011)
// "H 1998"         → ("H", 1998)
// "Mr Robot"       → ("Mr Robot", 0)
func StripFolderYear(name string) (stripped string, year int) {
	m := trailingYearRe.FindStringSubmatchIndex(name)
	if m == nil {
		return name, 0
	}
	y, _ := strconv.Atoi(name[m[2]:m[3]])
	return strings.TrimSpace(name[:m[0]]), y
}
