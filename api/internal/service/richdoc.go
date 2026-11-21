package service

import (
	"encoding/json"
	"math"
	"strings"
	"unicode/utf8"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/model"
)

// Document field reasons; the transport turns each into the copy the editor shows.
const (
	ReasonDocInvalid  = "doc_invalid"
	ReasonDocNode     = "doc_node"
	ReasonDocMark     = "doc_mark"
	ReasonDocAttr     = "doc_attr"
	ReasonDocTooLarge = "doc_too_large"
	ReasonDocAsset    = "doc_asset"
)

const (
	maxDocNodes     = 2000
	maxDocDepth     = 12
	maxDocBytes     = 100 << 10
	maxTableRows    = 30
	maxTableCols    = 12
	maxSpan         = 12
	maxLatexLen     = 2000
	maxCaptionLen   = 200
	maxAltLen       = 200
	maxImageSrcLen  = 2000
	maxAttrStartVal = 1 << 20
)

var (
	docBlocks  = map[string]bool{model.NodeParagraph: true, model.NodeBulletList: true, model.NodeOrderedList: true, model.NodeTable: true, model.NodeShape: true, model.NodeImage: true}
	itemBlocks = map[string]bool{model.NodeParagraph: true, model.NodeBulletList: true, model.NodeOrderedList: true}
	cellBlocks = map[string]bool{model.NodeParagraph: true, model.NodeBulletList: true, model.NodeOrderedList: true, model.NodeShape: true, model.NodeImage: true}
	shapeKinds = map[string]bool{"kubus": true, "balok": true, "tabung": true, "kerucut": true, "bola": true, "limas": true, "prisma": true}
	markOrder  = []string{model.MarkBold, model.MarkItalic, model.MarkUnderline}
)

// docRules says which `image.attrs.src` a document may carry and how strict the limits are. The zero value is
// what a client may write on create: no src at all.
type docRules struct {
	// legacy is a document converted from old text or imported from a spreadsheet: any http(s) src is kept and
	// an alt over the limit is cut instead of refused.
	legacy bool
	// srcs are the src values the question already holds; a client may keep them on update, never add one.
	srcs map[string]bool
}

func (r docRules) allowsSrc(src string) bool { return r.legacy || r.srcs[src] }

// docSrcs lists every image src in the documents, for the update rule above.
func docSrcs(docs ...model.RichDoc) map[string]bool {
	out := map[string]bool{}
	var walk func([]model.RichNode)
	walk = func(nodes []model.RichNode) {
		for _, n := range nodes {
			if src, ok := n.Attrs[model.AttrSrc].(string); ok && n.Type == model.NodeImage {
				out[src] = true
			}
			walk(n.Content)
		}
	}
	for _, d := range docs {
		walk(d.Content)
	}
	return out
}

func normalizeDoc(doc model.RichDoc) (model.RichDoc, string) {
	return normalizeDocWith(doc, docRules{})
}

// normalizeDocWith checks a document against the allow-list and returns its canonical form: unknown attributes
// are dropped, leaf nodes carry no content, marks are in a fixed order, adjacent text with the same marks is one
// node and trailing empty paragraphs (which the editor appends after a list, table, shape or image) are gone, so
// equal documents compare equal.
func normalizeDocWith(doc model.RichDoc, rules docRules) (model.RichDoc, string) {
	if doc.Type == "" && doc.Content == nil {
		return model.EmptyDoc(), ""
	}
	if doc.Type != model.NodeDoc {
		return model.RichDoc{}, ReasonDocInvalid
	}
	w := &docWalker{rules: rules}
	content, reason := w.blocks(doc.Content, docBlocks, 1)
	if reason != "" {
		return model.RichDoc{}, reason
	}
	for len(content) > 0 && content[len(content)-1].Type == model.NodeParagraph && len(content[len(content)-1].Content) == 0 {
		content = content[:len(content)-1]
	}
	out := model.RichDoc{Type: model.NodeDoc, Content: content}
	raw, err := json.Marshal(out)
	if err != nil {
		return model.RichDoc{}, ReasonDocInvalid
	}
	if len(raw) > maxDocBytes {
		return model.RichDoc{}, ReasonDocTooLarge
	}
	return out, ""
}

func validatedDoc(doc model.RichDoc, required bool, maxRunes int, lengthReason string) (model.RichDoc, string) {
	return validatedDocWith(doc, required, maxRunes, lengthReason, docRules{})
}

// validatedDocWith normalises and applies the text rules; lengthReason is what an empty or overlong document reports.
func validatedDocWith(doc model.RichDoc, required bool, maxRunes int, lengthReason string, rules docRules) (model.RichDoc, string) {
	out, reason := normalizeDocWith(doc, rules)
	if reason != "" {
		return model.RichDoc{}, reason
	}
	if model.DocEmpty(out) {
		if required {
			return model.RichDoc{}, lengthReason
		}
		return model.EmptyDoc(), ""
	}
	if utf8.RuneCountInString(model.DocText(out)) > maxRunes {
		return model.RichDoc{}, lengthReason
	}
	return out, ""
}

type docWalker struct {
	nodes int
	rules docRules
}

func (w *docWalker) blocks(in []model.RichNode, allowed map[string]bool, depth int) ([]model.RichNode, string) {
	out := make([]model.RichNode, 0, len(in))
	for _, n := range in {
		node, reason := w.node(n, allowed, depth)
		if reason != "" {
			return nil, reason
		}
		out = append(out, node)
	}
	return out, ""
}

func (w *docWalker) node(n model.RichNode, allowed map[string]bool, depth int) (model.RichNode, string) {
	w.nodes++
	if w.nodes > maxDocNodes || depth > maxDocDepth {
		return model.RichNode{}, ReasonDocTooLarge
	}
	if !allowed[n.Type] {
		return model.RichNode{}, ReasonDocNode
	}
	out := model.RichNode{Type: n.Type}
	var reason string
	switch n.Type {
	case model.NodeText:
		return w.text(n)
	case model.NodeHardBreak:
		return out, ""
	case model.NodeMath:
		out.Attrs, reason = mathAttrs(n.Attrs)
		return out, reason
	case model.NodeShape:
		out.Attrs, reason = shapeAttrs(n.Attrs)
		return out, reason
	case model.NodeImage:
		out.Attrs, reason = imageAttrs(n.Attrs, w.rules)
		return out, reason
	case model.NodeParagraph:
		out.Content, reason = w.blocks(n.Content, map[string]bool{model.NodeText: true, model.NodeHardBreak: true, model.NodeMath: true}, depth+1)
		out.Content = mergeText(out.Content)
	case model.NodeBulletList, model.NodeOrderedList:
		if n.Type == model.NodeOrderedList {
			out.Attrs, reason = startAttrs(n.Attrs)
			if reason != "" {
				return model.RichNode{}, reason
			}
		}
		if len(n.Content) == 0 {
			return model.RichNode{}, ReasonDocNode
		}
		out.Content, reason = w.blocks(n.Content, map[string]bool{model.NodeListItem: true}, depth+1)
	case model.NodeListItem:
		if len(n.Content) == 0 {
			return model.RichNode{}, ReasonDocNode
		}
		out.Content, reason = w.blocks(n.Content, itemBlocks, depth+1)
	case model.NodeTable:
		if len(n.Content) == 0 {
			return model.RichNode{}, ReasonDocNode
		}
		if len(n.Content) > maxTableRows {
			return model.RichNode{}, ReasonDocTooLarge
		}
		out.Content, reason = w.blocks(n.Content, map[string]bool{model.NodeTableRow: true}, depth+1)
	case model.NodeTableRow:
		if len(n.Content) == 0 {
			return model.RichNode{}, ReasonDocNode
		}
		if len(n.Content) > maxTableCols {
			return model.RichNode{}, ReasonDocTooLarge
		}
		out.Content, reason = w.blocks(n.Content, map[string]bool{model.NodeTableHeader: true, model.NodeTableCell: true}, depth+1)
	case model.NodeTableHeader, model.NodeTableCell:
		out.Attrs, reason = spanAttrs(n.Attrs)
		if reason != "" {
			return model.RichNode{}, reason
		}
		if len(n.Content) == 0 {
			return model.RichNode{}, ReasonDocNode
		}
		out.Content, reason = w.blocks(n.Content, cellBlocks, depth+1)
	default:
		return model.RichNode{}, ReasonDocNode
	}
	if reason != "" {
		return model.RichNode{}, reason
	}
	if len(out.Content) == 0 {
		out.Content = nil
	}
	return out, ""
}

func (w *docWalker) text(n model.RichNode) (model.RichNode, string) {
	if n.Text == "" {
		return model.RichNode{}, ReasonDocNode
	}
	have := map[string]bool{}
	for _, mark := range n.Marks {
		switch mark.Type {
		case model.MarkBold, model.MarkItalic, model.MarkUnderline:
		default:
			return model.RichNode{}, ReasonDocMark
		}
		if have[mark.Type] {
			return model.RichNode{}, ReasonDocMark
		}
		have[mark.Type] = true
	}
	out := model.RichNode{Type: model.NodeText, Text: n.Text}
	for _, name := range markOrder {
		if have[name] {
			out.Marks = append(out.Marks, model.RichMark{Type: name})
		}
	}
	return out, ""
}

// mergeText joins neighbouring text nodes with the same marks, the way the editor holds them.
func mergeText(nodes []model.RichNode) []model.RichNode {
	out := nodes[:0:0]
	for _, n := range nodes {
		if last := len(out) - 1; last >= 0 && n.Type == model.NodeText && out[last].Type == model.NodeText && sameMarks(out[last].Marks, n.Marks) {
			out[last].Text += n.Text
			continue
		}
		out = append(out, n)
	}
	return out
}

func sameMarks(a, b []model.RichMark) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Type != b[i].Type {
			return false
		}
	}
	return true
}

func mathAttrs(in map[string]any) (map[string]any, string) {
	latex, ok := in[model.AttrLatex].(string)
	latex = strings.TrimSpace(latex)
	if n := utf8.RuneCountInString(latex); !ok || n < 1 || n > maxLatexLen {
		return nil, ReasonDocAttr
	}
	return map[string]any{model.AttrLatex: latex}, ""
}

func shapeAttrs(in map[string]any) (map[string]any, string) {
	kind, ok := in[model.AttrKind].(string)
	if !ok || !shapeKinds[kind] {
		return nil, ReasonDocAttr
	}
	out := map[string]any{model.AttrKind: kind}
	if raw, present := in[model.AttrCaption]; present && raw != nil {
		caption, ok := raw.(string)
		caption = strings.TrimSpace(caption)
		if !ok || utf8.RuneCountInString(caption) > maxCaptionLen {
			return nil, ReasonDocAttr
		}
		if caption != "" {
			out[model.AttrCaption] = caption
		}
	}
	return out, ""
}

func imageAttrs(in map[string]any, rules docRules) (map[string]any, string) {
	idRaw, srcRaw := in[model.AttrAssetID], in[model.AttrSrc]
	if (idRaw != nil) == (srcRaw != nil) {
		return nil, ReasonDocAttr
	}
	out := map[string]any{}
	if idRaw != nil {
		id, ok := idRaw.(string)
		if !ok || !isUUID(id) {
			return nil, ReasonDocAttr
		}
		out[model.AttrAssetID] = strings.ToLower(id)
	} else {
		src, ok := srcRaw.(string)
		if !ok || utf8.RuneCountInString(src) > maxImageSrcLen || !(strings.HasPrefix(src, "http://") || strings.HasPrefix(src, "https://")) || !rules.allowsSrc(src) {
			return nil, ReasonDocAttr
		}
		out[model.AttrSrc] = src
	}
	if raw := in[model.AttrAlt]; raw != nil {
		alt, ok := raw.(string)
		if ok && rules.legacy && utf8.RuneCountInString(alt) > maxAltLen {
			alt = string([]rune(alt)[:maxAltLen])
		}
		if !ok || utf8.RuneCountInString(alt) > maxAltLen {
			return nil, ReasonDocAttr
		}
		if alt != "" {
			out[model.AttrAlt] = alt
		}
	}
	return out, ""
}

func startAttrs(in map[string]any) (map[string]any, string) {
	value, present := intAttr(in, model.AttrStart, 0, maxAttrStartVal)
	if present == attrBad {
		return nil, ReasonDocAttr
	}
	if present == attrSet && value != 1 {
		return map[string]any{model.AttrStart: value}, ""
	}
	return nil, ""
}

func spanAttrs(in map[string]any) (map[string]any, string) {
	out := map[string]any{}
	for _, key := range []string{model.AttrColspan, model.AttrRowspan} {
		value, state := intAttr(in, key, 1, maxSpan)
		switch {
		case state == attrBad:
			return nil, ReasonDocAttr
		case state == attrSet && value != 1:
			out[key] = value
		}
	}
	if len(out) == 0 {
		return nil, ""
	}
	return out, ""
}

const (
	attrAbsent = iota
	attrSet
	attrBad
)

// intAttr reads a whole number in [min, max]; null and a missing key are absent.
func intAttr(in map[string]any, key string, min, max int) (int, int) {
	raw, ok := in[key]
	if !ok || raw == nil {
		return 0, attrAbsent
	}
	var f float64
	switch v := raw.(type) {
	case float64:
		f = v
	case int:
		f = float64(v)
	default:
		return 0, attrBad
	}
	if f != math.Trunc(f) || f < float64(min) || f > float64(max) {
		return 0, attrBad
	}
	return int(f), attrSet
}

// resolveDoc turns what a repository read into a document: a stored one is already canonical, legacy text is
// converted and normalised exactly like client input, so an unedited row saves back unchanged. Text that cannot
// be made valid (more than 2000 nodes, say) is shown as converted; saving it is refused with the usual reason.
func resolveDoc(doc model.RichDoc) model.RichDoc {
	if doc.Type != "" {
		doc.Legacy = ""
		return doc
	}
	return docOfLegacyText(doc.Legacy)
}

func docOfLegacyText(text string) model.RichDoc {
	converted := model.DocFromLegacyText(text)
	if out, reason := normalizeDocWith(converted, docRules{legacy: true}); reason == "" {
		if model.DocEmpty(out) {
			return model.EmptyDoc()
		}
		return out
	}
	return converted
}

func resolveOptions(options []model.QuestionOption) {
	for i := range options {
		options[i].Doc = resolveDoc(options[i].Doc)
	}
}

func resolveQuestion(q *model.Question) {
	q.Content = resolveDoc(q.Content)
	q.Explanation = resolveDoc(q.Explanation)
	resolveOptions(q.Options)
}

func resolveQuestions(questions []model.Question) {
	for i := range questions {
		resolveQuestion(&questions[i])
	}
}

func resolveAttempt(aq *model.AttemptQuestion) {
	aq.Content = resolveDoc(aq.Content)
	resolveOptions(aq.Options)
}

func resolveTryout(t *model.Tryout) {
	for i := range t.Subtests {
		for j := range t.Subtests[i].Questions {
			a := &t.Subtests[i].Questions[j]
			a.Content = resolveDoc(a.Content)
		}
	}
}

func resolvePrint(p *model.PrintTryout) {
	for i := range p.Subtests {
		for j := range p.Subtests[i].Questions {
			q := &p.Subtests[i].Questions[j]
			q.Content = resolveDoc(q.Content)
			for k := range q.Options {
				q.Options[k] = resolveDoc(q.Options[k])
			}
		}
	}
}
