package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

const maxInputBytes = 10 << 20

type fixture struct {
	ID            string            `json:"id"`
	HTMLPath      string            `json:"html_path"`
	URL           string            `json:"url,omitempty"`
	ReferenceText string            `json:"reference_text,omitempty"`
	Annotations   map[string]string `json:"annotations,omitempty"`
}

func readBounded(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, maxInputBytes+1))
	if err != nil {
		return nil, err
	}
	if len(b) > maxInputBytes {
		return nil, fmt.Errorf("input exceeds %d bytes: %s", maxInputBytes, path)
	}
	return b, nil
}

func loadManifest(path string) ([]fixture, error) {
	b, err := readBounded(path)
	if err != nil {
		return nil, err
	}
	var fixtures []fixture
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&fixtures); err != nil {
		return nil, fmt.Errorf("manifest: %w", err)
	}
	if err := dec.Decode(new(any)); err != io.EOF {
		return nil, fmt.Errorf("manifest contains trailing data")
	}
	if len(fixtures) == 0 {
		return nil, fmt.Errorf("manifest has no fixtures")
	}
	seen := make(map[string]bool)
	for i := range fixtures {
		f := &fixtures[i]
		if f.ID == "" || f.HTMLPath == "" {
			return nil, fmt.Errorf("fixture %d requires id and html_path", i)
		}
		if seen[f.ID] {
			return nil, fmt.Errorf("duplicate fixture id: %s", f.ID)
		}
		seen[f.ID] = true
		if !filepath.IsAbs(f.HTMLPath) {
			f.HTMLPath = filepath.Join(filepath.Dir(path), f.HTMLPath)
		}
		if f.ReferenceText != "" && !filepath.IsAbs(f.ReferenceText) {
			f.ReferenceText = filepath.Join(filepath.Dir(path), f.ReferenceText)
		}
	}
	return fixtures, nil
}

func prepareInput(f fixture) (string, *url.URL, error) {
	b, err := readBounded(f.HTMLPath)
	if err != nil {
		return "", nil, fmt.Errorf("fixture %s: %w", f.ID, err)
	}
	html, frontURL, err := stripFrontmatter(string(b))
	if err != nil {
		return "", nil, err
	}
	u := f.URL
	if u == "" {
		u = frontURL
	}
	if u == "" {
		doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
		if err != nil {
			return "", nil, err
		}
		u, _ = doc.Find(`link[rel="canonical"]`).First().Attr("href")
		if u == "" {
			u, _ = doc.Find(`meta[property="og:url"]`).First().Attr("content")
		}
	}
	parsed, err := url.Parse(u)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "https" && parsed.Scheme != "http") {
		return "", nil, fmt.Errorf("fixture %s requires an absolute HTTP(S) URL", f.ID)
	}
	return html, parsed, nil
}

// Upstream fixtures use a leading HTML comment containing JSON metadata.
func stripFrontmatter(input string) (string, string, error) {
	s := strings.TrimSpace(strings.TrimPrefix(input, "\ufeff"))
	prefix := ""
	if strings.HasPrefix(strings.ToLower(s), "<!doctype") {
		if end := strings.Index(s, ">"); end >= 0 {
			prefix = s[:end+1] + "\n"
			s = strings.TrimSpace(s[end+1:])
		}
	}
	if !strings.HasPrefix(s, "<!--") {
		return input, "", nil
	}
	end := strings.Index(s, "-->")
	if end < 0 {
		if strings.HasPrefix(strings.TrimSpace(s[4:]), "{") {
			return "", "", fmt.Errorf("fixture frontmatter: unterminated JSON comment")
		}
		return input, "", nil
	}
	comment := strings.TrimSpace(s[4:end])
	if !strings.HasPrefix(comment, "{") {
		return input, "", nil
	}
	var meta struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal([]byte(comment), &meta); err != nil {
		return "", "", fmt.Errorf("fixture frontmatter: %w", err)
	}
	return prefix + strings.TrimSpace(s[end+3:]), meta.URL, nil
}
