package monitoring

import (
	"bytes"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
)

// ChangeDetector monitors content changes across crawls
type ChangeDetector struct {
	baselineDir     string
	enabled         bool
	webhookURL      string
	mutex           sync.RWMutex
	detectedChanges []ContentChange
}

// ContentChange represents a detected change in page content
type ContentChange struct {
	URL             string            `json:"url"`
	ChangeType      string            `json:"change_type"` // "new", "modified", "deleted", "title_changed", "meta_changed"
	Language        string            `json:"language"`
	OldContent      string            `json:"old_content,omitempty"`
	NewContent      string            `json:"new_content,omitempty"`
	OldHash         string            `json:"old_hash"`
	NewHash         string            `json:"new_hash"`
	Changes         []SpecificChange  `json:"changes"`
	DetectedAt      time.Time         `json:"detected_at"`
	Severity        string            `json:"severity"` // "low", "medium", "high", "critical"
	WordCountDelta  int               `json:"word_count_delta"`
	SimilarityScore float64           `json:"similarity_score"`
}

// SpecificChange represents a specific type of content change
type SpecificChange struct {
	Field       string `json:"field"`       // "title", "meta_desc", "content", "headings"
	OldValue    string `json:"old_value"`
	NewValue    string `json:"new_value"`
	ChangeRatio float64 `json:"change_ratio"`
}

// PageBaseline stores the baseline content for change comparison
type PageBaseline struct {
	URL           string            `json:"url"`
	Title         string            `json:"title"`
	MetaDesc      string            `json:"meta_description"`
	Content       string            `json:"content"`
	ContentHash   string            `json:"content_hash"`
	Headings      []string          `json:"headings"`
	WordCount     int               `json:"word_count"`
	Language      string            `json:"language"`
	LastUpdated   time.Time         `json:"last_updated"`
	StructureHash string            `json:"structure_hash"`
	Images        []string          `json:"images"`
	Links         []string          `json:"links"`
}

// AlertPayload for webhook notifications
type AlertPayload struct {
	Timestamp    time.Time       `json:"timestamp"`
	SiteName     string          `json:"site_name"`
	TotalChanges int             `json:"total_changes"`
	Changes      []ContentChange `json:"changes"`
	Summary      string          `json:"summary"`
	Priority     string          `json:"priority"`
}

func NewChangeDetector(baselineDir, webhookURL string, enabled bool) *ChangeDetector {
	if enabled && baselineDir != "" {
		os.MkdirAll(baselineDir, 0755)
	}
	
	return &ChangeDetector{
		baselineDir:     baselineDir,
		enabled:         enabled,
		webhookURL:      webhookURL,
		detectedChanges: make([]ContentChange, 0),
	}
}

// ExtractPageBaseline creates a baseline from a page document
func (cd *ChangeDetector) ExtractPageBaseline(doc *goquery.Document, pageURL, language string) *PageBaseline {
	if !cd.enabled {
		return nil
	}
	
	// Extract basic content
	title := strings.TrimSpace(doc.Find("title").Text())
	metaDesc, _ := doc.Find("meta[name='description']").Attr("content")
	
	// Extract main content
	content := ""
	doc.Find("main, article, .content, section").Each(func(i int, s *goquery.Selection) {
		if text := strings.TrimSpace(s.Text()); text != "" {
			content += text + " "
		}
	})
	
	if content == "" {
		content = strings.TrimSpace(doc.Find("body").Text())
	}
	
	// Extract headings
	headings := make([]string, 0)
	doc.Find("h1, h2, h3, h4, h5, h6").Each(func(i int, s *goquery.Selection) {
		if heading := strings.TrimSpace(s.Text()); heading != "" {
			headings = append(headings, heading)
		}
	})
	
	// Extract images and links for structure analysis
	images := make([]string, 0)
	doc.Find("img[src]").Each(func(i int, s *goquery.Selection) {
		if src, exists := s.Attr("src"); exists {
			images = append(images, src)
		}
	})
	
	links := make([]string, 0)
	doc.Find("a[href]").Each(func(i int, s *goquery.Selection) {
		if href, exists := s.Attr("href"); exists && !strings.HasPrefix(href, "#") {
			links = append(links, href)
		}
	})
	
	// Generate content hash
	contentHash := fmt.Sprintf("%x", md5.Sum([]byte(content)))
	
	// Generate structure hash (headings + images + links)
	structureData := strings.Join(headings, "|") + "|" + strings.Join(images, "|") + "|" + strings.Join(links, "|")
	structureHash := fmt.Sprintf("%x", md5.Sum([]byte(structureData)))
	
	baseline := &PageBaseline{
		URL:           pageURL,
		Title:         title,
		MetaDesc:      metaDesc,
		Content:       content,
		ContentHash:   contentHash,
		Headings:      headings,
		WordCount:     len(strings.Fields(content)),
		Language:      language,
		LastUpdated:   time.Now(),
		StructureHash: structureHash,
		Images:        images,
		Links:         links,
	}
	
	return baseline
}

// SaveBaseline stores a baseline for future comparison
func (cd *ChangeDetector) SaveBaseline(baseline *PageBaseline) error {
	if !cd.enabled || baseline == nil {
		return nil
	}
	
	// Generate filename from URL
	hash := md5.Sum([]byte(baseline.URL))
	filename := fmt.Sprintf("%x.json", hash)
	filepath := filepath.Join(cd.baselineDir, filename)
	
	data, err := json.MarshalIndent(baseline, "", "  ")
	if err != nil {
		return err
	}
	
	return os.WriteFile(filepath, data, 0644)
}

// LoadBaseline retrieves stored baseline for a URL
func (cd *ChangeDetector) LoadBaseline(pageURL string) (*PageBaseline, error) {
	if !cd.enabled {
		return nil, fmt.Errorf("change detection disabled")
	}
	
	hash := md5.Sum([]byte(pageURL))
	filename := fmt.Sprintf("%x.json", hash)
	filepath := filepath.Join(cd.baselineDir, filename)
	
	data, err := os.ReadFile(filepath)
	if err != nil {
		return nil, err
	}
	
	var baseline PageBaseline
	err = json.Unmarshal(data, &baseline)
	return &baseline, err
}

// DetectChanges compares current content with baseline
func (cd *ChangeDetector) DetectChanges(current *PageBaseline) *ContentChange {
	if !cd.enabled || current == nil {
		return nil
	}
	
	// Try to load baseline
	baseline, err := cd.LoadBaseline(current.URL)
	if err != nil {
		// No baseline exists, this is a new page
		cd.SaveBaseline(current)
		return &ContentChange{
			URL:         current.URL,
			ChangeType:  "new",
			Language:    current.Language,
			NewContent:  current.Content,
			NewHash:     current.ContentHash,
			DetectedAt:  time.Now(),
			Severity:    "medium",
			WordCountDelta: current.WordCount,
		}
	}
	
	// Compare content
	if baseline.ContentHash == current.ContentHash {
		// No changes detected
		return nil
	}
	
	// Analyze specific changes
	changes := make([]SpecificChange, 0)
	changeType := "modified"
	severity := "low"
	
	// Title changes
	if baseline.Title != current.Title {
		changes = append(changes, SpecificChange{
			Field:       "title",
			OldValue:    baseline.Title,
			NewValue:    current.Title,
			ChangeRatio: cd.calculateSimilarity(baseline.Title, current.Title),
		})
		if baseline.Title != "" && current.Title == "" {
			severity = "high"
		} else {
			severity = "medium"
		}
	}
	
	// Meta description changes
	if baseline.MetaDesc != current.MetaDesc {
		changes = append(changes, SpecificChange{
			Field:       "meta_desc",
			OldValue:    baseline.MetaDesc,
			NewValue:    current.MetaDesc,
			ChangeRatio: cd.calculateSimilarity(baseline.MetaDesc, current.MetaDesc),
		})
	}
	
	// Content changes
	contentSimilarity := cd.calculateSimilarity(baseline.Content, current.Content)
	if contentSimilarity < 0.9 { // 90% similarity threshold
		changes = append(changes, SpecificChange{
			Field:       "content",
			OldValue:    baseline.Content[:min(200, len(baseline.Content))], // Truncate for storage
			NewValue:    current.Content[:min(200, len(current.Content))],
			ChangeRatio: contentSimilarity,
		})
		
		// Determine severity based on similarity
		if contentSimilarity < 0.5 {
			severity = "high"
		} else if contentSimilarity < 0.8 {
			severity = "medium"
		}
	}
	
	// Structure changes (headings)
	if baseline.StructureHash != current.StructureHash {
		changes = append(changes, SpecificChange{
			Field:       "structure",
			OldValue:    fmt.Sprintf("%d headings", len(baseline.Headings)),
			NewValue:    fmt.Sprintf("%d headings", len(current.Headings)),
			ChangeRatio: cd.calculateHeadingSimilarity(baseline.Headings, current.Headings),
		})
	}
	
	if len(changes) == 0 {
		return nil // No significant changes
	}
	
	// Create change record
	change := &ContentChange{
		URL:             current.URL,
		ChangeType:      changeType,
		Language:        current.Language,
		OldContent:      baseline.Content,
		NewContent:      current.Content,
		OldHash:         baseline.ContentHash,
		NewHash:         current.ContentHash,
		Changes:         changes,
		DetectedAt:      time.Now(),
		Severity:        severity,
		WordCountDelta:  current.WordCount - baseline.WordCount,
		SimilarityScore: contentSimilarity,
	}
	
	// Update baseline
	cd.SaveBaseline(current)
	
	return change
}

// calculateSimilarity computes similarity between two strings
func (cd *ChangeDetector) calculateSimilarity(s1, s2 string) float64 {
	if s1 == s2 {
		return 1.0
	}
	
	if len(s1) == 0 || len(s2) == 0 {
		return 0.0
	}
	
	// Simple word-based similarity
	words1 := strings.Fields(strings.ToLower(s1))
	words2 := strings.Fields(strings.ToLower(s2))
	
	if len(words1) == 0 || len(words2) == 0 {
		return 0.0
	}
	
	// Create word sets
	set1 := make(map[string]bool)
	set2 := make(map[string]bool)
	
	for _, word := range words1 {
		set1[word] = true
	}
	for _, word := range words2 {
		set2[word] = true
	}
	
	// Calculate intersection
	intersection := 0
	for word := range set1 {
		if set2[word] {
			intersection++
		}
	}
	
	// Calculate union
	union := len(set1) + len(set2) - intersection
	
	if union == 0 {
		return 1.0
	}
	
	return float64(intersection) / float64(union)
}

// calculateHeadingSimilarity compares heading structures
func (cd *ChangeDetector) calculateHeadingSimilarity(h1, h2 []string) float64 {
	if len(h1) == 0 && len(h2) == 0 {
		return 1.0
	}
	
	if len(h1) == 0 || len(h2) == 0 {
		return 0.0
	}
	
	// Compare heading text
	set1 := make(map[string]bool)
	set2 := make(map[string]bool)
	
	for _, h := range h1 {
		set1[strings.ToLower(h)] = true
	}
	for _, h := range h2 {
		set2[strings.ToLower(h)] = true
	}
	
	intersection := 0
	for h := range set1 {
		if set2[h] {
			intersection++
		}
	}
	
	union := len(set1) + len(set2) - intersection
	if union == 0 {
		return 1.0
	}
	
	return float64(intersection) / float64(union)
}

// AddChange stores a detected change
func (cd *ChangeDetector) AddChange(change *ContentChange) {
	if !cd.enabled || change == nil {
		return
	}
	
	cd.mutex.Lock()
	defer cd.mutex.Unlock()
	
	cd.detectedChanges = append(cd.detectedChanges, *change)
	
	fmt.Printf("🚨 Change detected (%s): %s - %s\n", 
		change.Severity, change.ChangeType, change.URL)
}

// SendAlert sends webhook notification for detected changes
func (cd *ChangeDetector) SendAlert(siteName string) error {
	if !cd.enabled || cd.webhookURL == "" {
		return nil
	}
	
	cd.mutex.RLock()
	changes := make([]ContentChange, len(cd.detectedChanges))
	copy(changes, cd.detectedChanges)
	cd.mutex.RUnlock()
	
	if len(changes) == 0 {
		return nil
	}
	
	// Determine overall priority
	priority := "low"
	for _, change := range changes {
		if change.Severity == "critical" {
			priority = "critical"
			break
		} else if change.Severity == "high" {
			priority = "high"
		} else if change.Severity == "medium" && priority == "low" {
			priority = "medium"
		}
	}
	
	// Create summary
	summary := fmt.Sprintf("Detected %d content changes", len(changes))
	if len(changes) == 1 {
		summary = fmt.Sprintf("Content changed: %s", changes[0].URL)
	}
	
	alert := AlertPayload{
		Timestamp:    time.Now(),
		SiteName:     siteName,
		TotalChanges: len(changes),
		Changes:      changes,
		Summary:      summary,
		Priority:     priority,
	}
	
	// Send webhook
	jsonData, err := json.Marshal(alert)
	if err != nil {
		return err
	}
	
	resp, err := http.Post(cd.webhookURL, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		fmt.Printf("📬 Alert sent successfully (%d changes)\n", len(changes))
		// Clear sent changes
		cd.mutex.Lock()
		cd.detectedChanges = make([]ContentChange, 0)
		cd.mutex.Unlock()
	} else {
		return fmt.Errorf("webhook failed with status: %d", resp.StatusCode)
	}
	
	return nil
}

// GetChangeStats returns statistics about detected changes
func (cd *ChangeDetector) GetChangeStats() map[string]interface{} {
	if !cd.enabled {
		return map[string]interface{}{"enabled": false}
	}
	
	cd.mutex.RLock()
	defer cd.mutex.RUnlock()
	
	stats := map[string]interface{}{
		"enabled":       true,
		"baseline_dir":  cd.baselineDir,
		"webhook_url":   cd.webhookURL != "",
		"total_changes": len(cd.detectedChanges),
		"changes_by_type": make(map[string]int),
		"changes_by_severity": make(map[string]int),
	}
	
	for _, change := range cd.detectedChanges {
		changesByType := stats["changes_by_type"].(map[string]int)
		changesByType[change.ChangeType]++
		
		changesBySeverity := stats["changes_by_severity"].(map[string]int)
		changesBySeverity[change.Severity]++
	}
	
	return stats
}

// GetRecentChanges returns recent changes
func (cd *ChangeDetector) GetRecentChanges(hours int) []ContentChange {
	cd.mutex.RLock()
	defer cd.mutex.RUnlock()
	
	if hours <= 0 {
		return cd.detectedChanges
	}
	
	cutoff := time.Now().Add(-time.Duration(hours) * time.Hour)
	recent := make([]ContentChange, 0)
	
	for _, change := range cd.detectedChanges {
		if change.DetectedAt.After(cutoff) {
			recent = append(recent, change)
		}
	}
	
	return recent
}

// Helper function for min
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}