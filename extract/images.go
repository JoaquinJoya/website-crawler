package extract

import (
	"net/url"

	"github.com/PuerkitoBio/goquery"
)

func Images(doc *goquery.Document, baseURL string) []map[string]string {
	var images []map[string]string
	base, _ := url.Parse(baseURL)
	
	doc.Find("img[src]").Each(func(i int, s *goquery.Selection) {
		src, _ := s.Attr("src")
		
		if parsedURL, err := url.Parse(src); err == nil {
			resolvedURL := base.ResolveReference(parsedURL)
			
			image := map[string]string{
				"url": resolvedURL.String(),
			}
			
			if alt, exists := s.Attr("alt"); exists {
				image["alt"] = alt
			}
			
			if title, exists := s.Attr("title"); exists {
				image["title"] = title
			}
			
			if width, exists := s.Attr("width"); exists {
				image["width"] = width
			}
			
			if height, exists := s.Attr("height"); exists {
				image["height"] = height
			}
			
			images = append(images, image)
		}
	})
	
	return images
}