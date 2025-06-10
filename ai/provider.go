package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

type Provider interface {
	Process(content string, prompt string, ctx context.Context) (string, error)
}

type Config struct {
	Provider string
	Model    string
	APIKey   string
	Prompt   string
}

type PythonProvider struct {
	config Config
}

func NewPythonProvider(config Config) *PythonProvider {
	return &PythonProvider{
		config: config,
	}
}

func (p *PythonProvider) Process(content string, prompt string, ctx context.Context) (string, error) {
	payload := map[string]string{
		"provider": p.config.Provider,
		"model":    p.config.Model,
		"api_key":  p.config.APIKey,
		"prompt":   prompt,
		"content":  content,
	}
	
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("error marshaling payload: %v", err)
	}
	
	cmd := exec.CommandContext(ctx, "./venv/bin/python", "ai_processor.py")
	cmd.Stdin = strings.NewReader(string(payloadJSON))
	
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("AI Processing Error: %v", err)
	}
	
	return strings.TrimSpace(string(output)), nil
}

func PrepareContentForAI(title, url, content string, headings []map[string]string, paragraphs []string, links []map[string]string, images []map[string]string, htmlContent, markdown string) string {
	var result strings.Builder
	
	result.WriteString(fmt.Sprintf("Title: %s\n", title))
	result.WriteString(fmt.Sprintf("URL: %s\n\n", url))
	
	if content != "" {
		result.WriteString(fmt.Sprintf("Content:\n%s\n\n", content))
	}
	
	if len(headings) > 0 {
		result.WriteString("Headings:\n")
		for _, heading := range headings {
			result.WriteString(fmt.Sprintf("- %s: %s\n", heading["level"], heading["text"]))
		}
		result.WriteString("\n")
	}
	
	if len(paragraphs) > 0 {
		result.WriteString("Paragraphs:\n")
		for i, paragraph := range paragraphs {
			result.WriteString(fmt.Sprintf("Paragraph %d:\n%s\n\n", i+1, paragraph))
		}
	}
	
	if len(links) > 0 {
		result.WriteString("Links:\n")
		for _, link := range links {
			result.WriteString(fmt.Sprintf("- %s: %s\n", link["text"], link["url"]))
		}
		result.WriteString("\n")
	}
	
	if len(images) > 0 {
		result.WriteString("Images:\n")
		for _, image := range images {
			result.WriteString(fmt.Sprintf("- URL: %s", image["url"]))
			if image["alt"] != "" {
				result.WriteString(fmt.Sprintf(", Alt: %s", image["alt"]))
			}
			result.WriteString("\n")
		}
		result.WriteString("\n")
	}
	
	if markdown != "" {
		result.WriteString("Markdown Content:\n")
		// Limit markdown content to avoid overwhelming the AI
		maxMarkdownLength := 8000
		if len(markdown) > maxMarkdownLength {
			result.WriteString(markdown[:maxMarkdownLength])
			result.WriteString("\n... (Markdown truncated for length)\n\n")
		} else {
			result.WriteString(markdown)
			result.WriteString("\n\n")
		}
	}
	
	if htmlContent != "" {
		result.WriteString("HTML Structure:\n")
		// Include HTML content but limit to reasonable size for AI processing
		maxHTMLLength := 10000
		if len(htmlContent) > maxHTMLLength {
			result.WriteString(htmlContent[:maxHTMLLength])
			result.WriteString("\n... (HTML truncated for length)\n\n")
		} else {
			result.WriteString(htmlContent)
			result.WriteString("\n\n")
		}
	}
	
	return result.String()
}

// ProcessContent is a convenience function for processing content with AI
func ProcessContent(content, provider, model, apiKey string) (string, error) {
	config := Config{
		Provider: provider,
		Model:    model,
		APIKey:   apiKey,
		Prompt:   "Analyze and summarize the following content:",
	}
	
	pythonProvider := NewPythonProvider(config)
	ctx := context.Background()
	
	return pythonProvider.Process(content, config.Prompt, ctx)
}