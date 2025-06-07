package extract

import (
	"fmt"
	"strings"

	md "github.com/JohannesKaufmann/html-to-markdown"
	"github.com/PuerkitoBio/goquery"
)

func Title(doc *goquery.Document) string {
	title := doc.Find("title").Text()
	if title == "" {
		title = "No title"
	}
	return strings.TrimSpace(title)
}

func HeadData(doc *goquery.Document) map[string]string {
	headData := make(map[string]string)
	
	doc.Find("meta").Each(func(i int, s *goquery.Selection) {
		if name, exists := s.Attr("name"); exists {
			if content, exists := s.Attr("content"); exists {
				headData["meta_"+name] = content
			}
		}
		if property, exists := s.Attr("property"); exists {
			if content, exists := s.Attr("content"); exists {
				headData["meta_"+property] = content
			}
		}
	})
	
	headData["title"] = doc.Find("title").Text()
	if desc := doc.Find("meta[name='description']").AttrOr("content", ""); desc != "" {
		headData["description"] = desc
	}
	if keywords := doc.Find("meta[name='keywords']").AttrOr("content", ""); keywords != "" {
		headData["keywords"] = keywords
	}
	
	return headData
}

func FormattedText(doc *goquery.Document) string {
	var result strings.Builder
	
	if title := doc.Find("h1").First().Text(); title != "" {
		result.WriteString("# " + strings.TrimSpace(title) + "\n\n")
	}
	
	doc.Find("h1, h2, h3, h4, h5, h6, p, ul, ol, blockquote, pre").Each(func(i int, s *goquery.Selection) {
		tag := s.Get(0).Data
		text := strings.TrimSpace(s.Text())
		
		if text == "" {
			return
		}
		
		switch tag {
		case "h1":
			result.WriteString("# " + text + "\n\n")
		case "h2":
			result.WriteString("## " + text + "\n\n")
		case "h3":
			result.WriteString("### " + text + "\n\n")
		case "h4":
			result.WriteString("#### " + text + "\n\n")
		case "h5":
			result.WriteString("##### " + text + "\n\n")
		case "h6":
			result.WriteString("###### " + text + "\n\n")
		case "p":
			result.WriteString(text + "\n\n")
		case "blockquote":
			result.WriteString("> " + text + "\n\n")
		case "pre":
			result.WriteString("```\n" + text + "\n```\n\n")
		case "ul", "ol":
			s.Find("li").Each(func(j int, li *goquery.Selection) {
				listText := strings.TrimSpace(li.Text())
				if listText != "" {
					if tag == "ul" {
						result.WriteString("- " + listText + "\n")
					} else {
						result.WriteString(fmt.Sprintf("%d. %s\n", j+1, listText))
					}
				}
			})
			result.WriteString("\n")
		}
	})
	
	return strings.TrimSpace(result.String())
}

func Markdown(doc *goquery.Document) string {
	converter := md.NewConverter("", true, nil)
	bodyHTML, _ := doc.Find("body").Html()
	markdown, _ := converter.ConvertString(bodyHTML)
	return markdown
}

func Headings(doc *goquery.Document) []map[string]string {
	var headings []map[string]string
	doc.Find("h1, h2, h3, h4, h5, h6").Each(func(i int, s *goquery.Selection) {
		text := strings.TrimSpace(s.Text())
		if text != "" {
			tag := s.Get(0).Data
			level := strings.ToUpper(tag)
			
			heading := map[string]string{
				"level": level,
				"text":  text,
			}
			headings = append(headings, heading)
		}
	})
	return headings
}

func Paragraphs(doc *goquery.Document) []string {
	var paragraphs []string
	doc.Find("p").Each(func(i int, s *goquery.Selection) {
		text := strings.TrimSpace(s.Text())
		if text != "" && len(text) > 10 {
			paragraphs = append(paragraphs, text)
		}
	})
	return paragraphs
}