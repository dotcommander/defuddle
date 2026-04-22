package extractors

import "encoding/json"

// --- JSON model types for the NYT window.__preloadedData payload ---

type nytPreloadedData struct {
	InitialData *nytInitialData `json:"initialData"`
}

type nytInitialData struct {
	Data *nytData `json:"data"`
}

type nytData struct {
	Article *nytArticle `json:"article"`
}

type nytArticle struct {
	Headline       *nytHeadline `json:"headline"`
	Summary        string       `json:"summary"`
	FirstPublished string       `json:"firstPublished"`
	Bylines        []nytByline  `json:"bylines"`
	SprinkledBody  *nytBody     `json:"sprinkledBody"`
	Body           *nytBody     `json:"body"`
}

type nytHeadline struct {
	Default string `json:"default"`
}

type nytByline struct {
	Creators []nytCreator `json:"creators"`
}

type nytCreator struct {
	DisplayName string `json:"displayName"`
}

type nytBody struct {
	Content []json.RawMessage `json:"content"`
}

// nytBlock is the discriminated-union base — carries only __typename for dispatch.
type nytBlock struct {
	Typename string `json:"__typename"`
}

type nytParagraphBlock struct {
	Content []nytInline `json:"content"`
}

type nytHeadingBlock struct {
	Content []nytInline `json:"content"`
}

type nytImageBlock struct {
	Media *nytMedia `json:"media"`
}

type nytMedia struct {
	AltText string      `json:"altText"`
	Caption *nytCaption `json:"caption"`
	Credit  string      `json:"credit"`
	Crops   []nytCrop   `json:"crops"`
}

type nytCaption struct {
	Text string `json:"text"`
}

type nytCrop struct {
	Renditions []nytRendition `json:"renditions"`
}

type nytRendition struct {
	Name   string `json:"name"`
	URL    string `json:"url"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

type nytInline struct {
	Typename string      `json:"__typename"`
	Text     string      `json:"text"`
	Formats  []nytFormat `json:"formats"`
}

type nytFormat struct {
	Typename string `json:"__typename"`
	URL      string `json:"url"`
}
