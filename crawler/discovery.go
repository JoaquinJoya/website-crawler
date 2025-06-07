package crawler

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"github.com/PuerkitoBio/goquery"
)

type URLSource struct {
	Selector string
	Attr     string
	Context  string
}

// DiscoverURLsWithBackend allows choosing between manual and Colly implementations
func DiscoverURLsWithBackend(targetURL string, baseURL *url.URL, ctx context.Context, useColly bool, collyConfig CollyConfig) ([]string, error) {
	if useColly {
		return DiscoverURLsWithColly(targetURL, baseURL, ctx, collyConfig)
	}
	return DiscoverURLs(targetURL, baseURL, ctx)
}

// DiscoverURLsWithColly uses Colly for URL discovery
func DiscoverURLsWithColly(targetURL string, baseURL *url.URL, ctx context.Context, collyConfig CollyConfig) ([]string, error) {
	crawler := NewCollyCrawler(collyConfig)
	return crawler.DiscoverURLsWithColly(targetURL, baseURL, ctx)
}

func DiscoverURLs(targetURL string, baseURL *url.URL, ctx context.Context) ([]string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", targetURL, nil)
	if err != nil {
		return nil, err
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, err
	}

	urlChan := make(chan string, 100)
	var wg sync.WaitGroup

	urlSources := []URLSource{
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

	// CRITICAL: Check for language alternate links (hreflang)
	languageAlternates := []string{
		"link[rel='alternate'][hreflang]",
		"link[rel='canonical']", 
		"link[rel='alternate'][type='application/rss+xml']",
	}

	for _, source := range urlSources {
		doc.Find(source.Selector).Each(func(i int, s *goquery.Selection) {
			wg.Add(1)
			go func(sel *goquery.Selection, attribute, context string) {
				defer wg.Done()

				href, exists := sel.Attr(attribute)
				if !exists || href == "" {
					return
				}

				if shouldSkipURL(href) {
					return
				}

				parsedURL, err := url.Parse(href)
				if err != nil {
					return
				}

				resolvedURL := baseURL.ResolveReference(parsedURL)
				
				cleanURL := *resolvedURL
				cleanURL.Fragment = ""
				cleanURL.RawQuery = ""
				
				if isSameDomain(&cleanURL, baseURL) {
					cleanURLString := cleanURL.String()
					select {
					case urlChan <- cleanURLString:
					case <-ctx.Done():
						return
					}
				}
			}(s, source.Attr, source.Context)
		})
	}

	for _, metaSelector := range metaSelectors {
		doc.Find(metaSelector).Each(func(i int, s *goquery.Selection) {
			wg.Add(1)
			go func(sel *goquery.Selection) {
				defer wg.Done()

				content, exists := sel.Attr("content")
				if !exists || content == "" {
					return
				}

				parsedURL, err := url.Parse(content)
				if err != nil {
					return
				}

				resolvedURL := baseURL.ResolveReference(parsedURL)

				if isSameDomain(resolvedURL, baseURL) {
					select {
					case urlChan <- resolvedURL.String():
					case <-ctx.Done():
						return
					}
				}
			}(s)
		})
	}
	
	for _, altSelector := range languageAlternates {
		doc.Find(altSelector).Each(func(i int, s *goquery.Selection) {
			wg.Add(1)
			go func(sel *goquery.Selection) {
				defer wg.Done()

				href, exists := sel.Attr("href")
				if !exists || href == "" {
					return
				}

				if shouldSkipURL(href) {
					return
				}

				parsedURL, err := url.Parse(href)
				if err != nil {
					return
				}

				resolvedURL := baseURL.ResolveReference(parsedURL)
				
				cleanURL := *resolvedURL
				cleanURL.Fragment = ""
				cleanURL.RawQuery = ""

				if isSameDomain(&cleanURL, baseURL) {
					cleanURLString := cleanURL.String()
					select {
					case urlChan <- cleanURLString:
					case <-ctx.Done():
						return
					}
				}
			}(s)
		})
	}

	go func() {
		wg.Wait()
		close(urlChan)
	}()

	urlSet := make(map[string]bool)
	for urlStr := range urlChan {
		urlSet[urlStr] = true
	}

	// Add XML sitemap URLs
	fmt.Printf("Checking for XML sitemaps...\n")
	sitemapURLs := discoverSitemapURLs(baseURL, ctx)
	for _, sitemapURL := range sitemapURLs {
		urlSet[sitemapURL] = true
	}
	fmt.Printf("Total URLs after XML sitemap discovery: %d\n", len(urlSet))

	// Add HTML sitemap URLs
	fmt.Printf("Scanning for HTML sitemaps...\n")
	htmlSitemapURLs := ExtractHTMLSitemapURLs(doc, baseURL)
	for _, htmlURL := range htmlSitemapURLs {
		urlSet[htmlURL] = true
	}
	fmt.Printf("Total URLs after HTML sitemap discovery: %d\n", len(urlSet))

	// Extract URLs from JavaScript content
	fmt.Printf("Extracting URLs from JavaScript content...\n")
	jsURLs := ExtractJavaScriptURLs(doc, baseURL)
	for _, jsURL := range jsURLs {
		urlSet[jsURL] = true
	}
	fmt.Printf("Total URLs after JavaScript extraction: %d\n", len(urlSet))

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

	// Final cleanup: Remove any asset URLs that slipped through
	fmt.Printf("🧹 Final cleanup: removing asset URLs...\n")
	var urls []string
	for urlStr := range urlSet {
		if isPageURL(urlStr) {
			urls = append(urls, urlStr)
		} else {
			fmt.Printf("🚫 Final cleanup removed: %s\n", urlStr)
		}
	}
	fmt.Printf("URLs after final cleanup: %d\n", len(urls))

	// Debug: Show all discovered URLs
	fmt.Printf("🔍 DEBUG: All discovered URLs:\n")
	for _, discoveredURL := range urls {
		fmt.Printf("  📄 %s\n", discoveredURL)
	}

	return urls, nil
}

func shouldSkipURL(href string) bool {
	if strings.HasPrefix(href, "#") || strings.HasPrefix(href, "mailto:") || 
	   strings.HasPrefix(href, "tel:") || strings.HasPrefix(href, "javascript:") ||
	   strings.HasPrefix(href, "data:") || strings.HasPrefix(href, "ftp:") ||
	   strings.Contains(href, "void(0)") {
		return true
	}
	
	if strings.Contains(href, ".") {
		lastDot := strings.LastIndex(href, ".")
		lastSlash := strings.LastIndex(href, "/")
		
		if lastDot > lastSlash {
			ext := strings.ToLower(href[lastDot:])
			if queryIndex := strings.Index(ext, "?"); queryIndex != -1 {
				ext = ext[:queryIndex]
			}
			
			skipExtensions := []string{
				".jpg", ".jpeg", ".png", ".gif", ".svg", ".webp", ".bmp", ".ico", ".tiff", ".avif",
				".mp4", ".avi", ".mov", ".wmv", ".flv", ".webm", ".mkv", ".m4v",
				".mp3", ".wav", ".ogg", ".aac", ".flac", ".m4a",
				".zip", ".rar", ".tar", ".gz", ".7z", ".bz2",
				".woff", ".woff2", ".ttf", ".eot", ".otf",
				".css", ".js", ".json", ".xml", ".txt",
				".pdf", ".doc", ".docx", ".xls", ".xlsx", ".ppt", ".pptx",
			}
			
			for _, skipExt := range skipExtensions {
				if ext == skipExt {
					return true
				}
			}
		}
	}
	
	lowerHref := strings.ToLower(href)
	assetPatterns := []string{
		"/js/", "/css/", "/assets/", "/static/", "/files/",
		"javascript", "/api/", "/webhook", "/callback",
		"text/javascript", "text/css", "application/json",
	}
	
	for _, pattern := range assetPatterns {
		if strings.Contains(lowerHref, pattern) {
			return true
		}
	}
	
	return false
}

func isPageURL(urlString string) bool {
	lowerURL := strings.ToLower(urlString)
	
	if strings.Contains(urlString, ".") {
		lastDot := strings.LastIndex(urlString, ".")
		lastSlash := strings.LastIndex(urlString, "/")
		
		if lastDot > lastSlash {
			ext := strings.ToLower(urlString[lastDot:])
			if queryIndex := strings.Index(ext, "?"); queryIndex != -1 {
				ext = ext[:queryIndex]
			}
			
			assetExtensions := []string{
				".js", ".css", ".json", ".xml", ".txt",
				".jpg", ".jpeg", ".png", ".gif", ".svg", ".webp", ".bmp", ".ico", ".tiff", ".avif",
				".mp4", ".avi", ".mov", ".wmv", ".flv", ".webm", ".mkv", ".m4v",
				".mp3", ".wav", ".ogg", ".aac", ".flac", ".m4a",
				".zip", ".rar", ".tar", ".gz", ".7z", ".bz2",
				".woff", ".woff2", ".ttf", ".eot", ".otf",
				".pdf", ".doc", ".docx", ".xls", ".xlsx", ".ppt", ".pptx",
			}
			
			for _, assetExt := range assetExtensions {
				if ext == assetExt {
					return false
				}
			}
		}
	}
	
	assetPatterns := []string{
		"/js/", "/css/", "/assets/", "/static/", "/files/",
		"javascript", "/api/", "/webhook", "/callback",
		"text/javascript", "text/css", "application/json",
		"/packs/", "/dist/", "/build/", "/node_modules/",
	}
	
	for _, pattern := range assetPatterns {
		if strings.Contains(lowerURL, pattern) {
			return false
		}
	}
	
	return true
}

func discoverSitemapURLs(baseURL *url.URL, ctx context.Context) []string {
	var urls []string
	sitemapPaths := []string{"/sitemap.xml", "/sitemap.txt", "/sitemap_index.xml"}
	
	client := &http.Client{}
	
	for _, path := range sitemapPaths {
		sitemapURL := baseURL.ResolveReference(&url.URL{Path: path})
		
		req, err := http.NewRequestWithContext(ctx, "GET", sitemapURL.String(), nil)
		if err != nil {
			continue
		}
		
		resp, err := client.Do(req)
		if err != nil || resp.StatusCode != 200 {
			if resp != nil {
				resp.Body.Close()
			}
			continue
		}
		defer resp.Body.Close()
		
		doc, err := goquery.NewDocumentFromReader(resp.Body)
		if err != nil {
			continue
		}
		
		doc.Find("loc").Each(func(i int, s *goquery.Selection) {
			urlText := strings.TrimSpace(s.Text())
			if urlText != "" {
				if parsedURL, err := url.Parse(urlText); err == nil {
					if isSameDomain(parsedURL, baseURL) {
						urls = append(urls, urlText)
					}
				}
			}
		})
		
		fmt.Printf("Found %d URLs from sitemap: %s\n", len(urls), sitemapURL.String())
	}
	
	return urls
}

func isSameDomain(urlToCheck, baseURL *url.URL) bool {
	if urlToCheck.Scheme != "http" && urlToCheck.Scheme != "https" {
		return false
	}
	
	checkHost := strings.ToLower(urlToCheck.Host)
	baseHost := strings.ToLower(baseURL.Host)
	
	if strings.Contains(checkHost, ":") {
		checkHost = strings.Split(checkHost, ":")[0]
	}
	if strings.Contains(baseHost, ":") {
		baseHost = strings.Split(baseHost, ":")[0]
	}
	
	cdnPatterns := []string{
		"fonts.googleapis.com", "fonts.gstatic.com",
		"googletagmanager.com", "google-analytics.com",
		"facebook.com", "twitter.com", "instagram.com",
	}
	
	for _, pattern := range cdnPatterns {
		if strings.Contains(checkHost, pattern) {
			return false
		}
	}
	
	if checkHost == baseHost {
		return true
	}
	
	if strings.HasPrefix(checkHost, "www.") && baseHost == checkHost[4:] {
		return true
	}
	if strings.HasPrefix(baseHost, "www.") && checkHost == baseHost[4:] {
		return true
	}
	
	return false
}