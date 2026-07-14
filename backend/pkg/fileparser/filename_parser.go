package fileparser

import (
	"regexp"
	"strconv"
	"strings"
)

// ParsedMetadata holds fields extracted from video file names.
type ParsedMetadata struct {
	Title    string
	Year     int
	Quality  string
	Language string
}

// Regex rules for parsing
var (
	// Matches standard years e.g. 1999 or 2024
	yearRegex = regexp.MustCompile(`\b((?:19|20)\d{2})\b`)

	// Matches typical movie quality tags
	qualityRegex = regexp.MustCompile(`(?i)\b(2160p|1080p|720p|480p|360p|2k|4k|uhd)\b`)

	// Matches HDRip / quality-type tags (useful for stripping, not for quality field)
	qualityTypeRegex = regexp.MustCompile(`(?i)\b(HQ\s*HDRip|HDRip|BDRip|CAMRip|DVDScr|HDTS|PreDVDRip|HDCam)\b`)

	// Matches known languages in filenames
	languageRegex = regexp.MustCompile(`(?i)\b(malayalam|hindi|english|telugu|tamil|kannada|bengali|marathi|punjabi|gujarati|urdu|korean|japanese|chinese|spanish|french|german|italian|portuguese|russian|arabic|thai|indonesian|vietnamese|turkish|dutch|polish|swedish|norwegian|danish|finnish|czech|hungarian|romanian|greek|hebrew|persian)\b`)

	// URL-style prefix patterns (e.g., www.5MovieRulz.software, www.TamilRockers.com)
	urlPrefixRegex = regexp.MustCompile(`(?i)^(www\.\S+?\s+[\-–—]\s*|(?:https?://)\S+\s*[\-–—]?\s*)`)

	// Extra tags to strip from titles to make them clean
	junkRegex = regexp.MustCompile(`(?i)\b(bluray|web-dl|webrip|hdtv|dvdrip|brrip|x264|x265|hevc|h264|h265|aac|dts|dd5\.1|dd\+5\.1|ac3|atmos|multi|dual-audio|esub|e-sub|subtitle|subtitles|org|clean|web|rip|uncut|extended|director's\.cut|remastered|hdr|hdr10|hdr10\+|10bit|8bit|6ch|5\.1|2\.0|nf|amzn)\b`)

	// Size indicators like "3.5GB", "700MB", "400MB"
	sizeRegex = regexp.MustCompile(`(?i)\b\d+(\.\d+)?\s*(gb|mb|tb)\b`)

	// Codec/bitrate patterns like "768Kbps"
	bitrateRegex = regexp.MustCompile(`(?i)\b\d+\s*kbps\b`)

	// Extension matching
	extRegex = regexp.MustCompile(`\.[a-zA-Z0-9]+$`)
)

// ParseFilename processes a raw filename and extracts Title, Year, Quality, and Language.
func ParseFilename(filename string) ParsedMetadata {
	// Strip extension
	name := extRegex.ReplaceAllString(filename, "")

	// Strip URL-style prefix (e.g. "www.5MovieRulz.software - ")
	name = urlPrefixRegex.ReplaceAllString(name, "")

	// Extract Quality
	var quality string
	if match := qualityRegex.FindString(name); match != "" {
		quality = strings.ToLower(match)
	}

	// Extract Year
	var year int
	if match := yearRegex.FindString(name); match != "" {
		if val, err := strconv.Atoi(match); err == nil {
			year = val
		}
	}

	// Extract Language
	var language string
	if match := languageRegex.FindString(name); match != "" {
		language = strings.ToLower(match)
	}

	// Parse Title out: Clean up the name string by removing junk tags, year, quality.
	// We locate the index of the year or quality to split the title.
	// Often titles end before the year/quality.
	splitIndex := len(name)

	if loc := yearRegex.FindStringIndex(name); loc != nil && loc[0] < splitIndex {
		splitIndex = loc[0]
	}
	if loc := qualityRegex.FindStringIndex(name); loc != nil && loc[0] < splitIndex {
		splitIndex = loc[0]
	}
	if loc := qualityTypeRegex.FindStringIndex(name); loc != nil && loc[0] < splitIndex {
		splitIndex = loc[0]
	}
	if loc := languageRegex.FindStringIndex(name); loc != nil && loc[0] < splitIndex {
		splitIndex = loc[0]
	}

	titlePart := name[:splitIndex]

	// Clean separators
	titlePart = strings.ReplaceAll(titlePart, ".", " ")
	titlePart = strings.ReplaceAll(titlePart, "_", " ")
	titlePart = strings.ReplaceAll(titlePart, "-", " ")

	// Strip remaining junk words if any got left behind
	titlePart = junkRegex.ReplaceAllString(titlePart, "")

	// Strip size indicators
	titlePart = sizeRegex.ReplaceAllString(titlePart, "")
	titlePart = bitrateRegex.ReplaceAllString(titlePart, "")

	// Final cleanup
	title := strings.TrimSpace(titlePart)
	// Remove brackets/parentheses and their content if now empty
	title = regexp.MustCompile(`\(\s*\)`).ReplaceAllString(title, "")
	title = regexp.MustCompile(`\[\s*\]`).ReplaceAllString(title, "")
	title = regexp.MustCompile(`\s+`).ReplaceAllString(title, " ")
	title = strings.Trim(title, " ()[]{}-_.,&")

	if title == "" {
		title = strings.ReplaceAll(name, ".", " ")
	}

	return ParsedMetadata{
		Title:    title,
		Year:     year,
		Quality:  quality,
		Language: language,
	}
}
