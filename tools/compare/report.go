package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/dotcommander/defuddle/extractors"
)

type engineReport struct {
	Engine    string    `json:"engine"`
	Metadata  metadata  `json:"metadata"`
	Text      string    `json:"text"`
	HTML      string    `json:"html"`
	HTMLBytes int       `json:"html_bytes"`
	TextBytes int       `json:"text_bytes"`
	Words     int       `json:"words"`
	Structure structure `json:"structure"`
	ElapsedNS int64     `json:"elapsed_ns"`
	Error     string    `json:"error,omitempty"`
}

type agreement struct {
	Engines      [2]string `json:"engines"`
	TokenJaccard float64   `json:"token_jaccard"`
}

type fixtureReport struct {
	ID            string            `json:"id"`
	URL           string            `json:"url"`
	InputBytes    int               `json:"input_bytes"`
	ReferenceText string            `json:"reference_text,omitempty"`
	Annotations   map[string]string `json:"annotations,omitempty"`
	Results       []engineReport    `json:"results"`
	Agreement     []agreement       `json:"agreement"`
}

func writeReport(ctx context.Context, fixtures []fixture, output io.Writer) error {
	return writeReportWithExtractor(ctx, fixtures, output, extract)
}

func writeReportWithExtractor(ctx context.Context, fixtures []fixture, output io.Writer, extractor engineExtractor) error {
	enc := json.NewEncoder(output)
	// Initialize outside timed extraction; all engines still include their DOM parse.
	extractors.InitializeBuiltins()
	var failures []error
	for _, f := range fixtures {
		input, pageURL, err := prepareInput(f)
		if err != nil {
			return err
		}
		if f.ReferenceText != "" {
			if _, err := readBounded(f.ReferenceText); err != nil {
				return fmt.Errorf("reference %s: %w", f.ID, err)
			}
		}
		report := fixtureReport{ID: f.ID, URL: pageURL.String(), InputBytes: len(input), ReferenceText: f.ReferenceText, Annotations: f.Annotations}
		for _, engine := range []string{"defuddle", "trafilatura", "readability"} {
			start := time.Now()
			r, err := extractPage(ctx, f.ID, engine, input, pageURL, extractor)
			elapsed := time.Since(start).Nanoseconds()
			result := engineReport{Engine: engine, Metadata: r.Metadata, HTML: r.HTML, HTMLBytes: len(r.HTML), ElapsedNS: elapsed}
			if err == nil {
				result.Text, result.Structure, err = normalizeHTML(r.HTML)
			}
			if err != nil {
				result.Error = err.Error()
				failures = append(failures, fmt.Errorf("%s/%s: %w", f.ID, engine, err))
			}
			result.TextBytes, result.Words = len(result.Text), len(strings.Fields(result.Text))
			report.Results = append(report.Results, result)
		}
		for i := range report.Results {
			for j := i + 1; j < len(report.Results); j++ {
				a, b := report.Results[i], report.Results[j]
				if a.Error == "" && b.Error == "" {
					report.Agreement = append(report.Agreement, agreement{[2]string{a.Engine, b.Engine}, jaccard(a.Text, b.Text)})
				}
			}
		}
		if err := enc.Encode(report); err != nil {
			return err
		}
	}
	return errors.Join(failures...)
}

type adapterResult struct {
	ID       string   `json:"id"`
	Text     string   `json:"text"`
	Metadata metadata `json:"metadata"`
	Error    string   `json:"error,omitempty"`
}

func runAdapter(ctx context.Context, engine string, input io.Reader, output io.Writer) error {
	return runAdapterWithExtractor(ctx, engine, input, output, extract)
}

func runAdapterWithExtractor(ctx context.Context, engine string, input io.Reader, output io.Writer, extractor engineExtractor) error {
	scanner := bufio.NewScanner(input)
	scanner.Buffer(make([]byte, 4096), maxInputBytes)
	w := bufio.NewWriter(output)
	enc := json.NewEncoder(w)
	for scanner.Scan() {
		var f fixture
		err := json.Unmarshal(scanner.Bytes(), &f)
		var syntaxErr *json.SyntaxError
		if errors.As(err, &syntaxErr) && syntaxErr.Error() == "unexpected end of JSON input" {
			return fmt.Errorf("truncated protocol request: %w", err)
		}
		result := adapterResult{ID: f.ID, Metadata: metadata{Authors: []string{}}}
		if err == nil && (f.ID == "" || f.HTMLPath == "") {
			err = fmt.Errorf("request requires id and html_path")
		}
		if err == nil {
			var r extraction
			html, pageURL, prepErr := prepareInput(f)
			err = prepErr
			if err == nil {
				r, err = extractPage(ctx, f.ID, engine, html, pageURL, extractor)
			}
			if err == nil {
				result.Text, _, err = normalizeHTML(r.HTML)
				result.Metadata = r.Metadata
			}
		}
		if err != nil {
			result.Text = ""
			result.Error = err.Error()
		}
		if err := enc.Encode(result); err != nil {
			return err
		}
		if err := w.Flush(); err != nil {
			return err
		}
	}
	return scanner.Err()
}
