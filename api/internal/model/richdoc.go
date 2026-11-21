package model

import (
	"encoding/json"
	"strings"

	"github.com/google/uuid"
)

// Node, mark and attribute names of the question document (Tiptap/ProseMirror JSON).
const (
	NodeDoc         = "doc"
	NodeParagraph   = "paragraph"
	NodeText        = "text"
	NodeHardBreak   = "hardBreak"
	NodeBulletList  = "bulletList"
	NodeOrderedList = "orderedList"
	NodeListItem    = "listItem"
	NodeTable       = "table"
	NodeTableRow    = "tableRow"
	NodeTableHeader = "tableHeader"
	NodeTableCell   = "tableCell"
	NodeMath        = "math"
	NodeShape       = "shape"
	NodeImage       = "image"

	MarkBold      = "bold"
	MarkItalic    = "italic"
	MarkUnderline = "underline"

	AttrLatex   = "latex"
	AttrAssetID = "asset_id"
	AttrSrc     = "src"
	AttrAlt     = "alt"
	AttrKind    = "kind"
	AttrCaption = "caption"
	AttrStart   = "start"
	AttrColspan = "colspan"
	AttrRowspan = "rowspan"
)

type RichMark struct {
	Type string `json:"type"`
}

type RichNode struct {
	Type    string         `json:"type"`
	Text    string         `json:"text,omitempty"`
	Marks   []RichMark     `json:"marks,omitempty"`
	Attrs   map[string]any `json:"attrs,omitempty"`
	Content []RichNode     `json:"content,omitempty"`
}

type RichDoc struct {
	Type    string     `json:"type"`
	Content []RichNode `json:"content"`
	// Legacy is the text column of a row stored before documents existed. A repository sets it on a document with
	// no Type and the service converts it; it never reaches the wire or the database.
	Legacy string `json:"-"`
}

// MarshalJSON keeps `content` an array, never null, for a zero value or a nil slice.
func (d RichDoc) MarshalJSON() ([]byte, error) {
	type plain RichDoc
	if d.Type == "" {
		d.Type = NodeDoc
	}
	if d.Content == nil {
		d.Content = []RichNode{}
	}
	return json.Marshal(plain(d))
}

func EmptyDoc() RichDoc { return RichDoc{Type: NodeDoc, Content: []RichNode{}} }

// DocText is the plain-text projection used for search and summaries.
func DocText(doc RichDoc) string {
	return strings.TrimSpace(joinText(doc.Content, "\n"))
}

func joinText(nodes []RichNode, sep string) string {
	parts := make([]string, 0, len(nodes))
	for _, n := range nodes {
		parts = append(parts, nodeText(n))
	}
	return strings.Join(parts, sep)
}

func nodeText(n RichNode) string {
	switch n.Type {
	case NodeText:
		return n.Text
	case NodeHardBreak:
		return "\n"
	case NodeMath:
		s, _ := n.Attrs[AttrLatex].(string)
		return s
	case NodeShape, NodeImage:
		return ""
	case NodeTableRow:
		return joinText(n.Content, " ")
	case NodeParagraph:
		return joinText(n.Content, "")
	}
	return joinText(n.Content, "\n")
}

// DocEmpty is true when nothing a student could read or see is in the document.
func DocEmpty(doc RichDoc) bool {
	return !hasVisible(doc.Content)
}

func hasVisible(nodes []RichNode) bool {
	for _, n := range nodes {
		switch n.Type {
		case NodeMath, NodeShape, NodeImage:
			return true
		case NodeText:
			if strings.TrimSpace(n.Text) != "" {
				return true
			}
		}
		if hasVisible(n.Content) {
			return true
		}
	}
	return false
}

// DocAssetIDs lists the distinct asset ids of the image nodes across the documents.
func DocAssetIDs(docs ...RichDoc) []uuid.UUID {
	seen := map[uuid.UUID]bool{}
	out := []uuid.UUID{}
	var walk func([]RichNode)
	walk = func(nodes []RichNode) {
		for _, n := range nodes {
			if n.Type == NodeImage {
				if raw, ok := n.Attrs[AttrAssetID].(string); ok {
					if id, err := uuid.Parse(raw); err == nil && !seen[id] {
						seen[id] = true
						out = append(out, id)
					}
				}
			}
			walk(n.Content)
		}
	}
	for _, d := range docs {
		walk(d.Content)
	}
	return out
}
