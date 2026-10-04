# Saved HTML extraction comparison

This isolated developer module compares Defuddle from the local checkout,
Go Trafilatura v2.2.6, and Readeck Go Readability v2.1.3. It does not change
production dependencies or fetch pages. Run with `GOWORK=off` so the tool's
dependency pins and local `replace` apply independently of the production workspace.

```sh
cd tools/compare
GOWORK=off go run . report manifest.json -o /tmp/defuddle-comparison.jsonl
GOWORK=off go build -o /tmp/defuddle-compare .
```

The small manifest uses the existing `.reference/defuddle/tests/fixtures` corpus.
Missing files fail explicitly. The root real-world benchmarks skip only when
the entire optional reference fixture directory is absent; a missing named
fixture within that directory fails.

External manifests are JSON arrays of objects with `id`, `html_path`, optional
`url`, optional `reference_text` (a local plain text file), and optional
`annotations` (string values). Paths resolve relative to the manifest. IDs must
be unique. HTML, manifests, reference files, and each protocol request are capped
at 10 MiB. URLs resolve from explicit `url`, leading JSON comment frontmatter
(also after a doctype), canonical link, then `og:url`. An absolute HTTP(S) URL
is required; filenames never supply an inferred URL. Supplied URLs are context
only. The two synthetic sample fixture URLs are explicit manifest annotations.

Output is one JSON object per fixture in manifest order, with engines always
ordered Defuddle, Trafilatura, Readability. It includes HTML and normalized text,
metadata, content byte/word counts, structure counts, and elapsed nanoseconds.
Engine failures are recorded and make the report command exit nonzero after
processing the remaining valid fixtures. Input errors stop the command. Output
may therefore be partial on failure; check the exit code. `-o -` writes stdout.

Timings are single sequential extraction samples including DOM parsing and engine
HTML rendering, excluding input I/O and shared text normalization. Defuddle's
builtin registry is initialized before timing. There is no warmup, repetition,
allocation measurement, or statistical performance claim. Engine order, caches,
GC, and machine load affect results. Use the repaired root Go benchmarks for
Defuddle allocation measurements.

All engines receive the same stripped HTML and URL. Defuddle uses default
Trafilatura-backed generic extraction and builtin site extractors, with Markdown disabled. Trafilatura
uses balanced/default focus, fallback enabled, comments excluded, tables retained,
links and images included. Readeck uses its defaults. These are related settings,
not identical algorithms; Defuddle site extractors may retain discussions that
other engines exclude. No extra sanitization or Markdown conversion is added.
Structure differences include engine representation choices. Extracted HTML is
untrusted data: inspect JSON as text; do not serve artifacts as executable pages.
Production Defuddle's custom Markdown pipeline remains unchanged and unmeasured.

Shared text normalization removes script/style/noscript/template elements,
decodes HTML entities via DOM parsing, inserts whitespace around block elements,
and collapses whitespace. Pairwise `token_jaccard` is **agreement only**: tokenize
lowercased Unicode letter/number runs, discard frequency/order, divide set
intersection by union (two empty sets equal 1). It cannot show correctness,
precision, recall, F1, or superiority. Counts and metadata presence are likewise
descriptive. TS expected snapshots are parity targets, not independent gold.
`reference_text` is checked for availability and recorded, never scored here.

## Independent quality evaluation

The adapter implements the existing [content-extractor-benchmark protocol](https://github.com/markusmobius/content-extractor-benchmark#extractor-protocol)
for a separately prepared local corpus and its independent annotations:

```sh
/tmp/defuddle-compare adapter --engine defuddle < requests.jsonl > extracted.jsonl
/tmp/defuddle-compare adapter --engine trafilatura < requests.jsonl > extracted.jsonl
/tmp/defuddle-compare adapter --engine readability < requests.jsonl > extracted.jsonl
```

Each stdin line supplies `{"id":"page-1","url":"https://example.com/article","html_path":"/absolute/saved.html"}`.
Adapter paths resolve from the process cwd (use absolute paths). Each stdout
line contains `id`, normalized `text`, and `metadata` with `title`, `authors`,
and `date`; errors return empty text plus `error`. Every response is flushed
immediately; per-page failures allow subsequent requests. Oversized/truncated
protocol input or output failures stop with nonzero exit. Diagnostics go to
stderr. Author bylines remain one-element arrays; they are not guessed apart.
Date precision and source formats differ by engine.

Each synchronous engine invocation contains panics at the extraction boundary.
A panic discards the result and records the exact fixture ID, engine, and a
bounded quoted panic cause without a stack trace or input HTML dump. The adapter
keeps that page in the evaluation denominator with empty text and continues;
reports keep the failed engine record, omit its agreement pairs, continue other
engines/pages, and return aggregate failure. Each engine parses fresh input.
Recovery cannot contain a panic in a goroutine spawned by a dependency, process
termination, or runtime fatal faults. This containment does not fix engine bugs.

Use the upstream evaluator's documented corpus alignment/scoring procedure for
quality results and record the corpus, annotations, evaluator version, engine
pins/options, and normalization. This tool downloads no corpora and makes no
quality claim. The sample corpus overrepresents Defuddle regression cases; add
locally saved news, docs, code, social, tables, short pages, and adversarial
boilerplate with independent annotations before drawing general conclusions.

Owner verification commands (run after the complete slice):

```sh
GOWORK=off go build ./...
GOWORK=off go test ./...
GOWORK=off go vet ./...
# From the repository root:
go test -run '^$' -bench '^BenchmarkRealWorld_' -benchtime=1x -count=1 .
```
