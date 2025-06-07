package crawler

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/gocolly/colly/v2"
	"github.com/gocolly/colly/v2/debug"
	
	"web-crawler/monitoring"
)

type CollyPageFetcher struct {
	collector *colly.Collector
	config    CollyConfig
	monitor   *monitoring.Monitor
}

func NewCollyPageFetcher(config CollyConfig) *CollyPageFetcher {
	c := colly.NewCollector()
	
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

	// Set up debugging if enabled
	if config.DebugMode {
		c.SetDebugger(&debug.LogDebugger{})
	}

	// Set up error handling with retry logic
	c.OnError(func(r *colly.Response, err error) {
		fmt.Printf("Colly fetch error for %s: %v\n", r.Request.URL, err)
		// Colly has built-in retry mechanisms
		r.Request.Retry()
	})

	// Set request timeout
	c.SetRequestTimeout(45 * time.Second)

	return &CollyPageFetcher{
		collector: c,
		config:    config,
	}
}

func (cpf *CollyPageFetcher) FetchDocument(pageURL string, ctx context.Context) (*goquery.Document, error) {
	// Create a new collector instance for each request to avoid callback conflicts
	c := colly.NewCollector()
	
	// Configure user agent
	c.UserAgent = cpf.config.UserAgent
	
	// Set up rate limiting
	if cpf.config.Delay > 0 || cpf.config.Parallelism > 0 {
		c.Limit(&colly.LimitRule{
			DomainGlob:  cpf.config.DomainGlob,
			Parallelism: cpf.config.Parallelism,
			Delay:       cpf.config.Delay,
			RandomDelay: cpf.config.RandomDelay,
		})
	}
	
	// Set request timeout
	c.SetRequestTimeout(45 * time.Second)
	
	var doc *goquery.Document
	var fetchError error

	// Set up response callback to capture the full document
	c.OnResponse(func(r *colly.Response) {
		if strings.Contains(r.Headers.Get("Content-Type"), "text/html") {
			var err error
			doc, err = goquery.NewDocumentFromReader(strings.NewReader(string(r.Body)))
			if err != nil {
				fetchError = fmt.Errorf("failed to parse HTML: %v", err)
			}
		} else {
			fetchError = fmt.Errorf("response is not HTML: content-type %s", r.Headers.Get("Content-Type"))
		}
	})

	// Set up error handling
	c.OnError(func(r *colly.Response, err error) {
		fetchError = fmt.Errorf("Colly fetch error for %s: %v", r.Request.URL, err)
	})

	// Visit the URL
	err := c.Visit(pageURL)
	if err != nil {
		return nil, fmt.Errorf("Colly visit failed: %v", err)
	}

	// Wait for async requests if needed
	if cpf.config.Async {
		c.Wait()
	}

	if fetchError != nil {
		return nil, fetchError
	}

	if doc == nil {
		return nil, fmt.Errorf("no document retrieved for %s", pageURL)
	}

	return doc, nil
}

func (cpf *CollyPageFetcher) ApplyTargetSelector(doc *goquery.Document, targetSelector *TargetSelector) (*goquery.Document, error) {
	// This method remains the same as the original implementation
	// since it works with goquery.Document directly
	if targetSelector == nil {
		return doc, nil
	}

	selection := doc.Find(targetSelector.Selector)
	if selection.Length() == 0 {
		return doc, nil
	}

	var targetHTML string
	
	if targetSelector.Mode == "element" {
		targetHTML, _ = goquery.OuterHtml(selection)
	} else {
		targetHTML, _ = selection.Html()
	}
	
	if targetHTML == "" {
		return doc, nil
	}

	targetDoc, err := goquery.NewDocumentFromReader(strings.NewReader("<html><body>" + targetHTML + "</body></html>"))
	if err != nil {
		return doc, err
	}

	return targetDoc, nil
}

// CleanupCollector cleans up the collector after use
func (cpf *CollyPageFetcher) CleanupCollector() {
	// Remove all callbacks to prevent memory leaks
	cpf.collector.OnHTML("html", nil)
	cpf.collector.OnResponse(nil)
}