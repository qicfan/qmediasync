package helpers

import (
	"path/filepath"
	"testing"
)

func TestExtractID(t *testing.T) {
	tests := []struct {
		name          string
		filename      string
		expectTmdbID  int64
		expectDouban  int64
		expectType    string
		expectSeason  int
		expectEpisode int
	}{
		{
			name:          "TMDB ID with bracket",
			filename:      "Movie Name {tmdbid-12345}.mkv",
			expectTmdbID:  12345,
			expectDouban:  0,
			expectType:    "",
			expectSeason:  0,
			expectEpisode: 0,
		},
		{
			name:          "TMDB ID with equals",
			filename:      "Movie Name [tmdbid=67890].mp4",
			expectTmdbID:  67890,
			expectDouban:  0,
			expectType:    "",
			expectSeason:  0,
			expectEpisode: 0,
		},
		{
			name:          "Douban ID",
			filename:      "Movie Name {doubanid=11111}.mkv",
			expectTmdbID:  0,
			expectDouban:  11111,
			expectType:    "",
			expectSeason:  0,
			expectEpisode: 0,
		},
		{
			name:          "Full format with season and episode",
			filename:      "TV Show {tmdbid=99999;type=tv;s=1;e=5}.mkv",
			expectTmdbID:  99999,
			expectDouban:  0,
			expectType:    "tv",
			expectSeason:  1,
			expectEpisode: 5,
		},
		{
			name:          "No ID",
			filename:      "Regular Movie Name (2023).mkv",
			expectTmdbID:  0,
			expectDouban:  0,
			expectType:    "",
			expectSeason:  0,
			expectEpisode: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmdbID, doubanID, mediaType, season, episode := ExtractID(tt.filename)
			if tmdbID != tt.expectTmdbID {
				t.Errorf("ExtractID() tmdbID = %v, want %v", tmdbID, tt.expectTmdbID)
			}
			if doubanID != tt.expectDouban {
				t.Errorf("ExtractID() doubanID = %v, want %v", doubanID, tt.expectDouban)
			}
			if mediaType != tt.expectType {
				t.Errorf("ExtractID() mediaType = %v, want %v", mediaType, tt.expectType)
			}
			if season != tt.expectSeason {
				t.Errorf("ExtractID() season = %v, want %v", season, tt.expectSeason)
			}
			if episode != tt.expectEpisode {
				t.Errorf("ExtractID() episode = %v, want %v", episode, tt.expectEpisode)
			}
		})
	}
}

func TestExtractMetadataFromPath(t *testing.T) {
	videoExt := []string{".mkv", ".mp4", ".avi"}
	excludePatterns := []string{}

	tests := []struct {
		name           string
		filePath       string
		expectName     string
		expectYear     int
		expectSeason   int
		expectEpisode  int
	}{
		{
			name:          "Movie with year in filename",
			filePath:      "/movies/Inception (2010).mkv",
			expectName:    "inception",
			expectYear:    2010,
			expectSeason:  -1,
			expectEpisode: -1,
		},
		{
			name:          "TV show with season in parent dir",
			filePath:      "/tvshows/Breaking Bad/Season 1/Breaking Bad S01E01.mkv",
			expectName:    "breaking bad",
			expectYear:    0,
			expectSeason:  1,
			expectEpisode: 1,
		},
		{
			name:          "Movie with year in parent dir",
			filePath:      "/movies/The Matrix (1999)/The Matrix.mkv",
			expectName:    "the matrix",
			expectYear:    1999,
			expectSeason:  -1,
			expectEpisode: -1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			meta := ExtractMetadataFromPath(tt.filePath, false, videoExt, excludePatterns...)
			if meta.Name != tt.expectName {
				t.Errorf("ExtractMetadataFromPath() name = %v, want %v", meta.Name, tt.expectName)
			}
			if meta.Year != tt.expectYear {
				t.Errorf("ExtractMetadataFromPath() year = %v, want %v", meta.Year, tt.expectYear)
			}
			if meta.Season != tt.expectSeason {
				t.Errorf("ExtractMetadataFromPath() season = %v, want %v", meta.Season, tt.expectSeason)
			}
			if meta.Episode != tt.expectEpisode {
				t.Errorf("ExtractMetadataFromPath() episode = %v, want %v", meta.Episode, tt.expectEpisode)
			}
		})
	}
}

func TestAutoDetectMediaType(t *testing.T) {
	tests := []struct {
		name         string
		filePath     string
		meta         *MediaInfo
		expectResult string
	}{
		{
			name:         "TV show with season in path",
			filePath:     "/tvshows/Breaking Bad/Season 1/episode.mkv",
			meta:         &MediaInfo{Season: -1, Episode: -1},
			expectResult: MediaTypeTV,
		},
		{
			name:         "TV show with episode info",
			filePath:     "/movies/episode.mkv",
			meta:         &MediaInfo{Season: 1, Episode: 5},
			expectResult: MediaTypeTV,
		},
		{
			name:         "Movie without season/episode",
			filePath:     "/movies/Inception.mkv",
			meta:         &MediaInfo{Season: -1, Episode: -1},
			expectResult: MediaTypeMovie,
		},
		{
			name:         "TV show with S0X pattern",
			filePath:     "/tvshows/show/S01/episode.mkv",
			meta:         &MediaInfo{Season: -1, Episode: -1},
			expectResult: MediaTypeTV,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := AutoDetectMediaType(tt.filePath, tt.meta)
			if result != tt.expectResult {
				t.Errorf("AutoDetectMediaType() = %v, want %v", result, tt.expectResult)
			}
		})
	}
}

func TestFormatMoviePath(t *testing.T) {
	tests := []struct {
		name         string
		title        string
		year         int
		originalPath string
		expectResult string
	}{
		{
			name:         "Movie with year",
			title:        "Inception",
			year:         2010,
			originalPath: "/path/to/movie.mkv",
			expectResult: filepath.Join("Inception (2010)", "Inception (2010).mkv"),
		},
		{
			name:         "Movie without year",
			title:        "The Matrix",
			year:         0,
			originalPath: "/path/to/movie.mp4",
			expectResult: filepath.Join("The Matrix", "The Matrix.mp4"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatMoviePath(tt.title, tt.year, tt.originalPath)
			if result != tt.expectResult {
				t.Errorf("FormatMoviePath() = %v, want %v", result, tt.expectResult)
			}
		})
	}
}

func TestFormatTVPath(t *testing.T) {
	tests := []struct {
		name         string
		title        string
		season       int
		episode      int
		episodeTitle string
		originalPath string
		expectResult string
	}{
		{
			name:         "TV episode with title",
			title:        "Breaking Bad",
			season:       1,
			episode:      1,
			episodeTitle: "Pilot",
			originalPath: "/path/to/episode.mkv",
			expectResult: filepath.Join("Breaking Bad", "Season 01", "Breaking Bad - S01E01 - Pilot.mkv"),
		},
		{
			name:         "TV episode without title",
			title:        "Game of Thrones",
			season:       2,
			episode:      5,
			episodeTitle: "",
			originalPath: "/path/to/episode.mp4",
			expectResult: filepath.Join("Game of Thrones", "Season 02", "Game of Thrones - S02E05.mp4"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatTVPath(tt.title, tt.season, tt.episode, tt.episodeTitle, tt.originalPath)
			if result != tt.expectResult {
				t.Errorf("FormatTVPath() = %v, want %v", result, tt.expectResult)
			}
		})
	}
}
