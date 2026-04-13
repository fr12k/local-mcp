package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"time"

	"github.com/fr12k/rodwer"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

type SearchResult struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Link        string `json:"link"`
}

func webSearch(query string) ([]SearchResult, error) {
	browser, err := rodwer.NewBrowser(rodwer.BrowserOptions{Headless: true})
	if err != nil {
		return nil, fmt.Errorf("browser creation failed: %w", err)
	}
	defer func() { _ = browser.Close() }()

	page, err := browser.NewPage()
	if err != nil {
		return nil, fmt.Errorf("page creation failed: %w", err)
	}
	defer func() { _ = page.Close() }()

	searchURL := "https://html.duckduckgo.com/html/?q=" + url.QueryEscape(query)
	if err := page.Navigate(searchURL); err != nil {
		return nil, fmt.Errorf("navigation failed: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if _, err := page.WaitForElementWithContext(ctx, ".result"); err != nil {
		return nil, fmt.Errorf("waiting for results failed: %w", err)
	}

	elements, err := page.Elements(".result")
	if err != nil {
		return nil, fmt.Errorf("finding results failed: %w", err)
	}

	var results []SearchResult
	for _, el := range elements {
		if len(results) >= 10 {
			break
		}

		var sr SearchResult

		if title, err := el.Element(".result__a"); err == nil {
			sr.Name, _ = title.Text()
			sr.Link, _ = title.Attribute("href")
		}
		if snippet, err := el.Element(".result__snippet"); err == nil {
			sr.Description, _ = snippet.Text()
		}

		// DuckDuckGo wraps links in redirects; extract the actual URL
		if sr.Link != "" {
			if parsed, err := url.Parse(sr.Link); err == nil {
				if actual := parsed.Query().Get("uddg"); actual != "" {
					sr.Link = actual
				}
			}
		}

		if sr.Name != "" && sr.Link != "" {
			results = append(results, sr)
		}
	}

	return results, nil
}

func main() {
	s := server.NewMCPServer("local-mcp", "1.0.0")

	tool := mcp.NewTool("web_search",
		mcp.WithDescription("Search the web and return the first 10 results with name, description, and link"),
		mcp.WithString("query",
			mcp.Required(),
			mcp.Description("The search query"),
		),
	)

	s.AddTool(tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		query := request.GetArguments()["query"].(string)

		results, err := webSearch(query)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		jsonBytes, err := json.MarshalIndent(results, "", "  ")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		return mcp.NewToolResultText(string(jsonBytes)), nil
	})

	if err := server.ServeStdio(s); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
