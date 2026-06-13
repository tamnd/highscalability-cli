package highscalability_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tamnd/highscalability-cli/highscalability"
)

// sampleFeed is a minimal valid RSS feed for testing.
const sampleFeed = `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0" xmlns:dc="http://purl.org/dc/elements/1.1/">
  <channel>
    <title>High Scalability</title>
    <link>http://highscalability.com/blog/</link>
    <item>
      <title>How Google Scales</title>
      <link>http://highscalability.com/blog/2023/1/1/how-google-scales.html</link>
      <pubDate>Sun, 01 Jan 2023 12:00:00 +0000</pubDate>
      <description><![CDATA[<p>Google uses many techniques to scale.</p>]]></description>
      <dc:creator>Todd Hoff</dc:creator>
    </item>
    <item>
      <title>Amazon Dynamo</title>
      <link>http://highscalability.com/blog/2022/12/1/amazon-dynamo.html</link>
      <pubDate>Thu, 01 Dec 2022 10:00:00 +0000</pubDate>
      <description><![CDATA[<p>Dynamo is a key-value store.</p>]]></description>
      <dc:creator>Todd Hoff</dc:creator>
    </item>
    <item>
      <title>Cassandra Architecture</title>
      <link>http://highscalability.com/blog/2022/11/1/cassandra.html</link>
      <pubDate>Tue, 01 Nov 2022 08:00:00 +0000</pubDate>
      <description><![CDATA[<p>Cassandra is a distributed database.</p>]]></description>
      <dc:creator>Jane Smith</dc:creator>
    </item>
  </channel>
</rss>`

func newTestClient(t *testing.T, handler http.HandlerFunc) (*httptest.Server, *highscalability.Client) {
	t.Helper()
	ts := httptest.NewServer(handler)
	t.Cleanup(ts.Close)
	cfg := highscalability.DefaultConfig()
	cfg.BaseURL = ts.URL
	cfg.FeedURL = ts.URL + "/feed"
	cfg.Rate = 0
	return ts, highscalability.NewClient(cfg)
}

func TestLatest(t *testing.T) {
	ts, c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") == "" {
			t.Error("missing User-Agent header")
		}
		_, _ = w.Write([]byte(sampleFeed))
	})
	_ = ts

	articles, err := c.Latest(context.Background(), 10)
	if err != nil {
		t.Fatalf("Latest: %v", err)
	}
	if len(articles) != 3 {
		t.Fatalf("got %d articles, want 3", len(articles))
	}
	if articles[0].Title != "How Google Scales" {
		t.Errorf("articles[0].Title = %q, want %q", articles[0].Title, "How Google Scales")
	}
	if articles[0].Author != "Todd Hoff" {
		t.Errorf("articles[0].Author = %q, want %q", articles[0].Author, "Todd Hoff")
	}
	if articles[0].URL == "" {
		t.Error("articles[0].URL is empty")
	}
	if articles[0].Date == "" {
		t.Error("articles[0].Date is empty")
	}
}

func TestLatestLimit(t *testing.T) {
	_, c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(sampleFeed))
	})

	articles, err := c.Latest(context.Background(), 2)
	if err != nil {
		t.Fatalf("Latest: %v", err)
	}
	if len(articles) != 2 {
		t.Fatalf("got %d articles, want 2", len(articles))
	}
}

func TestSearch(t *testing.T) {
	_, c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(sampleFeed))
	})

	hits, err := c.Search(context.Background(), "google", 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(hits) != 1 {
		t.Fatalf("got %d hits, want 1", len(hits))
	}
	if !strings.Contains(strings.ToLower(hits[0].Title), "google") {
		t.Errorf("unexpected hit: %q", hits[0].Title)
	}
}

func TestSearchLimit(t *testing.T) {
	_, c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(sampleFeed))
	})

	hits, err := c.Search(context.Background(), "a", 1)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(hits) > 1 {
		t.Errorf("got %d hits with limit=1, want at most 1", len(hits))
	}
}

func TestArticle(t *testing.T) {
	ts, c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/feed" {
			_, _ = w.Write([]byte(sampleFeed))
			return
		}
		_, _ = w.Write([]byte(`<!DOCTYPE html><html><head>
<title>How Google Scales - High Scalability</title>
<meta property="og:title" content="How Google Scales" />
<meta property="og:description" content="Google uses Bigtable, Spanner and more." />
</head><body>
<div class="entry-content"><p>Google is a huge company that needs massive scale.</p></div>
</body></html>`))
	})

	a, err := c.Article(context.Background(), ts.URL+"/blog/2023/1/1/how-google-scales.html")
	if err != nil {
		t.Fatalf("Article: %v", err)
	}
	if a.Title == "" {
		t.Error("Article.Title is empty")
	}
	if a.URL == "" {
		t.Error("Article.URL is empty")
	}
}

func TestGetRetriesOn503(t *testing.T) {
	var hits int
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		if hits < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write([]byte(sampleFeed))
	}))
	defer ts.Close()

	cfg := highscalability.DefaultConfig()
	cfg.BaseURL = ts.URL
	cfg.FeedURL = ts.URL + "/feed"
	cfg.Rate = 0
	cfg.Retries = 5
	c := highscalability.NewClient(cfg)

	articles, err := c.Latest(context.Background(), 10)
	if err != nil {
		t.Fatalf("Latest after retries: %v", err)
	}
	if len(articles) == 0 {
		t.Error("expected articles after retries")
	}
	if hits != 3 {
		t.Errorf("server saw %d hits, want 3", hits)
	}
}

func TestSummaryTruncation(t *testing.T) {
	longDesc := "<p>" + strings.Repeat("a", 300) + "</p>"
	feed := `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0"><channel><title>HS</title><link>http://example.com</link>
<item>
  <title>Long Article</title>
  <link>http://example.com/long</link>
  <pubDate>Sun, 01 Jan 2023 12:00:00 +0000</pubDate>
  <description><![CDATA[` + longDesc + `]]></description>
</item>
</channel></rss>`

	_, c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(feed))
	})

	articles, err := c.Latest(context.Background(), 10)
	if err != nil {
		t.Fatalf("Latest: %v", err)
	}
	if len(articles) != 1 {
		t.Fatalf("got %d articles, want 1", len(articles))
	}
	if len([]rune(articles[0].Summary)) > 200 {
		t.Errorf("summary too long: %d runes", len([]rune(articles[0].Summary)))
	}
}
