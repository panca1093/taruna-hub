package model

import (
	"encoding/json"
	"regexp"
	"strings"
	"unicode"

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

var (
	legacyImageLine = regexp.MustCompile(`^!\[([^\]]*)\]\(([^)]+)\)$`)
	legacyBullet    = regexp.MustCompile(`^-\s+(.*)$`)
	legacyNumbered  = regexp.MustCompile(`^\d+\.\s+(.*)$`)
)

type legacyLine struct {
	list string
	text string
	raw  string
}

// DocFromLegacyText converts the old markdown-ish string; it mirrors what web/components/latex.tsx rendered.
func DocFromLegacyText(text string) RichDoc {
	doc := EmptyDoc()
	if text == "" {
		return doc
	}
	var buf []legacyLine
	flush := func() {
		for len(buf) > 0 {
			end := 1
			for end < len(buf) && buf[end].list == buf[0].list {
				end++
			}
			run := buf[:end]
			buf = buf[end:]
			if len(run) == 1 {
				doc.Content = append(doc.Content, RichNode{Type: NodeParagraph, Content: legacyInline(run[0].raw)})
				continue
			}
			list := RichNode{Type: run[0].list}
			for _, item := range run {
				list.Content = append(list.Content, RichNode{Type: NodeListItem, Content: []RichNode{
					{Type: NodeParagraph, Content: legacyInline(item.text)},
				}})
			}
			doc.Content = append(doc.Content, list)
		}
	}
	for _, line := range strings.Split(text, "\n") {
		if m := legacyImageLine.FindStringSubmatch(line); m != nil {
			flush()
			if strings.HasPrefix(m[2], "http://") || strings.HasPrefix(m[2], "https://") {
				attrs := map[string]any{AttrSrc: m[2]}
				if m[1] != "" {
					attrs[AttrAlt] = m[1]
				}
				doc.Content = append(doc.Content, RichNode{Type: NodeImage, Attrs: attrs})
			} else {
				doc.Content = append(doc.Content, RichNode{Type: NodeParagraph, Content: legacyInline(line)})
			}
			continue
		}
		if m := legacyBullet.FindStringSubmatch(line); m != nil {
			buf = append(buf, legacyLine{list: NodeBulletList, text: m[1], raw: line})
			continue
		}
		if m := legacyNumbered.FindStringSubmatch(line); m != nil {
			buf = append(buf, legacyLine{list: NodeOrderedList, text: m[1], raw: line})
			continue
		}
		flush()
		doc.Content = append(doc.Content, RichNode{Type: NodeParagraph, Content: legacyInline(line)})
	}
	flush()
	return doc
}

// legacyInline splits `$...$` first, then **bold** and *italic* whose markers hug their content.
func legacyInline(line string) []RichNode {
	var out []RichNode
	var plain strings.Builder
	flushPlain := func() {
		if plain.Len() > 0 {
			out = append(out, RichNode{Type: NodeText, Text: plain.String()})
			plain.Reset()
		}
	}
	runes := []rune(line)
	for i := 0; i < len(runes); {
		switch runes[i] {
		case '$':
			if j := indexRune(runes, '$', i+1); j > i+1 {
				flushPlain()
				out = append(out, RichNode{Type: NodeMath, Attrs: map[string]any{AttrLatex: string(runes[i+1 : j])}})
				i = j + 1
				continue
			}
		case '*':
			if body, width, mark := legacyEmphasis(runes, i); width > 0 {
				flushPlain()
				out = append(out, RichNode{Type: NodeText, Text: body, Marks: []RichMark{{Type: mark}}})
				i += width
				continue
			}
		}
		plain.WriteRune(runes[i])
		i++
	}
	flushPlain()
	return out
}

// legacyEmphasis matches the old `\*\*(?!\s)[^*]+(?<!\s)\*\*|\*(?!\s)[^*]+(?<!\s)\*` at i, bold first.
func legacyEmphasis(runes []rune, i int) (string, int, string) {
	if i+1 < len(runes) && runes[i+1] == '*' {
		if body, end, ok := hugged(runes, i+2); ok && end+1 < len(runes) && runes[end+1] == '*' {
			return body, end + 2 - i, MarkBold
		}
	}
	if body, end, ok := hugged(runes, i+1); ok {
		return body, end + 1 - i, MarkItalic
	}
	return "", 0, ""
}

// hugged reads a non-empty run without `*` from `from`, with a non-space first and last rune; end is the
// index of the `*` that stopped it.
func hugged(runes []rune, from int) (string, int, bool) {
	end := indexRune(runes, '*', from)
	if end < 0 || end == from || unicode.IsSpace(runes[from]) || unicode.IsSpace(runes[end-1]) {
		return "", 0, false
	}
	return string(runes[from:end]), end, true
}

func indexRune(runes []rune, target rune, from int) int {
	for i := from; i < len(runes); i++ {
		if runes[i] == target {
			return i
		}
	}
	return -1
}

// DocFromColumn reads a stored document; a NULL column is a row from before documents existed, so the legacy text
// is converted instead.
func DocFromColumn(raw []byte, legacy string) (RichDoc, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return DocFromLegacyText(legacy), nil
	}
	var doc RichDoc
	if err := json.Unmarshal(raw, &doc); err != nil {
		return RichDoc{}, err
	}
	if doc.Content == nil {
		doc.Content = []RichNode{}
	}
	return doc, nil
}

// OptionsWithDocs fills the document of every option stored without one from its label.
func OptionsWithDocs(options []QuestionOption) []QuestionOption {
	for i := range options {
		if options[i].Doc.Type == "" {
			options[i].Doc = DocFromLegacyText(options[i].Label)
		}
	}
	return options
}
