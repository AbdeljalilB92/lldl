package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/abdeljalil/linkedin-learning-downloader/internal/model"
)

const (
	userAgent  = "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:88.0) Gecko/20100101 Firefox/88.0"
	maxRetries = 5
)

var (
	// regexCourseURL matches a LinkedIn Learning course URL and extracts the slug.
	regexCourseURL = regexp.MustCompile(`https?://(?:www\.)?linkedin\.com/learning/([a-zA-Z0-9-]+)`)
	// regexTrialLink detects an invalid / unauthenticated session.
	regexTrialLink = regexp.MustCompile(`nav__button-tertiary.*\n?.\r?.*Start free trial`)
	// regexEnterpriseProfileHash extracts the enterprise hash from the page body.
	regexEnterpriseProfileHash = regexp.MustCompile(`enterpriseProfileHash":"(.*?)"`)
)

// Extractor authenticates with LinkedIn Learning and fetches course data + video download URLs.
type Extractor struct {
	courseSlug     string
	quality        model.Quality
	delay          int // seconds between API calls
	client         *http.Client
	csrfToken      string
	enterpriseHash string
}

// Client returns the authenticated HTTP client used by the extractor.
// This can be used by the downloader to fetch resources that require authentication
// (e.g., exercise files that need to go through LinkedIn's ambry CDN).
func (e *Extractor) Client() *http.Client {
	return e.client
}

// CSRFToken returns the CSRF token used for authenticated requests.
func (e *Extractor) CSRFToken() string {
	return e.csrfToken
}

// EnterpriseHash returns the enterprise profile hash if available.
func (e *Extractor) EnterpriseHash() string {
	return e.enterpriseHash
}

// NewExtractor creates a new Extractor. The token is the li_at cookie value.
func NewExtractor(courseURL, token string, quality model.Quality, delay int) (*Extractor, error) {
	// Normalize URL
	urlStr := courseURL
	if !strings.HasPrefix(urlStr, "https://") && !strings.HasPrefix(urlStr, "http://") {
		urlStr = "https://" + urlStr
	}

	matches := regexCourseURL.FindStringSubmatch(urlStr)
	if matches == nil {
		return nil, fmt.Errorf("invalid course URL: %s", courseURL)
	}
	slug := matches[1]

	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, fmt.Errorf("creating cookie jar: %w", err)
	}

	// Add li_at cookie
	liAtCookie := &http.Cookie{
		Name:     "li_at",
		Value:    token,
		Path:     "/",
		Domain:   ".www.linkedin.com",
		Secure:   true,
		HttpOnly: true,
	}
	u := "https://www.linkedin.com"
	jar.SetCookies(mustParseURL(u), []*http.Cookie{liAtCookie})

	client := &http.Client{
		Jar:     jar,
		Timeout: 60 * time.Second,
	}

	return &Extractor{
		courseSlug: slug,
		quality:    quality,
		delay:      delay,
		client:     client,
	}, nil
}

// ValidateToken checks that the li_at token is valid by fetching the LinkedIn Learning homepage.
// It extracts the CSRF token (JSESSIONID cookie) and optionally the enterprise profile hash.
func (e *Extractor) ValidateToken(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://www.linkedin.com/learning", nil)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := e.client.Do(req)
	if err != nil {
		return fmt.Errorf("fetching learning homepage: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading response body: %w", err)
	}
	body := html.UnescapeString(string(bodyBytes))

	// Check for "Start free trial" which indicates an invalid/expired token
	if regexTrialLink.MatchString(body) {
		return errors.New("invalid token: \"Start free trial\" detected, the li_at token is expired or invalid")
	}

	// Extract JSESSIONID from cookies for CSRF
	learningURL := mustParseURL("https://www.linkedin.com/learning")
	cookies := e.client.Jar.Cookies(learningURL)
	var jsessionID string
	for _, c := range cookies {
		if c.Name == "JSESSIONID" {
			jsessionID = c.Value
			break
		}
	}
	if jsessionID == "" {
		return errors.New("JSESSIONID cookie not found in response; token may be invalid")
	}
	e.csrfToken = jsessionID

	// Extract enterprise profile hash if present
	if m := regexEnterpriseProfileHash.FindStringSubmatch(body); len(m) > 1 {
		e.enterpriseHash = m[1]
	}

	return nil
}

// GetCourse fetches the full course including all chapters and video download URLs.
func (e *Extractor) GetCourse(ctx context.Context) (*model.Course, error) {
	// Fetch course structure
	apiURL := fmt.Sprintf(
		"https://www.linkedin.com/learning-api/detailedCourses?courseSlug=%s&fields=chapters,title,exerciseFiles&addParagraphsToTranscript=true&q=slugs",
		e.courseSlug,
	)

	body, err := e.doGetWithRetry(ctx, apiURL)
	if err != nil {
		return nil, fmt.Errorf("fetching course: %w", err)
	}

	if strings.Contains(string(body), "CSRF check failed") {
		return nil, errors.New("token is expired (CSRF check failed); please use a new li_at token")
	}

	// Parse into generic map
	var raw map[string]interface{}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("parsing course JSON: %w", err)
	}

	elementsVal, ok := extractPath(raw, "elements")
	if !ok {
		return nil, errors.New("course response missing 'elements' array")
	}
	elements, ok := elementsVal.([]interface{})
	if !ok || len(elements) == 0 {
		return nil, errors.New("course response 'elements' is empty or not an array")
	}
	element0, ok := elements[0].(map[string]interface{})
	if !ok {
		return nil, errors.New("course response elements[0] is not an object")
	}

	// Title
	title := getString(element0, "title")

	// Chapters
	chapters := parseChapters(element0)

	// Exercise files
	var exerciseFiles []model.ExerciseFile
	if efVal, ok := element0["exerciseFiles"]; ok {
		if efArr, ok := efVal.([]interface{}); ok {
			for _, ef := range efArr {
				if efMap, ok := ef.(map[string]interface{}); ok {
					exerciseFiles = append(exerciseFiles, model.ExerciseFile{
						FileName:    getString(efMap, "name"),
						DownloadURL: getString(efMap, "url"),
					})
				}
			}
		}
	}

	if len(exerciseFiles) > 0 {
		slog.Info("Found exercise files", "count", len(exerciseFiles))
	}

	// Now fetch video details for each video in each chapter
	totalVideos := 0
	for _, ch := range chapters {
		totalVideos += len(ch.Videos)
	}

	videoIndex := 0
	for ci := range chapters {
		for vi := range chapters[ci].Videos {
			videoIndex++
			slug := chapters[ci].Videos[vi].Slug
			slog.Info("fetching video details", "video", slug, "progress", fmt.Sprintf("%d/%d", videoIndex, totalVideos))

			details, err := e.fetchVideoDetails(ctx, slug)
			if err != nil {
				return nil, fmt.Errorf("fetching video %q: %w", slug, err)
			}
			// Preserve the original slug from the course structure
			details.Slug = slug
			if details.DownloadURL == "" {
				return nil, fmt.Errorf("failed to extract download URL for video %q; the token may be invalid", slug)
			}
			chapters[ci].Videos[vi] = *details

			// Delay between API calls (not after the last video)
			if videoIndex < totalVideos && e.delay > 0 {
				select {
				case <-time.After(time.Duration(e.delay) * time.Second):
				case <-ctx.Done():
					return nil, ctx.Err()
				}
			}
		}
	}

	return &model.Course{
		Title:         title,
		Slug:          e.courseSlug,
		Chapters:      chapters,
		ExerciseFiles: exerciseFiles,
	}, nil
}

// fetchVideoDetails fetches the download URL and transcript for a single video.
func (e *Extractor) fetchVideoDetails(ctx context.Context, videoSlug string) (*model.Video, error) {
	apiURL := fmt.Sprintf(
		"https://www.linkedin.com/learning-api/detailedCourses?courseSlug=%s&resolution=_%s&q=slugs&fields=selectedVideo&videoSlug=%s",
		e.courseSlug,
		e.quality.String(),
		videoSlug,
	)

	body, err := e.doGetWithRetry(ctx, apiURL)
	if err != nil {
		return nil, fmt.Errorf("fetching video details: %w", err)
	}

	if strings.Contains(string(body), "CSRF check failed") {
		return nil, errors.New("token is expired (CSRF check failed)")
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("parsing video JSON: %w", err)
	}

	// Navigate to elements[0].selectedVideo
	selectedVideoVal, ok := extractPath(raw, "elements[0].selectedVideo")
	if !ok {
		return nil, fmt.Errorf("video response missing elements[0].selectedVideo")
	}
	sv, ok := selectedVideoVal.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("selectedVideo is not an object")
	}

	video := &model.Video{
		Title:    getString(sv, "title"),
		Duration: getInt(sv, "durationInSeconds"),
	}

	// Download URL: selectedVideo.url.progressiveUrl
	if urlObj, ok := sv["url"].(map[string]interface{}); ok {
		video.DownloadURL = getString(urlObj, "progressiveUrl")
	}

	// Transcript lines: selectedVideo.transcript.lines
	if transcriptObj, ok := sv["transcript"].(map[string]interface{}); ok {
		if linesArr, ok := transcriptObj["lines"].([]interface{}); ok {
			for _, line := range linesArr {
				if lineMap, ok := line.(map[string]interface{}); ok {
					video.TranscriptLines = append(video.TranscriptLines, model.TranscriptLine{
						Caption:  getString(lineMap, "caption"),
						StartsAt: getInt64(lineMap, "transcriptStartAt"),
					})
				}
			}
		}
	}

	// Build SRT transcript
	video.FormTranscript()

	return video, nil
}

// doGetWithRetry performs a GET request with retry logic (up to maxRetries attempts).
// It does not retry on DNS errors or HTTP 404 responses.
func (e *Extractor) doGetWithRetry(ctx context.Context, reqURL string) ([]byte, error) {
	var lastErr error
	for attempt := 1; attempt <= maxRetries; attempt++ {
		if attempt > 1 {
			slog.Error("retrying request", "attempt", attempt, "url", reqURL)
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
		if err != nil {
			return nil, fmt.Errorf("creating request: %w", err)
		}
		req.Header.Set("User-Agent", userAgent)
		if e.csrfToken != "" {
			req.Header.Set("Csrf-Token", e.csrfToken)
		}
		if e.enterpriseHash != "" {
			req.Header.Set("x-li-identity", e.enterpriseHash)
		}

		resp, err := e.client.Do(req)
		if err != nil {
			// Don't retry DNS lookup errors
			if isDNSError(err) {
				return nil, fmt.Errorf("DNS error (not retrying): %w", err)
			}
			lastErr = err
			slog.Error("request failed", "attempt", attempt, "error", err)
			continue
		}

		body, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			lastErr = readErr
			slog.Error("reading response body failed", "attempt", attempt, "error", readErr)
			continue
		}

		// Don't retry 404
		if resp.StatusCode == http.StatusNotFound {
			return nil, fmt.Errorf("HTTP 404 Not Found for %s", reqURL)
		}

		if resp.StatusCode >= 400 {
			lastErr = fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
			slog.Error("HTTP error", "attempt", attempt, "status", resp.StatusCode, "url", reqURL)
			continue
		}

		return body, nil
	}

	return nil, fmt.Errorf("failed after %d attempts: %w", maxRetries, lastErr)
}

// ---------------------------------------------------------------------------
// Deep-path JSON helper (replaces C# JToken.SelectToken)
// ---------------------------------------------------------------------------

// extractPath navigates a nested map[string]interface{} using a dot-separated path
// with optional array indexing, e.g. "elements[0].selectedVideo.title".
func extractPath(m map[string]interface{}, path string) (interface{}, bool) {
	parts := splitPath(path)
	var current interface{} = m

	for _, part := range parts {
		// Check for array indexing: "elements[0]"
		arrayName, index := parseArrayIndex(part)
		if arrayName != "" {
			// First get the array by name
			obj, ok := current.(map[string]interface{})
			if !ok {
				return nil, false
			}
			arr, ok := obj[arrayName]
			if !ok {
				return nil, false
			}
			arrTyped, ok := arr.([]interface{})
			if !ok || index < 0 || index >= len(arrTyped) {
				return nil, false
			}
			current = arrTyped[index]
			continue
		}

		// Regular key
		obj, ok := current.(map[string]interface{})
		if !ok {
			return nil, false
		}
		val, ok := obj[part]
		if !ok {
			return nil, false
		}
		current = val
	}

	return current, true
}

// splitPath splits a dot-separated path, respecting that parts can contain "[N]".
func splitPath(path string) []string {
	return strings.Split(path, ".")
}

// parseArrayIndex parses "name[N]" into (name, N). Returns ("", -1) if not an array index.
func parseArrayIndex(part string) (string, int) {
	idx := strings.Index(part, "[")
	if idx < 0 {
		return "", -1
	}
	name := part[:idx]
	closeIdx := strings.Index(part, "]")
	if closeIdx < 0 {
		return "", -1
	}
	indexStr := part[idx+1 : closeIdx]
	n, err := strconv.Atoi(indexStr)
	if err != nil {
		return "", -1
	}
	return name, n
}

// ---------------------------------------------------------------------------
// Utility helpers
// ---------------------------------------------------------------------------

func getString(m map[string]interface{}, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func getInt(m map[string]interface{}, key string) int {
	if v, ok := m[key]; ok {
		switch n := v.(type) {
		case float64:
			return int(n)
		case int:
			return n
		case json.Number:
			if i, err := n.Int64(); err == nil {
				return int(i)
			}
		case string:
			if i, err := strconv.Atoi(n); err == nil {
				return i
			}
		}
	}
	return 0
}

func getInt64(m map[string]interface{}, key string) int64 {
	if v, ok := m[key]; ok {
		switch n := v.(type) {
		case float64:
			return int64(n)
		case int64:
			return n
		case json.Number:
			if i, err := n.Int64(); err == nil {
				return i
			}
		case string:
			if i, err := strconv.ParseInt(n, 10, 64); err == nil {
				return i
			}
		}
	}
	return 0
}

// GetExerciseFileURLs fetches the course page HTML and extracts pre-signed ambry exercise file URLs
// from the embedded BPR JSON data. The API returns dead akamaihd.net URLs; this method provides
// working alternatives.
func (e *Extractor) GetExerciseFileURLs(ctx context.Context, courseSlug string, firstVideoSlug string) ([]model.ExerciseFile, error) {
	pageURL := fmt.Sprintf("https://www.linkedin.com/learning/%s/%s", courseSlug, firstVideoSlug)
	if e.enterpriseHash != "" {
		pageURL += "?u=" + e.enterpriseHash
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pageURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating course page request: %w", err)
	}
	req.Header.Set("User-Agent", userAgent)
	if e.csrfToken != "" {
		req.Header.Set("Csrf-Token", e.csrfToken)
	}

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching course page: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading course page body: %w", err)
	}
	body := string(bodyBytes)

	re := regexp.MustCompile(`(?s)<code[^>]*\bid="bpr-guid-[^"]*"[^>]*>(.*?)</code>`)
	matches := re.FindAllStringSubmatch(body, -1)

	var exerciseFiles []model.ExerciseFile
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		decoded := html.UnescapeString(match[1])

		var bpr map[string]interface{}
		if err := json.Unmarshal([]byte(decoded), &bpr); err != nil {
			continue
		}

		included, ok := bpr["included"].([]interface{})
		if !ok {
			continue
		}

		for _, item := range included {
			itemMap, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			if getString(itemMap, "$type") != "com.linkedin.learning.api.deco.content.Course" {
				continue
			}
			efArr, ok := itemMap["exerciseFiles"].([]interface{})
			if !ok {
				continue
			}
			for _, ef := range efArr {
				efMap, ok := ef.(map[string]interface{})
				if !ok {
					continue
				}
				exerciseFiles = append(exerciseFiles, model.ExerciseFile{
					FileName:    getString(efMap, "name"),
					DownloadURL: getString(efMap, "url"),
					FileSize:    getInt64(efMap, "sizeInBytes"),
				})
			}
		}
	}

	if len(exerciseFiles) > 0 {
		slog.Info("Found exercise files with ambry URLs", "count", len(exerciseFiles))
	} else {
		slog.Info("No exercise files found in page HTML")
	}

	return exerciseFiles, nil
}

func parseChapters(element0 map[string]interface{}) []model.Chapter {
	var chapters []model.Chapter
	chaptersVal, ok := element0["chapters"]
	if !ok {
		return chapters
	}
	chaptersArr, ok := chaptersVal.([]interface{})
	if !ok {
		return chapters
	}

	for ci, ch := range chaptersArr {
		chMap, ok := ch.(map[string]interface{})
		if !ok {
			continue
		}
		chapter := model.Chapter{
			Title:         getString(chMap, "title"),
			Slug:          getString(chMap, "slug"),
			IndexInCourse: ci,
		}

		if videosVal, ok := chMap["videos"]; ok {
			if videosArr, ok := videosVal.([]interface{}); ok {
				for _, v := range videosArr {
					if vMap, ok := v.(map[string]interface{}); ok {
						chapter.Videos = append(chapter.Videos, model.Video{
							Title: getString(vMap, "title"),
							Slug:  getString(vMap, "slug"),
						})
					}
				}
			}
		}

		chapters = append(chapters, chapter)
	}

	return chapters
}

// isDNSError checks if the error is a DNS lookup failure.
func isDNSError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "no such host") ||
		strings.Contains(errStr, "lookup") ||
		strings.Contains(errStr, "DNS")
}

func mustParseURL(s string) *url.URL {
	u, err := url.Parse(s)
	if err != nil {
		panic(err)
	}
	return u
}
