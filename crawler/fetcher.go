package crawler

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

type PageFetcher struct {
	client    *http.Client
	userAgent string
}

// PageFetcherInterface defines the interface for page fetchers
type PageFetcherInterface interface {
	FetchDocument(pageURL string, ctx context.Context) (*goquery.Document, error)
	ApplyTargetSelector(doc *goquery.Document, targetSelector *TargetSelector) (*goquery.Document, error)
}

func NewPageFetcher() *PageFetcher {
	return &PageFetcher{
		client: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:        200,
				MaxIdleConnsPerHost: 50,
				MaxConnsPerHost:     100,
				IdleConnTimeout:     30 * time.Second,
				TLSHandshakeTimeout: 10 * time.Second,
				DisableKeepAlives:   false,
				ForceAttemptHTTP2:   true,
			},
		},
		userAgent: "Mozilla/5.0 (compatible; WebCrawler/1.0)",
	}
}

// NewPageFetcherWithBackend creates a page fetcher based on the backend choice
func NewPageFetcherWithBackend(useColly bool, collyConfig CollyConfig) PageFetcherInterface {
	if useColly {
		return NewCollyPageFetcher(collyConfig)
	}
	return NewPageFetcher()
}

func (f *PageFetcher) FetchDocument(pageURL string, ctx context.Context) (*goquery.Document, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", pageURL, nil)
	if err != nil {
		return nil, fmt.Errorf("request creation failed: %v", err)
	}

	req.Header.Set("User-Agent", f.userAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, err
	}

	return doc, nil
}

func (f *PageFetcher) ApplyTargetSelector(doc *goquery.Document, targetSelector *TargetSelector) (*goquery.Document, error) {
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

type TargetSelector struct {
	Selector    string
	Type        string
	Description string
	Mode        string
}