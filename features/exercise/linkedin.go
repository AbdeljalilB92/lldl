package exercise

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"net/url"
	"regexp"

	shareddomain "github.com/AbdeljalilB92/lldl/shared/domain"
	sharedhttp "github.com/AbdeljalilB92/lldl/shared/http"
	"github.com/AbdeljalilB92/lldl/shared/logging"
)

// Compile-time check: linkedinResolver satisfies the Resolver interface.
var _ Resolver = (*linkedinResolver)(nil)

// regexBPRCodeBlock matches <code> tags with bpr-guid IDs and captures their
// inner HTML content, which contains JSON-encoded BPR data.
var regexBPRCodeBlock = regexp.MustCompile(`(?s)<code[^>]*\bid="bpr-guid-[^"]*"[^>]*>(.*?)</code>`)

// defaultMaxRetries is the number of retry attempts for exercise file resolution.
const defaultMaxRetries = 3

// linkedinResolver resolves exercise file URLs by scraping the course page HTML
// for embedded BPR JSON data containing ambry CDN links.
type linkedinResolver struct {
	client         sharedhttp.AuthenticatedClient
	enterpriseHash string
}

// NewLinkedInResolver creates a Resolver that scrapes course pages for exercise
// file URLs. The enterpriseHash is appended as a query parameter when non-empty.
func NewLinkedInResolver(client sharedhttp.AuthenticatedClient, enterpriseHash string) Resolver {
	return &linkedinResolver{
		client:         client,
		enterpriseHash: enterpriseHash,
	}
}

// Resolve fetches the course page HTML with retry, extracts BPR code blocks,
// parses the embedded JSON, and returns all exercise file entries with ambry
// download URLs. Returns an empty Result (no error) when the page contains no
// exercise files.
func (r *linkedinResolver) Resolve(ctx context.Context, courseSlug string, firstVideoSlug string) (*Result, error) {
	logger := logging.New("[Exercise][Resolve]")
	logger.Info("resolving exercise files", "course", courseSlug, "video", firstVideoSlug)

	pageURL := buildCoursePageURL(courseSlug, firstVideoSlug, r.enterpriseHash)
	logger.Debug("fetching course page", "url", pageURL)

	bodyBytes, err := r.client.GetWithRetry(ctx, pageURL, defaultMaxRetries)
	if err != nil {
		return nil, fmt.Errorf("exercise resolve failed for %s: %w", pageURL, err)
	}
	body := string(bodyBytes)

	files, err := extractExerciseFilesFromHTML(body)
	if err != nil {
		return nil, err
	}

	logger.Info("exercise files resolved", "count", len(files))
	return &Result{Files: files}, nil
}

// extractExerciseFilesFromHTML parses BPR code blocks from the page HTML and
// collects all exercise file entries that have a non-empty ambry URL.
func extractExerciseFilesFromHTML(body string) ([]shareddomain.ExerciseFile, error) {
	matches := regexBPRCodeBlock.FindAllStringSubmatch(body, -1)

	var files []shareddomain.ExerciseFile
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}

		// HTML entities in the BPR JSON must be unescaped before parsing.
		decoded := html.UnescapeString(match[1])

		var bpr bprData
		if err := json.Unmarshal([]byte(decoded), &bpr); err != nil {
			// Malformed JSON in one BPR block is not fatal; skip it.
			// The page may contain multiple BPR blocks for different content types.
			continue
		}

		for _, item := range bpr.Included {
			// Only Course-type items contain exerciseFiles.
			if item.DollarType != "com.linkedin.learning.api.deco.content.Course" {
				continue
			}
			for _, ef := range item.ExerciseFiles {
				if ef.URL == "" {
					continue
				}
				files = append(files, shareddomain.ExerciseFile{
					FileName:    ef.Name,
					DownloadURL: ef.URL,
					FileSize:    ef.SizeInBytes,
				})
			}
		}
	}

	return files, nil
}

// buildCoursePageURL constructs the course page URL using url.URL construction.
// Enterprise hash is appended as a "u" query parameter when non-empty.
func buildCoursePageURL(courseSlug, firstVideoSlug, enterpriseHash string) string {
	u := &url.URL{
		Scheme: "https",
		Host:   "www.linkedin.com",
		Path:   "/learning/" + courseSlug + "/" + firstVideoSlug,
	}
	if enterpriseHash != "" {
		q := u.Query()
		q.Set("u", enterpriseHash)
		u.RawQuery = q.Encode()
	}
	return u.String()
}
