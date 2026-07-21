package markdown

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"

	"github.com/JohannesKaufmann/html-to-markdown/v2/converter"
	"golang.org/x/net/html"
)

// --- Renderers ---

func renderCodeBlock(_ converter.Context, w converter.Writer, n *html.Node) converter.RenderStatus {
	if n.Type != html.ElementNode || n.Data != "pre" {
		return converter.RenderTryNext
	}

	// Find the <code> child
	var codeNode *html.Node
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode && c.Data == "code" {
			codeNode = c
			break
		}
	}
	if codeNode == nil {
		return converter.RenderTryNext
	}

	// Detect language from data attributes (code, then pre)
	lang := getAttr(codeNode, "data-lang")
	if lang == "" {
		lang = getAttr(codeNode, "data-language")
	}
	if lang == "" {
		lang = getAttr(n, "data-lang")
	}
	if lang == "" {
		lang = getAttr(n, "data-language")
	}
	// Detect from class tokens (code, then pre)
	if lang == "" {
		lang = extractLangFromClass(getAttr(codeNode, "class"))
	}
	if lang == "" {
		lang = extractLangFromClass(getAttr(n, "class"))
	}

	code := extractText(codeNode)
	code = strings.TrimSpace(code)

	// Choose a fence that doesn't conflict with the code content.
	fence := "```"
	if strings.Contains(code, "```") {
		fence = "````"
	}

	w.WriteString("\n" + fence + lang + "\n")
	w.WriteString(code)
	w.WriteString("\n" + fence + "\n")
	return converter.RenderSuccess
}

func renderFigure(ctx converter.Context, w converter.Writer, n *html.Node) converter.RenderStatus {
	if n.Type != html.ElementNode || n.Data != "figure" {
		return converter.RenderTryNext
	}

	imgNode, captionNode := figureImgAndCaption(n)

	if imgNode == nil {
		return converter.RenderTryNext
	}

	alt := getAttr(imgNode, "alt")
	src := getBestImageSrc(imgNode)

	var caption string
	if captionNode != nil {
		var capBuf bytes.Buffer
		ctx.RenderChildNodes(ctx, &capBuf, captionNode)
		caption = strings.TrimSpace(capBuf.String())
	}

	fmt.Fprintf(w, "\n![%s](%s)\n", alt, src)
	if caption != "" {
		w.WriteString("\n" + caption + "\n")
	}
	w.WriteString("\n")
	return converter.RenderSuccess
}

// figureImgAndCaption returns the first <img> and first <figcaption> child nodes of n.
func figureImgAndCaption(n *html.Node) (imgNode, captionNode *html.Node) {
	walkChildren(n, func(child *html.Node) bool {
		if child.Type == html.ElementNode {
			if child.Data == "img" && imgNode == nil {
				imgNode = child
			}
			if child.Data == "figcaption" && captionNode == nil {
				captionNode = child
			}
		}
		return true
	})
	return imgNode, captionNode
}

func renderHighlight(ctx converter.Context, w converter.Writer, n *html.Node) converter.RenderStatus {
	if n.Type != html.ElementNode || n.Data != "mark" {
		return converter.RenderTryNext
	}
	var buf bytes.Buffer
	ctx.RenderChildNodes(ctx, &buf, n)
	content := strings.TrimSpace(buf.String())
	w.WriteString("==" + content + "==")
	return converter.RenderSuccess
}

var (
	youtubeRe    = regexp.MustCompile(`(?:youtube\.com|youtube-nocookie\.com|youtu\.be)/(?:embed/|watch\?v=)?([a-zA-Z0-9_-]+)`)
	tweetRe      = regexp.MustCompile(`(?:twitter\.com|x\.com)/([^/]+)/status/([0-9]+)`)
	tweetEmbedRe = regexp.MustCompile(`(?:platform\.)?twitter\.com/embed/Tweet\.html\?.*?id=([0-9]+)`)
)

func renderEmbed(_ converter.Context, w converter.Writer, n *html.Node) converter.RenderStatus {
	if n.Type != html.ElementNode || n.Data != "iframe" {
		return converter.RenderTryNext
	}

	src := getAttr(n, "src")
	if src == "" {
		return converter.RenderTryNext
	}

	if m := youtubeRe.FindStringSubmatch(src); m != nil {
		w.WriteString("\n![](https://www.youtube.com/watch?v=" + m[1] + ")\n")
		return converter.RenderSuccess
	}

	// Direct tweet URL: /user/status/id
	if m := tweetRe.FindStringSubmatch(src); m != nil {
		w.WriteString("\n![](https://x.com/" + m[1] + "/status/" + m[2] + ")\n")
		return converter.RenderSuccess
	}

	// Platform embed: ?id=
	if m := tweetEmbedRe.FindStringSubmatch(src); m != nil {
		w.WriteString("\n![](https://x.com/i/status/" + m[1] + ")\n")
		return converter.RenderSuccess
	}

	return converter.RenderTryNext
}
