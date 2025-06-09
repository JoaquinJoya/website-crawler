package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"

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
	Error       string              `json:"error,omitempty"`
}

type CrawlResponse struct {
	URL   string        `json:"url"`
	URLs  []string      `json:"urls"`
	Pages []PageContent `json:"pages"`
	Count int           `json:"count"`
	Error string        `json:"error,omitempty"`
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

	s.streamPageContents(c, urls, ctx, options, aiConfig, targetSelector, maxPages)
}

func (s *Server) streamPageContents(c *gin.Context, urls []string, ctx context.Context, options ContentOptions, aiConfig AIConfig, targetSelector *TargetSelector, maxPages int) {
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
				page = fetchPageContent(pageURL, pageCtx, options, aiConfig, targetSelector, fetcher)
				
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
			Head: true, HTML: true, Text: true,
		}, AIConfig{}, nil, fetcher)
		pages = append(pages, page)
	}
	
	return pages
}

func fetchPageContent(pageURL string, ctx context.Context, options ContentOptions, aiConfig AIConfig, targetSelector *TargetSelector, fetcher crawler.PageFetcherInterface) PageContent {
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
		content := ai.PrepareContentForAI(page.Title, page.URL, page.Content, page.Headings, page.Paragraphs, page.Links, page.Images)
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
		content := ai.PrepareContentForAI(page.Title, page.URL, page.Content, page.Headings, page.Paragraphs, page.Links, page.Images)
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