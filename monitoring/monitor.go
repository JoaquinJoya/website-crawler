package monitoring

import (
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
)

// Monitor integrates all monitoring capabilities
type Monitor struct {
	cacheManager     *CacheManager
	multiLangMonitor *MultiLangMonitor
	changeDetector   *ChangeDetector
	baseURL          *url.URL
	siteName         string
	enabled          bool
	stats            *MonitoringStats
	mutex            sync.RWMutex
}

// MonitoringStats tracks overall monitoring statistics
type MonitoringStats struct {
	StartTime           time.Time              `json:"start_time"`
	TotalPagesCrawled   int                    `json:"total_pages_crawled"`
	CacheHitRate        float64                `json:"cache_hit_rate"`
	LanguagesDetected   int                    `json:"languages_detected"`
	ChangesDetected     int                    `json:"changes_detected"`
	LastCrawlTime       time.Time              `json:"last_crawl_time"`
	AverageCrawlTime    time.Duration          `json:"average_crawl_time"`
	MonitoringEnabled   map[string]bool        `json:"monitoring_enabled"`
	DetailedStats       map[string]interface{} `json:"detailed_stats"`
}

// MonitorConfig contains configuration for all monitoring features
type MonitorConfig struct {
	CacheDir            string
	CacheTTL            time.Duration
	CacheEnabled        bool
	MultiLangEnabled    bool
	ChangeDetection     bool
	WebhookURL          string
	BaselineDir         string
	ComparisonThreshold float64
}

func NewMonitor(baseURL *url.URL, siteName string, config MonitorConfig) *Monitor {
	// Initialize components
	cacheManager := NewCacheManager(config.CacheDir, config.CacheTTL, config.CacheEnabled)
	multiLangMonitor := NewMultiLangMonitor(baseURL, config.MultiLangEnabled)
	changeDetector := NewChangeDetector(config.BaselineDir, config.WebhookURL, config.ChangeDetection)
	
	stats := &MonitoringStats{
		StartTime:         time.Now(),
		MonitoringEnabled: map[string]bool{
			"cache":       config.CacheEnabled,
			"multilang":   config.MultiLangEnabled,
			"changes":     config.ChangeDetection,
		},
		DetailedStats: make(map[string]interface{}),
	}
	
	monitor := &Monitor{
		cacheManager:     cacheManager,
		multiLangMonitor: multiLangMonitor,
		changeDetector:   changeDetector,
		baseURL:          baseURL,
		siteName:         siteName,
		enabled:          config.CacheEnabled || config.MultiLangEnabled || config.ChangeDetection,
		stats:            stats,
	}
	
	return monitor
}

// ProcessPage handles a single page through all monitoring systems
func (m *Monitor) ProcessPage(doc *goquery.Document, pageURL string) (*ProcessingResult, error) {
	if !m.enabled {
		return &ProcessingResult{URL: pageURL}, nil
	}
	
	start := time.Now()
	
	m.mutex.Lock()
	m.stats.TotalPagesCrawled++
	m.stats.LastCrawlTime = time.Now()
	m.mutex.Unlock()
	
	result := &ProcessingResult{
		URL:           pageURL,
		ProcessedAt:   start,
		CacheHit:      false,
		LanguageInfo:  nil,
		ChangeInfo:    nil,
		ProcessingTime: 0,
	}
	
	// 1. Language Detection & Multi-language Monitoring
	if m.multiLangMonitor.enabled {
		langVersion := m.multiLangMonitor.AnalyzeLanguageVersion(doc, pageURL)
		if langVersion != nil {
			m.multiLangMonitor.AddLanguageVersion(langVersion)
			result.LanguageInfo = langVersion
			
			// Find language alternates for comprehensive monitoring
			alternates := m.multiLangMonitor.FindLanguageAlternates(doc, pageURL)
			result.LanguageAlternates = alternates
			
			fmt.Printf("🌍 Language analysis: %s (%s) - %d words\n", 
				langVersion.Language, pageURL, langVersion.WordCount)
		}
	}
	
	// 2. Content Change Detection
	if m.changeDetector.enabled {
		// Extract current content as baseline
		language := ""
		if result.LanguageInfo != nil {
			language = result.LanguageInfo.Language
		}
		
		baseline := m.changeDetector.ExtractPageBaseline(doc, pageURL, language)
		if baseline != nil {
			change := m.changeDetector.DetectChanges(baseline)
			if change != nil {
				m.changeDetector.AddChange(change)
				result.ChangeInfo = change
				
				fmt.Printf("🚨 Content change detected: %s (%s)\n", 
					change.ChangeType, change.URL)
			}
		}
	}
	
	// 3. Cache Management (save for future use)
	if m.cacheManager.enabled {
		// Extract content and headers for caching
		content, _ := doc.Html()
		headers := map[string]string{
			"content-type": "text/html",
			"processed-at": start.Format(time.RFC3339),
		}
		
		err := m.cacheManager.SaveToCache(pageURL, content, headers)
		if err != nil {
			fmt.Printf("⚠️ Cache save failed for %s: %v\n", pageURL, err)
		}
	}
	
	result.ProcessingTime = time.Since(start)
	
	return result, nil
}

// ProcessingResult contains the results of monitoring a single page
type ProcessingResult struct {
	URL                string                 `json:"url"`
	ProcessedAt        time.Time              `json:"processed_at"`
	ProcessingTime     time.Duration          `json:"processing_time"`
	CacheHit           bool                   `json:"cache_hit"`
	LanguageInfo       *LanguageVersion       `json:"language_info,omitempty"`
	LanguageAlternates map[string]string      `json:"language_alternates,omitempty"`
	ChangeInfo         *ContentChange         `json:"change_info,omitempty"`
	Errors             []string               `json:"errors,omitempty"`
}

// CheckFromCache attempts to retrieve page from cache
func (m *Monitor) CheckFromCache(pageURL string) (*goquery.Document, bool) {
	if !m.cacheManager.enabled || !m.cacheManager.IsCached(pageURL) {
		return nil, false
	}
	
	entry, err := m.cacheManager.GetCacheEntry(pageURL)
	if err != nil {
		return nil, false
	}
	
	// Parse cached content
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(entry.Content))
	if err != nil {
		return nil, false
	}
	
	fmt.Printf("💾 Cache hit: %s (saved %.2fs)\n", pageURL, 
		time.Since(entry.CachedAt).Seconds())
	
	m.mutex.Lock()
	m.stats.TotalPagesCrawled++
	m.mutex.Unlock()
	
	return doc, true
}

// AnalyzeLanguageSync performs comprehensive language synchronization analysis
func (m *Monitor) AnalyzeLanguageSync() *LanguageSyncReport {
	if !m.multiLangMonitor.enabled {
		return &LanguageSyncReport{Enabled: false}
	}
	
	issues := m.multiLangMonitor.CompareLanguageVersions(0.8) // 80% similarity threshold
	stats := m.multiLangMonitor.GetLanguageStats()
	
	report := &LanguageSyncReport{
		Enabled:      true,
		GeneratedAt:  time.Now(),
		TotalPages:   stats["total_pages"].(int),
		Languages:    stats["languages"].(map[string]int),
		SyncIssues:   issues,
		IssueCount:   len(issues),
		Recommendations: m.generateSyncRecommendations(issues),
	}
	
	return report
}

// LanguageSyncReport provides comprehensive language synchronization analysis
type LanguageSyncReport struct {
	Enabled         bool                   `json:"enabled"`
	GeneratedAt     time.Time              `json:"generated_at"`
	TotalPages      int                    `json:"total_pages"`
	Languages       map[string]int         `json:"languages"`
	SyncIssues      []SyncIssue           `json:"sync_issues"`
	IssueCount      int                    `json:"issue_count"`
	Recommendations []string               `json:"recommendations"`
}

// generateSyncRecommendations creates actionable recommendations
func (m *Monitor) generateSyncRecommendations(issues []SyncIssue) []string {
	recommendations := make([]string, 0)
	
	missingTranslations := 0
	contentMismatches := 0
	
	for _, issue := range issues {
		switch issue.Type {
		case "missing_translation":
			missingTranslations++
		case "content_mismatch":
			contentMismatches++
		}
	}
	
	if missingTranslations > 0 {
		recommendations = append(recommendations, 
			fmt.Sprintf("🔴 %d pages need translation - prioritize high-traffic pages", missingTranslations))
	}
	
	if contentMismatches > 0 {
		recommendations = append(recommendations, 
			fmt.Sprintf("🟡 %d pages have content length mismatches - review translation completeness", contentMismatches))
	}
	
	if len(issues) == 0 {
		recommendations = append(recommendations, "✅ All language versions are synchronized")
	}
	
	return recommendations
}

// SendAlerts sends all pending alerts
func (m *Monitor) SendAlerts() error {
	if !m.changeDetector.enabled {
		return nil
	}
	
	return m.changeDetector.SendAlert(m.siteName)
}

// GetComprehensiveStats returns detailed monitoring statistics
func (m *Monitor) GetComprehensiveStats() *MonitoringStats {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	
	// Update detailed stats
	if m.cacheManager.enabled {
		m.stats.DetailedStats["cache"] = m.cacheManager.GetCacheStats()
	}
	
	if m.multiLangMonitor.enabled {
		m.stats.DetailedStats["languages"] = m.multiLangMonitor.GetLanguageStats()
	}
	
	if m.changeDetector.enabled {
		m.stats.DetailedStats["changes"] = m.changeDetector.GetChangeStats()
	}
	
	// Calculate cache hit rate
	if cacheStats, ok := m.stats.DetailedStats["cache"].(map[string]interface{}); ok {
		if totalFiles, ok := cacheStats["total_files"].(int); ok && m.stats.TotalPagesCrawled > 0 {
			m.stats.CacheHitRate = float64(totalFiles) / float64(m.stats.TotalPagesCrawled)
		}
	}
	
	// Count detected languages
	if langStats, ok := m.stats.DetailedStats["languages"].(map[string]interface{}); ok {
		if languages, ok := langStats["languages"].(map[string]int); ok {
			m.stats.LanguagesDetected = len(languages)
		}
	}
	
	// Count total changes
	if changeStats, ok := m.stats.DetailedStats["changes"].(map[string]interface{}); ok {
		if totalChanges, ok := changeStats["total_changes"].(int); ok {
			m.stats.ChangesDetected = totalChanges
		}
	}
	
	return m.stats
}

// CleanupResources performs maintenance tasks
func (m *Monitor) CleanupResources() error {
	var errs []error
	
	// Clean expired cache
	if m.cacheManager.enabled {
		if err := m.cacheManager.CleanExpiredCache(); err != nil {
			errs = append(errs, fmt.Errorf("cache cleanup failed: %v", err))
		}
	}
	
	// Send pending alerts
	if m.changeDetector.enabled {
		if err := m.SendAlerts(); err != nil {
			errs = append(errs, fmt.Errorf("alert sending failed: %v", err))
		}
	}
	
	if len(errs) > 0 {
		return fmt.Errorf("cleanup errors: %v", errs)
	}
	
	return nil
}

// IsEnabled returns whether monitoring is active
func (m *Monitor) IsEnabled() bool {
	return m.enabled
}

