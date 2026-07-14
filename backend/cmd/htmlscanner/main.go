package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

// Target contains details of a parsed magnet link.
type Target struct {
	Title string
	Size  string
	Link  string
}

func main() {
	urlFlag := flag.String("url", "", "The HTML page URL to scan for magnet links")
	flag.Parse()

	if *urlFlag == "" {
		// Default to the provided MovieRulz link as example if none is specified
		*urlFlag = "https://www.5movierulz.school/aashaan-2026-hdrip-malayalam-7049.html"
		fmt.Printf("No URL specified. Defaulting to example: %s\n\n", *urlFlag)
	}

	fmt.Printf("Fetching HTML page from: %s ...\n", *urlFlag)
	resp, err := http.Get(*urlFlag)
	if err != nil {
		log.Fatalf("Failed to fetch URL: %v", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatalf("Failed to read response body: %v", err)
	}
	htmlContent := string(bodyBytes)

	// Regex to match anchor tags containing magnet links
	// Captures the whole tag to allow parsing inner elements/text/attributes
	anchorRegex := regexp.MustCompile(`(?i)<a\s+[^>]*href=["'](magnet:\?[^"']+)["'][^>]*>([\s\S]*?)<\/a>`)
	matches := anchorRegex.FindAllStringSubmatch(htmlContent, -1)

	if len(matches) == 0 {
		fmt.Println("No magnet links found in the page.")
		return
	}

	fmt.Printf("Found %d magnet links on the page:\n\n", len(matches))

	for i, match := range matches {
		magnetURL := htmlContentEscaped(match[1])
		innerContent := match[2]

		// 1. Try to find the title inside the dn parameter of the magnet link itself
		resolvedTitle := ""
		if parsedMagnet, err := url.Parse(magnetURL); err == nil {
			dn := parsedMagnet.Query().Get("dn")
			if dn != "" {
				resolvedTitle = dn
			}
		}

		// 2. Try to find size if present inside inner text (e.g. from MovieRulz size badge)
		size := "Unknown Size"
		sizeRegex := regexp.MustCompile(`(?i)class=["']btn-size["'][^>]*>([^<]+)`)
		if sizeMatch := sizeRegex.FindStringSubmatch(innerContent); len(sizeMatch) > 1 {
			size = strings.TrimSpace(sizeMatch[1])
		}

		// 3. If dn title is empty, clean up the inner HTML content to use as fallback title
		if resolvedTitle == "" {
			cleanText := cleanHTML(innerContent)
			if cleanText != "" {
				resolvedTitle = cleanText
			} else {
				resolvedTitle = fmt.Sprintf("MagnetLink_%d", i+1)
			}
		}

		fmt.Printf("[%d] Title: %s\n", i+1, resolvedTitle)
		fmt.Printf("    Size:  %s\n", size)
		fmt.Printf("    Link:  %s\n\n", magnetURL)
	}
}

// cleanHTML strips HTML tags and normalizes whitespace.
func cleanHTML(html string) string {
	tagRegex := regexp.MustCompile(`<[^>]*>`)
	cleaned := tagRegex.ReplaceAllString(html, " ")
	return strings.TrimSpace(strings.Join(strings.Fields(cleaned), " "))
}

// htmlContentEscaped decodes basic XML/HTML character entities.
func htmlContentEscaped(val string) string {
	val = strings.ReplaceAll(val, "&amp;", "&")
	return val
}
