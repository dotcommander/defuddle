package constants

// ExactSelectors are selectors to be removed exactly
// JavaScript original code: (first part of EXACT_SELECTORS array)
var ExactSelectors = []string{
	// scripts, styles
	"noscript",
	`script:not([type^="math/"])`,
	"style",
	"meta",
	"link",

	// empty media elements (src set by JS at runtime, not in raw HTML)
	`audio:not([src])`,

	// ads
	`.ad:not([class*="gradient"])`,
	`[class^="ad-" i]`,
	`[class$="-ad" i]`,
	`[id^="ad-" i]`,
	`[id$="-ad" i]`,
	`[role="banner" i]`,
	`[alt*="advert" i]`,
	".promo",
	".Promo",
	"#barrier-page", // ft.com
	".alert",

	// comments
	`[id="comments" i]`,
	`[id="comment" i]`,

	// cover images
	`div[class*="cover-"]`,
	`div[id*="cover-"]`,

	// breadcrumbs (custom web component tag)
	"ads-breadcrumbs",

	// header, nav
	// Exclude headers that contain paragraph text — some sites (e.g. Webflow blogs)
	// use <header> as the main content wrapper rather than a navigation container.
	"header:not(:has(p))",
	`.header:not(.banner)`,
	"#header",
	"#Header",
	"#banner",
	"#Banner",
	"nav",
	".navigation",
	"#navigation",
	// ".hero", // see issue #132 — too broad
	`[role="navigation" i]`,
	`[role="dialog" i]`,
	`[role*="complementary" i]`,
	`[class*="pagination" i]`,
	".menu",
	// "#menu", // see issue #106 — too broad
	"#siteSub",
	".previous",

	// metadata
	".author",
	".Author",
	`[class$="_bio"]`,
	"#categories",
	".contributor",
	".date",
	"#date",
	"[data-date]",
	".entry-meta",
	".meta",
	".tags",
	"#tags",
	`[rel="tag"]`,
	".toc",
	".Toc",
	"#toc",
	".headline",
	"#headline",
	"#title",
	"#Title",
	"#articleTag",
	// "[href*=\"/category\"]", // see issue #131
	// "[href*=\"/categories\"]", // see issue #131
	`[href*="/tag/"]`,
	`[href*="/tags/"]`,
	// "[href*=\"/topics\"]", // see issue #131
	`[href*="/author/"]`,
	`[href*="/author?"]`,
	`[href$="/author"]`,
	`a[href*="copyright.com"]`,
	`a[href*="google.com/preferences"]`,
	`[href*="#toc"]`,
	`[href="#top"]`,
	`[href="#Top"]`,
	`[href="#page-header"]`,
	`[href="#content"]`,
	`[href="#site-content"]`,
	`[href="#main-content"]`,
	`[href^="#main"]`,
	`[src*="author"]`,

	// footer
	"footer",

	// inputs, forms, elements
	".aside",
	`aside:not([class*="callout"])`,
	"button",
	"canvas",
	"date",
	"dialog",
	"fieldset",
	"form",
	`input:not([type="checkbox"])`,
	"label",
	"option",
	"select",
	`[role="listbox"]`,
	`[role="option"]`,
	"textarea",
	"time",
	"relative-time",

	// hidden
	"[hidden]",
	`[aria-hidden="true"]:not([class*="math"])`,
	// Note: [style*="display: none"] removed — substring match causes false positives
	// with CSS custom properties. The removeHiddenElements step handles inline style
	// detection with proper regex.
	".hidden",
	".invisible",

	// iframes — keep YouTube, Vimeo, Twitter/X, and Datawrapper embeds
	"instaread-player",
	`iframe:not([src*="youtube"]):not([src*="youtu.be"]):not([src*="vimeo"]):not([src*="twitter"]):not([src*="x.com"]):not([src*="datawrapper"])`,

	// logos
	`[class="logo" i]`,
	"#logo",
	"#Logo",

	// newsletter
	"#newsletter",
	"#Newsletter",
	".subscribe",

	// hidden for print
	".noprint",
	`[data-print-layout="hide" i]`,
	`[data-block="donotprint" i]`,

	// footnotes, citations
	`[class*="clickable-icon" i]`,
	`li span[class*="ltx_tag" i][class*="ltx_tag_item" i]`,
	`a[href^="#"][class*="anchor" i]`,
	`a[href^="#"][class*="ref" i]:not(.ltx_ref)`,

	// link lists
	`[data-container*="most-viewed" i]`,

	// sidebar
	".sidebar",
	".Sidebar",
	"#sidebar",
	"#Sidebar",
	"#side-bar",
	"#sitesub",

	// skip links
	`[data-link-name*="skip" i]`,
	`[aria-label*="skip" i]`,

	// other
	".copyright",
	"#copyright",
	".licensebox",
	"#page-info",
	"#rss",
	"#feed",
	".gutter",
	"#primaryaudio", // NPR
	"#NYT_ABOVE_MAIN_CONTENT_REGION",
	`[data-testid="photoviewer-children-figure"] > span`, // New York Times
	"table.infobox",
	`[data-optimizely="related-articles-section" i]`, // The Economist
	`[data-orientation="vertical"]`,

	// GitHub - avoid duplicate metadata
	".gh-header-sticky",                     // GitHub
	`[data-testid="issue-metadata-sticky"]`, // GitHub

	// Substack - layout wrappers that are not content
	`.pencraft:not(.pc-display-contents)`,
}

// TestAttributes are attributes to test against for partial matches
// JavaScript original code:
// export const TEST_ATTRIBUTES = [
//
//	'class',
//	'id',
//	'data-test',
//	'data-testid',
//	'data-test-id',
//	'data-qa',
//	'data-cy'
//
// ];
var TestAttributes = []string{
	"class",
	"id",
	"data-component",
	"data-test",
	"data-testid",
	"data-test-id",
	"data-qa",
	"data-cy",
}
