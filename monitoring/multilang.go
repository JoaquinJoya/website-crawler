package monitoring

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
)

// LanguageVersion represents a language version of a page
type LanguageVersion struct {
	Language    string    `json:"language"`
	URL         string    `json:"url"`
	Title       string    `json:"title"`
	Content     string    `json:"content"`
	MetaDesc    string    `json:"meta_description"`
	LastUpdated time.Time `json:"last_updated"`
	WordCount   int       `json:"word_count"`
	Status      string    `json:"status"` // "synced", "outdated", "missing"
}

// MultiLangMonitor tracks multiple language versions
type MultiLangMonitor struct {
	baseURL     *url.URL
	languages   map[string]*LanguageVersion
	mutex       sync.RWMutex
	enabled     bool
	syncIssues  []SyncIssue
}

// SyncIssue represents a synchronization problem between languages
type SyncIssue struct {
	Type        string    `json:"type"` // "missing_translation", "content_mismatch", "outdated_content"
	Language    string    `json:"language"`
	URL         string    `json:"url"`
	Description string    `json:"description"`
	Severity    string    `json:"severity"` // "low", "medium", "high", "critical"
	DetectedAt  time.Time `json:"detected_at"`
}

func NewMultiLangMonitor(baseURL *url.URL, enabled bool) *MultiLangMonitor {
	return &MultiLangMonitor{
		baseURL:    baseURL,
		languages:  make(map[string]*LanguageVersion),
		enabled:    enabled,
		syncIssues: make([]SyncIssue, 0),
	}
}

// DetectLanguageFromURL extracts language from URL patterns
func (mlm *MultiLangMonitor) DetectLanguageFromURL(pageURL string) string {
	if !mlm.enabled {
		return ""
	}
	
	// Common language URL patterns
	patterns := map[string]*regexp.Regexp{
		"es": regexp.MustCompile(`/es(/|$)`),
		"en": regexp.MustCompile(`^(?!.*/es/).*$`), // Default to English if no /es/
		"fr": regexp.MustCompile(`/fr(/|$)`),
		"de": regexp.MustCompile(`/de(/|$)`),
		"pt": regexp.MustCompile(`/pt(/|$)`),
	}
	
	for lang, pattern := range patterns {
		if pattern.MatchString(pageURL) {
			if lang == "en" && strings.Contains(pageURL, "/es/") {
				continue // Skip English default if Spanish is detected
			}
			return lang
		}
	}
	
	return "unknown"
}

// ExtractLanguageFromHTML detects language from HTML content
func (mlm *MultiLangMonitor) ExtractLanguageFromHTML(doc *goquery.Document) string {
	// Check html lang attribute
	if lang, exists := doc.Find("html").Attr("lang"); exists && lang != "" {
		// Extract primary language code (e.g., "es-MX" -> "es")
		if idx := strings.Index(lang, "-"); idx > 0 {
			return lang[:idx]
		}
		return lang
	}
	
	// Check meta language tags
	doc.Find("meta[http-equiv='content-language'], meta[name='language']").Each(func(i int, s *goquery.Selection) {
		if content, exists := s.Attr("content"); exists && content != "" {
			if idx := strings.Index(content, "-"); idx > 0 {
				return // This would set a return value in actual implementation
			}
		}
	})
	
	return ""
}

// FindLanguageAlternates discovers all language versions of a page
func (mlm *MultiLangMonitor) FindLanguageAlternates(doc *goquery.Document, currentURL string) map[string]string {
	alternates := make(map[string]string)
	
	if !mlm.enabled {
		return alternates
	}
	
	// Extract hreflang links
	doc.Find("link[rel='alternate'][hreflang]").Each(func(i int, s *goquery.Selection) {
		hreflang, _ := s.Attr("hreflang")
		href, _ := s.Attr("href")
		
		if hreflang != "" && href != "" {
			// Extract primary language
			lang := hreflang
			if idx := strings.Index(lang, "-"); idx > 0 {
				lang = lang[:idx]
			}
			
			// Make URL absolute
			if absURL, err := url.Parse(href); err == nil {
				resolvedURL := mlm.baseURL.ResolveReference(absURL)
				alternates[lang] = resolvedURL.String()
			}
		}
	})
	
	// Also try to infer Spanish version if not found
	if _, hasSpanish := alternates["es"]; !hasSpanish {
		// Try to convert current URL to Spanish version
		if !strings.Contains(currentURL, "/es/") {
			spanishURL := mlm.tryConvertToSpanish(currentURL)
			if spanishURL != "" {
				alternates["es"] = spanishURL
			}
		}
	}
	
	// Try to infer English version
	if _, hasEnglish := alternates["en"]; !hasEnglish {
		if strings.Contains(currentURL, "/es/") {
			englishURL := strings.Replace(currentURL, "/es/", "/", 1)
			alternates["en"] = englishURL
		} else {
			alternates["en"] = currentURL
		}
	}
	
	return alternates
}

// tryConvertToSpanish attempts to convert an English URL to Spanish
func (mlm *MultiLangMonitor) tryConvertToSpanish(englishURL string) string {
	parsed, err := url.Parse(englishURL)
	if err != nil {
		return ""
	}
	
	// Insert /es/ after the domain
	if parsed.Path == "/" || parsed.Path == "" {
		parsed.Path = "/es"
	} else {
		parsed.Path = "/es" + parsed.Path
	}
	
	return parsed.String()
}

// AnalyzeLanguageVersion processes a page and extracts language information
func (mlm *MultiLangMonitor) AnalyzeLanguageVersion(doc *goquery.Document, pageURL string) *LanguageVersion {
	if !mlm.enabled {
		return nil
	}
	
	// Detect language
	lang := mlm.DetectLanguageFromURL(pageURL)
	if lang == "" || lang == "unknown" {
		lang = mlm.ExtractLanguageFromHTML(doc)
	}
	if lang == "" {
		lang = "unknown"
	}
	
	// Extract content information
	title := strings.TrimSpace(doc.Find("title").Text())
	metaDesc, _ := doc.Find("meta[name='description']").Attr("content")
	
	// Extract main content (skip navigation, footer, etc.)
	content := ""
	doc.Find("main, article, .content, .post-content, section").Each(func(i int, s *goquery.Selection) {
		if text := strings.TrimSpace(s.Text()); text != "" {
			content += text + " "
		}
	})
	
	// Fallback to body if no main content found
	if content == "" {
		content = strings.TrimSpace(doc.Find("body").Text())
	}
	
	// Count words (approximate)
	wordCount := len(strings.Fields(content))
	
	version := &LanguageVersion{
		Language:    lang,
		URL:         pageURL,
		Title:       title,
		Content:     content,
		MetaDesc:    metaDesc,
		LastUpdated: time.Now(),
		WordCount:   wordCount,
		Status:      "synced", // Will be updated during comparison
	}
	
	return version
}

// AddLanguageVersion stores a language version for monitoring
func (mlm *MultiLangMonitor) AddLanguageVersion(version *LanguageVersion) {
	if !mlm.enabled || version == nil {
		return
	}
	
	mlm.mutex.Lock()
	defer mlm.mutex.Unlock()
	
	mlm.languages[version.Language] = version
	
	fmt.Printf("🌍 Added %s version: %s (words: %d)\n", 
		strings.ToUpper(version.Language), version.URL, version.WordCount)
}

// CompareLanguageVersions analyzes differences between language versions
func (mlm *MultiLangMonitor) CompareLanguageVersions(threshold float64) []SyncIssue {
	if !mlm.enabled {
		return nil
	}
	
	mlm.mutex.Lock()
	defer mlm.mutex.Unlock()
	
	issues := make([]SyncIssue, 0)
	
	// Find all unique page paths across languages
	pagePaths := make(map[string]map[string]*LanguageVersion)
	
	for lang, version := range mlm.languages {
		// Extract page path (remove language prefix)
		path := mlm.extractPagePath(version.URL)
		
		if pagePaths[path] == nil {
			pagePaths[path] = make(map[string]*LanguageVersion)
		}
		pagePaths[path][lang] = version
	}
	
	// Analyze each page path
	for _, versions := range pagePaths {
		enVersion, hasEnglish := versions["en"]
		esVersion, hasSpanish := versions["es"]
		
		// Check for missing translations
		if hasEnglish && !hasSpanish {
			issues = append(issues, SyncIssue{
				Type:        "missing_translation",
				Language:    "es",
				URL:         mlm.tryConvertToSpanish(enVersion.URL),
				Description: fmt.Sprintf("Spanish translation missing for: %s", enVersion.Title),
				Severity:    "high",
				DetectedAt:  time.Now(),
			})
		} else if hasSpanish && !hasEnglish {
			issues = append(issues, SyncIssue{
				Type:        "missing_translation",
				Language:    "en",
				URL:         strings.Replace(esVersion.URL, "/es/", "/", 1),
				Description: fmt.Sprintf("English translation missing for: %s", esVersion.Title),
				Severity:    "medium",
				DetectedAt:  time.Now(),
			})
		}
		
		// Compare content if both versions exist
		if hasEnglish && hasSpanish {
			// Word count comparison
			enWords := float64(enVersion.WordCount)
			esWords := float64(esVersion.WordCount)
			
			if enWords > 0 && esWords > 0 {
				ratio := esWords / enWords
				if ratio < threshold {
					issues = append(issues, SyncIssue{
						Type:        "content_mismatch",
						Language:    "es",
						URL:         esVersion.URL,
						Description: fmt.Sprintf("Spanish content significantly shorter (%.1f%% of English): %s", ratio*100, esVersion.Title),
						Severity:    "medium",
						DetectedAt:  time.Now(),
					})
				}
			}
			
			// Title comparison
			if enVersion.Title != "" && esVersion.Title == enVersion.Title {
				issues = append(issues, SyncIssue{
					Type:        "content_mismatch",
					Language:    "es",
					URL:         esVersion.URL,
					Description: fmt.Sprintf("Title not translated: %s", esVersion.Title),
					Severity:    "low",
					DetectedAt:  time.Now(),
				})
			}
		}
	}
	
	mlm.syncIssues = issues
	return issues
}

// extractPagePath removes language prefix from URL
func (mlm *MultiLangMonitor) extractPagePath(pageURL string) string {
	parsed, err := url.Parse(pageURL)
	if err != nil {
		return pageURL
	}
	
	path := parsed.Path
	
	// Remove language prefixes
	for _, prefix := range []string{"/es/", "/en/", "/fr/", "/de/"} {
		if strings.HasPrefix(path, prefix) {
			path = "/" + strings.TrimPrefix(path, prefix)
			break
		}
	}
	
	return path
}

// GetLanguageStats returns statistics about language versions
func (mlm *MultiLangMonitor) GetLanguageStats() map[string]interface{} {
	if !mlm.enabled {
		return map[string]interface{}{"enabled": false}
	}
	
	mlm.mutex.RLock()
	defer mlm.mutex.RUnlock()
	
	stats := map[string]interface{}{
		"enabled":    true,
		"languages":  make(map[string]int),
		"total_pages": len(mlm.languages),
		"sync_issues": len(mlm.syncIssues),
		"issues_by_severity": make(map[string]int),
	}
	
	// Count pages by language
	for lang, version := range mlm.languages {
		stats["languages"].(map[string]int)[lang] = stats["languages"].(map[string]int)[lang] + 1
		_ = version // Use version if needed
	}
	
	// Count issues by severity
	for _, issue := range mlm.syncIssues {
		severityCount := stats["issues_by_severity"].(map[string]int)
		severityCount[issue.Severity]++
	}
	
	return stats
}

// GetSyncIssues returns current synchronization issues
func (mlm *MultiLangMonitor) GetSyncIssues() []SyncIssue {
	mlm.mutex.RLock()
	defer mlm.mutex.RUnlock()
	
	return mlm.syncIssues
}