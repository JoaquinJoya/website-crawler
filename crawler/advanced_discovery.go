package crawler

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// ExtractHTMLSitemapURLs extracts URLs from HTML sitemap structures
func ExtractHTMLSitemapURLs(doc *goquery.Document, baseURL *url.URL) []string {
	var urls []string
	
	sitemapSelectors := []string{
		"ul.sitemap_list a[href]",
		"ul[role='list'] a[href]",
		".sitemap a[href]",
		".site-map a[href]",
		"#sitemap a[href]",
		"nav.sitemap a[href]",
		"footer a[href]",
		"ul li a[href]",
	}
	
	foundUrls := make(map[string]bool)
	
	for _, selector := range sitemapSelectors {
		doc.Find(selector).Each(func(i int, s *goquery.Selection) {
			href, exists := s.Attr("href")
			if !exists || href == "" || href == "#" {
				return
			}
			
			if strings.HasPrefix(href, "mailto:") || strings.HasPrefix(href, "tel:") || 
			   strings.HasPrefix(href, "javascript:") {
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
				if !foundUrls[cleanURLString] {
					foundUrls[cleanURLString] = true
					urls = append(urls, cleanURLString)
					fmt.Printf("📋 Found HTML sitemap URL: %s (from %s)\n", cleanURLString, selector)
				}
			}
		})
	}
	
	fmt.Printf("HTML sitemap extraction found %d URLs\n", len(urls))
	return urls
}

// ExtractJavaScriptURLs extracts URLs from JavaScript content
func ExtractJavaScriptURLs(doc *goquery.Document, baseURL *url.URL) []string {
	var urls []string
	foundUrls := make(map[string]bool)
	
	doc.Find("script").Each(func(i int, s *goquery.Selection) {
		scriptContent := s.Text()
		if scriptContent == "" {
			return
		}
		
		urlPatterns := []string{
			`["']([^"']*(?:\.html|\.php|\.asp|\.jsp|/[^"'?\s#]*))["']`,
			`["'](/[^"'?\s#]*)["']`,
			`["'](/\w+[^"'?\s#]*)["']`,
			`href\s*[:=]\s*["']([^"'#]+)["']`,
			`url\s*[:=]\s*["']([^"'#]+)["']`,
			`link\s*[:=]\s*["']([^"'#]+)["']`,
			`route\s*[:=]\s*["']([^"'#]+)["']`,
			`path\s*[:=]\s*["']([^"'#]+)["']`,
		}
		
		for _, pattern := range urlPatterns {
			re := regexp.MustCompile(pattern)
			matches := re.FindAllStringSubmatch(scriptContent, -1)
			
			for _, match := range matches {
				if len(match) > 1 {
					href := match[1]
					
					if strings.HasPrefix(href, "data:") || strings.HasPrefix(href, "blob:") ||
					   strings.HasPrefix(href, "javascript:") || strings.Contains(href, "void(0)") ||
					   len(href) < 2 {
						continue
					}
					
					if parsedURL, err := url.Parse(href); err == nil {
						resolvedURL := baseURL.ResolveReference(parsedURL)
						
						cleanURL := *resolvedURL
						cleanURL.Fragment = ""
						cleanURL.RawQuery = ""
						
						if isSameDomain(&cleanURL, baseURL) {
							cleanURLString := cleanURL.String()
							if !foundUrls[cleanURLString] {
								foundUrls[cleanURLString] = true
								urls = append(urls, cleanURLString)
								fmt.Printf("🔧 Found JS URL: %s\n", cleanURLString)
							}
						}
					}
				}
			}
		}
	})
	
	fmt.Printf("JavaScript extraction found %d URLs\n", len(urls))
	return urls
}

// GenerateLanguageURLPatterns intelligently detects existing languages and generates patterns
func GenerateLanguageURLPatterns(discoveredURLs map[string]bool, baseURL *url.URL) []string {
	var newURLs []string
	foundUrls := make(map[string]bool)
	
	// Detect which language prefixes actually exist in discovered URLs
	detectedLanguages := make(map[string]bool)
	
	for discoveredURL := range discoveredURLs {
		if parsedURL, err := url.Parse(discoveredURL); err == nil {
			path := parsedURL.Path
			// Check if path starts with a language prefix pattern (/xx/ where xx is 2-3 letters)
			if len(path) > 3 && path[0] == '/' && path[3] == '/' {
				langPrefix := path[:3] // e.g., "/es"
				detectedLanguages[langPrefix] = true
				fmt.Printf("🌍 Detected language prefix: %s\n", langPrefix)
			}
		}
	}
	
	if len(detectedLanguages) == 0 {
		fmt.Printf("🌍 No language prefixes detected, skipping pattern generation\n")
		return newURLs
	}
	
	var languagePrefixes []string
	for langPrefix := range detectedLanguages {
		languagePrefixes = append(languagePrefixes, langPrefix)
	}
	
	fmt.Printf("🌍 Generating patterns for %d detected languages: %v\n", len(languagePrefixes), languagePrefixes)
	
	for discoveredURL := range discoveredURLs {
		parsedURL, err := url.Parse(discoveredURL)
		if err != nil {
			continue
		}
		
		path := parsedURL.Path
		
		// For each discovered URL, try language prefix variations
		for _, langPrefix := range languagePrefixes {
			if strings.HasPrefix(path, langPrefix+"/") {
				continue
			}
			
			// Try adding language prefix to existing path
			newPath := langPrefix + path
			newURL := baseURL.Scheme + "://" + baseURL.Host + newPath
			
			if !discoveredURLs[newURL] && !foundUrls[newURL] {
				foundUrls[newURL] = true
				newURLs = append(newURLs, newURL)
				fmt.Printf("🌍 Generated language URL: %s\n", newURL)
			}
			
			// If this is a root or simple path, try essential page patterns
			if path == "/" || path == "" {
				commonPages := []string{
					"/about", "/contact", "/services", "/blog",
				}
				
				for _, commonPage := range commonPages {
					newPath := langPrefix + commonPage
					newURL := baseURL.Scheme + "://" + baseURL.Host + newPath
					
					if !discoveredURLs[newURL] && !foundUrls[newURL] {
						foundUrls[newURL] = true
						newURLs = append(newURLs, newURL)
						fmt.Printf("🌍 Generated common page URL: %s\n", newURL)
					}
				}
			}
		}
		
		// Try removing language prefix if it exists (to find base version)
		for _, langPrefix := range languagePrefixes {
			if strings.HasPrefix(path, langPrefix+"/") {
				basePath := strings.TrimPrefix(path, langPrefix)
				newURL := baseURL.Scheme + "://" + baseURL.Host + basePath
				
				if !discoveredURLs[newURL] && !foundUrls[newURL] {
					foundUrls[newURL] = true
					newURLs = append(newURLs, newURL)
					fmt.Printf("🌍 Generated base URL (removed %s): %s\n", langPrefix, newURL)
				}
				break
			}
		}
	}
	
	fmt.Printf("Generated %d new language pattern URLs\n", len(newURLs))
	return newURLs
}

// DiscoverRecursiveLanguagePages crawls successful language pages to find more pages
func DiscoverRecursiveLanguagePages(existingURLs map[string]bool, baseURL *url.URL, ctx context.Context) []string {
	var newURLs []string
	foundUrls := make(map[string]bool)
	
	var languagePages []string
	for urlString := range existingURLs {
		if parsedURL, err := url.Parse(urlString); err == nil {
			path := parsedURL.Path
			if len(path) >= 3 && path[0] == '/' && (len(path) == 3 || path[3] == '/') {
				langCode := path[1:3]
				if len(langCode) == 2 && isAlpha(langCode) {
					languagePages = append(languagePages, urlString)
					fmt.Printf("🌍 Found language page to explore: %s (lang: %s)\n", urlString, langCode)
				}
			}
		}
	}
	
	fmt.Printf("🔍 Found %d language pages to recursively explore\n", len(languagePages))
	
	// Limit to first 5 language pages to avoid overwhelming the server
	if len(languagePages) > 5 {
		languagePages = languagePages[:5]
	}
	
	client := &http.Client{}
	
	for _, pageURL := range languagePages {
		fmt.Printf("🔍 Recursively exploring: %s\n", pageURL)
		
		// Quick check if this page exists
		req, err := http.NewRequestWithContext(ctx, "HEAD", pageURL, nil)
		if err != nil {
			continue
		}
		
		resp, err := client.Do(req)
		if err != nil || resp.StatusCode >= 400 {
			if resp != nil {
				resp.Body.Close()
			}
			fmt.Printf("🚫 Skipping %d error for %s\n", resp.StatusCode, pageURL)
			continue
		}
		resp.Body.Close()
		
		// Now crawl this page for more links
		req, err = http.NewRequestWithContext(ctx, "GET", pageURL, nil)
		if err != nil {
			continue
		}
		
		resp, err = client.Do(req)
		if err != nil || resp.StatusCode >= 400 {
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
		
		// Extract the language prefix from current page
		parsedPageURL, _ := url.Parse(pageURL)
		langPrefix := ""
		path := parsedPageURL.Path
		if len(path) >= 3 && path[0] == '/' && (len(path) == 3 || path[3] == '/') {
			langCode := path[1:3]
			if len(langCode) == 2 && isAlpha(langCode) {
				langPrefix = path[:3] // e.g., "/es"
				fmt.Printf("🌍 Exploring language prefix: %s\n", langPrefix)
			}
		}
		
		// Find all links on this page that belong to the same language
		doc.Find("a[href]").Each(func(i int, s *goquery.Selection) {
			href, exists := s.Attr("href")
			if !exists || href == "" {
				return
			}
			
			if strings.HasPrefix(href, "#") || strings.HasPrefix(href, "mailto:") || 
			   strings.HasPrefix(href, "tel:") || strings.HasPrefix(href, "javascript:") {
				return
			}
			
			parsedURL, err := url.Parse(href)
			if err != nil {
				return
			}
			
			resolvedURL := baseURL.ResolveReference(parsedURL)
			
			// Only keep URLs from the same language folder
			if langPrefix != "" && strings.HasPrefix(resolvedURL.Path, langPrefix+"/") {
				cleanURL := *resolvedURL
				cleanURL.Fragment = ""
				cleanURL.RawQuery = ""
				
				if isSameDomain(&cleanURL, baseURL) {
					cleanURLString := cleanURL.String()
					if !existingURLs[cleanURLString] && !foundUrls[cleanURLString] {
						foundUrls[cleanURLString] = true
						newURLs = append(newURLs, cleanURLString)
						fmt.Printf("🌍 Found recursive %s page: %s\n", langPrefix, cleanURLString)
					}
				}
			}
		})
	}
	
	fmt.Printf("Recursive discovery found %d new URLs\n", len(newURLs))
	return newURLs
}

// isAlpha checks if a string contains only alphabetic characters
func isAlpha(s string) bool {
	for _, r := range s {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')) {
			return false
		}
	}
	return true
}