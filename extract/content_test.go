package extract

import (
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
)

func TestTitle(t *testing.T) {
	tests := []struct {
		name     string
		html     string
		expected string
	}{
		{
			name:     "Basic title",
			html:     "<html><head><title>Test Page</title></head></html>",
			expected: "Test Page",
		},
		{
			name:     "Empty title",
			html:     "<html><head><title></title></head></html>",
			expected: "No title",
		},
		{
			name:     "No title tag",
			html:     "<html><head></head></html>",
			expected: "No title",
		},
		{
			name:     "Title with whitespace",
			html:     "<html><head><title>  Spaced Title  </title></head></html>",
			expected: "Spaced Title",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc, err := goquery.NewDocumentFromReader(strings.NewReader(tt.html))
			if err != nil {
				t.Fatalf("Failed to parse HTML: %v", err)
			}

			result := Title(doc)
			if result != tt.expected {
				t.Errorf("Title() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestHeadings(t *testing.T) {
	html := `
		<html>
			<body>
				<h1>Main Title</h1>
				<h2>Subtitle</h2>
				<h3>Sub-subtitle</h3>
			</body>
		</html>
	`

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		t.Fatalf("Failed to parse HTML: %v", err)
	}

	headings := Headings(doc)
	
	expected := []map[string]string{
		{"level": "H1", "text": "Main Title"},
		{"level": "H2", "text": "Subtitle"},
		{"level": "H3", "text": "Sub-subtitle"},
	}

	if len(headings) != len(expected) {
		t.Fatalf("Expected %d headings, got %d", len(expected), len(headings))
	}

	for i, heading := range headings {
		if heading["level"] != expected[i]["level"] {
			t.Errorf("Heading %d level = %q, want %q", i, heading["level"], expected[i]["level"])
		}
		if heading["text"] != expected[i]["text"] {
			t.Errorf("Heading %d text = %q, want %q", i, heading["text"], expected[i]["text"])
		}
	}
}

func TestParagraphs(t *testing.T) {
	html := `
		<html>
			<body>
				<p>This is a long enough paragraph to be included.</p>
				<p>Short</p>
				<p>Another sufficiently long paragraph for inclusion.</p>
			</body>
		</html>
	`

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		t.Fatalf("Failed to parse HTML: %v", err)
	}

	paragraphs := Paragraphs(doc)
	
	// Should filter out short paragraphs
	expected := []string{
		"This is a long enough paragraph to be included.",
		"Another sufficiently long paragraph for inclusion.",
	}

	if len(paragraphs) != len(expected) {
		t.Fatalf("Expected %d paragraphs, got %d", len(expected), len(paragraphs))
	}

	for i, para := range paragraphs {
		if para != expected[i] {
			t.Errorf("Paragraph %d = %q, want %q", i, para, expected[i])
		}
	}
}