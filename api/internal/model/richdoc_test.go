package model

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/google/uuid"
)

func para(children ...RichNode) RichNode { return RichNode{Type: NodeParagraph, Content: children} }
func txt(s string) RichNode              { return RichNode{Type: NodeText, Text: s} }
func marked(s, m string) RichNode {
	return RichNode{Type: NodeText, Text: s, Marks: []RichMark{{Type: m}}}
}
func mathNode(s string) RichNode {
	return RichNode{Type: NodeMath, Attrs: map[string]any{AttrLatex: s}}
}
func list(kind string, items ...string) RichNode {
	out := RichNode{Type: kind}
	for _, item := range items {
		out.Content = append(out.Content, RichNode{Type: NodeListItem, Content: []RichNode{para(txt(item))}})
	}
	return out
}

func TestDocFromLegacyText(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want []RichNode
	}{
		{"empty string", "", []RichNode{}},
		{"plain", "Hitung luas persegi", []RichNode{para(txt("Hitung luas persegi"))}},
		{"math", "Nilai $x^2$ adalah", []RichNode{para(txt("Nilai "), mathNode("x^2"), txt(" adalah"))}},
		{"bold and italic", "**tebal** dan *miring*", []RichNode{para(marked("tebal", MarkBold), txt(" dan "), marked("miring", MarkItalic))}},
		{"spaced stars stay plain", "3 * 4 * 5", []RichNode{para(txt("3 * 4 * 5"))}},
		{"space inside marker stays plain", "** a**", []RichNode{para(txt("** a**"))}},
		{"markers inside math are literal", "$a*b*c$", []RichNode{para(mathNode("a*b*c"))}},
		{"lone dash line stays literal", "- 5 adalah akar", []RichNode{para(txt("- 5 adalah akar"))}},
		{"lone numbered line stays literal", "1. satu", []RichNode{para(txt("1. satu"))}},
		{"two bullets", "- a\n- b", []RichNode{list(NodeBulletList, "a", "b")}},
		{"two numbered", "1. a\n2. b", []RichNode{list(NodeOrderedList, "a", "b")}},
		{"bullets then lone numbered", "- a\n- b\n1. c", []RichNode{list(NodeBulletList, "a", "b"), para(txt("1. c"))}},
		{"bullets then numbered", "- a\n- b\n1. c\n2. d", []RichNode{list(NodeBulletList, "a", "b"), list(NodeOrderedList, "c", "d")}},
		{"list then text", "- a\n- b\nlalu", []RichNode{list(NodeBulletList, "a", "b"), para(txt("lalu"))}},
		{"http image", "![peta](https://x.test/a.png)", []RichNode{{Type: NodeImage, Attrs: map[string]any{AttrSrc: "https://x.test/a.png", AttrAlt: "peta"}}}},
		{"image without alt", "![](http://x.test/a.png)", []RichNode{{Type: NodeImage, Attrs: map[string]any{AttrSrc: "http://x.test/a.png"}}}},
		{"data image stays literal", "![x](data:image/png;base64,AAA)", []RichNode{para(txt("![x](data:image/png;base64,AAA)"))}},
		{"empty lines", "a\n\nb", []RichNode{para(txt("a")), para(), para(txt("b"))}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := DocFromLegacyText(tc.in)
			if got.Type != NodeDoc {
				t.Fatalf("type %q", got.Type)
			}
			if !reflect.DeepEqual(got.Content, tc.want) {
				g, _ := json.Marshal(got.Content)
				w, _ := json.Marshal(tc.want)
				t.Fatalf("got  %s\nwant %s", g, w)
			}
		})
	}
}

func TestDocMarshalsContentAsArray(t *testing.T) {
	for _, doc := range []RichDoc{{}, EmptyDoc(), {Type: NodeDoc}} {
		raw, err := json.Marshal(doc)
		if err != nil || string(raw) != `{"type":"doc","content":[]}` {
			t.Fatalf("got %s, %v", raw, err)
		}
	}
}

func TestDocText(t *testing.T) {
	table := RichNode{Type: NodeTable, Content: []RichNode{
		{Type: NodeTableRow, Content: []RichNode{
			{Type: NodeTableHeader, Content: []RichNode{para(txt("A"))}},
			{Type: NodeTableHeader, Content: []RichNode{para(txt("B"))}},
		}},
		{Type: NodeTableRow, Content: []RichNode{
			{Type: NodeTableCell, Content: []RichNode{para(txt("1"))}},
			{Type: NodeTableCell, Content: []RichNode{para(mathNode("x+1"))}},
		}},
	}}
	doc := RichDoc{Type: NodeDoc, Content: []RichNode{
		para(txt("Soal "), mathNode("a^2"), RichNode{Type: NodeHardBreak}, txt("lanjut")),
		list(NodeBulletList, "satu", "dua"),
		table,
		{Type: NodeShape, Attrs: map[string]any{AttrKind: "kubus"}},
		{Type: NodeImage, Attrs: map[string]any{AttrSrc: "https://x.test/a.png"}},
	}}
	want := "Soal a^2\nlanjut\nsatu\ndua\nA B\n1 x+1"
	if got := DocText(doc); got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	if DocText(EmptyDoc()) != "" {
		t.Fatal("empty doc has text")
	}
}

func TestDocEmpty(t *testing.T) {
	cases := []struct {
		name string
		doc  RichDoc
		want bool
	}{
		{"no content", EmptyDoc(), true},
		{"empty paragraph", RichDoc{Content: []RichNode{para()}}, true},
		{"whitespace text", RichDoc{Content: []RichNode{para(txt("  "))}}, true},
		{"text", RichDoc{Content: []RichNode{para(txt("a"))}}, false},
		{"math only", RichDoc{Content: []RichNode{para(mathNode("x"))}}, false},
		{"shape only", RichDoc{Content: []RichNode{{Type: NodeShape}}}, false},
		{"image only", RichDoc{Content: []RichNode{{Type: NodeImage}}}, false},
		{"nested list text", RichDoc{Content: []RichNode{list(NodeBulletList, "a")}}, false},
	}
	for _, tc := range cases {
		if got := DocEmpty(tc.doc); got != tc.want {
			t.Errorf("%s: got %v", tc.name, got)
		}
	}
}

func TestDocAssetIDs(t *testing.T) {
	a, b := uuid.New(), uuid.New()
	img := func(id uuid.UUID) RichNode {
		return RichNode{Type: NodeImage, Attrs: map[string]any{AttrAssetID: id.String()}}
	}
	d1 := RichDoc{Content: []RichNode{img(a), {Type: NodeImage, Attrs: map[string]any{AttrSrc: "https://x.test/a.png"}}}}
	d2 := RichDoc{Content: []RichNode{{Type: NodeTable, Content: []RichNode{{Type: NodeTableRow, Content: []RichNode{
		{Type: NodeTableCell, Content: []RichNode{img(b), img(a)}},
	}}}}}}
	got := DocAssetIDs(d1, d2)
	if !reflect.DeepEqual(got, []uuid.UUID{a, b}) {
		t.Fatalf("got %v", got)
	}
	if len(DocAssetIDs()) != 0 {
		t.Fatal("expected none")
	}
}
