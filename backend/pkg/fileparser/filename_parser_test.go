package fileparser

import (
	"testing"
)

func TestParseFilename(t *testing.T) {
	tests := []struct {
		filename string
		wantTitle string
		wantYear  int
		wantQual  string
		wantLang  string
	}{
		{
			filename:  "Inception.2010.1080p.BluRay.x264.mp4",
			wantTitle: "Inception",
			wantYear:  2010,
			wantQual:  "1080p",
			wantLang:  "",
		},
		{
			filename:  "3 Idiots (2009) Hindi 720p.mkv",
			wantTitle: "3 Idiots",
			wantYear:  2009,
			wantQual:  "720p",
			wantLang:  "hindi",
		},
		{
			filename:  "The.Dark.Knight.2008.UHD.x265.mp4",
			wantTitle: "The Dark Knight",
			wantYear:  2008,
			wantQual:  "uhd",
			wantLang:  "",
		},
		{
			filename:  "Dangal.1080p.BluRay.mkv",
			wantTitle: "Dangal",
			wantYear:  0,
			wantQual:  "1080p",
			wantLang:  "",
		},
		{
			filename:  "random_movie_file.avi",
			wantTitle: "random movie file",
			wantYear:  0,
			wantQual:  "",
			wantLang:  "",
		},
		{
			filename:  "Sholay 1975 Remastered.mp4",
			wantTitle: "Sholay",
			wantYear:  1975,
			wantQual:  "",
			wantLang:  "",
		},
		{
			filename:  "www.5MovieRulz.software - Dridam (2026) Malayalam HQ HDRip - 1080p - x264 - (DD+5.1 - ATMOS - 768Kbps & AAC 2.0) - 3.5GB - ESub.mkv",
			wantTitle: "Dridam",
			wantYear:  2026,
			wantQual:  "1080p",
			wantLang:  "malayalam",
		},
		{
			filename:  "www.5MovieRulz.software - Dridam (2026) Malayalam HQ HDRip - x264 - AAC - 400MB - ESub.mkv",
			wantTitle: "Dridam",
			wantYear:  2026,
			wantQual:  "",
			wantLang:  "malayalam",
		},
		{
			filename:  "Athu_Thalore_Video_Song_Suriyas_Karuppu_RJ_Balaji_SaiAbhyankkar_Dream_Warrior_Pictures.mp4",
			wantTitle: "Athu Thalore Video Song Suriyas Karuppu RJ Balaji SaiAbhyankkar Dream Warrior Pictures",
			wantYear:  0,
			wantQual:  "",
			wantLang:  "",
		},
	}

	for _, tt := range tests {
		got := ParseFilename(tt.filename)
		if got.Title != tt.wantTitle {
			t.Errorf("ParseFilename(%q) Title = %q, want %q", tt.filename, got.Title, tt.wantTitle)
		}
		if got.Year != tt.wantYear {
			t.Errorf("ParseFilename(%q) Year = %d, want %d", tt.filename, got.Year, tt.wantYear)
		}
		if got.Quality != tt.wantQual {
			t.Errorf("ParseFilename(%q) Quality = %q, want %q", tt.filename, got.Quality, tt.wantQual)
		}
		if got.Language != tt.wantLang {
			t.Errorf("ParseFilename(%q) Language = %q, want %q", tt.filename, got.Language, tt.wantLang)
		}
	}
}
