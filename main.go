package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/PuerkitoBio/goquery"

	"web-crawler/ai"
	"web-crawler/config"
	"web-crawler/crawler"
	"web-crawler/extract"
	"web-crawler/monitoring"
)

// Type definitions
type CrawlRequest struct {
	URL string `json:"url" form:"url" binding:"required"`
}

type ContentOptions struct {
	Head       bool `json:"head"`
	HTML       bool `json:"html"`
	Text       bool `json:"text"`
	Markdown   bool `json:"markdown"`
	Headings   bool `json:"headings"`
	Paragraphs bool `json:"paragraphs"`
	Links      bool `json:"links"`
	Images     bool `json:"images"`
}

type QAOptions struct {
	ValidateLinks bool `json:"validate_links"`
	CheckImages   bool `json:"check_images"`
	Accessibility bool `json:"accessibility"`
	SEOBasics     bool `json:"seo_basics"`
}

type TargetSelector struct {
	Selector    string `json:"selector"`
	Type        string `json:"type"`
	Description string `json:"description"`
	Mode        string `json:"mode"`
}

type AIConfig struct {
	Provider string `json:"provider"`
	Model    string `json:"model"`
	APIKey   string `json:"api_key"`
	Prompt   string `json:"prompt"`
}

type PageContent struct {
	URL         string              `json:"url"`
	Title       string              `json:"title"`
	Content     string              `json:"content"`
	HTMLContent string              `json:"html_content,omitempty"`
	Markdown    string              `json:"markdown,omitempty"`
	HeadData    map[string]string   `json:"head_data,omitempty"`
	Headings    []map[string]string `json:"headings,omitempty"`
	Paragraphs  []string            `json:"paragraphs,omitempty"`
	Links       []map[string]string `json:"links,omitempty"`
	Images      []map[string]string `json:"images,omitempty"`
	AIAnalysis  string              `json:"ai_analysis,omitempty"`
	AIPrompt    string              `json:"ai_prompt,omitempty"`
	QAResults   *QAResults          `json:"qa_results,omitempty"`
	Error       string              `json:"error,omitempty"`
}

type QAResults struct {
	LinkValidation   *LinkValidationResult   `json:"link_validation,omitempty"`
	ImageValidation  *ImageValidationResult  `json:"image_validation,omitempty"`
	AccessibilityAudit *AccessibilityResult  `json:"accessibility_audit,omitempty"`
	SEOBasics        *SEOBasicsResult        `json:"seo_basics,omitempty"`
}

type SiteWideQAResults struct {
	TotalPagesAnalyzed int                     `json:"total_pages_analyzed"`
	LinkValidation     *SiteWideLinkResults    `json:"link_validation,omitempty"`
	ImageValidation    *SiteWideImageResults   `json:"image_validation,omitempty"`
	AccessibilityAudit *SiteWideAccessibility  `json:"accessibility_audit,omitempty"`
	SEOBasics          *SiteWideSEOResults     `json:"seo_basics,omitempty"`
	PerPageResults     []PageQAResult          `json:"per_page_results,omitempty"`
}

type SiteWideLinkResults struct {
	TotalLinksFound   int                    `json:"total_links_found"`
	TotalValidLinks   int                    `json:"total_valid_links"`
	TotalBrokenLinks  int                    `json:"total_broken_links"`
	TotalExternalLinks int                   `json:"total_external_links"`
	TotalInternalLinks int                   `json:"total_internal_links"`
	BrokenLinksByPage map[string][]BrokenLinkDetail `json:"broken_links_by_page,omitempty"`
	UniqueURLsChecked int                    `json:"unique_urls_checked"`
}

type SiteWideImageResults struct {
	TotalImagesFound     int                              `json:"total_images_found"`
	TotalValidImages     int                              `json:"total_valid_images"`
	TotalBrokenImages    int                              `json:"total_broken_images"`
	TotalMissingAltText  int                              `json:"total_missing_alt_text"`
	BrokenImagesByPage   map[string][]BrokenImageDetail   `json:"broken_images_by_page,omitempty"`
}

type SiteWideAccessibility struct {
	TotalMissingAltTags    int               `json:"total_missing_alt_tags"`
	TotalMissingAriaLabels int               `json:"total_missing_aria_labels"`
	OverallScore           int               `json:"overall_score"`
	IssuesByPage           map[string][]string `json:"issues_by_page,omitempty"`
}

type SiteWideSEOResults struct {
	PagesWithTitle         int               `json:"pages_with_title"`
	PagesWithMetaDesc      int               `json:"pages_with_meta_desc"`
	PagesWithProperH1      int               `json:"pages_with_proper_h1"`
	OverallSEOScore        int               `json:"overall_seo_score"`
	IssuesByPage           map[string][]string `json:"issues_by_page,omitempty"`
}

type PageQAResult struct {
	URL       string     `json:"url"`
	Title     string     `json:"title"`
	QAResults *QAResults `json:"qa_results"`
}

type LinkValidationResult struct {
	TotalLinks    int                    `json:"total_links"`
	ValidLinks    int                    `json:"valid_links"`
	BrokenLinks   int                    `json:"broken_links"`
	ExternalLinks int                    `json:"external_links"`
	InternalLinks int                    `json:"internal_links"`
	BrokenDetails []BrokenLinkDetail     `json:"broken_details,omitempty"`
}

type BrokenLinkDetail struct {
	URL        string `json:"url"`
	Text       string `json:"text"`
	StatusCode int    `json:"status_code"`
	Error      string `json:"error"`
}

type ImageValidationResult struct {
	TotalImages    int                  `json:"total_images"`
	ValidImages    int                  `json:"valid_images"`
	BrokenImages   int                  `json:"broken_images"`
	MissingAltText int                  `json:"missing_alt_text"`
	BrokenDetails  []BrokenImageDetail  `json:"broken_details,omitempty"`
}

type BrokenImageDetail struct {
	URL        string `json:"url"`
	AltText    string `json:"alt_text"`
	StatusCode int    `json:"status_code"`
	Error      string `json:"error"`
}

type AccessibilityResult struct {
	MissingAltTags     int      `json:"missing_alt_tags"`
	MissingAriaLabels  int      `json:"missing_aria_labels"`
	HeadingStructure   string   `json:"heading_structure"`
	AccessibilityScore int      `json:"accessibility_score"`
	Issues             []string `json:"issues,omitempty"`
}

type SEOBasicsResult struct {
	HasTitle          bool     `json:"has_title"`
	HasMetaDescription bool    `json:"has_meta_description"`
	TitleLength       int      `json:"title_length"`
	MetaDescLength    int      `json:"meta_desc_length"`
	H1Count           int      `json:"h1_count"`
	HeadingOrder      bool     `json:"heading_order"`
	SEOScore          int      `json:"seo_score"`
	Issues            []string `json:"issues,omitempty"`
}

type CrawlResponse struct {
	URL   string        `json:"url"`
	URLs  []string      `json:"urls"`
	Pages []PageContent `json:"pages"`
	Count int           `json:"count"`
	Error string        `json:"error,omitempty"`
}

// Comparison structs
type ComparisonRequest struct {
	URLs         []string `json:"urls" binding:"required"`
	CustomPrompt string   `json:"custom_prompt"`
	AIConfig     AIConfig `json:"ai_config"`
	CompareAll   bool     `json:"compare_all"`
}

type PromptTemplate struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Template    string `json:"template"`
}

type ComparisonResponse struct {
	SessionID   string         `json:"session_id"`
	URLs        []string       `json:"urls"`
	Analysis    string         `json:"analysis"`
	Pages       []PageContent  `json:"pages"`
	Prompt      string         `json:"prompt"`
	GeneratedAt time.Time      `json:"generated_at"`
	Error       string         `json:"error,omitempty"`
}

// Server type
type Server struct {
	router *gin.Engine
	port   string
}

func main() {
	cfg := config.Load()
	
	server := NewServer(cfg.Server.Port)
	server.SetupRoutes()
	
	log.Printf("Starting web crawler server on port %s", cfg.Server.Port)
	if err := server.Run(); err != nil {
		log.Fatal("Failed to start server:", err)
	}
}

// Server methods
func NewServer(port string) *Server {
	r := gin.Default()
	
	r.Use(gin.CustomRecovery(func(c *gin.Context, recovered interface{}) {
		c.AbortWithStatus(500)
	}))
	
	r.LoadHTMLGlob("templates/*")
	
	return &Server{
		router: r,
		port:   port,
	}
}

func (s *Server) SetupRoutes() {
	s.router.GET("/", s.homeHandler)
	s.router.POST("/crawl", s.crawlHandler)
	s.router.GET("/stream-crawl", s.streamCrawlHandler)
	s.router.POST("/retry-ai", s.retryAIHandler)
	s.router.POST("/extract-additional", s.extractAdditionalHandler)
	s.router.GET("/monitoring", s.monitoringHandler)
	s.router.GET("/monitoring/stats", s.monitoringStatsHandler)
	s.router.GET("/monitoring/language-sync", s.languageSyncHandler)
	
	// Comparison endpoints
	s.router.POST("/compare-pages", s.comparePagesHandler)
	s.router.GET("/prompt-templates", s.promptTemplatesHandler)
}

func (s *Server) Run() error {
	return s.router.Run(":" + s.port)
}

func (s *Server) homeHandler(c *gin.Context) {
	c.HTML(http.StatusOK, "index.html", gin.H{
		"title": "Web Crawler",
	})
}

func (s *Server) crawlHandler(c *gin.Context) {
	var req CrawlRequest
	if err := c.ShouldBind(&req); err != nil {
		c.JSON(http.StatusBadRequest, CrawlResponse{
			Error: "Invalid URL provided",
		})
		return
	}
	
	urls, pages, err := crawlURLWithContent(req.URL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, CrawlResponse{
			URL:   req.URL,
			Error: err.Error(),
		})
		return
	}
	
	c.JSON(http.StatusOK, CrawlResponse{
		URL:   req.URL,
		URLs:  urls,
		Pages: pages,
		Count: len(urls),
	})
}

func (s *Server) streamCrawlHandler(c *gin.Context) {
	targetURL := c.Query("url")
	if targetURL == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "URL parameter required"})
		return
	}

	options := ContentOptions{
		Head:       c.Query("head") == "true",
		HTML:       c.Query("html") == "true",
		Text:       c.Query("text") == "true",
		Markdown:   c.Query("markdown") == "true",
		Headings:   c.Query("headings") == "true",
		Paragraphs: c.Query("paragraphs") == "true",
		Links:      c.Query("links") == "true",
		Images:     c.Query("images") == "true",
	}

	qaOptions := QAOptions{
		ValidateLinks: c.Query("qa_validate_links") == "true",
		CheckImages:   c.Query("qa_check_images") == "true",
		Accessibility: c.Query("qa_accessibility") == "true",
		SEOBasics:     c.Query("qa_seo_basics") == "true",
	}

	var targetSelector *TargetSelector
	if selectorType := c.Query("target_type"); selectorType != "" {
		selector := c.Query("target_selector")
		description := c.Query("target_description")
		mode := c.Query("target_mode")
		
		if mode == "" {
			mode = "content"
		}
		
		var cssSelector string
		switch selectorType {
		case "id":
			cssSelector = "#" + selector
		case "class":
			cssSelector = "." + selector
		case "tag":
			cssSelector = selector
		case "custom":
			cssSelector = selector
		}
		
		if cssSelector != "" {
			targetSelector = &TargetSelector{
				Selector:    cssSelector,
				Type:        selectorType,
				Description: description,
				Mode:        mode,
			}
		}
	}

	aiConfig := AIConfig{
		Provider: c.Query("ai_provider"),
		Model:    c.Query("ai_model"),
		APIKey:   c.Query("ai_api_key"),
		Prompt:   c.Query("ai_prompt"),
	}
	
	var maxPages, maxDepth int
	if maxPagesStr := c.Query("max_pages"); maxPagesStr != "" {
		if parsed, err := strconv.Atoi(maxPagesStr); err == nil && parsed > 0 {
			maxPages = parsed
		}
	}
	if maxDepthStr := c.Query("max_depth"); maxDepthStr != "" {
		if parsed, err := strconv.Atoi(maxDepthStr); err == nil && parsed > 0 {
			maxDepth = parsed
		}
	}

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Access-Control-Allow-Origin", "*")

	baseURL, err := url.Parse(targetURL)
	if err != nil {
		c.SSEvent("error", gin.H{"error": err.Error()})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	urls := discoverURLs(targetURL, baseURL, ctx, maxDepth)
	
	if maxPages > 0 && len(urls) > maxPages {
		urls = urls[:maxPages]
	}

	c.SSEvent("urls", gin.H{"urls": urls, "count": len(urls)})

	s.streamPageContents(c, urls, ctx, options, aiConfig, targetSelector, maxPages, qaOptions)
}

func (s *Server) streamPageContents(c *gin.Context, urls []string, ctx context.Context, options ContentOptions, aiConfig AIConfig, targetSelector *TargetSelector, maxPages int, qaOptions QAOptions) {
	cfg := config.Load()
	
	// Use Colly or traditional crawler based on configuration
	collyConfig := crawler.CollyConfig{
		Enabled:            cfg.Colly.Enabled,
		UserAgent:          cfg.Colly.UserAgent,
		Delay:              cfg.Colly.Delay,
		RandomDelay:        cfg.Colly.RandomDelay,
		Parallelism:        cfg.Colly.Parallelism,
		DomainGlob:         cfg.Colly.DomainGlob,
		RespectRobotsTxt:   cfg.Colly.RespectRobotsTxt,
		AllowURLRevisit:    cfg.Colly.AllowURLRevisit,
		CacheDir:           cfg.Colly.CacheDir,
		DebugMode:          cfg.Colly.DebugMode,
		Async:              cfg.Colly.Async,
	}
	
	var wg sync.WaitGroup
	rate := time.NewTicker(200 * time.Millisecond)
	defer rate.Stop()

	var processedCount int32
	totalURLs := len(urls)
	
	// Track all pages for site-wide QA analysis
	var allPages []PageContent
	var pagesMutex sync.Mutex
	
	semaphore := make(chan struct{}, 5)
	fetcher := crawler.NewPageFetcherWithBackend(cfg.Colly.Enabled, collyConfig)

	clientGone := make(chan bool, 1)
	go func() {
		<-c.Request.Context().Done()
		clientGone <- true
	}()

	for _, url := range urls {
		wg.Add(1)
		go func(pageURL string) {
			defer func() {
				if r := recover(); r != nil {
					fmt.Printf("Panic processing %s: %v\n", pageURL, r)
				}
				atomic.AddInt32(&processedCount, 1)
				<-semaphore
				wg.Done()
			}()

			if maxPages > 0 && atomic.LoadInt32(&processedCount) >= int32(maxPages) {
				return
			}
			
			select {
			case semaphore <- struct{}{}:
			case <-ctx.Done():
				return
			case <-clientGone:
				return
			}

			select {
			case <-rate.C:
			case <-ctx.Done():
				return
			case <-clientGone:
				return
			}

			pageCtx, pageCancel := context.WithTimeout(ctx, 45*time.Second)
			defer pageCancel()

			var page PageContent
			
			for attempt := 1; attempt <= 3; attempt++ {
				page = fetchPageContent(pageURL, pageCtx, options, aiConfig, targetSelector, fetcher, qaOptions)
				
				if page.Error == "" {
					break
				}
				
				if attempt < 3 {
					time.Sleep(time.Duration(attempt) * time.Second)
				} else {
					if strings.Contains(page.Error, "404") || strings.Contains(page.Error, "HTTP 4") {
						return
					}
					page.Title = fmt.Sprintf("Failed to load: %s", pageURL)
					page.Content = fmt.Sprintf("Error: %s", page.Error)
				}
			}
			
			// Store page for site-wide QA analysis
			pagesMutex.Lock()
			allPages = append(allPages, page)
			pagesMutex.Unlock()
			
			data, err := json.Marshal(page)
			if err != nil {
				return
			}
			
			select {
			case <-clientGone:
				return
			case <-ctx.Done():
				return
			default:
				func() {
					defer func() {
						if r := recover(); r != nil {
							fmt.Printf("Error sending page %s: %v\n", pageURL, r)
						}
					}()
					c.SSEvent("page", string(data))
					if flusher, ok := c.Writer.(http.Flusher); ok {
						flusher.Flush()
					}
				}()
			}
		}(url)
	}

	wg.Wait()
	
	finalCount := atomic.LoadInt32(&processedCount)
	
	// Perform site-wide QA analysis if any QA options were selected
	if qaOptions.ValidateLinks || qaOptions.CheckImages || qaOptions.Accessibility || qaOptions.SEOBasics {
		select {
		case <-clientGone:
			return
		default:
			func() {
				defer func() {
					if r := recover(); r != nil {
						fmt.Printf("Error performing site-wide QA: %v\n", r)
					}
				}()
				
				fmt.Printf("[SITE-WIDE QA] Starting site-wide analysis for %d pages\n", len(allPages))
				siteWideQA := performSiteWideQA(allPages, qaOptions, ctx)
				
				qaData, err := json.Marshal(siteWideQA)
				if err == nil {
					c.SSEvent("sitewide_qa", string(qaData))
					if flusher, ok := c.Writer.(http.Flusher); ok {
						flusher.Flush()
					}
				}
			}()
		}
	}
	
	select {
	case <-clientGone:
		return
	default:
		func() {
			defer func() {
				if r := recover(); r != nil {
					fmt.Printf("Error sending completion message: %v\n", r)
				}
			}()
			c.SSEvent("complete", gin.H{
				"message": fmt.Sprintf("Crawling completed: %d/%d pages processed", finalCount, totalURLs),
				"processed": finalCount,
				"total": totalURLs,
			})
			if flusher, ok := c.Writer.(http.Flusher); ok {
				flusher.Flush()
			}
		}()
	}
}

func crawlURLWithContent(targetURL string) ([]string, []PageContent, error) {
	cfg := config.Load()
	
	baseURL, err := url.Parse(targetURL)
	if err != nil {
		return nil, nil, err
	}
	
	ctx := context.Background()
	
	// Use Colly or traditional crawler based on configuration
	collyConfig := crawler.CollyConfig{
		Enabled:            cfg.Colly.Enabled,
		UserAgent:          cfg.Colly.UserAgent,
		Delay:              cfg.Colly.Delay,
		RandomDelay:        cfg.Colly.RandomDelay,
		Parallelism:        cfg.Colly.Parallelism,
		DomainGlob:         cfg.Colly.DomainGlob,
		RespectRobotsTxt:   cfg.Colly.RespectRobotsTxt,
		AllowURLRevisit:    cfg.Colly.AllowURLRevisit,
		CacheDir:           cfg.Colly.CacheDir,
		DebugMode:          cfg.Colly.DebugMode,
		Async:              cfg.Colly.Async,
	}
	
	urls, err := crawler.DiscoverURLsWithBackend(targetURL, baseURL, ctx, cfg.Colly.Enabled, collyConfig)
	if err != nil {
		return nil, nil, err
	}
	
	pages := fetchPageContents(urls, ctx, cfg.Colly.Enabled, collyConfig)
	
	return urls, pages, nil
}

func fetchPageContents(urls []string, ctx context.Context, useColly bool, collyConfig crawler.CollyConfig) []PageContent {
	var pages []PageContent
	fetcher := crawler.NewPageFetcherWithBackend(useColly, collyConfig)
	
	for _, pageURL := range urls {
		page := fetchPageContent(pageURL, ctx, ContentOptions{
			Head: true, HTML: true, Text: true, Markdown: true,
			Headings: true, Paragraphs: true, Links: true, Images: true,
		}, AIConfig{}, nil, fetcher, QAOptions{})
		pages = append(pages, page)
	}
	
	return pages
}

func fetchPageContent(pageURL string, ctx context.Context, options ContentOptions, aiConfig AIConfig, targetSelector *TargetSelector, fetcher crawler.PageFetcherInterface, qaOptions QAOptions) PageContent {
	doc, err := fetcher.FetchDocument(pageURL, ctx)
	if err != nil {
		return PageContent{
			URL:   pageURL,
			Error: err.Error(),
		}
	}

	workingDoc := doc
	if targetSelector != nil {
		if targetDoc, err := fetcher.ApplyTargetSelector(doc, &crawler.TargetSelector{
			Selector:    targetSelector.Selector,
			Type:        targetSelector.Type,
			Description: targetSelector.Description,
			Mode:        targetSelector.Mode,
		}); err == nil && targetDoc != nil {
			workingDoc = targetDoc
		}
	}

	page := PageContent{
		URL:   pageURL,
		Title: extract.Title(doc),
	}
	
	if targetSelector != nil {
		modeText := "content inside"
		if targetSelector.Mode == "element" {
			modeText = "full element with tag"
		}
		page.Content = fmt.Sprintf("Target Selector: %s (%s)\nMode: %s\nDescription: %s\n\n", 
			targetSelector.Selector, targetSelector.Type, modeText, targetSelector.Description)
	}

	if options.Text {
		if targetSelector != nil {
			page.Content += extract.FormattedText(workingDoc)
		} else {
			page.Content = extract.FormattedText(workingDoc)
		}
	}

	if options.HTML {
		if targetSelector != nil {
			htmlContent, _ := workingDoc.Html()
			page.HTMLContent = htmlContent
		} else {
			htmlContent, _ := doc.Html()
			page.HTMLContent = htmlContent
		}
	}

	if options.Head {
		page.HeadData = extract.HeadData(doc)
	}

	if options.Markdown {
		page.Markdown = extract.Markdown(workingDoc)
	}

	if options.Headings {
		page.Headings = extract.Headings(workingDoc)
	}

	if options.Paragraphs {
		page.Paragraphs = extract.Paragraphs(workingDoc)
	}

	if options.Links {
		page.Links = extract.Links(workingDoc, pageURL)
	}

	if options.Images {
		page.Images = extract.Images(workingDoc, pageURL)
	}

	if aiConfig.Provider != "" && aiConfig.APIKey != "" && aiConfig.Prompt != "" {
		content := ai.PrepareContentForAI(page.Title, page.URL, page.Content, page.Headings, page.Paragraphs, page.Links, page.Images, page.HTMLContent, page.Markdown)
		fullPrompt := fmt.Sprintf("%s\n\nContent to analyze:\n%s", aiConfig.Prompt, content)
		page.AIPrompt = fullPrompt
		
		provider := ai.NewPythonProvider(ai.Config{
			Provider: aiConfig.Provider,
			Model:    aiConfig.Model,
			APIKey:   aiConfig.APIKey,
			Prompt:   aiConfig.Prompt,
		})
		
		if analysis, err := provider.Process(content, aiConfig.Prompt, ctx); err == nil {
			page.AIAnalysis = analysis
		} else {
			page.AIAnalysis = fmt.Sprintf("AI Processing Error: %v", err)
		}
	}

	// Perform QA checks if requested
	if qaOptions.ValidateLinks || qaOptions.CheckImages || qaOptions.Accessibility || qaOptions.SEOBasics {
		page.QAResults = performQAChecks(workingDoc, page, pageURL, qaOptions, ctx)
	}

	return page
}

func discoverURLs(targetURL string, baseURL *url.URL, ctx context.Context, maxDepth int) []string {
	cfg := config.Load()
	
	// Use Colly or traditional crawler based on configuration
	collyConfig := crawler.CollyConfig{
		Enabled:            cfg.Colly.Enabled,
		UserAgent:          cfg.Colly.UserAgent,
		Delay:              cfg.Colly.Delay,
		RandomDelay:        cfg.Colly.RandomDelay,
		Parallelism:        cfg.Colly.Parallelism,
		DomainGlob:         cfg.Colly.DomainGlob,
		RespectRobotsTxt:   cfg.Colly.RespectRobotsTxt,
		AllowURLRevisit:    cfg.Colly.AllowURLRevisit,
		CacheDir:           cfg.Colly.CacheDir,
		DebugMode:          cfg.Colly.DebugMode,
		Async:              cfg.Colly.Async,
	}
	
	urls, err := crawler.DiscoverURLsWithBackend(targetURL, baseURL, ctx, cfg.Colly.Enabled, collyConfig)
	if err != nil {
		return []string{targetURL}
	}
	
	if maxDepth > 0 {
		filteredURLs := make([]string, 0)
		for _, urlStr := range urls {
			if parsedURL, err := url.Parse(urlStr); err == nil {
				depth := calculateURLDepth(parsedURL, baseURL)
				if depth <= maxDepth {
					filteredURLs = append(filteredURLs, urlStr)
				}
			}
		}
		return filteredURLs
	}
	
	return urls
}

func calculateURLDepth(targetURL, baseURL *url.URL) int {
	basePath := strings.Trim(baseURL.Path, "/")
	targetPath := strings.Trim(targetURL.Path, "/")
	
	if basePath == "" {
		if targetPath == "" {
			return 0
		}
		return len(strings.Split(targetPath, "/"))
	}
	
	if !strings.HasPrefix(targetPath, basePath) {
		return 0
	}
	
	remainingPath := strings.TrimPrefix(targetPath, basePath)
	remainingPath = strings.Trim(remainingPath, "/")
	
	if remainingPath == "" {
		return 0
	}
	
	return len(strings.Split(remainingPath, "/"))
}

// AI retry handler
func (s *Server) retryAIHandler(c *gin.Context) {
	var request struct {
		URL      string `json:"url" binding:"required"`
		Prompt   string `json:"prompt" binding:"required"`
		Provider string `json:"provider" binding:"required"`
		Model    string `json:"model"`
		APIKey   string `json:"api_key" binding:"required"`
		// Add content extraction options to preserve user's original selection
		Head       bool `json:"head"`
		HTML       bool `json:"html"`
		Text       bool `json:"text"`
		Markdown   bool `json:"markdown"`
		Headings   bool `json:"headings"`
		Paragraphs bool `json:"paragraphs"`
		Links      bool `json:"links"`
		Images     bool `json:"images"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request data",
			"details": err.Error(),
		})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	cfg := config.Load()
	
	// Create colly config for page fetching
	collyConfig := crawler.CollyConfig{
		Enabled:            cfg.Colly.Enabled,
		UserAgent:          cfg.Colly.UserAgent,
		Delay:              cfg.Colly.Delay,
		RandomDelay:        cfg.Colly.RandomDelay,
		Parallelism:        cfg.Colly.Parallelism,
		DomainGlob:         cfg.Colly.DomainGlob,
		RespectRobotsTxt:   cfg.Colly.RespectRobotsTxt,
		AllowURLRevisit:    cfg.Colly.AllowURLRevisit,
		CacheDir:           cfg.Colly.CacheDir,
		DebugMode:          cfg.Colly.DebugMode,
		Async:              cfg.Colly.Async,
		CacheEnabled:       cfg.Colly.CacheEnabled,
		CacheTTL:           cfg.Colly.CacheTTL,
	}

	// Fetch the page content
	fetcher := crawler.NewPageFetcherWithBackend(cfg.Colly.Enabled, collyConfig)
	doc, err := fetcher.FetchDocument(request.URL, ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to fetch page content",
			"details": err.Error(),
		})
		return
	}

	// Extract page content for AI analysis using original content options
	page := PageContent{
		URL:   request.URL,
		Title: extract.Title(doc),
	}
	
	// Only extract content types that were originally selected
	if request.Text {
		page.Content = extract.FormattedText(doc)
	}
	
	if request.HTML {
		htmlContent, _ := doc.Html()
		page.HTMLContent = htmlContent
	}
	
	if request.Head {
		page.HeadData = extract.HeadData(doc)
	}
	
	if request.Markdown {
		page.Markdown = extract.Markdown(doc)
	}
	
	if request.Headings {
		page.Headings = extract.Headings(doc)
	}
	
	if request.Paragraphs {
		page.Paragraphs = extract.Paragraphs(doc)
	}
	
	if request.Links {
		page.Links = extract.Links(doc, request.URL)
	}
	
	if request.Images {
		page.Images = extract.Images(doc, request.URL)
	}

	// Prepare AI configuration
	aiConfig := AIConfig{
		Provider: request.Provider,
		Model:    request.Model,
		APIKey:   request.APIKey,
		Prompt:   request.Prompt,
	}

	// Process with AI
	if aiConfig.Provider != "" && aiConfig.APIKey != "" && aiConfig.Prompt != "" {
		content := ai.PrepareContentForAI(page.Title, page.URL, page.Content, page.Headings, page.Paragraphs, page.Links, page.Images, page.HTMLContent, page.Markdown)
		fullPrompt := fmt.Sprintf("%s\n\nContent to analyze:\n%s", aiConfig.Prompt, content)
		page.AIPrompt = fullPrompt
		
		provider := ai.NewPythonProvider(ai.Config{
			Provider: aiConfig.Provider,
			Model:    aiConfig.Model,
			APIKey:   aiConfig.APIKey,
			Prompt:   aiConfig.Prompt,
		})
		
		if analysis, err := provider.Process(content, aiConfig.Prompt, ctx); err == nil {
			page.AIAnalysis = analysis
			c.JSON(http.StatusOK, gin.H{
				"success": true,
				"ai_analysis": analysis,
				"ai_prompt": fullPrompt,
			})
		} else {
			page.AIAnalysis = fmt.Sprintf("AI Processing Error: %v", err)
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"error": page.AIAnalysis,
				"ai_prompt": fullPrompt,
			})
		}
	} else {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Missing required AI configuration",
		})
	}
}

// Monitoring handlers
func (s *Server) monitoringHandler(c *gin.Context) {
	c.HTML(http.StatusOK, "monitoring.html", gin.H{
		"title": "Web Crawler Monitoring Dashboard",
	})
}

func (s *Server) monitoringStatsHandler(c *gin.Context) {
	cfg := config.Load()
	
	// Create a temporary monitor to get stats
	baseURL, _ := url.Parse("https://hisonrisa-wip.webflow.io/")
	monitorConfig := monitoring.MonitorConfig{
		CacheDir:            cfg.Colly.CacheDir,
		CacheTTL:            cfg.Colly.CacheTTL,
		CacheEnabled:        cfg.Colly.CacheEnabled,
		MultiLangEnabled:    cfg.Monitoring.MultiLangEnabled,
		ChangeDetection:     cfg.Monitoring.ChangeDetection,
		WebhookURL:          cfg.Monitoring.AlertWebhookURL,
		BaselineDir:         "./baselines",
		ComparisonThreshold: cfg.Monitoring.ComparisonThreshold,
	}
	
	monitor := monitoring.NewMonitor(baseURL, "hisonrisa", monitorConfig)
	stats := monitor.GetComprehensiveStats()
	
	c.JSON(http.StatusOK, gin.H{
		"status":     "success",
		"monitoring": stats,
		"timestamp":  time.Now(),
	})
}

func (s *Server) languageSyncHandler(c *gin.Context) {
	targetURL := c.Query("url")
	if targetURL == "" {
		targetURL = "https://hisonrisa-wip.webflow.io/"
	}
	
	cfg := config.Load()
	baseURL, err := url.Parse(targetURL)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid URL"})
		return
	}
	
	// Create monitor for language analysis
	monitorConfig := monitoring.MonitorConfig{
		CacheDir:            cfg.Colly.CacheDir,
		CacheTTL:            cfg.Colly.CacheTTL,
		CacheEnabled:        cfg.Colly.CacheEnabled,
		MultiLangEnabled:    true, // Force enable for this analysis
		ChangeDetection:     cfg.Monitoring.ChangeDetection,
		WebhookURL:          cfg.Monitoring.AlertWebhookURL,
		BaselineDir:         "./baselines",
		ComparisonThreshold: cfg.Monitoring.ComparisonThreshold,
	}
	
	monitor := monitoring.NewMonitor(baseURL, "hisonrisa", monitorConfig)
	
	// Quick crawl to populate language data
	ctx := context.Background()
	collyConfig := crawler.CollyConfig{
		Enabled:            cfg.Colly.Enabled,
		UserAgent:          cfg.Colly.UserAgent,
		Delay:              cfg.Colly.Delay,
		RandomDelay:        cfg.Colly.RandomDelay,
		Parallelism:        cfg.Colly.Parallelism,
		DomainGlob:         cfg.Colly.DomainGlob,
		RespectRobotsTxt:   cfg.Colly.RespectRobotsTxt,
		AllowURLRevisit:    cfg.Colly.AllowURLRevisit,
		CacheDir:           cfg.Colly.CacheDir,
		DebugMode:          cfg.Colly.DebugMode,
		Async:              cfg.Colly.Async,
		CacheEnabled:       cfg.Colly.CacheEnabled,
		CacheTTL:           cfg.Colly.CacheTTL,
	}
	
	// Discover URLs
	urls, err := crawler.DiscoverURLsWithBackend(targetURL, baseURL, ctx, cfg.Colly.Enabled, collyConfig)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	// Process a sample of pages to analyze language sync
	fetcher := crawler.NewPageFetcherWithBackend(cfg.Colly.Enabled, collyConfig)
	processedCount := 0
	maxProcessed := 20 // Limit for demo
	
	for _, pageURL := range urls {
		if processedCount >= maxProcessed {
			break
		}
		
		doc, err := fetcher.FetchDocument(pageURL, ctx)
		if err != nil {
			continue
		}
		
		// Process page through monitoring
		monitor.ProcessPage(doc, pageURL)
		processedCount++
	}
	
	// Generate language sync report
	report := monitor.AnalyzeLanguageSync()
	
	c.JSON(http.StatusOK, gin.H{
		"status":           "success",
		"target_url":       targetURL,
		"processed_pages":  processedCount,
		"language_report":  report,
		"timestamp":        time.Now(),
	})
}

// Extract additional content handler
func (s *Server) extractAdditionalHandler(c *gin.Context) {
	var request struct {
		URL              string `json:"url" binding:"required"`
		ContentTypes     []string `json:"content_types" binding:"required"`
		UseCache        bool   `json:"use_cache"`
		TargetSelector  string `json:"target_selector"`
		TargetType      string `json:"target_type"`
		TargetMode      string `json:"target_mode"`
		TargetDescription string `json:"target_description"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request data",
			"details": err.Error(),
		})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	cfg := config.Load()
	
	// Create colly config for page fetching
	collyConfig := crawler.CollyConfig{
		Enabled:            cfg.Colly.Enabled,
		UserAgent:          cfg.Colly.UserAgent,
		Delay:              cfg.Colly.Delay,
		RandomDelay:        cfg.Colly.RandomDelay,
		Parallelism:        cfg.Colly.Parallelism,
		DomainGlob:         cfg.Colly.DomainGlob,
		RespectRobotsTxt:   cfg.Colly.RespectRobotsTxt,
		AllowURLRevisit:    cfg.Colly.AllowURLRevisit,
		CacheDir:           cfg.Colly.CacheDir,
		DebugMode:          cfg.Colly.DebugMode,
		Async:              cfg.Colly.Async,
		CacheEnabled:       cfg.Colly.CacheEnabled,
		CacheTTL:           cfg.Colly.CacheTTL,
	}

	// Fetch the page content (will use cache if available and requested)
	fetcher := crawler.NewPageFetcherWithBackend(cfg.Colly.Enabled, collyConfig)
	doc, err := fetcher.FetchDocument(request.URL, ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to fetch page content",
			"details": err.Error(),
		})
		return
	}

	workingDoc := doc
	
	// Apply target selector if specified
	if request.TargetType != "" && request.TargetSelector != "" {
		var cssSelector string
		switch request.TargetType {
		case "id":
			cssSelector = "#" + request.TargetSelector
		case "class":
			cssSelector = "." + request.TargetSelector
		case "tag":
			cssSelector = request.TargetSelector
		case "custom":
			cssSelector = request.TargetSelector
		}
		
		if cssSelector != "" {
			if targetDoc, err := fetcher.ApplyTargetSelector(doc, &crawler.TargetSelector{
				Selector:    cssSelector,
				Type:        request.TargetType,
				Description: request.TargetDescription,
				Mode:        request.TargetMode,
			}); err == nil && targetDoc != nil {
				workingDoc = targetDoc
			}
		}
	}

	// Initialize response with basic page info
	result := PageContent{
		URL:   request.URL,
		Title: extract.Title(doc),
	}

	// Extract only the requested content types
	for _, contentType := range request.ContentTypes {
		switch contentType {
		case "text":
			result.Content = extract.FormattedText(workingDoc)
		case "html":
			htmlContent, _ := workingDoc.Html()
			result.HTMLContent = htmlContent
		case "head":
			result.HeadData = extract.HeadData(doc)
		case "markdown":
			result.Markdown = extract.Markdown(workingDoc)
		case "headings":
			result.Headings = extract.Headings(workingDoc)
		case "paragraphs":
			result.Paragraphs = extract.Paragraphs(workingDoc)
		case "links":
			result.Links = extract.Links(workingDoc, request.URL)
		case "images":
			result.Images = extract.Images(workingDoc, request.URL)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"content": result,
		"extracted_types": request.ContentTypes,
		"from_cache": request.UseCache,
	})
}

// Comparison handlers
func (s *Server) comparePagesHandler(c *gin.Context) {
	var req ComparisonRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ComparisonResponse{
			Error: "Invalid comparison request: " + err.Error(),
		})
		return
	}

	// Validate URLs
	if len(req.URLs) == 0 {
		c.JSON(http.StatusBadRequest, ComparisonResponse{
			Error: "At least one URL is required for comparison",
		})
		return
	}

	if len(req.URLs) > 50 {
		c.JSON(http.StatusBadRequest, ComparisonResponse{
			Error: "Maximum 50 URLs allowed for comparison",
		})
		return
	}

	// Extract content from all URLs
	var pages []PageContent
	var wg sync.WaitGroup
	var mu sync.Mutex
	var extractionErrors []string

	for _, pageURL := range req.URLs {
		wg.Add(1)
		go func(url string) {
			defer wg.Done()
			
			// Crawl individual page with full content extraction
			_, pageContents, err := crawlURLWithContent(url)
			if err != nil {
				mu.Lock()
				extractionErrors = append(extractionErrors, fmt.Sprintf("Error crawling %s: %s", url, err.Error()))
				mu.Unlock()
				return
			}

			// Find the main page (usually the first one with the exact URL)
			for _, page := range pageContents {
				if page.URL == url || strings.Contains(page.URL, url) {
					mu.Lock()
					pages = append(pages, page)
					mu.Unlock()
					break
				}
			}
		}(pageURL)
	}

	wg.Wait()

	if len(pages) == 0 {
		errorMsg := "No content could be extracted from the provided URLs"
		if len(extractionErrors) > 0 {
			errorMsg = strings.Join(extractionErrors, "; ")
		}
		c.JSON(http.StatusBadRequest, ComparisonResponse{
			Error: errorMsg,
		})
		return
	}

	// Prepare comparison prompt
	comparisonPrompt := req.CustomPrompt
	if comparisonPrompt == "" {
		comparisonPrompt = "Compare and analyze the content, messaging, and structure of the following pages. Identify key differences, similarities, and patterns."
	}

	// Generate AI analysis if AI config is provided
	var analysis string
	if req.AIConfig.Provider != "" && req.AIConfig.APIKey != "" {
		formattedContent := formatMultiPageContent(pages, comparisonPrompt)
		
		if result, err := ai.ProcessContent(formattedContent, req.AIConfig.Provider, req.AIConfig.Model, req.AIConfig.APIKey); err == nil {
			analysis = result
		} else {
			analysis = "AI analysis failed: " + err.Error()
		}
	}

	// Generate session ID
	sessionID := fmt.Sprintf("comp_%d", time.Now().Unix())

	response := ComparisonResponse{
		SessionID:   sessionID,
		URLs:        req.URLs,
		Analysis:    analysis,
		Pages:       pages,
		Prompt:      comparisonPrompt,
		GeneratedAt: time.Now(),
	}

	if len(extractionErrors) > 0 {
		response.Error = "Some pages could not be processed: " + strings.Join(extractionErrors, "; ")
	}

	c.JSON(http.StatusOK, response)
}

func (s *Server) promptTemplatesHandler(c *gin.Context) {
	templates := []PromptTemplate{
		{
			ID:          "content_comparison",
			Name:        "Content Comparison",
			Description: "Compare main content themes and messaging",
			Template:    "Compare the main content, messaging, and value propositions of the following pages. Identify similarities, differences, and which approach might be most effective:",
		},
		{
			ID:          "seo_analysis",
			Name:        "SEO Analysis",
			Description: "Compare SEO elements and optimization",
			Template:    "Analyze the SEO elements of these pages including titles, meta descriptions, headings structure, and content optimization. Identify which pages are better optimized and provide recommendations:",
		},
		{
			ID:          "design_patterns",
			Name:        "Design & UX Patterns",
			Description: "Compare layout, navigation, and user experience",
			Template:    "Compare the design patterns, navigation structure, layout, and user experience elements of these pages. Identify which design approaches work best and why:",
		},
		{
			ID:          "conversion_focus",
			Name:        "Conversion Analysis",
			Description: "Compare calls-to-action and conversion elements",
			Template:    "Analyze the conversion elements, calls-to-action, forms, and persuasion techniques used on these pages. Compare their effectiveness and provide recommendations for optimization:",
		},
		{
			ID:          "content_strategy",
			Name:        "Content Strategy",
			Description: "Compare content strategy and audience targeting",
			Template:    "Compare the content strategy, tone of voice, audience targeting, and messaging approach across these pages. Identify which strategy is most effective for different goals:",
		},
	}

	c.JSON(http.StatusOK, gin.H{
		"templates": templates,
	})
}

// Helper function to format multiple pages content for AI analysis
func formatMultiPageContent(pages []PageContent, customPrompt string) string {
	var content strings.Builder
	
	content.WriteString(customPrompt)
	content.WriteString("\n\n--- PAGES TO COMPARE ---\n\n")
	
	for i, page := range pages {
		content.WriteString(fmt.Sprintf("=== PAGE %d: %s ===\n", i+1, page.URL))
		content.WriteString(fmt.Sprintf("Title: %s\n\n", page.Title))
		
		if page.Content != "" {
			content.WriteString("Content:\n")
			content.WriteString(page.Content)
			content.WriteString("\n\n")
		}
		
		if len(page.Headings) > 0 {
			content.WriteString("Headings:\n")
			for _, heading := range page.Headings {
				content.WriteString(fmt.Sprintf("- %s: %s\n", heading["level"], heading["text"]))
			}
			content.WriteString("\n")
		}
		
		if page.Markdown != "" {
			content.WriteString("Markdown Content:\n")
			maxMarkdownLength := 8000
			if len(page.Markdown) > maxMarkdownLength {
				content.WriteString(page.Markdown[:maxMarkdownLength])
				content.WriteString("\n... (Markdown truncated for length)\n\n")
			} else {
				content.WriteString(page.Markdown)
				content.WriteString("\n\n")
			}
		}
		
		if len(page.Paragraphs) > 0 {
			content.WriteString("Paragraphs:\n")
			for i, paragraph := range page.Paragraphs {
				if i >= 10 { // Limit to first 10 paragraphs to avoid overwhelming the AI
					content.WriteString(fmt.Sprintf("... and %d more paragraphs\n", len(page.Paragraphs)-i))
					break
				}
				content.WriteString(fmt.Sprintf("Paragraph %d: %s\n", i+1, paragraph))
			}
			content.WriteString("\n")
		}
		
		if len(page.Links) > 0 {
			content.WriteString("Links:\n")
			for i, link := range page.Links {
				if i >= 20 { // Limit to first 20 links
					content.WriteString(fmt.Sprintf("... and %d more links\n", len(page.Links)-i))
					break
				}
				content.WriteString(fmt.Sprintf("- %s: %s\n", link["text"], link["url"]))
			}
			content.WriteString("\n")
		}
		
		if len(page.Images) > 0 {
			content.WriteString("Images:\n")
			for i, image := range page.Images {
				if i >= 10 { // Limit to first 10 images
					content.WriteString(fmt.Sprintf("... and %d more images\n", len(page.Images)-i))
					break
				}
				altText := image["alt"]
				if altText == "" {
					altText = "No alt text"
				}
				content.WriteString(fmt.Sprintf("- %s (Alt: %s)\n", image["url"], altText))
			}
			content.WriteString("\n")
		}
		
		if page.HTMLContent != "" {
			content.WriteString("HTML Structure:\n")
			// Include full HTML content, but limit to reasonable size for AI processing
			maxHTMLLength := 10000
			if len(page.HTMLContent) > maxHTMLLength {
				content.WriteString(page.HTMLContent[:maxHTMLLength])
				content.WriteString("\n... (HTML truncated for length)\n\n")
			} else {
				content.WriteString(page.HTMLContent)
				content.WriteString("\n\n")
			}
		}
		
		content.WriteString("---\n\n")
	}
	
	return content.String()
}

// Helper function for min
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// performQAChecks performs quality assurance checks on the page content
func performQAChecks(doc interface{}, page PageContent, pageURL string, qaOptions QAOptions, ctx context.Context) *QAResults {
	results := &QAResults{}

	if qaOptions.ValidateLinks {
		// Extract links directly from the HTML document for QA
		links := extractAllLinksForQA(doc, pageURL)
		
		// Debug: Log the number of links found
		fmt.Printf("[QA DEBUG] Found %d links on page %s\n", len(links), pageURL)
		if len(links) > 0 {
			fmt.Printf("[QA DEBUG] First few links: ")
			for i, link := range links[:min(3, len(links))] {
				if i > 0 {
					fmt.Printf(", ")
				}
				fmt.Printf("%s", link["url"])
			}
			fmt.Printf("\n")
		}
		
		results.LinkValidation = validateLinks(links, pageURL, ctx)
	}

	if qaOptions.CheckImages {
		// Extract images directly from the HTML document for QA
		images := extractAllImagesForQA(doc, pageURL)
		results.ImageValidation = validateImages(images, ctx)
	}

	if qaOptions.Accessibility {
		// Use extracted images and headings for accessibility check
		images := extractAllImagesForQA(doc, pageURL)
		results.AccessibilityAudit = checkAccessibility(images, page.Headings)
	}

	if qaOptions.SEOBasics {
		results.SEOBasics = checkSEOBasics(page.HeadData, page.Headings)
	}

	return results
}

// extractAllLinksForQA extracts ALL links from the HTML document using goquery
func extractAllLinksForQA(doc interface{}, pageURL string) []map[string]string {
	// Type assert to goquery document
	gqDoc, ok := doc.(*goquery.Document)
	if !ok {
		fmt.Printf("[QA DEBUG] Document type assertion failed for %s\n", pageURL)
		return []map[string]string{}
	}

	var links []map[string]string
	base, err := url.Parse(pageURL)
	if err != nil {
		fmt.Printf("[QA DEBUG] Failed to parse base URL %s: %v\n", pageURL, err)
		return links
	}

	// Debug: Check if document has any content
	htmlContent, _ := gqDoc.Html()
	fmt.Printf("[QA DEBUG] Document HTML length: %d characters\n", len(htmlContent))

	// Find ALL anchor tags (including those without href for debugging)
	allAnchorCount := 0
	gqDoc.Find("a").Each(func(i int, s *goquery.Selection) {
		allAnchorCount++
	})
	fmt.Printf("[QA DEBUG] Total anchor tags found: %d\n", allAnchorCount)

	// Find ALL anchor tags with href attributes
	gqDoc.Find("a[href]").Each(func(i int, s *goquery.Selection) {
		href, exists := s.Attr("href")
		if !exists || href == "" {
			return
		}

		text := strings.TrimSpace(s.Text())
		if text == "" {
			// If no text, try to get aria-label or title
			if ariaLabel, hasAria := s.Attr("aria-label"); hasAria {
				text = ariaLabel
			} else if title, hasTitle := s.Attr("title"); hasTitle {
				text = title
			} else {
				text = href // fallback to href
			}
		}

		// Create link entry
		link := map[string]string{
			"url":  href,
			"text": text,
		}

		// Try to resolve relative URLs
		if parsedURL, err := url.Parse(href); err == nil {
			resolvedURL := base.ResolveReference(parsedURL)
			link["url"] = resolvedURL.String()
		}

		// Add additional attributes if present
		if title, hasTitle := s.Attr("title"); hasTitle {
			link["title"] = title
		}

		if class, hasClass := s.Attr("class"); hasClass {
			link["class"] = class
		}

		if id, hasID := s.Attr("id"); hasID {
			link["id"] = id
		}

		links = append(links, link)
	})

	fmt.Printf("[QA DEBUG] Anchor tags with href found: %d\n", len(links))

	// Also check for JavaScript-based navigation (onclick handlers, etc.)
	jsLinkCount := 0
	gqDoc.Find("[onclick*='location'], [onclick*='window.open'], [onclick*='href']").Each(func(i int, s *goquery.Selection) {
		onclick, exists := s.Attr("onclick")
		if !exists {
			return
		}

		// Extract URLs from onclick handlers using regex
		urlRegex := regexp.MustCompile(`(?:location\.href|window\.open|href)\s*=\s*['"]([^'"]+)['"]`)
		matches := urlRegex.FindStringSubmatch(onclick)
		if len(matches) > 1 {
			href := matches[1]
			text := strings.TrimSpace(s.Text())
			if text == "" {
				text = "JavaScript link"
			}

			link := map[string]string{
				"url":  href,
				"text": text,
				"type": "javascript",
			}

			// Try to resolve relative URLs
			if parsedURL, err := url.Parse(href); err == nil {
				resolvedURL := base.ResolveReference(parsedURL)
				link["url"] = resolvedURL.String()
			}

			links = append(links, link)
			jsLinkCount++
		}
	})

	fmt.Printf("[QA DEBUG] JavaScript links found: %d\n", jsLinkCount)
	fmt.Printf("[QA DEBUG] Total links extracted: %d\n", len(links))

	return links
}

// extractAllImagesForQA extracts ALL images from the HTML document using goquery
func extractAllImagesForQA(doc interface{}, pageURL string) []map[string]string {
	// Type assert to goquery document
	gqDoc, ok := doc.(*goquery.Document)
	if !ok {
		return []map[string]string{}
	}

	var images []map[string]string
	base, err := url.Parse(pageURL)
	if err != nil {
		return images
	}

	// Find ALL img tags
	gqDoc.Find("img").Each(func(i int, s *goquery.Selection) {
		src, exists := s.Attr("src")
		if !exists || src == "" {
			// Check for data-src (lazy loading)
			if dataSrc, hasDataSrc := s.Attr("data-src"); hasDataSrc {
				src = dataSrc
			} else {
				return
			}
		}

		image := map[string]string{
			"url": src,
		}

		// Try to resolve relative URLs
		if parsedURL, err := url.Parse(src); err == nil {
			resolvedURL := base.ResolveReference(parsedURL)
			image["url"] = resolvedURL.String()
		}

		// Get alt text
		if alt, hasAlt := s.Attr("alt"); hasAlt {
			image["alt"] = alt
		} else {
			image["alt"] = "" // Explicitly mark as missing
		}

		// Get additional attributes
		if title, hasTitle := s.Attr("title"); hasTitle {
			image["title"] = title
		}

		if width, hasWidth := s.Attr("width"); hasWidth {
			image["width"] = width
		}

		if height, hasHeight := s.Attr("height"); hasHeight {
			image["height"] = height
		}

		images = append(images, image)
	})

	// Also check for CSS background images
	gqDoc.Find("[style*='background-image']").Each(func(i int, s *goquery.Selection) {
		style, exists := s.Attr("style")
		if !exists {
			return
		}

		// Extract URLs from background-image CSS
		urlRegex := regexp.MustCompile(`background-image:\s*url\(['"]?([^'"()]+)['"]?\)`)
		matches := urlRegex.FindStringSubmatch(style)
		if len(matches) > 1 {
			src := matches[1]

			image := map[string]string{
				"url":  src,
				"alt":  "", // Background images don't have alt text
				"type": "background",
			}

			// Try to resolve relative URLs
			if parsedURL, err := url.Parse(src); err == nil {
				resolvedURL := base.ResolveReference(parsedURL)
				image["url"] = resolvedURL.String()
			}

			images = append(images, image)
		}
	})

	return images
}

// validateLinks checks all links for broken/404 responses
func validateLinks(links []map[string]string, baseURL string, ctx context.Context) *LinkValidationResult {
	result := &LinkValidationResult{
		TotalLinks: len(links),
		BrokenDetails: []BrokenLinkDetail{},
	}

	if len(links) == 0 {
		return result
	}

	// Parse base URL for relative link resolution
	base, err := url.Parse(baseURL)
	if err != nil {
		return result
	}

	// Track unique URLs to avoid duplicate checks
	checkedURLs := make(map[string]bool)
	var wg sync.WaitGroup
	var mu sync.Mutex
	
	// Limit concurrent requests to avoid overwhelming servers
	semaphore := make(chan struct{}, 10)

	for _, link := range links {
		linkURL := link["url"]
		linkText := link["text"]

		// Skip empty URLs, anchors, and non-HTTP(S) links
		if linkURL == "" || strings.HasPrefix(linkURL, "#") || 
		   strings.HasPrefix(linkURL, "mailto:") || 
		   strings.HasPrefix(linkURL, "tel:") ||
		   strings.HasPrefix(linkURL, "javascript:") {
			continue
		}

		// Resolve relative URLs
		resolvedURL, err := base.Parse(linkURL)
		if err != nil {
			mu.Lock()
			result.BrokenLinks++
			result.BrokenDetails = append(result.BrokenDetails, BrokenLinkDetail{
				URL: linkURL,
				Text: linkText,
				Error: "Invalid URL format",
			})
			mu.Unlock()
			continue
		}

		urlString := resolvedURL.String()
		
		// Skip if we've already checked this URL
		mu.Lock()
		if checkedURLs[urlString] {
			mu.Unlock()
			continue
		}
		checkedURLs[urlString] = true
		mu.Unlock()

		// Check if internal or external
		mu.Lock()
		if resolvedURL.Host == base.Host {
			result.InternalLinks++
		} else {
			result.ExternalLinks++
		}
		mu.Unlock()

		// Validate the link concurrently
		wg.Add(1)
		go func(url, text string) {
			defer wg.Done()
			
			// Acquire semaphore
			semaphore <- struct{}{}
			defer func() { <-semaphore }()
			
			if status, err := checkURLStatus(url, ctx); err != nil || status >= 400 {
				mu.Lock()
				result.BrokenLinks++
				result.BrokenDetails = append(result.BrokenDetails, BrokenLinkDetail{
					URL: url,
					Text: text,
					StatusCode: status,
					Error: func() string {
						if err != nil {
							return err.Error()
						}
						return fmt.Sprintf("HTTP %d", status)
					}(),
				})
				mu.Unlock()
			} else {
				mu.Lock()
				result.ValidLinks++
				mu.Unlock()
			}
		}(urlString, linkText)
	}

	// Wait for all link checks to complete
	wg.Wait()

	return result
}

// validateImages checks all images for broken/404 responses and missing alt text
func validateImages(images []map[string]string, ctx context.Context) *ImageValidationResult {
	if len(images) == 0 {
		return &ImageValidationResult{
			TotalImages: 0,
			ValidImages: 0,
			BrokenImages: 0,
			MissingAltText: 0,
		}
	}

	result := &ImageValidationResult{
		TotalImages: len(images),
		BrokenDetails: []BrokenImageDetail{},
	}

	for _, image := range images {
		imageURL := image["url"]
		altText := image["alt"]

		// Check for missing alt text
		if altText == "" {
			result.MissingAltText++
		}

		// Skip data URLs and relative URLs for now
		if strings.HasPrefix(imageURL, "data:") || !strings.HasPrefix(imageURL, "http") {
			continue
		}

		// Validate the image URL
		if status, err := checkURLStatus(imageURL, ctx); err != nil || status >= 400 {
			result.BrokenImages++
			result.BrokenDetails = append(result.BrokenDetails, BrokenImageDetail{
				URL: imageURL,
				AltText: altText,
				StatusCode: status,
				Error: err.Error(),
			})
		} else {
			result.ValidImages++
		}
	}

	return result
}

// checkAccessibility performs basic accessibility checks
func checkAccessibility(images []map[string]string, headings []map[string]string) *AccessibilityResult {
	result := &AccessibilityResult{
		Issues: []string{},
	}

	// Check missing alt tags
	for _, image := range images {
		if image["alt"] == "" {
			result.MissingAltTags++
		}
	}

	// Analyze heading structure
	if len(headings) > 0 {
		result.HeadingStructure = analyzeHeadingStructure(headings)
	}

	// Calculate accessibility score (0-100)
	score := 100
	if result.MissingAltTags > 0 {
		score -= min(result.MissingAltTags*10, 50) // Reduce up to 50 points for missing alt tags
	}

	if len(result.Issues) > 0 {
		score -= len(result.Issues) * 5 // Reduce 5 points per issue
	}

	result.AccessibilityScore = max(score, 0)

	return result
}

// checkSEOBasics performs basic SEO checks
func checkSEOBasics(headData map[string]string, headings []map[string]string) *SEOBasicsResult {
	result := &SEOBasicsResult{
		Issues: []string{},
	}

	// Check title
	title := headData["title"]
	result.HasTitle = title != ""
	result.TitleLength = len(title)

	if !result.HasTitle {
		result.Issues = append(result.Issues, "Missing page title")
	} else if result.TitleLength < 30 {
		result.Issues = append(result.Issues, "Title too short (recommended: 30-60 characters)")
	} else if result.TitleLength > 60 {
		result.Issues = append(result.Issues, "Title too long (recommended: 30-60 characters)")
	}

	// Check meta description
	metaDesc := headData["description"]
	result.HasMetaDescription = metaDesc != ""
	result.MetaDescLength = len(metaDesc)

	if !result.HasMetaDescription {
		result.Issues = append(result.Issues, "Missing meta description")
	} else if result.MetaDescLength < 120 {
		result.Issues = append(result.Issues, "Meta description too short (recommended: 120-160 characters)")
	} else if result.MetaDescLength > 160 {
		result.Issues = append(result.Issues, "Meta description too long (recommended: 120-160 characters)")
	}

	// Check H1 tags
	h1Count := 0
	for _, heading := range headings {
		if heading["level"] == "h1" {
			h1Count++
		}
	}
	result.H1Count = h1Count

	if h1Count == 0 {
		result.Issues = append(result.Issues, "Missing H1 tag")
	} else if h1Count > 1 {
		result.Issues = append(result.Issues, "Multiple H1 tags found (recommended: exactly one)")
	}

	// Check heading order
	result.HeadingOrder = checkHeadingOrder(headings)
	if !result.HeadingOrder {
		result.Issues = append(result.Issues, "Improper heading hierarchy")
	}

	// Calculate SEO score (0-100)
	score := 100
	if !result.HasTitle {
		score -= 25
	}
	if !result.HasMetaDescription {
		score -= 20
	}
	if result.H1Count != 1 {
		score -= 15
	}
	if !result.HeadingOrder {
		score -= 10
	}
	score -= len(result.Issues) * 5

	result.SEOScore = max(score, 0)

	return result
}

// checkURLStatus performs a HEAD request to check URL status
func checkURLStatus(urlStr string, ctx context.Context) (int, error) {
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	req, err := http.NewRequestWithContext(ctx, "HEAD", urlStr, nil)
	if err != nil {
		return 0, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	return resp.StatusCode, nil
}

// analyzeHeadingStructure analyzes the heading hierarchy
func analyzeHeadingStructure(headings []map[string]string) string {
	if len(headings) == 0 {
		return "No headings found"
	}

	structure := make([]string, len(headings))
	for i, heading := range headings {
		structure[i] = heading["level"]
	}

	return strings.Join(structure, " → ")
}

// checkHeadingOrder checks if headings follow proper hierarchy
func checkHeadingOrder(headings []map[string]string) bool {
	if len(headings) == 0 {
		return true
	}

	prevLevel := 0
	for _, heading := range headings {
		level := 1
		switch heading["level"] {
		case "h1":
			level = 1
		case "h2":
			level = 2
		case "h3":
			level = 3
		case "h4":
			level = 4
		case "h5":
			level = 5
		case "h6":
			level = 6
		}

		// Allow going to next level or staying at same level or going back
		// But don't allow skipping levels (e.g., h1 → h3)
		if prevLevel > 0 && level > prevLevel+1 {
			return false
		}
		prevLevel = level
	}

	return true
}

// max helper function
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// performSiteWideQA aggregates QA results from all crawled pages
func performSiteWideQA(allPages []PageContent, qaOptions QAOptions, ctx context.Context) *SiteWideQAResults {
	result := &SiteWideQAResults{
		TotalPagesAnalyzed: len(allPages),
		PerPageResults:     []PageQAResult{},
	}
	
	if len(allPages) == 0 {
		return result
	}
	
	fmt.Printf("[SITE-WIDE QA] Processing %d pages for comprehensive QA analysis\n", len(allPages))
	
	// Initialize aggregated results
	if qaOptions.ValidateLinks {
		result.LinkValidation = &SiteWideLinkResults{
			BrokenLinksByPage: make(map[string][]BrokenLinkDetail),
		}
	}
	
	if qaOptions.CheckImages {
		result.ImageValidation = &SiteWideImageResults{
			BrokenImagesByPage: make(map[string][]BrokenImageDetail),
		}
	}
	
	if qaOptions.Accessibility {
		result.AccessibilityAudit = &SiteWideAccessibility{
			IssuesByPage: make(map[string][]string),
		}
	}
	
	if qaOptions.SEOBasics {
		result.SEOBasics = &SiteWideSEOResults{
			IssuesByPage: make(map[string][]string),
		}
	}
	
	// Track unique URLs for link validation
	uniqueURLsChecked := make(map[string]bool)
	
	// Process each page and aggregate results
	for _, page := range allPages {
		if page.Error != "" {
			fmt.Printf("[SITE-WIDE QA] Skipping page with error: %s\n", page.URL)
			continue
		}
		
		fmt.Printf("[SITE-WIDE QA] Processing page: %s\n", page.URL)
		
		// Create page QA result entry
		pageQAResult := PageQAResult{
			URL:   page.URL,
			Title: page.Title,
		}
		
		// If the page already has QA results (from individual processing), use them
		// Otherwise, we'll need to perform QA analysis now
		if page.QAResults == nil {
			// Perform QA analysis for this page
			// Note: We need to re-fetch the document for QA analysis
			cfg := config.Load()
			collyConfig := crawler.CollyConfig{
				Enabled:            cfg.Colly.Enabled,
				UserAgent:          cfg.Colly.UserAgent,
				Delay:              cfg.Colly.Delay,
				RandomDelay:        cfg.Colly.RandomDelay,
				Parallelism:        cfg.Colly.Parallelism,
				DomainGlob:         cfg.Colly.DomainGlob,
				RespectRobotsTxt:   cfg.Colly.RespectRobotsTxt,
				AllowURLRevisit:    cfg.Colly.AllowURLRevisit,
				CacheDir:           cfg.Colly.CacheDir,
				DebugMode:          cfg.Colly.DebugMode,
				Async:              cfg.Colly.Async,
			}
			
			fetcher := crawler.NewPageFetcherWithBackend(cfg.Colly.Enabled, collyConfig)
			doc, err := fetcher.FetchDocument(page.URL, ctx)
			if err != nil {
				fmt.Printf("[SITE-WIDE QA] Failed to fetch document for %s: %v\n", page.URL, err)
				continue
			}
			
			// Perform QA checks on this page
			page.QAResults = performQAChecks(doc, page, page.URL, qaOptions, ctx)
		}
		
		pageQAResult.QAResults = page.QAResults
		result.PerPageResults = append(result.PerPageResults, pageQAResult)
		
		// Aggregate link validation results
		if qaOptions.ValidateLinks && page.QAResults.LinkValidation != nil {
			lv := page.QAResults.LinkValidation
			result.LinkValidation.TotalLinksFound += lv.TotalLinks
			result.LinkValidation.TotalValidLinks += lv.ValidLinks
			result.LinkValidation.TotalBrokenLinks += lv.BrokenLinks
			result.LinkValidation.TotalExternalLinks += lv.ExternalLinks
			result.LinkValidation.TotalInternalLinks += lv.InternalLinks
			
			// Store broken links by page
			if len(lv.BrokenDetails) > 0 {
				result.LinkValidation.BrokenLinksByPage[page.URL] = lv.BrokenDetails
			}
			
			// Track unique URLs (simplified - we'll count all checked URLs)
			for _, broken := range lv.BrokenDetails {
				uniqueURLsChecked[broken.URL] = true
			}
		}
		
		// Aggregate image validation results
		if qaOptions.CheckImages && page.QAResults.ImageValidation != nil {
			iv := page.QAResults.ImageValidation
			result.ImageValidation.TotalImagesFound += iv.TotalImages
			result.ImageValidation.TotalValidImages += iv.ValidImages
			result.ImageValidation.TotalBrokenImages += iv.BrokenImages
			result.ImageValidation.TotalMissingAltText += iv.MissingAltText
			
			// Store broken images by page
			if len(iv.BrokenDetails) > 0 {
				result.ImageValidation.BrokenImagesByPage[page.URL] = iv.BrokenDetails
			}
		}
		
		// Aggregate accessibility results
		if qaOptions.Accessibility && page.QAResults.AccessibilityAudit != nil {
			acc := page.QAResults.AccessibilityAudit
			result.AccessibilityAudit.TotalMissingAltTags += acc.MissingAltTags
			result.AccessibilityAudit.TotalMissingAriaLabels += acc.MissingAriaLabels
			
			// Store issues by page
			if len(acc.Issues) > 0 {
				result.AccessibilityAudit.IssuesByPage[page.URL] = acc.Issues
			}
		}
		
		// Aggregate SEO results
		if qaOptions.SEOBasics && page.QAResults.SEOBasics != nil {
			seo := page.QAResults.SEOBasics
			if seo.HasTitle {
				result.SEOBasics.PagesWithTitle++
			}
			if seo.HasMetaDescription {
				result.SEOBasics.PagesWithMetaDesc++
			}
			if seo.H1Count == 1 {
				result.SEOBasics.PagesWithProperH1++
			}
			
			// Store issues by page
			if len(seo.Issues) > 0 {
				result.SEOBasics.IssuesByPage[page.URL] = seo.Issues
			}
		}
	}
	
	// Calculate overall scores
	if qaOptions.ValidateLinks && result.LinkValidation != nil {
		result.LinkValidation.UniqueURLsChecked = len(uniqueURLsChecked)
	}
	
	if qaOptions.Accessibility && result.AccessibilityAudit != nil {
		// Calculate overall accessibility score
		totalIssues := result.AccessibilityAudit.TotalMissingAltTags + result.AccessibilityAudit.TotalMissingAriaLabels
		for _, issues := range result.AccessibilityAudit.IssuesByPage {
			totalIssues += len(issues)
		}
		
		if totalIssues == 0 {
			result.AccessibilityAudit.OverallScore = 100
		} else {
			// Simple scoring: start at 100, deduct points for issues
			score := 100 - min(totalIssues*5, 100)
			result.AccessibilityAudit.OverallScore = max(score, 0)
		}
	}
	
	if qaOptions.SEOBasics && result.SEOBasics != nil {
		// Calculate overall SEO score
		totalPages := result.TotalPagesAnalyzed
		if totalPages > 0 {
			titleScore := (result.SEOBasics.PagesWithTitle * 100) / totalPages
			metaScore := (result.SEOBasics.PagesWithMetaDesc * 100) / totalPages
			h1Score := (result.SEOBasics.PagesWithProperH1 * 100) / totalPages
			
			// Average the scores
			result.SEOBasics.OverallSEOScore = (titleScore + metaScore + h1Score) / 3
		}
	}
	
	fmt.Printf("[SITE-WIDE QA] Site-wide analysis completed\n")
	if result.LinkValidation != nil {
		fmt.Printf("[SITE-WIDE QA] Total links found: %d, valid: %d, broken: %d\n", 
			result.LinkValidation.TotalLinksFound,
			result.LinkValidation.TotalValidLinks,
			result.LinkValidation.TotalBrokenLinks)
	}
	
	return result
}