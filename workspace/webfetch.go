package workspace

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/flashcatcloud/flashduty-runner/protocol"

	htmltomarkdown "github.com/JohannesKaufmann/html-to-markdown/v2"
)

const (
	defaultFetchTimeout   = 30 * time.Second
	maxFetchTimeout       = 120 * time.Second
	maxResponseSize       = 5 * 1024 * 1024 // 5MB
	defaultFetchUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"
)

// WebFetch fetches content from a URL and converts it to readable format.
func (w *Workspace) WebFetch(ctx context.Context, args *protocol.WebFetchArgs) (*protocol.WebFetchResult, error) {
	if args.URL == "" || (!strings.HasPrefix(args.URL, "http://") && !strings.HasPrefix(args.URL, "https://")) {
		return nil, fmt.Errorf("valid http/https url is required")
	}

	timeout := defaultFetchTimeout
	if args.Timeout > 0 {
		timeout = time.Duration(args.Timeout) * time.Second
		if timeout > maxFetchTimeout {
			timeout = maxFetchTimeout
		}
	}

	format := args.Format
	if format == "" {
		format = "markdown"
	}

	resp, err := w.fetchURL(ctx, args.URL, format, timeout)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	body, err := readResponseBody(resp)
	if err != nil {
		return nil, err
	}

	content := convertContent(string(body), format, resp.Header.Get("Content-Type"))
	processor := NewLargeOutputProcessor(w, DefaultLargeOutputConfig())
	processed, err := processor.Process(ctx, content, "webfetch")
	if err != nil {
		return nil, err
	}

	return &protocol.WebFetchResult{
		Content:   processed.Content,
		URL:       resp.Request.URL.String(),
		Truncated: processed.Truncated,
		FilePath:  processed.FilePath,
		TotalSize: processed.TotalSize,
	}, nil
}

// fetchURL performs the HTTP request.
func (w *Workspace) fetchURL(ctx context.Context, url, format string, timeout time.Duration) (*http.Response, error) {
	httpCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(httpCtx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	setRequestHeaders(req, format)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		if httpCtx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("request timed out")
		}
		return nil, fmt.Errorf("request failed: %w", err)
	}

	return resp, nil
}

// setRequestHeaders sets appropriate headers for the request.
func setRequestHeaders(req *http.Request, format string) {
	req.Header.Set("User-Agent", defaultFetchUserAgent)
	req.Header.Set("Accept-Language", "en-US,en;q=0.9,zh-CN;q=0.8,zh;q=0.7")

	switch format {
	case "markdown":
		req.Header.Set("Accept", "text/markdown, text/html;q=0.9, */*;q=0.8")
	case "text":
		req.Header.Set("Accept", "text/plain, text/html;q=0.9, */*;q=0.8")
	default:
		req.Header.Set("Accept", "text/html, */*;q=0.8")
	}
}

// readResponseBody reads and validates the response body.
func readResponseBody(resp *http.Response) ([]byte, error) {
	if resp.ContentLength > maxResponseSize {
		return nil, fmt.Errorf("response too large (exceeds %dMB limit)", maxResponseSize/(1024*1024))
	}

	limitReader := io.LimitReader(resp.Body, maxResponseSize+1)
	body, err := io.ReadAll(limitReader)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if int64(len(body)) > maxResponseSize {
		return nil, fmt.Errorf("response too large (exceeds %dMB limit)", maxResponseSize/(1024*1024))
	}

	return body, nil
}

// convertContent converts content based on format and content type.
func convertContent(content, format, contentType string) string {
	isHTML := strings.Contains(contentType, "text/html")

	switch format {
	case "markdown":
		if isHTML {
			return convertHTMLToMarkdown(content)
		}
	case "text":
		if isHTML {
			return convertHTMLToText(content)
		}
	}

	return content
}

// convertHTMLToMarkdown converts HTML content to Markdown.
func convertHTMLToMarkdown(html string) string {
	markdown, err := htmltomarkdown.ConvertString(html)
	if err != nil {
		// Fallback to text extraction on error
		return convertHTMLToText(html)
	}
	return cleanupMarkdown(markdown)
}

// convertHTMLToText extracts plain text from HTML.
func convertHTMLToText(html string) string {
	// Remove script and style tags with their content
	scriptRe := regexp.MustCompile(`(?is)<script[^>]*>.*?</script>`)
	html = scriptRe.ReplaceAllString(html, "")

	styleRe := regexp.MustCompile(`(?is)<style[^>]*>.*?</style>`)
	html = styleRe.ReplaceAllString(html, "")

	// Remove all HTML tags
	tagRe := regexp.MustCompile(`<[^>]+>`)
	text := tagRe.ReplaceAllString(html, " ")

	// Decode common HTML entities
	text = strings.ReplaceAll(text, "&nbsp;", " ")
	text = strings.ReplaceAll(text, "&amp;", "&")
	text = strings.ReplaceAll(text, "&lt;", "<")
	text = strings.ReplaceAll(text, "&gt;", ">")
	text = strings.ReplaceAll(text, "&quot;", "\"")
	text = strings.ReplaceAll(text, "&#39;", "'")

	// Normalize whitespace
	spaceRe := regexp.MustCompile(`\s+`)
	text = spaceRe.ReplaceAllString(text, " ")

	return strings.TrimSpace(text)
}

// cleanupMarkdown removes excessive whitespace and normalizes the markdown.
func cleanupMarkdown(md string) string {
	// Remove excessive blank lines (more than 2 consecutive)
	blankLinesRe := regexp.MustCompile(`\n{3,}`)
	md = blankLinesRe.ReplaceAllString(md, "\n\n")
	return strings.TrimSpace(md)
}
