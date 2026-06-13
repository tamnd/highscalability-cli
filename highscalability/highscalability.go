// Package highscalability is the library behind the hsc command: the HTTP
// client, request shaping, and typed data models for the High Scalability blog.
//
// Feed: http://feeds.feedburner.com/HighScalability (RSS 2.0, Squarespace/V5)
// No API key required; all data is publicly available.
package highscalability

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const feedURL = "http://feeds.feedburner.com/HighScalability"

// DefaultUserAgent identifies the client to the server.
const DefaultUserAgent = "hsc/dev (+https://github.com/tamnd/highscalability-cli)"

// Config holds constructor parameters.
type Config struct {
	BaseURL   string
	FeedURL   string
	UserAgent string
	Rate      time.Duration
	Retries   int
	Timeout   time.Duration
}

// DefaultConfig returns sensible defaults.
func DefaultConfig() Config {
	return Config{
		BaseURL:   "http://highscalability.com",
		FeedURL:   feedURL,
		UserAgent: DefaultUserAgent,
		Rate:      200 * time.Millisecond,
		Retries:   3,
		Timeout:   30 * time.Second,
	}
}

// Client talks to the High Scalability blog over HTTP.
type Client struct {
	httpClient *http.Client
	userAgent  string
	rate       time.Duration
	retries    int
	feedURL    string
	baseURL    string
	last       time.Time
}

// NewClient returns a Client with the given config.
func NewClient(cfg Config) *Client {
	fu := cfg.FeedURL
	if fu == "" {
		fu = feedURL
	}
	return &Client{
		httpClient: &http.Client{Timeout: cfg.Timeout},
		userAgent:  cfg.UserAgent,
		rate:       cfg.Rate,
		retries:    cfg.Retries,
		feedURL:    fu,
		baseURL:    cfg.BaseURL,
	}
}

// get fetches a URL with pacing and retries.
func (c *Client) get(ctx context.Context, rawURL string) ([]byte, error) {
	var lastErr error
	for attempt := 0; attempt <= c.retries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff(attempt)):
			}
		}
		body, retry, err := c.do(ctx, rawURL)
		if err == nil {
			return body, nil
		}
		lastErr = err
		if !retry {
			return nil, err
		}
	}
	return nil, fmt.Errorf("get %s: %w", rawURL, lastErr)
}

func (c *Client) do(ctx context.Context, rawURL string) ([]byte, bool, error) {
	c.pace()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, false, err
	}
	req.Header.Set("User-Agent", c.userAgent)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, true, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
		return nil, true, fmt.Errorf("http %d", resp.StatusCode)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, false, fmt.Errorf("http %d", resp.StatusCode)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, true, err
	}
	return b, false, nil
}

func (c *Client) pace() {
	if c.rate <= 0 {
		return
	}
	if wait := c.rate - time.Since(c.last); wait > 0 {
		time.Sleep(wait)
	}
	c.last = time.Now()
}

func backoff(attempt int) time.Duration {
	d := time.Duration(attempt) * 500 * time.Millisecond
	if d > 5*time.Second {
		d = 5 * time.Second
	}
	return d
}

// fetchFeed downloads and parses the RSS feed, returning raw items.
func (c *Client) fetchFeed(ctx context.Context) ([]rssItem, error) {
	body, err := c.get(ctx, c.feedURL)
	if err != nil {
		return nil, fmt.Errorf("fetch feed: %w", err)
	}
	var feed rssFeed
	if err := xml.Unmarshal(body, &feed); err != nil {
		return nil, fmt.Errorf("parse feed: %w", err)
	}
	return feed.Items, nil
}

// rssItemToArticle converts a raw RSS item to an Article.
func rssItemToArticle(it rssItem) Article {
	return Article{
		Title:   strings.TrimSpace(it.Title),
		Date:    parseRSSDate(it.PubDate),
		Summary: truncateSummary(it.Description, 200),
		URL:     strings.TrimSpace(it.Link),
		Author:  strings.TrimSpace(it.Author),
	}
}

// Latest returns the most recent articles from the feed.
func (c *Client) Latest(ctx context.Context, limit int) ([]Article, error) {
	items, err := c.fetchFeed(ctx)
	if err != nil {
		return nil, err
	}
	if limit > 0 && limit < len(items) {
		items = items[:limit]
	}
	out := make([]Article, len(items))
	for i, it := range items {
		out[i] = rssItemToArticle(it)
	}
	return out, nil
}

// Search filters articles from the feed by a query string (title + summary).
func (c *Client) Search(ctx context.Context, query string, limit int) ([]Article, error) {
	items, err := c.fetchFeed(ctx)
	if err != nil {
		return nil, err
	}
	q := strings.ToLower(query)
	var out []Article
	for _, it := range items {
		a := rssItemToArticle(it)
		if strings.Contains(strings.ToLower(a.Title), q) ||
			strings.Contains(strings.ToLower(a.Summary), q) ||
			strings.Contains(strings.ToLower(stripTags(it.Description)), q) {
			out = append(out, a)
			if limit > 0 && len(out) >= limit {
				break
			}
		}
	}
	return out, nil
}

// Article fetches and parses a single article page by URL.
func (c *Client) Article(ctx context.Context, rawURL string) (Article, error) {
	body, err := c.get(ctx, rawURL)
	if err != nil {
		return Article{}, fmt.Errorf("fetch article: %w", err)
	}
	return parseArticlePage(body, rawURL), nil
}

// parseArticlePage extracts article metadata from a raw HTML page.
func parseArticlePage(body []byte, rawURL string) Article {
	html := string(body)

	title := extractMeta(html, "og:title")
	if title == "" {
		title = extractHTMLTitle(html)
	}

	description := extractMeta(html, "og:description")

	// Try to find the main content body
	content := extractMainContent(html)
	if content == "" {
		content = description
	}

	return Article{
		Title:   strings.TrimSpace(title),
		Date:    "",
		Summary: truncateSummary(content, 200),
		URL:     rawURL,
		Author:  "",
	}
}

// extractMeta extracts an og: meta tag content value.
func extractMeta(html, property string) string {
	search := `property="` + property + `"`
	idx := strings.Index(html, search)
	if idx == -1 {
		search = `property='` + property + `'`
		idx = strings.Index(html, search)
	}
	if idx == -1 {
		return ""
	}
	rest := html[idx:]
	ci := strings.Index(rest, `content="`)
	if ci == -1 {
		ci = strings.Index(rest, `content='`)
		if ci == -1 {
			return ""
		}
		rest = rest[ci+9:]
		end := strings.Index(rest, `'`)
		if end == -1 {
			return ""
		}
		return stripTags(rest[:end])
	}
	rest = rest[ci+9:]
	end := strings.Index(rest, `"`)
	if end == -1 {
		return ""
	}
	return stripTags(rest[:end])
}

// extractHTMLTitle extracts the <title> tag content.
func extractHTMLTitle(html string) string {
	start := strings.Index(html, "<title>")
	if start == -1 {
		return ""
	}
	start += 7
	end := strings.Index(html[start:], "</title>")
	if end == -1 {
		return ""
	}
	return stripTags(html[start : start+end])
}

// extractMainContent tries to find the article body text.
func extractMainContent(html string) string {
	// Try Squarespace/TypePad entry content
	for _, marker := range []string{
		`class="entry-content"`,
		`class="post-content"`,
		`class="sqs-block-content"`,
		`itemprop="articleBody"`,
	} {
		idx := strings.Index(html, marker)
		if idx == -1 {
			continue
		}
		// Find the enclosing tag start
		start := strings.LastIndex(html[:idx], "<")
		if start == -1 {
			continue
		}
		// Walk forward past the opening tag
		tagEnd := strings.Index(html[start:], ">")
		if tagEnd == -1 {
			continue
		}
		content := html[start+tagEnd+1:]
		// Take up to 2000 chars of raw HTML then strip
		if len(content) > 2000 {
			content = content[:2000]
		}
		text := stripTags(content)
		if len(strings.TrimSpace(text)) > 50 {
			return text
		}
	}
	return ""
}
