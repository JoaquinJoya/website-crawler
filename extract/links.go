package extract

import (
	"net/url"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

func Links(doc *goquery.Document, baseURL string) []map[string]string {
	var links []map[string]string
	base, _ := url.Parse(baseURL)
	
	doc.Find("a[href]").Each(func(i int, s *goquery.Selection) {
		href, _ := s.Attr("href")
		text := strings.TrimSpace(s.Text())
		
		if parsedURL, err := url.Parse(href); err == nil {
			resolvedURL := base.ResolveReference(parsedURL)
			
			link := map[string]string{
				"url":  resolvedURL.String(),
				"text": text,
			}
			
			if title, exists := s.Attr("title"); exists {
				link["title"] = title
			}
			
			if class, exists := s.Attr("class"); exists {
				if strings.Contains(strings.ToLower(class), "btn") || 
				   strings.Contains(strings.ToLower(class), "cta") ||
				   strings.Contains(strings.ToLower(class), "button") {
					link["type"] = "cta"
				}
			}
			
			if strings.Contains(strings.ToLower(text), "download") ||
			   strings.Contains(strings.ToLower(text), "subscribe") ||
			   strings.Contains(strings.ToLower(text), "sign up") ||
			   strings.Contains(strings.ToLower(text), "get started") {
				link["type"] = "cta"
			}
			
			if link["type"] == "" {
				link["type"] = "link"
			}
			
			links = append(links, link)
		}
	})
	
	return links
}

func HTMLSitemapURLs(doc *goquery.Document, baseURL *url.URL) []string {
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
				}
			}
		})
	}
	
	return urls
}

func JavaScriptURLs(doc *goquery.Document, baseURL *url.URL) []string {
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
							}
						}
					}
				}
			}
		}
	})
	
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