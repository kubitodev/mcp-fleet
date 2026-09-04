package jellyfin

import (
	"reflect"
	"testing"
)

// --- ToMediaItem ---

func TestToMediaItem_AllFields(t *testing.T) {
	m := map[string]any{
		"id":                  "abc-123",
		"name":                "Test Movie",
		"type":                "Movie",
		"year":                float64(2024),
		"overview":            "A test movie overview.",
		"community_rating":    float64(8.5),
		"official_rating":     "PG-13",
		"runtime_minutes":     float64(120),
		"played":              true,
		"progress":            "45%",
		"favorite":            true,
		"series_name":         "Test Series",
		"index_number":        float64(3),
		"parent_index_number": float64(1),
		"last_played":         "2024-06-15",
		"date_added":          "2024-01-01",
	}

	got := ToMediaItem(m)

	want := MediaItem{
		ID:                "abc-123",
		Name:              "Test Movie",
		Type:              "Movie",
		Year:              2024,
		Overview:          "A test movie overview.",
		CommunityRating:   8.5,
		OfficialRating:    "PG-13",
		RuntimeMinutes:    120,
		Played:            true,
		Progress:          "45%",
		Favorite:          true,
		SeriesName:        "Test Series",
		IndexNumber:       3,
		ParentIndexNumber: 1,
		LastPlayed:        "2024-06-15",
		DateAdded:         "2024-01-01",
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("ToMediaItem all fields:\n got  %+v\n want %+v", got, want)
	}
}

func TestToMediaItem_IntNumericFields(t *testing.T) {
	// When maps are built internally (not via JSON), numeric fields may be int or int64.
	m := map[string]any{
		"id":              "item-int",
		"name":            "Int Movie",
		"type":            "Movie",
		"year":            int(2023),
		"runtime_minutes": int64(90),
		"index_number":    int(5),
	}

	got := ToMediaItem(m)

	if got.Year != 2023 {
		t.Errorf("Year: got %d, want 2023", got.Year)
	}
	if got.RuntimeMinutes != 90 {
		t.Errorf("RuntimeMinutes: got %d, want 90", got.RuntimeMinutes)
	}
	if got.IndexNumber != 5 {
		t.Errorf("IndexNumber: got %d, want 5", got.IndexNumber)
	}
}

func TestToMediaItem_EmptyMap(t *testing.T) {
	got := ToMediaItem(map[string]any{})

	want := MediaItem{}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ToMediaItem empty map:\n got  %+v\n want %+v", got, want)
	}
}

func TestToMediaItem_MissingFields(t *testing.T) {
	m := map[string]any{
		"id":   "minimal",
		"name": "Minimal",
		"type": "Series",
	}

	got := ToMediaItem(m)

	if got.ID != "minimal" || got.Name != "Minimal" || got.Type != "Series" {
		t.Errorf("basic fields wrong: %+v", got)
	}
	if got.Year != 0 || got.RuntimeMinutes != 0 || got.CommunityRating != 0 {
		t.Error("numeric fields should be zero for missing keys")
	}
	if got.Played || got.Favorite {
		t.Error("bool fields should be false for missing keys")
	}
	if got.Overview != "" || got.Progress != "" || got.SeriesName != "" {
		t.Error("string fields should be empty for missing keys")
	}
}

// --- ToMediaItems ---

func TestToMediaItems_Multiple(t *testing.T) {
	items := []map[string]any{
		{"id": "a", "name": "Alpha", "type": "Movie"},
		{"id": "b", "name": "Beta", "type": "Series"},
		{"id": "c", "name": "Gamma", "type": "Episode"},
	}

	got := ToMediaItems(items)

	if len(got) != 3 {
		t.Fatalf("expected 3 items, got %d", len(got))
	}
	if got[0].ID != "a" || got[1].ID != "b" || got[2].ID != "c" {
		t.Errorf("IDs mismatch: %v, %v, %v", got[0].ID, got[1].ID, got[2].ID)
	}
}

func TestToMediaItems_Empty(t *testing.T) {
	got := ToMediaItems([]map[string]any{})

	if got == nil {
		t.Fatal("expected non-nil empty slice, got nil")
	}
	if len(got) != 0 {
		t.Errorf("expected 0 items, got %d", len(got))
	}
}

func TestToMediaItems_Nil(t *testing.T) {
	got := ToMediaItems(nil)

	if got == nil {
		t.Fatal("expected non-nil empty slice, got nil")
	}
	if len(got) != 0 {
		t.Errorf("expected 0 items, got %d", len(got))
	}
}

// --- ToLibraryInfo ---

func TestToLibraryInfo_PathsAsStringSlice(t *testing.T) {
	m := map[string]any{
		"name":            "Movies",
		"collection_type": "movies",
		"item_id":         "lib-1",
		"paths":           []string{"/media/movies", "/media/movies2"},
	}

	got := ToLibraryInfo(m)

	want := LibraryInfo{
		Name:           "Movies",
		CollectionType: "movies",
		ItemID:         "lib-1",
		Paths:          []string{"/media/movies", "/media/movies2"},
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("ToLibraryInfo []string paths:\n got  %+v\n want %+v", got, want)
	}
}

func TestToLibraryInfo_PathsAsAnySlice(t *testing.T) {
	m := map[string]any{
		"name":            "TV Shows",
		"collection_type": "tvshows",
		"item_id":         "lib-2",
		"paths":           []any{"/media/tv", "/media/tv2"},
	}

	got := ToLibraryInfo(m)

	want := LibraryInfo{
		Name:           "TV Shows",
		CollectionType: "tvshows",
		ItemID:         "lib-2",
		Paths:          []string{"/media/tv", "/media/tv2"},
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("ToLibraryInfo []any paths:\n got  %+v\n want %+v", got, want)
	}
}

func TestToLibraryInfo_MissingPaths(t *testing.T) {
	m := map[string]any{
		"name":            "Music",
		"collection_type": "music",
		"item_id":         "lib-3",
	}

	got := ToLibraryInfo(m)

	if got.Paths != nil {
		t.Errorf("expected nil Paths, got %v", got.Paths)
	}
}

func TestToLibraryInfo_EmptyAnyPaths(t *testing.T) {
	// []any with no strings yields nil Paths
	m := map[string]any{
		"name":            "Mixed",
		"collection_type": "mixed",
		"item_id":         "lib-4",
		"paths":           []any{42, true},
	}

	got := ToLibraryInfo(m)

	if got.Paths != nil {
		t.Errorf("expected nil Paths when []any has no strings, got %v", got.Paths)
	}
}

// --- ToLibraryInfos ---

func TestToLibraryInfos_Multiple(t *testing.T) {
	items := []map[string]any{
		{"name": "Movies", "collection_type": "movies", "item_id": "1"},
		{"name": "TV", "collection_type": "tvshows", "item_id": "2"},
	}

	got := ToLibraryInfos(items)

	if len(got) != 2 {
		t.Fatalf("expected 2 libraries, got %d", len(got))
	}
	if got[0].Name != "Movies" || got[1].Name != "TV" {
		t.Errorf("names mismatch: %s, %s", got[0].Name, got[1].Name)
	}
}

func TestToLibraryInfos_Empty(t *testing.T) {
	got := ToLibraryInfos([]map[string]any{})

	if len(got) != 0 {
		t.Errorf("expected 0 items, got %d", len(got))
	}
}

// --- ToSessionInfo ---

func TestToSessionInfo_WithNowPlayingAndPlayState(t *testing.T) {
	m := map[string]any{
		"session_id":    "sess-1",
		"user":          "Alice",
		"client":        "Jellyfin Web",
		"device_name":   "Firefox",
		"last_activity": "2024-06-15T12:00:00Z",
		"now_playing": map[string]any{
			"id":              "np-item-1",
			"name":            "Windmill Meadow",
			"type":            "Movie",
			"runtime_minutes": float64(10),
		},
		"play_state": map[string]any{
			"is_paused":        false,
			"position_seconds": float64(300),
			"position_ticks":   float64(3000000000),
			"volume":           float64(80),
		},
	}

	got := ToSessionInfo(m)

	if got.SessionID != "sess-1" {
		t.Errorf("SessionID: got %q, want %q", got.SessionID, "sess-1")
	}
	if got.User != "Alice" {
		t.Errorf("User: got %q, want %q", got.User, "Alice")
	}
	if got.Client != "Jellyfin Web" {
		t.Errorf("Client: got %q, want %q", got.Client, "Jellyfin Web")
	}
	if got.DeviceName != "Firefox" {
		t.Errorf("DeviceName: got %q, want %q", got.DeviceName, "Firefox")
	}

	if got.NowPlaying == nil {
		t.Fatal("NowPlaying should not be nil")
	}
	if got.NowPlaying.ID != "np-item-1" {
		t.Errorf("NowPlaying.ID: got %q, want %q", got.NowPlaying.ID, "np-item-1")
	}
	if got.NowPlaying.Name != "Windmill Meadow" {
		t.Errorf("NowPlaying.Name: got %q", got.NowPlaying.Name)
	}
	if got.NowPlaying.RuntimeMinutes != 10 {
		t.Errorf("NowPlaying.RuntimeMinutes: got %d, want 10", got.NowPlaying.RuntimeMinutes)
	}

	if got.PlayState == nil {
		t.Fatal("PlayState should not be nil")
	}
	if got.PlayState.IsPaused {
		t.Error("PlayState.IsPaused should be false")
	}
	if got.PlayState.PositionSeconds != 300 {
		t.Errorf("PlayState.PositionSeconds: got %d, want 300", got.PlayState.PositionSeconds)
	}
	if got.PlayState.PositionTicks != 3000000000 {
		t.Errorf("PlayState.PositionTicks: got %d, want 3000000000", got.PlayState.PositionTicks)
	}
	if got.PlayState.Volume != 80 {
		t.Errorf("PlayState.Volume: got %d, want 80", got.PlayState.Volume)
	}
}

func TestToSessionInfo_IdleSession(t *testing.T) {
	m := map[string]any{
		"session_id":    "sess-idle",
		"user":          "Bob",
		"client":        "Android",
		"device_name":   "Pixel 8",
		"last_activity": "2024-06-14T08:00:00Z",
	}

	got := ToSessionInfo(m)

	if got.NowPlaying != nil {
		t.Errorf("NowPlaying should be nil for idle session, got %+v", got.NowPlaying)
	}
	if got.PlayState != nil {
		t.Errorf("PlayState should be nil for idle session, got %+v", got.PlayState)
	}
	if got.User != "Bob" {
		t.Errorf("User: got %q, want %q", got.User, "Bob")
	}
}

func TestToSessionInfo_NumericFieldsInt64(t *testing.T) {
	// Internally built maps may use int64 instead of float64.
	m := map[string]any{
		"session_id":  "sess-int",
		"user":        "Carol",
		"client":      "iOS",
		"device_name": "iPad",
		"now_playing": map[string]any{
			"id":              "np-2",
			"name":            "Test",
			"type":            "Episode",
			"runtime_minutes": int64(45),
		},
		"play_state": map[string]any{
			"position_seconds": int64(600),
			"position_ticks":   int64(6000000000),
			"volume":           int(50),
		},
	}

	got := ToSessionInfo(m)

	if got.NowPlaying == nil {
		t.Fatal("NowPlaying should not be nil")
	}
	if got.NowPlaying.RuntimeMinutes != 45 {
		t.Errorf("NowPlaying.RuntimeMinutes: got %d, want 45", got.NowPlaying.RuntimeMinutes)
	}

	if got.PlayState == nil {
		t.Fatal("PlayState should not be nil")
	}
	if got.PlayState.PositionSeconds != 600 {
		t.Errorf("PositionSeconds: got %d, want 600", got.PlayState.PositionSeconds)
	}
	if got.PlayState.PositionTicks != 6000000000 {
		t.Errorf("PositionTicks: got %d, want 6000000000", got.PlayState.PositionTicks)
	}
	if got.PlayState.Volume != 50 {
		t.Errorf("Volume: got %d, want 50", got.PlayState.Volume)
	}
}

// --- ToSessionInfos ---

func TestToSessionInfos_Multiple(t *testing.T) {
	items := []map[string]any{
		{"session_id": "s1", "user": "A", "client": "Web", "device_name": "Chrome"},
		{"session_id": "s2", "user": "B", "client": "Android", "device_name": "Phone"},
	}

	got := ToSessionInfos(items)

	if len(got) != 2 {
		t.Fatalf("expected 2 sessions, got %d", len(got))
	}
	if got[0].SessionID != "s1" || got[1].SessionID != "s2" {
		t.Errorf("session IDs mismatch")
	}
}

func TestToSessionInfos_Empty(t *testing.T) {
	got := ToSessionInfos([]map[string]any{})

	if len(got) != 0 {
		t.Errorf("expected 0 sessions, got %d", len(got))
	}
}

// --- ToDetailedItemOutput ---

func TestToDetailedItemOutput_FullItem(t *testing.T) {
	m := map[string]any{
		"id":                   "detail-1",
		"name":                 "Lucid Horizon",
		"type":                 "Movie",
		"year":                 float64(2010),
		"overview":             "A rogue architect bends reality in layered dreamscapes.",
		"community_rating":     float64(8.8),
		"official_rating":      "PG-13",
		"critic_rating":        float64(87.0),
		"runtime_minutes":      float64(148),
		"premiere_date":        "2010-07-16",
		"end_date":             "",
		"file_path":            "/movies/lucid-horizon/lucid-horizon.mkv",
		"file_name":            "lucid-horizon.mkv",
		"album":                "",
		"status":               "Released",
		"series_name":          "",
		"progress":             "",
		"played":               true,
		"favorite":             true,
		"has_subtitles":        true,
		"has_lyrics":           false,
		"index_number":         float64(0),
		"parent_index_number":  float64(0),
		"child_count":          float64(0),
		"recursive_item_count": float64(0),
		"taglines":             []any{"Every layer hides another truth."},
		"genres":               []any{"Sci-Fi", "Action", "Thriller"},
		"studios":              []any{"Ridgeline Studios"},
		"artists":              []string{"Elara Voss"},
		"people": []map[string]any{
			{"name": "Marcus Hale", "type": "Actor", "role": "Cobb"},
			{"name": "Delia Soren", "type": "Director", "role": ""},
		},
		"provider_ids":  map[string]string{"Imdb": "tt0000000", "Tmdb": "99999"},
		"external_urls": map[string]string{"IMDb": "https://www.imdb.com/title/tt0000000"},
		"user_data": map[string]any{
			"played":            true,
			"favorite":          true,
			"play_count":        float64(3),
			"played_percentage": float64(100.0),
			"last_played":       "2024-06-10",
		},
		"media_sources": []map[string]any{
			{
				"container":       "mkv",
				"path":            "/movies/lucid-horizon/lucid-horizon.mkv",
				"bitrate_kbps":    float64(15000),
				"size_mb":         float64(12000),
				"video_codec":     "hevc",
				"resolution":      "1920x1080",
				"video_profile":   "Main 10",
				"video_bit_depth": float64(10),
				"video_range":     "HDR",
				"audio_streams": []map[string]any{
					{
						"index":         float64(1),
						"codec":         "truehd",
						"channels":      float64(8),
						"language":      "eng",
						"display_title": "TrueHD 7.1 Atmos",
						"is_default":    true,
					},
					{
						"index":         float64(2),
						"codec":         "aac",
						"channels":      float64(2),
						"language":      "eng",
						"display_title": "AAC Stereo",
						"is_default":    false,
					},
				},
				"subtitle_streams": []map[string]any{
					{
						"index":         float64(3),
						"codec":         "srt",
						"language":      "eng",
						"display_title": "English (SRT)",
						"is_external":   true,
						"is_default":    true,
						"is_forced":     false,
					},
				},
			},
		},
	}

	got := ToDetailedItemOutput(m)

	if got == nil {
		t.Fatal("expected non-nil result")
	}

	// Basic fields
	if got.ID != "detail-1" {
		t.Errorf("ID: got %q", got.ID)
	}
	if got.Name != "Lucid Horizon" {
		t.Errorf("Name: got %q", got.Name)
	}
	if got.Type != "Movie" {
		t.Errorf("Type: got %q", got.Type)
	}
	if got.Year != 2010 {
		t.Errorf("Year: got %d", got.Year)
	}
	if got.CommunityRating != 8.8 {
		t.Errorf("CommunityRating: got %f", got.CommunityRating)
	}
	if got.CriticRating != 87.0 {
		t.Errorf("CriticRating: got %f", got.CriticRating)
	}
	if got.RuntimeMinutes != 148 {
		t.Errorf("RuntimeMinutes: got %d", got.RuntimeMinutes)
	}
	if !got.Played {
		t.Error("Played should be true")
	}
	if !got.Favorite {
		t.Error("Favorite should be true")
	}
	if !got.HasSubtitles {
		t.Error("HasSubtitles should be true")
	}
	if got.HasLyrics {
		t.Error("HasLyrics should be false")
	}
	if got.Status != "Released" {
		t.Errorf("Status: got %q", got.Status)
	}

	// Taglines
	wantTaglines := []string{"Every layer hides another truth."}
	if !reflect.DeepEqual(got.Taglines, wantTaglines) {
		t.Errorf("Taglines: got %v, want %v", got.Taglines, wantTaglines)
	}

	// Genres
	wantGenres := []string{"Sci-Fi", "Action", "Thriller"}
	if !reflect.DeepEqual(got.Genres, wantGenres) {
		t.Errorf("Genres: got %v, want %v", got.Genres, wantGenres)
	}

	// Studios
	wantStudios := []string{"Ridgeline Studios"}
	if !reflect.DeepEqual(got.Studios, wantStudios) {
		t.Errorf("Studios: got %v, want %v", got.Studios, wantStudios)
	}

	// Artists (as []string, handled by ToStringSlice)
	wantArtists := []string{"Elara Voss"}
	if !reflect.DeepEqual(got.Artists, wantArtists) {
		t.Errorf("Artists: got %v, want %v", got.Artists, wantArtists)
	}

	// People
	if len(got.People) != 2 {
		t.Fatalf("People: expected 2, got %d", len(got.People))
	}
	if got.People[0].Name != "Marcus Hale" || got.People[0].Role != "Cobb" {
		t.Errorf("People[0]: %+v", got.People[0])
	}
	if got.People[1].Name != "Delia Soren" || got.People[1].Type != "Director" {
		t.Errorf("People[1]: %+v", got.People[1])
	}

	// Provider IDs
	wantPIDs := map[string]string{"Imdb": "tt0000000", "Tmdb": "99999"}
	if !reflect.DeepEqual(got.ProviderIDs, wantPIDs) {
		t.Errorf("ProviderIDs: got %v, want %v", got.ProviderIDs, wantPIDs)
	}

	// External URLs
	wantURLs := map[string]string{"IMDb": "https://www.imdb.com/title/tt0000000"}
	if !reflect.DeepEqual(got.ExternalURLs, wantURLs) {
		t.Errorf("ExternalURLs: got %v, want %v", got.ExternalURLs, wantURLs)
	}

	// User data
	if got.UserData == nil {
		t.Fatal("UserData should not be nil")
	}
	if !got.UserData.Played {
		t.Error("UserData.Played should be true")
	}
	if !got.UserData.Favorite {
		t.Error("UserData.Favorite should be true")
	}
	if got.UserData.PlayCount != 3 {
		t.Errorf("UserData.PlayCount: got %d", got.UserData.PlayCount)
	}
	if got.UserData.PlayedPercentage != 100.0 {
		t.Errorf("UserData.PlayedPercentage: got %f", got.UserData.PlayedPercentage)
	}
	if got.UserData.LastPlayed != "2024-06-10" {
		t.Errorf("UserData.LastPlayed: got %q", got.UserData.LastPlayed)
	}

	// Media sources
	if len(got.MediaSources) != 1 {
		t.Fatalf("MediaSources: expected 1, got %d", len(got.MediaSources))
	}
	src := got.MediaSources[0]
	if src.Container != "mkv" {
		t.Errorf("Container: got %q", src.Container)
	}
	if src.VideoCodec != "hevc" {
		t.Errorf("VideoCodec: got %q", src.VideoCodec)
	}
	if src.Resolution != "1920x1080" {
		t.Errorf("Resolution: got %q", src.Resolution)
	}
	if src.BitrateKbps != 15000 {
		t.Errorf("BitrateKbps: got %d", src.BitrateKbps)
	}
	if src.SizeMB != 12000 {
		t.Errorf("SizeMB: got %d", src.SizeMB)
	}
	if src.VideoBitDepth != 10 {
		t.Errorf("VideoBitDepth: got %d", src.VideoBitDepth)
	}
	if src.VideoRange != "HDR" {
		t.Errorf("VideoRange: got %q", src.VideoRange)
	}
	if src.VideoProfile != "Main 10" {
		t.Errorf("VideoProfile: got %q", src.VideoProfile)
	}

	// Audio streams
	if len(src.AudioStreams) != 2 {
		t.Fatalf("AudioStreams: expected 2, got %d", len(src.AudioStreams))
	}
	a0 := src.AudioStreams[0]
	if a0.Index != 1 || a0.Codec != "truehd" || a0.Channels != 8 || a0.Language != "eng" || !a0.IsDefault {
		t.Errorf("AudioStreams[0]: %+v", a0)
	}
	a1 := src.AudioStreams[1]
	if a1.Index != 2 || a1.Codec != "aac" || a1.Channels != 2 || a1.IsDefault {
		t.Errorf("AudioStreams[1]: %+v", a1)
	}

	// Subtitle streams
	if len(src.SubtitleStreams) != 1 {
		t.Fatalf("SubtitleStreams: expected 1, got %d", len(src.SubtitleStreams))
	}
	s0 := src.SubtitleStreams[0]
	if s0.Index != 3 || s0.Codec != "srt" || s0.Language != "eng" || !s0.IsExternal || !s0.IsDefault || s0.IsForced {
		t.Errorf("SubtitleStreams[0]: %+v", s0)
	}
}

func TestToDetailedItemOutput_MinimalItem(t *testing.T) {
	m := map[string]any{
		"id":   "minimal-1",
		"name": "Minimal",
		"type": "Movie",
	}

	got := ToDetailedItemOutput(m)

	if got == nil {
		t.Fatal("expected non-nil result")
	}
	if got.ID != "minimal-1" {
		t.Errorf("ID: got %q", got.ID)
	}
	if got.Name != "Minimal" {
		t.Errorf("Name: got %q", got.Name)
	}
	if got.UserData != nil {
		t.Error("UserData should be nil when not in map")
	}
	if got.People != nil {
		t.Errorf("People should be nil, got %v", got.People)
	}
	if got.MediaSources != nil {
		t.Errorf("MediaSources should be nil, got %v", got.MediaSources)
	}
	if got.ProviderIDs != nil {
		t.Errorf("ProviderIDs should be nil, got %v", got.ProviderIDs)
	}
	if got.ExternalURLs != nil {
		t.Errorf("ExternalURLs should be nil, got %v", got.ExternalURLs)
	}
	if got.Taglines != nil {
		t.Errorf("Taglines should be nil, got %v", got.Taglines)
	}
	if got.Genres != nil {
		t.Errorf("Genres should be nil, got %v", got.Genres)
	}
}

func TestToDetailedItemOutput_IntNumericFields(t *testing.T) {
	m := map[string]any{
		"id":                   "int-detail",
		"name":                 "IntTest",
		"type":                 "Movie",
		"year":                 int(2022),
		"runtime_minutes":      int64(120),
		"child_count":          int(5),
		"recursive_item_count": int(50),
		"index_number":         int(2),
		"parent_index_number":  int(1),
		"media_sources": []map[string]any{
			{
				"container":       "mp4",
				"bitrate_kbps":    int64(8000),
				"size_mb":         int64(5000),
				"video_bit_depth": int(8),
				"audio_streams": []map[string]any{
					{
						"index":    int(0),
						"codec":    "aac",
						"channels": int(6),
					},
				},
				"subtitle_streams": []map[string]any{
					{
						"index": int(0),
						"codec": "subrip",
					},
				},
			},
		},
		"user_data": map[string]any{
			"play_count":        int(7),
			"played_percentage": float64(85.5),
		},
	}

	got := ToDetailedItemOutput(m)

	if got.Year != 2022 {
		t.Errorf("Year: got %d, want 2022", got.Year)
	}
	if got.RuntimeMinutes != 120 {
		t.Errorf("RuntimeMinutes: got %d, want 120", got.RuntimeMinutes)
	}
	if got.ChildCount != 5 {
		t.Errorf("ChildCount: got %d, want 5", got.ChildCount)
	}
	if got.RecursiveItemCount != 50 {
		t.Errorf("RecursiveItemCount: got %d, want 50", got.RecursiveItemCount)
	}
	if got.IndexNumber != 2 {
		t.Errorf("IndexNumber: got %d, want 2", got.IndexNumber)
	}
	if got.ParentIndexNumber != 1 {
		t.Errorf("ParentIndexNumber: got %d, want 1", got.ParentIndexNumber)
	}

	if len(got.MediaSources) != 1 {
		t.Fatalf("expected 1 media source, got %d", len(got.MediaSources))
	}
	src := got.MediaSources[0]
	if src.BitrateKbps != 8000 {
		t.Errorf("BitrateKbps: got %d, want 8000", src.BitrateKbps)
	}
	if src.SizeMB != 5000 {
		t.Errorf("SizeMB: got %d, want 5000", src.SizeMB)
	}
	if src.VideoBitDepth != 8 {
		t.Errorf("VideoBitDepth: got %d, want 8", src.VideoBitDepth)
	}

	if len(src.AudioStreams) != 1 {
		t.Fatalf("expected 1 audio stream, got %d", len(src.AudioStreams))
	}
	if src.AudioStreams[0].Channels != 6 {
		t.Errorf("AudioStreams[0].Channels: got %d, want 6", src.AudioStreams[0].Channels)
	}

	if len(src.SubtitleStreams) != 1 {
		t.Fatalf("expected 1 subtitle stream, got %d", len(src.SubtitleStreams))
	}
	if src.SubtitleStreams[0].Index != 0 {
		t.Errorf("SubtitleStreams[0].Index: got %d, want 0", src.SubtitleStreams[0].Index)
	}

	if got.UserData == nil {
		t.Fatal("UserData should not be nil")
	}
	if got.UserData.PlayCount != 7 {
		t.Errorf("UserData.PlayCount: got %d, want 7", got.UserData.PlayCount)
	}
	if got.UserData.PlayedPercentage != 85.5 {
		t.Errorf("UserData.PlayedPercentage: got %f, want 85.5", got.UserData.PlayedPercentage)
	}
}

func TestToDetailedItemOutput_EmptyMediaSources(t *testing.T) {
	m := map[string]any{
		"id":            "no-media",
		"name":          "NoMedia",
		"type":          "Movie",
		"media_sources": []map[string]any{},
	}

	got := ToDetailedItemOutput(m)

	if got.MediaSources != nil {
		t.Errorf("expected nil MediaSources for empty slice, got %v", got.MediaSources)
	}
}

func TestToDetailedItemOutput_NoAudioOrSubtitleStreams(t *testing.T) {
	m := map[string]any{
		"id":   "no-streams",
		"name": "NoStreams",
		"type": "Movie",
		"media_sources": []map[string]any{
			{
				"container":   "avi",
				"video_codec": "mpeg4",
			},
		},
	}

	got := ToDetailedItemOutput(m)

	if len(got.MediaSources) != 1 {
		t.Fatalf("expected 1 media source, got %d", len(got.MediaSources))
	}
	src := got.MediaSources[0]
	if src.AudioStreams != nil {
		t.Errorf("expected nil AudioStreams, got %v", src.AudioStreams)
	}
	if src.SubtitleStreams != nil {
		t.Errorf("expected nil SubtitleStreams, got %v", src.SubtitleStreams)
	}
}

// --- ToStringSlice (sanity check; extensive tests are in helpers_test.go) ---

func TestToStringSlice_Sanity(t *testing.T) {
	// []string pass-through
	got := ToStringSlice([]string{"a", "b"})
	if !reflect.DeepEqual(got, []string{"a", "b"}) {
		t.Errorf("[]string: got %v", got)
	}

	// []any with strings
	got = ToStringSlice([]any{"x", "y", "z"})
	if !reflect.DeepEqual(got, []string{"x", "y", "z"}) {
		t.Errorf("[]any: got %v", got)
	}

	// nil
	got = ToStringSlice(nil)
	if got != nil {
		t.Errorf("nil: expected nil, got %v", got)
	}

	// Unsupported type
	got = ToStringSlice(42)
	if got != nil {
		t.Errorf("int: expected nil, got %v", got)
	}

	// []any with no strings yields nil
	got = ToStringSlice([]any{1, 2, 3})
	if got != nil {
		t.Errorf("[]any no strings: expected nil, got %v", got)
	}
}
