package model

import (
	"encoding/json"
	"regexp"
	"strconv"
	"strings"
)

// jsSpace and jsAny are JavaScript's `\s` and `.`, which the old renderer's regular expressions used.
const (
	jsSpace = `[\t\n\v\f\r \x{a0}\x{1680}\x{2000}-\x{200a}\x{2028}\x{2029}\x{202f}\x{205f}\x{3000}\x{feff}]`
	jsAny   = `[^\n\r\x{2028}\x{2029}]`
)

var (
	legacyImageLine = regexp.MustCompile(`^!\[([^\]]*)\]\(([^)]+)\)$`)
	legacyBullet    = regexp.MustCompile(`^-` + jsSpace + `+(` + jsAny + `*)$`)
	legacyNumbered  = regexp.MustCompile(`^\d+\.` + jsSpace + `+(` + jsAny + `*)$`)
)

func isJSSpace(r rune) bool {
	switch {
	case r == ' ' || (r >= '\t' && r <= '\r'), r == 0xa0, r == 0x1680, r >= 0x2000 && r <= 0x200a,
		r == 0x2028, r == 0x2029, r == 0x202f, r == 0x205f, r == 0x3000, r == 0xfeff:
		return true
	}
	return false
}

type legacyLine struct {
	list string
	text string
	raw  string
}

// DocFromLegacyText converts the old markdown-ish string with the rules web/components/latex.tsx rendered it by:
// `$...$` is split first and only the pieces between formulas get **bold** and *italic*; a run of two or more
// consecutive list lines is one list of the first line's kind, bullet and numbered lines mixed; a lone list line
// stays a paragraph with its marker.
func DocFromLegacyText(text string) RichDoc {
	doc := EmptyDoc()
	if text == "" {
		return doc
	}
	text = strings.NewReplacer("\r\n", "\n", "\r", "\n").Replace(text)
	var buf []legacyLine
	flush := func() {
		if len(buf) == 1 {
			doc.Content = append(doc.Content, RichNode{Type: NodeParagraph, Content: legacyInline(buf[0].raw)})
		} else if len(buf) > 1 {
			list := RichNode{Type: buf[0].list}
			for _, item := range buf {
				list.Content = append(list.Content, RichNode{Type: NodeListItem, Content: []RichNode{
					{Type: NodeParagraph, Content: legacyInline(item.text)},
				}})
			}
			doc.Content = append(doc.Content, list)
		}
		buf = nil
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

// legacyInline splits `\$[^$]+\$` first; only the pieces between formulas get bold and italic. A formula whose
// LaTeX is blank stays literal text, since the editor has no empty formula.
func legacyInline(line string) []RichNode {
	var out []RichNode
	runes := []rune(line)
	start := 0
	for i := 0; i < len(runes); {
		if runes[i] != '$' {
			i++
			continue
		}
		j := indexRune(runes, '$', i+1)
		if j <= i+1 {
			i++
			continue
		}
		out = append(out, legacyEmphasisRun(runes[start:i])...)
		if strings.TrimSpace(string(runes[i+1:j])) != "" {
			out = append(out, RichNode{Type: NodeMath, Attrs: map[string]any{AttrLatex: string(runes[i+1 : j])}})
		} else {
			out = append(out, RichNode{Type: NodeText, Text: string(runes[i : j+1])})
		}
		i = j + 1
		start = i
	}
	return append(out, legacyEmphasisRun(runes[start:])...)
}

// legacyEmphasisRun reads **bold** and *italic* whose markers hug their content.
func legacyEmphasisRun(runes []rune) []RichNode {
	var out []RichNode
	var plain strings.Builder
	flushPlain := func() {
		if plain.Len() > 0 {
			out = append(out, RichNode{Type: NodeText, Text: plain.String()})
			plain.Reset()
		}
	}
	for i := 0; i < len(runes); {
		if runes[i] == '*' {
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
	if end < 0 || end == from || isJSSpace(runes[from]) || isJSSpace(runes[end-1]) {
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

// DocLegacyText is the projection written to the text columns (`content`, `explanation`, option `label`), which a
// down migration keeps and the `q` search reads: formulas as `$latex$`, **bold**, *italic*, lists as `- ` and
// `1. ` lines, legacy URL images as `![alt](src)` lines, table rows as cells joined by ` | `. Shapes and uploaded
// images have no text form and are omitted, and underline is dropped.
func DocLegacyText(doc RichDoc) string {
	return strings.TrimSpace(strings.Join(legacyBlockLines(doc.Content), "\n"))
}

func legacyBlockLines(nodes []RichNode) []string {
	var lines []string
	for _, n := range nodes {
		switch n.Type {
		case NodeParagraph:
			lines = append(lines, legacyInlineText(n.Content))
		case NodeBulletList, NodeOrderedList:
			for i, item := range n.Content {
				prefix := "- "
				if n.Type == NodeOrderedList {
					prefix = strconv.Itoa(i+1) + ". "
				}
				lines = append(lines, prefix+legacyFlat(item.Content))
			}
		case NodeTable:
			for _, row := range n.Content {
				cells := make([]string, 0, len(row.Content))
				for _, cell := range row.Content {
					cells = append(cells, legacyFlat(cell.Content))
				}
				lines = append(lines, strings.Join(cells, " | "))
			}
		case NodeImage:
			if src, ok := n.Attrs[AttrSrc].(string); ok {
				alt, _ := n.Attrs[AttrAlt].(string)
				lines = append(lines, "!["+alt+"]("+src+")")
			}
		}
	}
	return lines
}

// legacyFlat is the one-line text of a list item or a table cell.
func legacyFlat(nodes []RichNode) string {
	return strings.Join(strings.Fields(strings.Join(legacyBlockLines(nodes), " ")), " ")
}

func legacyInlineText(nodes []RichNode) string {
	var b strings.Builder
	for _, n := range nodes {
		switch n.Type {
		case NodeText:
			b.WriteString(legacyMarked(n))
		case NodeHardBreak:
			b.WriteByte('\n')
		case NodeMath:
			latex, _ := n.Attrs[AttrLatex].(string)
			b.WriteString("$" + latex + "$")
		}
	}
	return b.String()
}

// legacyMarked wraps the text in the markers the old renderer read. Markers must hug their content, and a `*`
// inside cannot be written back, so such texts stay plain.
func legacyMarked(n RichNode) string {
	marker := ""
	for _, mark := range n.Marks {
		switch mark.Type {
		case MarkBold:
			marker = "**"
		case MarkItalic:
			if marker == "" {
				marker = "*"
			}
		}
	}
	core := strings.TrimFunc(n.Text, isJSSpace)
	if marker == "" || core == "" || strings.Contains(core, "*") {
		return n.Text
	}
	at := strings.Index(n.Text, core)
	return n.Text[:at] + marker + core + marker + n.Text[at+len(core):]
}

// DocFromColumn reads a stored document. A NULL column is a row from before documents existed: the result
// carries the legacy text and no Type, for the service to convert.
func DocFromColumn(raw []byte, legacy string) (RichDoc, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return RichDoc{Legacy: legacy}, nil
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

// OptionsWithDocs marks every option stored without a document as legacy text, for the service to convert.
func OptionsWithDocs(options []QuestionOption) []QuestionOption {
	for i := range options {
		if options[i].Doc.Type == "" {
			options[i].Doc = RichDoc{Legacy: options[i].Label}
		}
	}
	return options
}
