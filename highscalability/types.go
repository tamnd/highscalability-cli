package highscalability

import (
	"strings"
	"time"
)

// Article is the record emitted for a High Scalability blog post.
type Article struct {
	Title   string `json:"title"`
	Date    string `json:"date"`
	Summary string `json:"summary"`
	URL     string `json:"url"`
	Author  string `json:"author"`
}

// ─── RSS/Atom wire types ──────────────────────────────────────────────────────

// rssFeed is the top-level RSS 2.0 envelope.
type rssFeed struct {
	Items []rssItem `xml:"channel>item"`
}

// rssItem is one <item> in the RSS feed.
type rssItem struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	PubDate     string `xml:"pubDate"`
	Description string `xml:"description"`
	Author      string `xml:"creator"`
}

// ─── helpers ─────────────────────────────────────────────────────────────────

// parseRSSDate parses the RFC1123Z date format used by the feedburner RSS feed.
func parseRSSDate(s string) string {
	s = strings.TrimSpace(s)
	for _, layout := range []string{
		time.RFC1123Z,
		time.RFC1123,
		"Mon, 2 Jan 2006 15:04:05 -0700",
		"Mon, 2 Jan 2006 15:04:05 MST",
	} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC().Format(time.RFC3339)
		}
	}
	return s
}

// truncateSummary strips HTML and caps at maxLen runes.
func truncateSummary(s string, maxLen int) string {
	plain := stripTags(s)
	plain = strings.TrimSpace(plain)
	rs := []rune(plain)
	if len(rs) <= maxLen {
		return plain
	}
	return string(rs[:maxLen-3]) + "..."
}

// stripTags removes HTML tags and decodes common entities.
func stripTags(s string) string {
	var b strings.Builder
	inTag := false
	for _, r := range s {
		switch {
		case r == '<':
			inTag = true
		case r == '>':
			inTag = false
		case !inTag:
			b.WriteRune(r)
		}
	}
	out := b.String()
	out = strings.ReplaceAll(out, "&amp;", "&")
	out = strings.ReplaceAll(out, "&lt;", "<")
	out = strings.ReplaceAll(out, "&gt;", ">")
	out = strings.ReplaceAll(out, "&quot;", `"`)
	out = strings.ReplaceAll(out, "&#39;", "'")
	out = strings.ReplaceAll(out, "&apos;", "'")
	out = strings.ReplaceAll(out, "&nbsp;", " ")
	out = strings.ReplaceAll(out, "&rsquo;", "'")
	out = strings.ReplaceAll(out, "&lsquo;", "'")
	out = strings.ReplaceAll(out, "&rdquo;", `"`)
	out = strings.ReplaceAll(out, "&ldquo;", `"`)
	out = strings.ReplaceAll(out, "&mdash;", "-")
	out = strings.ReplaceAll(out, "&ndash;", "-")
	out = strings.TrimSpace(out)
	return out
}
