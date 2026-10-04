package elements

import (
	"strings"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/net/html/atom"
)

// RoleProcessor handles conversion of ARIA roles to semantic HTML elements
type RoleProcessor struct {
	doc   *goquery.Document
	scope *goquery.Selection
}

// RoleProcessingOptions configures role processing behavior
type RoleProcessingOptions struct {
	ConvertParagraphs bool
	ConvertLists      bool
	ConvertButtons    bool
	ConvertLinks      bool
}

// DefaultRoleProcessingOptions returns default options for role processing
func DefaultRoleProcessingOptions() *RoleProcessingOptions {
	return &RoleProcessingOptions{
		ConvertParagraphs: true,
		ConvertLists:      true,
		ConvertButtons:    true,
		ConvertLinks:      true,
	}
}

// NewRoleProcessor creates a new role processor
func NewRoleProcessor(doc *goquery.Document) *RoleProcessor {
	return &RoleProcessor{
		doc: doc,
	}
}

// ProcessRoles processes all role-based elements in the document
func (p *RoleProcessor) ProcessRoles(options *RoleProcessingOptions) {
	if options == nil {
		options = DefaultRoleProcessingOptions()
	}

	if options.ConvertParagraphs {
		p.convertParagraphRoles()
	}

	if options.ConvertLists {
		p.convertListRoles()
	}

	if options.ConvertButtons {
		p.convertButtonRoles()
	}

	if options.ConvertLinks {
		p.convertLinkRoles()
	}
}

// convertParagraphRoles converts elements with role="paragraph" to <p> tags
func (p *RoleProcessor) convertParagraphRoles() {
	p.find(`[role="paragraph"]`).Each(func(_ int, s *goquery.Selection) {
		p.replaceElementTag(s, "p")
	})
}

// convertListRoles converts role-based lists to semantic HTML lists
func (p *RoleProcessor) convertListRoles() {
	// Convert role="list" to <ol> or <ul>
	p.find(`[role="list"]`).Each(func(_ int, listElement *goquery.Selection) {
		// Check if it's an ordered list by looking for numbered items
		isOrdered := p.isOrderedList(listElement)

		var newTag string
		if isOrdered {
			newTag = "ol"
		} else {
			newTag = "ul"
		}

		// Convert list items first
		listElement.Find(`[role="listitem"]`).Each(func(_ int, itemElement *goquery.Selection) {
			p.convertListItem(itemElement)
		})

		// Convert the list container
		p.replaceElementTag(listElement, newTag)
	})
}

// isOrderedList determines if a role-based list should be an ordered list
func (p *RoleProcessor) isOrderedList(listElement *goquery.Selection) bool {
	// Look for numbered labels in list items
	hasNumbers := false
	listElement.Find(`[role="listitem"]`).Each(func(_ int, itemElement *goquery.Selection) {
		labelElement := itemElement.Find(".label").First()
		if labelElement.Length() > 0 {
			labelText := strings.TrimSpace(labelElement.Text())
			// Check for patterns like "1)", "2.", "1.", etc.
			if strings.Contains(labelText, ")") || strings.Contains(labelText, ".") {
				hasNumbers = true
			}
		}
	})
	return hasNumbers
}

// convertListItem converts a role="listitem" to <li>
func (p *RoleProcessor) convertListItem(itemElement *goquery.Selection) {
	// Remove label elements (like "1)", "2)", etc.)
	itemElement.Find(".label").Remove()

	// Convert content divs to paragraphs if they have role="paragraph"
	itemElement.Find(`[role="paragraph"]`).Each(func(_ int, s *goquery.Selection) {
		p.replaceElementTag(s, "p")
	})

	// Convert the list item itself
	p.replaceElementTag(itemElement, "li")
}

// convertButtonRoles converts elements with role="button" to <button> tags
func (p *RoleProcessor) convertButtonRoles() {
	p.find(`[role="button"]`).Each(func(_ int, s *goquery.Selection) {
		p.replaceElementTag(s, "button")
	})
}

// convertLinkRoles converts elements with role="link" to <a> tags
func (p *RoleProcessor) convertLinkRoles() {
	p.find(`[role="link"]`).Each(func(_ int, s *goquery.Selection) {
		p.replaceElementTag(s, "a")
	})
}

// replaceElementTag replaces an element's tag while preserving content and attributes
func (p *RoleProcessor) replaceElementTag(s *goquery.Selection, newTagName string) {
	if s.Length() == 0 {
		return
	}

	// Mutate the node so attributes and child markup keep their raw values.
	node := s.Get(0)
	node.Data = newTagName
	node.DataAtom = atom.Lookup([]byte(newTagName))
	s.RemoveAttr("role")
}

// ProcessRolesInScope applies configurable conversions only within scope.
func ProcessRolesInScope(scope *goquery.Selection, options *RoleProcessingOptions) {
	if scope == nil || scope.Length() == 0 {
		return
	}
	processor := &RoleProcessor{scope: scope}
	processor.ProcessRoles(options)
}

func (p *RoleProcessor) find(selector string) *goquery.Selection {
	if p.scope != nil {
		return p.scope.Find(selector).AddSelection(p.scope.Filter(selector))
	}
	return p.doc.Find(selector)
}
