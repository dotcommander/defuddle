const NODE_TYPE = {
	ELEMENT_NODE: 1,
	ATTRIBUTE_NODE: 2,
	TEXT_NODE: 3,
	CDATA_SECTION_NODE: 4,
	ENTITY_REFERENCE_NODE: 5,
	ENTITY_NODE: 6,
	PROCESSING_INSTRUCTION_NODE: 7,
	COMMENT_NODE: 8,
	DOCUMENT_NODE: 9,
	DOCUMENT_TYPE_NODE: 10,
	DOCUMENT_FRAGMENT_NODE: 11,
	NOTATION_NODE: 12
};

export function isElement(node: Node): node is Element {
	return node.nodeType === NODE_TYPE.ELEMENT_NODE;
}

export function isTextNode(node: Node): node is Text {
	return node.nodeType === NODE_TYPE.TEXT_NODE;
}

export function isCommentNode(node: Node): node is Comment {
	return node.nodeType === NODE_TYPE.COMMENT_NODE;
}

export function getComputedStyle(element: Element): CSSStyleDeclaration | null {
	const win = getWindow(element.ownerDocument);
	if (!win) return null;
	return win.getComputedStyle(element);
}

export function getWindow(doc: Document): Window | null {
	// First try defaultView
	if (doc.defaultView) {
		return doc.defaultView;
	}
	
	// Then try ownerWindow
	if ((doc as any).ownerWindow) {
		return (doc as any).ownerWindow;
	}
	
	// Finally try to get window from document
	if ((doc as any).window) {
		return (doc as any).window;
	}
	
	return null;
}

export function logDebug(message: string, ...args: any[]): void {
	if (typeof window !== 'undefined' && (window as any).defuddleDebug) {
		console.log('Defuddle:', message, ...args);
	}
} 