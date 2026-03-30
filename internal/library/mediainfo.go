package library

import (
	"encoding/json"
	"fmt"
	"log"
	"os/exec"
	"strconv"
	"time"

	"github.com/MaxBly/dew/internal/media"
	"github.com/MaxBly/dew/internal/store"
)

// ScanMediaInfo runs ffprobe on every tracked file that is not yet in the cache.
// Only processes files absent from cache; existing entries are left unchanged.
func ScanMediaInfo(
	movies *store.JsonStore[media.MovieStore],
	seasons *store.JsonStore[media.SeasonStore],
	mi *store.JsonStore[media.MediaInfoStore],
) error {
	cache := mi.Get()
	if cache == nil {
		cache = make(media.MediaInfoStore)
	}

	// Collect all tracked paths.
	paths := map[string]bool{}
	if ms := movies.Get(); ms != nil {
		for _, m := range ms {
			for _, f := range m.Files {
				paths[f] = true
			}
		}
	}
	if ss := seasons.Get(); ss != nil {
		for _, seriesSeasons := range ss {
			for _, eps := range seriesSeasons {
				for _, ep := range eps {
					if ep.File != "" {
						paths[ep.File] = true
					}
				}
			}
		}
	}

	changed := false
	for path := range paths {
		if _, ok := cache[path]; ok {
			continue
		}
		info, err := probeFile(path)
		if err != nil {
			log.Printf("mediainfo: probe failed for %s: %v", path, err)
			continue
		}
		cache[path] = info
		changed = true
		log.Printf("mediainfo: %s — %s %s %ds", path, info.VideoCodec, info.Resolution, info.Duration)
	}

	if changed {
		return mi.Set(cache)
	}
	return nil
}

// probeFile runs ffprobe on a single file and returns structured MediaInfo.
func probeFile(path string) (media.MediaInfo, error) {
	out, err := exec.Command("ffprobe",
		"-v", "quiet",
		"-print_format", "json",
		"-show_streams",
		"-show_format",
		path,
	).Output()
	if err != nil {
		return media.MediaInfo{}, fmt.Errorf("ffprobe: %w", err)
	}

	var probe struct {
		Format struct {
			Duration string `json:"duration"`
		} `json:"format"`
		Streams []struct {
			Index     int    `json:"index"`
			CodecType string `json:"codec_type"`
			CodecName string `json:"codec_name"`
			Width     int    `json:"width"`
			Height    int    `json:"height"`
			Channels  int    `json:"channels"`
			Tags      struct {
				Language string `json:"language"`
				Title    string `json:"title"`
			} `json:"tags"`
			Disposition struct {
				Default int `json:"default"`
				Forced  int `json:"forced"`
			} `json:"disposition"`
		} `json:"streams"`
	}
	if err := json.Unmarshal(out, &probe); err != nil {
		return media.MediaInfo{}, err
	}

	durationSecs := 0
	if d, err := strconv.ParseFloat(probe.Format.Duration, 64); err == nil {
		durationSecs = int(d)
	}

	var resolution, videoCodec string
	var audio []media.AudioTrack
	var subs []media.SubtitleTrack

	for _, st := range probe.Streams {
		switch st.CodecType {
		case "video":
			if videoCodec == "" {
				videoCodec = st.CodecName
				if st.Height > 0 {
					resolution = fmt.Sprintf("%dx%d", st.Width, st.Height)
				}
			}
		case "audio":
			audio = append(audio, media.AudioTrack{
				Index:    st.Index,
				Lang:     st.Tags.Language,
				Default:  st.Disposition.Default == 1,
				Format:   st.CodecName,
				Channels: fmt.Sprintf("%d", st.Channels),
				Title:    st.Tags.Title,
			})
		case "subtitle":
			subs = append(subs, media.SubtitleTrack{
				Index:   st.Index,
				Lang:    st.Tags.Language,
				Default: st.Disposition.Default == 1,
				Forced:  st.Disposition.Forced == 1,
				Format:  st.CodecName,
				Title:   st.Tags.Title,
			})
		}
	}

	return media.MediaInfo{
		Resolution:     resolution,
		VideoCodec:     videoCodec,
		Duration:       durationSecs,
		AudioTracks:    audio,
		SubtitleTracks: subs,
		ScannedAt:      time.Now().UTC().Format(time.RFC3339),
	}, nil
}
