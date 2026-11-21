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

// normalizeDoc checks a document against the allow-list and returns its canonical form: unknown attributes are
// dropped, leaf nodes carry no content, and marks are in a fixed order, so equal documents compare equal.
func normalizeDoc(doc model.RichDoc) (model.RichDoc, string) {
	if doc.Type == "" && doc.Content == nil {
		return model.EmptyDoc(), ""
	}
	if doc.Type != model.NodeDoc {
		return model.RichDoc{}, ReasonDocInvalid
	}
	w := &docWalker{}
	content, reason := w.blocks(doc.Content, docBlocks, 1)
	if reason != "" {
		return model.RichDoc{}, reason
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

// validatedDoc normalises and applies the text rules; lengthReason is what an empty or overlong document reports.
func validatedDoc(doc model.RichDoc, required bool, maxRunes int, lengthReason string) (model.RichDoc, string) {
	out, reason := normalizeDoc(doc)
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

type docWalker struct{ nodes int }

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
		out.Attrs, reason = imageAttrs(n.Attrs)
		return out, reason
	case model.NodeParagraph:
		out.Content, reason = w.blocks(n.Content, map[string]bool{model.NodeText: true, model.NodeHardBreak: true, model.NodeMath: true}, depth+1)
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

func imageAttrs(in map[string]any) (map[string]any, string) {
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
		if !ok || utf8.RuneCountInString(src) > maxImageSrcLen || !(strings.HasPrefix(src, "http://") || strings.HasPrefix(src, "https://")) {
			return nil, ReasonDocAttr
		}
		out[model.AttrSrc] = src
	}
	if raw := in[model.AttrAlt]; raw != nil {
		alt, ok := raw.(string)
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
	value, present := intAttr(in, model.AttrStart, 1, maxAttrStartVal)
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
