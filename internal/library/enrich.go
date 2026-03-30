package library

import (
	"fmt"
	"log"
	"sort"
	"time"

	"github.com/MaxBly/dew/internal/media"
)

// sequentialEnrich matches disk episodes to TMDB episodes using runtime-based slot counting.
// A TMDB episode with runtime ≈ N×base occupies N disk slots (split into N parts).
//
// Primary signal: TMDB runtimes (no MediaInfo required).
// Secondary: MediaInfo disk durations used to infer base when TMDB has no runtimes.
//
// Returns true if the total expected slots exactly matches the disk episode count.
func sequentialEnrich(
	eps []media.Episode,
	tmdbEps []media.Episode,
	miDurations map[string]int, // absolute path → duration seconds (may be nil)
	seriesSlug string,
	seasonNum int,
) bool {
	base := baseRuntime(tmdbEps)

	// If TMDB has no runtimes, try to infer from MediaInfo disk durations.
	if base == 0 && len(miDurations) > 0 {
		base = inferBaseFromDisk(eps, miDurations)
	}
	if base == 0 {
		return false
	}

	totalSlots := expectedSlots(tmdbEps, base)

	// If TMDB-derived slots don't match, try again with MediaInfo-inferred base.
	if totalSlots != len(eps) && len(miDurations) > 0 {
		diskBase := inferBaseFromDisk(eps, miDurations)
		if diskBase > 0 && diskBase != base {
			alt := expectedSlots(tmdbEps, diskBase)
			if alt == len(eps) {
				base = diskBase
				totalSlots = alt
			}
		}
	}

	if totalSlots != len(eps) {
		log.Printf("library: %s S%02d — sequential match: expected %d slots (base=%dmin), got %d disk files",
			seriesSlug, seasonNum, totalSlots, base, len(eps))
		return false
	}

	log.Printf("library: %s S%02d — sequential enrichment: %d TMDB → %d disk (base=%dmin)",
		seriesSlug, seasonNum, len(tmdbEps), len(eps), base)

	diskIdx := 0
	for _, te := range tmdbEps {
		slots := runtimeSlots(te.Runtime, base)
		for part := 1; part <= slots && diskIdx < len(eps); part++ {
			applyTMDB(&eps[diskIdx], te, part, slots)
			diskIdx++
		}
	}
	return true
}

// baseRuntime returns the minimum non-zero runtime (minutes) across TMDB episodes.
// This is the "standard" episode length used as the unit for slot calculations.
func baseRuntime(tmdbEps []media.Episode) int {
	min := 0
	for _, e := range tmdbEps {
		if e.Runtime > 0 && (min == 0 || e.Runtime < min) {
			min = e.Runtime
		}
	}
	return min
}

// inferBaseFromDisk infers the base episode duration (minutes) from MediaInfo disk durations.
// Uses the mode of 5-minute-bucketed durations.
func inferBaseFromDisk(eps []media.Episode, miDurations map[string]int) int {
	freq := make(map[int]int)
	for _, ep := range eps {
		secs, ok := miDurations[ep.File]
		if !ok || secs == 0 {
			continue
		}
		mins := (secs + 30) / 60
		bucket := ((mins + 2) / 5) * 5
		if bucket > 0 {
			freq[bucket]++
		}
	}
	best, bestCount := 0, 0
	for bucket, count := range freq {
		if count > bestCount || (count == bestCount && bucket < best) {
			best, bestCount = bucket, count
		}
	}
	return best
}

// expectedSlots computes the total number of disk slots expected from a set of TMDB episodes.
func expectedSlots(tmdbEps []media.Episode, base int) int {
	total := 0
	for _, te := range tmdbEps {
		total += runtimeSlots(te.Runtime, base)
	}
	return total
}

// runtimeSlots returns how many disk episode slots a TMDB episode occupies.
// e.g. base=22, runtime=44 → 2; base=22, runtime=88 → 4; base=22, runtime=22 → 1.
func runtimeSlots(runtime, base int) int {
	if runtime == 0 || base == 0 {
		return 1
	}
	slots := (runtime + base/2) / base
	if slots < 1 {
		return 1
	}
	return slots
}

// applyTMDB copies TMDB episode metadata to a disk episode.
// When parts > 1 (split episode), appends "(Part N)" to the name.
func applyTMDB(ep *media.Episode, te media.Episode, part, parts int) {
	ep.Overview = te.Overview
	ep.AirDate = te.AirDate
	ep.Runtime = te.Runtime / parts
	ep.VoteAverage = te.VoteAverage
	ep.StillPath = te.StillPath
	if parts > 1 {
		ep.Name = fmt.Sprintf("%s (Part %d)", te.Name, part)
	} else {
		ep.Name = te.Name
	}
}

// specialEntry pairs a TMDB special episode with its parsed air date.
type specialEntry struct {
	ep      media.Episode
	airTime time.Time
}

// enrichFromSpecials tries to match unenriched disk episodes to TMDB S00 specials
// using air-date proximity and optionally MediaInfo duration similarity.
//
// For each unenriched episode it:
//  1. Estimates its air date from surrounding enriched neighbors.
//  2. Finds the closest S00 special within a ±18-month window.
//  3. If MediaInfo durations are available, also validates duration similarity.
//
// Returns the number of episodes matched.
func enrichFromSpecials(
	seasonStore map[string][]media.Episode,
	s00Eps []media.Episode,
	miDurations map[string]int,
	seriesSlug string,
) int {
	if len(s00Eps) == 0 {
		return 0
	}

	// Build a pool of S00 episodes with parsed air dates.
	pool := make([]specialEntry, 0, len(s00Eps))
	for _, e := range s00Eps {
		if t, err := time.Parse("2006-01-02", e.AirDate); err == nil {
			pool = append(pool, specialEntry{e, t})
		}
	}
	if len(pool) == 0 {
		return 0
	}
	sort.Slice(pool, func(i, j int) bool { return pool[i].airTime.Before(pool[j].airTime) })

	matched := 0
	// Process seasons in numeric order for deterministic behavior.
	seasonKeys := make([]string, 0, len(seasonStore))
	for k := range seasonStore {
		seasonKeys = append(seasonKeys, k)
	}
	sort.Strings(seasonKeys)

	for _, seasonKey := range seasonKeys {
		eps := seasonStore[seasonKey]

		// Collect enriched air dates for this season to establish the window.
		var airTimes []time.Time
		for _, ep := range eps {
			if ep.Name != "" && ep.AirDate != "" {
				if t, err := time.Parse("2006-01-02", ep.AirDate); err == nil {
					airTimes = append(airTimes, t)
				}
			}
		}
		if len(airTimes) == 0 {
			continue
		}
		sort.Slice(airTimes, func(i, j int) bool { return airTimes[i].Before(airTimes[j]) })
		seasonStart := airTimes[0]
		seasonEnd := airTimes[len(airTimes)-1]
		windowStart := seasonStart.AddDate(0, -3, 0)
		windowEnd := seasonEnd.AddDate(0, 9, 0)

		for i, ep := range eps {
			if ep.Name != "" {
				continue
			}

			est := estimateAirDate(eps, i)
			if est.IsZero() {
				est = seasonEnd.AddDate(0, 0, 7)
			}

			// Find best matching special within the air-date window.
			bestIdx := -1
			bestDist := time.Duration(1 << 62)
			for j, se := range pool {
				if se.airTime.Before(windowStart) || se.airTime.After(windowEnd) {
					continue
				}
				d := se.airTime.Sub(est)
				if d < 0 {
					d = -d
				}
				// Optional: if MediaInfo duration is available, penalise large duration mismatches.
				if len(miDurations) > 0 && se.ep.Runtime > 0 {
					diskSecs, ok := miDurations[ep.File]
					if ok && diskSecs > 0 {
						diskMins := diskSecs / 60
						delta := se.ep.Runtime - diskMins
						if delta < 0 {
							delta = -delta
						}
						// Skip if durations differ by more than 50% of the special's runtime.
						if delta*2 > se.ep.Runtime {
							continue
						}
					}
				}
				if d < bestDist {
					bestDist = d
					bestIdx = j
				}
			}

			if bestIdx < 0 || bestDist > 365*24*time.Hour {
				continue
			}

			best := pool[bestIdx]
			eps[i].Name = best.ep.Name
			eps[i].Overview = best.ep.Overview
			eps[i].AirDate = best.ep.AirDate
			eps[i].Runtime = best.ep.Runtime
			eps[i].VoteAverage = best.ep.VoteAverage
			eps[i].StillPath = best.ep.StillPath
			log.Printf("library: %s S%sE%02d → special %q (air %s, Δ%dd)",
				seriesSlug, seasonKey, ep.EpisodeNumber, best.ep.Name,
				best.ep.AirDate, int(bestDist.Hours()/24))
			matched++
			seasonStore[seasonKey] = eps
			// Remove matched special from pool to prevent duplicate assignments.
			pool = append(pool[:bestIdx], pool[bestIdx+1:]...)
		}
	}
	return matched
}

// estimateAirDate estimates the air date for an unenriched episode at index i
// based on the air dates of its enriched neighbors.
func estimateAirDate(eps []media.Episode, i int) time.Time {
	var prev, next time.Time
	for j := i - 1; j >= 0; j-- {
		if eps[j].AirDate != "" {
			if t, err := time.Parse("2006-01-02", eps[j].AirDate); err == nil {
				prev = t
				break
			}
		}
	}
	for j := i + 1; j < len(eps); j++ {
		if eps[j].AirDate != "" {
			if t, err := time.Parse("2006-01-02", eps[j].AirDate); err == nil {
				next = t
				break
			}
		}
	}
	if !prev.IsZero() && !next.IsZero() {
		return prev.Add(next.Sub(prev) / 2)
	}
	if !prev.IsZero() {
		return prev.AddDate(0, 0, 7)
	}
	if !next.IsZero() {
		return next.AddDate(0, 0, -7)
	}
	return time.Time{}
}
