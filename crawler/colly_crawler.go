package crawler

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gocolly/colly/v2"
	"github.com/gocolly/colly/v2/debug"
)

type CollyCrawler struct {
	collector    *colly.Collector
	urlDiscovery *colly.Collector
	foundURLs    map[string]bool
	mutex        sync.RWMutex
	baseURL      *url.URL
	config       CollyConfig
}

type CollyConfig struct {
	Enabled            bool
	UserAgent          string
	Delay              time.Duration
	RandomDelay        time.Duration
	Parallelism        int
	DomainGlob         string
	RespectRobotsTxt   bool
	AllowURLRevisit    bool
	CacheDir           string
	DebugMode          bool
	Async              bool
	CacheEnabled       bool
	CacheTTL           time.Duration
}

func NewCollyCrawler(config CollyConfig) *CollyCrawler {
	// Create main collector for URL discovery
	c := colly.NewCollector(
		colly.AllowedDomains(), // Will be set dynamically per request
	)

	// Configure user agent
	c.UserAgent = config.UserAgent

	// Set up rate limiting
	if config.Delay > 0 || config.Parallelism > 0 {
		c.Limit(&colly.LimitRule{
			DomainGlob:  config.DomainGlob,
			Parallelism: config.Parallelism,
			Delay:       config.Delay,
			RandomDelay: config.RandomDelay,
		})
	}

	// Set up caching if enabled (disabled for now due to import issues)
	if config.CacheDir != "" {
		// TODO: Add caching support if needed
		fmt.Printf("Note: Caching not implemented yet\n")
	}

	// Set up debugging if enabled
	if config.DebugMode {
		c.SetDebugger(&debug.LogDebugger{})
	}

	// Enable async mode if requested
	if config.Async {
		c.Async = true
	}

	// Disable URL revisiting by default unless explicitly allowed
	if !config.AllowURLRevisit {
		c.AllowURLRevisit = false
	}

	return &CollyCrawler{
		collector: c,
		foundURLs: make(map[string]bool),
		config:    config,
	}
}

// DiscoverURLsWithColly discovers URLs using Colly with all existing advanced patterns
func (cc *CollyCrawler) DiscoverURLsWithColly(targetURL string, baseURL *url.URL, ctx context.Context) ([]string, error) {
	cc.baseURL = baseURL
	cc.foundURLs = make(map[string]bool)

	// Set allowed domains dynamically
	cc.collector.AllowedDomains = []string{baseURL.Host}
	if strings.HasPrefix(baseURL.Host, "www.") {
		// Also allow non-www version
		cc.collector.AllowedDomains = append(cc.collector.AllowedDomains, baseURL.Host[4:])
	} else {
		// Also allow www version
		cc.collector.AllowedDomains = append(cc.collector.AllowedDomains, "www."+baseURL.Host)
	}

	// Set up comprehensive URL discovery using all existing patterns
	cc.setupURLDiscoveryCallbacks()

	// Set up error handling
	cc.collector.OnError(func(r *colly.Response, err error) {
		fmt.Printf("Colly error on %s: %v\n", r.Request.URL, err)
	})

	// Start crawling
	err := cc.collector.Visit(targetURL)
	if err != nil {
		return nil, fmt.Errorf("failed to start Colly crawling: %v", err)
	}

	// Wait for all requests to complete if async
	if cc.config.Async {
		cc.collector.Wait()
	}

	// Apply existing advanced discovery methods
	urls := cc.getFoundURLs()
	
	// Use existing advanced discovery functions to enhance results
	urls = cc.enhanceWithAdvancedDiscovery(urls, baseURL, ctx)

	return urls, nil
}

func (cc *CollyCrawler) setupURLDiscoveryCallbacks() {
	// All the comprehensive selectors from the original discovery.go
	urlSources := []struct {
		selector string
		attr     string
		context  string
	}{
		// Primary navigation sources
		{"a[href]", "href", "links"},
		{"form[action]", "action", "forms"},
		{"link[href]", "href", "link-tags"},
		{"area[href]", "href", "image-maps"},
		{"base[href]", "href", "base-tags"},
		
		// Navigation and menu structures
		{"nav a[href]", "href", "navigation"},
		{"header a[href]", "href", "header"},
		{"footer a[href]", "href", "footer"},
		{"aside a[href]", "href", "sidebar"},
		
		// CRITICAL: Webflow dropdown menus (even if hidden)
		{".w-dropdown a[href]", "href", "webflow-dropdown"},
		{".dropdown-wrapper a[href]", "href", "dropdown-wrapper"},
		{".w-dropdown-list a[href]", "href", "dropdown-list"},
		{".w-dropdown-link[href]", "href", "dropdown-link"},
		{".w-dropdown-toggle + .w-dropdown-list a[href]", "href", "webflow-toggle-list"},
		{".w-dropdown-nav a[href]", "href", "webflow-nav"},
		{"nav.dropdown-wrapper a[href]", "href", "nav-dropdown-wrapper"},
		{"[style*='opacity: 0'] a[href]", "href", "hidden-opacity"},
		{"[style*='display: none'] a[href]", "href", "hidden-display"},
		{"[style*='visibility: hidden'] a[href]", "href", "hidden-visibility"},
		{".nav-dropdown a[href]", "href", "nav-dropdown"},
		{".navbar-dropdown a[href]", "href", "navbar-dropdown"},
		
		// Language switchers (common in multilingual sites)
		{".language-selector a[href]", "href", "language-selector"},
		{".lang-switch a[href]", "href", "lang-switch"},
		{".locale-nav a[href]", "href", "locale-nav"},
		
		// Menu and navigation classes (common patterns)
		{".menu a[href]", "href", "menu-class"},
		{".nav a[href]", "href", "nav-class"},
		{".navigation a[href]", "href", "navigation-class"},
		{".navbar a[href]", "href", "navbar-class"},
		{".main-nav a[href]", "href", "main-nav-class"},
		{".primary-nav a[href]", "href", "primary-nav-class"},
		{".secondary-nav a[href]", "href", "secondary-nav-class"},
		{".breadcrumb a[href]", "href", "breadcrumb"},
		{".breadcrumbs a[href]", "href", "breadcrumbs"},
		
		// Sitemap and directory structures
		{".sitemap a[href]", "href", "sitemap-class"},
		{".site-map a[href]", "href", "site-map-class"},
		{"#sitemap a[href]", "href", "sitemap-id"},
		{".directory a[href]", "href", "directory"},
		
		// Content area links
		{"main a[href]", "href", "main-content"},
		{"article a[href]", "href", "articles"},
		{"section a[href]", "href", "sections"},
		{".content a[href]", "href", "content-class"},
		{".post a[href]", "href", "posts"},
		{".page a[href]", "href", "pages"},
		
		// List structures (often contain navigation)
		{"ul a[href]", "href", "unordered-lists"},
		{"ol a[href]", "href", "ordered-lists"},
		{"dl a[href]", "href", "definition-lists"},
		
		// Button and CTA links
		{".button[href]", "href", "button-class"},
		{".btn[href]", "href", "btn-class"},
		{".cta[href]", "href", "cta-class"},
		{".call-to-action[href]", "href", "call-to-action"},
		
		// Language and localization
		{".language a[href]", "href", "language-switcher"},
		{".lang a[href]", "href", "lang-switcher"},
		{".locale a[href]", "href", "locale-switcher"},
		
		// Pagination
		{".pagination a[href]", "href", "pagination"},
		{".pager a[href]", "href", "pager"},
		{".page-numbers a[href]", "href", "page-numbers"},
		
		// Social and external (but we'll filter by domain)
		{".social a[href]", "href", "social-links"},
		
		// Generic containers that might have links
		{"div[class*='nav'] a[href]", "href", "nav-divs"},
		{"div[class*='menu'] a[href]", "href", "menu-divs"},
		{"div[id*='nav'] a[href]", "href", "nav-id-divs"},
		{"div[id*='menu'] a[href]", "href", "menu-id-divs"},
	}

	// Set up callbacks for each selector
	for _, source := range urlSources {
		selector := source.selector
		attr := source.attr
		context := source.context

		cc.collector.OnHTML(selector, func(e *colly.HTMLElement) {
			href := e.Attr(attr)
			if href == "" {
				return
			}

			// Skip non-URL values
			if shouldSkipURL(href) {
				return
			}

			// Resolve URL
			absoluteURL := e.Request.AbsoluteURL(href)
			parsedURL, err := url.Parse(absoluteURL)
			if err != nil {
				return
			}

			// Clean URL (remove fragments and query params like original)
			cleanURL := *parsedURL
			cleanURL.Fragment = ""
			cleanURL.RawQuery = ""
			cleanURLString := cleanURL.String()

			// Check if it's a page URL
			if isPageURL(cleanURLString) {
				cc.addFoundURL(cleanURLString, context)
				fmt.Printf("🔗 Colly found URL via %s: %s\n", context, cleanURLString)
			}
		})
	}

	// Set up callbacks for meta tags and language alternates
	cc.setupMetaTagCallbacks()
	cc.setupLanguageAlternateCallbacks()
}

func (cc *CollyCrawler) setupMetaTagCallbacks() {
	metaSelectors := []string{
		"meta[property='og:url']",
		"meta[name='twitter:url']",
		"meta[property='al:web:url']",
		"meta[property='og:image']",
		"meta[name='twitter:image']",
		"meta[http-equiv='refresh']",
		"meta[property='article:author']",
		"meta[name='canonical']",
		"meta[property='og:video']",
	}

	for _, selector := range metaSelectors {
		cc.collector.OnHTML(selector, func(e *colly.HTMLElement) {
			content := e.Attr("content")
			if content == "" {
				return
			}

			absoluteURL := e.Request.AbsoluteURL(content)
			if absoluteURL != "" && isPageURL(absoluteURL) {
				cc.addFoundURL(absoluteURL, "meta-tags")
			}
		})
	}
}

func (cc *CollyCrawler) setupLanguageAlternateCallbacks() {
	languageAlternates := []string{
		"link[rel='alternate'][hreflang]",
		"link[rel='canonical']",
		"link[rel='alternate'][type='application/rss+xml']",
	}

	for _, selector := range languageAlternates {
		cc.collector.OnHTML(selector, func(e *colly.HTMLElement) {
			href := e.Attr("href")
			if href == "" || shouldSkipURL(href) {
				return
			}

			absoluteURL := e.Request.AbsoluteURL(href)
			if absoluteURL != "" && isPageURL(absoluteURL) {
				hreflang := e.Attr("hreflang")
				context := "alternate-links"
				if hreflang != "" {
					context = fmt.Sprintf("hreflang-%s", hreflang)
				}
				cc.addFoundURL(absoluteURL, context)
				fmt.Printf("🌍 Colly found language URL (%s): %s\n", context, absoluteURL)
			}
		})
	}
}

func (cc *CollyCrawler) addFoundURL(urlStr, context string) {
	cc.mutex.Lock()
	defer cc.mutex.Unlock()
	cc.foundURLs[urlStr] = true
}

func (cc *CollyCrawler) getFoundURLs() []string {
	cc.mutex.RLock()
	defer cc.mutex.RUnlock()
	
	var urls []string
	for urlStr := range cc.foundURLs {
		urls = append(urls, urlStr)
	}
	return urls
}

func (cc *CollyCrawler) enhanceWithAdvancedDiscovery(initialURLs []string, baseURL *url.URL, ctx context.Context) []string {
	// Convert to map for processing
	urlSet := make(map[string]bool)
	for _, urlStr := range initialURLs {
		urlSet[urlStr] = true
	}

	fmt.Printf("Colly initial discovery: %d URLs\n", len(urlSet))

	// Apply existing advanced discovery methods
	
	// Add XML sitemap URLs
	fmt.Printf("Checking for XML sitemaps...\n")
	sitemapURLs := discoverSitemapURLs(baseURL, ctx)
	for _, sitemapURL := range sitemapURLs {
		urlSet[sitemapURL] = true
	}
	fmt.Printf("Total URLs after XML sitemap discovery: %d\n", len(urlSet))

	// Generate smart language URL patterns
	fmt.Printf("Generating smart language URL patterns...\n")
	patternURLs := GenerateLanguageURLPatterns(urlSet, baseURL)
	for _, patternURL := range patternURLs {
		urlSet[patternURL] = true
	}
	fmt.Printf("Total URLs after smart language pattern generation: %d\n", len(urlSet))

	// Recursively discover more pages in language folders
	fmt.Printf("Recursively discovering more pages in language folders...\n")
	recursiveURLs := DiscoverRecursiveLanguagePages(urlSet, baseURL, ctx)
	for _, recursiveURL := range recursiveURLs {
		urlSet[recursiveURL] = true
	}
	fmt.Printf("Total URLs after recursive language discovery: %d\n", len(urlSet))

	// Final cleanup
	fmt.Printf("🧹 Final cleanup: removing asset URLs...\n")
	var finalURLs []string
	for urlStr := range urlSet {
		if isPageURL(urlStr) {
			finalURLs = append(finalURLs, urlStr)
		}
	}
	fmt.Printf("URLs after final cleanup: %d\n", len(finalURLs))

	return finalURLs
}