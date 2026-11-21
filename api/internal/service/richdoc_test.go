package service

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/model"
)

// d is the document a plain string stands for in fixtures.
func d(text string) model.RichDoc { return model.DocFromLegacyText(text) }

// imageDoc is a document holding one image node with the given attributes.
func imageDoc(attrs map[string]any) model.RichDoc {
	return model.RichDoc{Type: model.NodeDoc, Content: []model.RichNode{{Type: model.NodeImage, Attrs: attrs}}}
}

func docOf(t *testing.T, raw string) model.RichDoc {
	t.Helper()
	var doc model.RichDoc
	if err := json.Unmarshal([]byte(raw), &doc); err != nil {
		t.Fatalf("fixture: %v", err)
	}
	return doc
}

func normalizedJSON(t *testing.T, raw string) (string, string) {
	t.Helper()
	out, reason := normalizeDoc(docOf(t, raw))
	if reason != "" {
		return "", reason
	}
	b, err := json.Marshal(out)
	if err != nil {
		t.Fatal(err)
	}
	return string(b), ""
}

func TestNormalizeDocAccepts(t *testing.T) {
	id := uuid.NewString()
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"empty", `{"type":"doc","content":[]}`, `{"type":"doc","content":[]}`},
		{"paragraph marks reordered", `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"a","marks":[{"type":"underline"},{"type":"bold"}]},{"type":"hardBreak"},{"type":"math","attrs":{"latex":"  x^2 "}}]}]}`,
			`{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"a","marks":[{"type":"bold"},{"type":"underline"}]},{"type":"hardBreak"},{"type":"math","attrs":{"latex":"x^2"}}]}]}`},
		{"empty paragraph between blocks is kept", `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"a"}]},{"type":"paragraph","content":[]},{"type":"paragraph","content":[{"type":"text","text":"b"}]}]}`,
			`{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"a"}]},{"type":"paragraph"},{"type":"paragraph","content":[{"type":"text","text":"b"}]}]}`},
		{"trailing empty paragraphs are dropped", `{"type":"doc","content":[{"type":"bulletList","content":[{"type":"listItem","content":[{"type":"paragraph","content":[{"type":"text","text":"a"}]}]}]},{"type":"paragraph"},{"type":"paragraph","content":[]}]}`,
			`{"type":"doc","content":[{"type":"bulletList","content":[{"type":"listItem","content":[{"type":"paragraph","content":[{"type":"text","text":"a"}]}]}]}]}`},
		{"only empty paragraphs is the empty document", `{"type":"doc","content":[{"type":"paragraph"},{"type":"paragraph","content":[]}]}`, `{"type":"doc","content":[]}`},
		{"empty paragraph in a list item is kept", `{"type":"doc","content":[{"type":"bulletList","content":[{"type":"listItem","content":[{"type":"paragraph"}]}]}]}`,
			`{"type":"doc","content":[{"type":"bulletList","content":[{"type":"listItem","content":[{"type":"paragraph"}]}]}]}`},
		{"adjacent text with equal marks merges", `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"a"},{"type":"text","text":"b"},{"type":"text","text":"c","marks":[{"type":"bold"}]},{"type":"text","text":"d","marks":[{"type":"bold"}]},{"type":"text","text":"e"}]}]}`,
			`{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"ab"},{"type":"text","text":"cd","marks":[{"type":"bold"}]},{"type":"text","text":"e"}]}]}`},
		{"ordered list drops default start and extras", `{"type":"doc","content":[{"type":"orderedList","attrs":{"start":1,"type":null},"content":[{"type":"listItem","content":[{"type":"paragraph"}]}]}]}`,
			`{"type":"doc","content":[{"type":"orderedList","content":[{"type":"listItem","content":[{"type":"paragraph"}]}]}]}`},
		{"ordered list keeps start", `{"type":"doc","content":[{"type":"orderedList","attrs":{"start":3},"content":[{"type":"listItem","content":[{"type":"paragraph"}]}]}]}`,
			`{"type":"doc","content":[{"type":"orderedList","attrs":{"start":3},"content":[{"type":"listItem","content":[{"type":"paragraph"}]}]}]}`},
		{"nested bullet lists", `{"type":"doc","content":[{"type":"bulletList","content":[{"type":"listItem","content":[{"type":"paragraph"},{"type":"bulletList","content":[{"type":"listItem","content":[{"type":"paragraph"}]}]}]}]}]}`,
			`{"type":"doc","content":[{"type":"bulletList","content":[{"type":"listItem","content":[{"type":"paragraph"},{"type":"bulletList","content":[{"type":"listItem","content":[{"type":"paragraph"}]}]}]}]}]}`},
		{"table cell attrs", `{"type":"doc","content":[{"type":"table","content":[{"type":"tableRow","content":[{"type":"tableHeader","attrs":{"colspan":2,"rowspan":1,"colwidth":null},"content":[{"type":"paragraph"}]},{"type":"tableCell","attrs":{"colspan":1,"rowspan":3},"content":[{"type":"shape","attrs":{"kind":"bola"}},{"type":"image","attrs":{"asset_id":"` + id + `"}}]}]}]}]}`,
			`{"type":"doc","content":[{"type":"table","content":[{"type":"tableRow","content":[{"type":"tableHeader","attrs":{"colspan":2},"content":[{"type":"paragraph"}]},{"type":"tableCell","attrs":{"rowspan":3},"content":[{"type":"shape","attrs":{"kind":"bola"}},{"type":"image","attrs":{"asset_id":"` + id + `"}}]}]}]}]}`},
		{"shape caption", `{"type":"doc","content":[{"type":"shape","attrs":{"kind":"kubus","caption":" ","extra":1},"content":[{"type":"paragraph"}]}]}`,
			`{"type":"doc","content":[{"type":"shape","attrs":{"kind":"kubus"}}]}`},
		{"shape with caption", `{"type":"doc","content":[{"type":"shape","attrs":{"kind":"limas","caption":"gbr 1"}}]}`,
			`{"type":"doc","content":[{"type":"shape","attrs":{"caption":"gbr 1","kind":"limas"}}]}`},
		{"non-text node loses text and marks", `{"type":"doc","content":[{"type":"paragraph","text":"zz","marks":[{"type":"bold"}],"content":[]},{"type":"shape","attrs":{"kind":"bola"}}]}`, `{"type":"doc","content":[{"type":"paragraph"},{"type":"shape","attrs":{"kind":"bola"}}]}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, reason := normalizedJSON(t, tc.in)
			if reason != "" {
				t.Fatalf("rejected: %s", reason)
			}
			if got != tc.want {
				t.Fatalf("got  %s\nwant %s", got, tc.want)
			}
		})
	}
}

func TestNormalizeDocRejects(t *testing.T) {
	id := uuid.NewString()
	p := `{"type":"paragraph"}`
	wrap := func(blocks string) string { return `{"type":"doc","content":[` + blocks + `]}` }
	cases := []struct {
		name   string
		in     string
		reason string
	}{
		{"wrong root", `{"type":"paragraph","content":[]}`, ReasonDocInvalid},
		{"no root type", `{"content":[]}`, ReasonDocInvalid},
		{"unknown node", wrap(`{"type":"script","content":[]}`), ReasonDocNode},
		{"heading not allowed", wrap(`{"type":"heading","attrs":{"level":1}}`), ReasonDocNode},
		{"text at block level", wrap(`{"type":"text","text":"a"}`), ReasonDocNode},
		{"empty text", wrap(`{"type":"paragraph","content":[{"type":"text","text":""}]}`), ReasonDocNode},
		{"list item at block level", wrap(`{"type":"listItem","content":[` + p + `]}`), ReasonDocNode},
		{"empty list", wrap(`{"type":"bulletList","content":[]}`), ReasonDocNode},
		{"list holding a paragraph", wrap(`{"type":"bulletList","content":[` + p + `]}`), ReasonDocNode},
		{"table in table", wrap(`{"type":"table","content":[{"type":"tableRow","content":[{"type":"tableCell","content":[{"type":"table","content":[]}]}]}]}`), ReasonDocNode},
		{"table in list item", wrap(`{"type":"bulletList","content":[{"type":"listItem","content":[{"type":"table"}]}]}`), ReasonDocNode},
		{"empty table", wrap(`{"type":"table","content":[]}`), ReasonDocNode},
		{"empty row", wrap(`{"type":"table","content":[{"type":"tableRow","content":[]}]}`), ReasonDocNode},
		{"paragraph in row", wrap(`{"type":"table","content":[{"type":"tableRow","content":[` + p + `]}]}`), ReasonDocNode},
		{"shape in paragraph", wrap(`{"type":"paragraph","content":[{"type":"shape","attrs":{"kind":"bola"}}]}`), ReasonDocNode},
		{"unknown mark", wrap(`{"type":"paragraph","content":[{"type":"text","text":"a","marks":[{"type":"link"}]}]}`), ReasonDocMark},
		{"duplicate mark", wrap(`{"type":"paragraph","content":[{"type":"text","text":"a","marks":[{"type":"bold"},{"type":"bold"}]}]}`), ReasonDocMark},
		{"math without latex", wrap(`{"type":"paragraph","content":[{"type":"math","attrs":{}}]}`), ReasonDocAttr},
		{"math blank latex", wrap(`{"type":"paragraph","content":[{"type":"math","attrs":{"latex":"  "}}]}`), ReasonDocAttr},
		{"math long latex", wrap(`{"type":"paragraph","content":[{"type":"math","attrs":{"latex":"` + strings.Repeat("x", 2001) + `"}}]}`), ReasonDocAttr},
		{"unknown shape kind", wrap(`{"type":"shape","attrs":{"kind":"segitiga"}}`), ReasonDocAttr},
		{"long caption", wrap(`{"type":"shape","attrs":{"kind":"bola","caption":"` + strings.Repeat("c", 201) + `"}}`), ReasonDocAttr},
		{"image without source", wrap(`{"type":"image","attrs":{}}`), ReasonDocAttr},
		{"image with both sources", wrap(`{"type":"image","attrs":{"asset_id":"` + id + `","src":"https://x.test/a.png"}}`), ReasonDocAttr},
		{"image bad asset id", wrap(`{"type":"image","attrs":{"asset_id":"nope"}}`), ReasonDocAttr},
		{"image src without an allowance", wrap(`{"type":"image","attrs":{"src":"https://x.test/a.png"}}`), ReasonDocAttr},
		{"image in a list item", wrap(`{"type":"bulletList","content":[{"type":"listItem","content":[{"type":"image","attrs":{"asset_id":"` + id + `"}}]}]}`), ReasonDocNode},
		{"shape in a list item", wrap(`{"type":"orderedList","content":[{"type":"listItem","content":[{"type":"shape","attrs":{"kind":"bola"}}]}]}`), ReasonDocNode},
		{"table in a table cell", wrap(`{"type":"table","content":[{"type":"tableRow","content":[{"type":"tableCell","content":[{"type":"table","content":[{"type":"tableRow","content":[{"type":"tableCell","content":[` + p + `]}]}]}]}]}]}`), ReasonDocNode},
		{"image data url", wrap(`{"type":"image","attrs":{"src":"data:image/png;base64,AAA"}}`), ReasonDocAttr},
		{"image javascript url", wrap(`{"type":"image","attrs":{"src":"javascript:alert(1)"}}`), ReasonDocAttr},
		{"image long alt", wrap(`{"type":"image","attrs":{"asset_id":"` + id + `","alt":"` + strings.Repeat("a", 201) + `"}}`), ReasonDocAttr},
		{"colspan too big", wrap(`{"type":"table","content":[{"type":"tableRow","content":[{"type":"tableCell","attrs":{"colspan":13},"content":[` + p + `]}]}]}`), ReasonDocAttr},
		{"rowspan fractional", wrap(`{"type":"table","content":[{"type":"tableRow","content":[{"type":"tableCell","attrs":{"rowspan":1.5},"content":[` + p + `]}]}]}`), ReasonDocAttr},
		{"start zero", wrap(`{"type":"orderedList","attrs":{"start":0},"content":[{"type":"listItem","content":[` + p + `]}]}`), ReasonDocAttr},
		{"start string", wrap(`{"type":"orderedList","attrs":{"start":"2"},"content":[{"type":"listItem","content":[` + p + `]}]}`), ReasonDocAttr},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, reason := normalizedJSON(t, tc.in); reason != tc.reason {
				t.Fatalf("got reason %q, want %q", reason, tc.reason)
			}
		})
	}
}

func TestNormalizeDocLimits(t *testing.T) {
	p := `{"type":"paragraph"}`
	row := func(cells int) string {
		return `{"type":"tableRow","content":[` + strings.TrimSuffix(strings.Repeat(`{"type":"tableCell","content":[`+p+`]},`, cells), ",") + `]}`
	}
	table := func(rows, cells int) string {
		return `{"type":"doc","content":[{"type":"table","content":[` + strings.TrimSuffix(strings.Repeat(row(cells)+",", rows), ",") + `]}]}`
	}
	if _, reason := normalizedJSON(t, table(30, 12)); reason != "" {
		t.Fatalf("30x12 table: %s", reason)
	}
	if _, reason := normalizedJSON(t, table(31, 1)); reason != ReasonDocTooLarge {
		t.Fatalf("31 rows: %q", reason)
	}
	if _, reason := normalizedJSON(t, table(1, 13)); reason != ReasonDocTooLarge {
		t.Fatalf("13 cells: %q", reason)
	}

	nest := func(depth int) string {
		inner := p
		for i := 0; i < depth; i++ {
			inner = `{"type":"bulletList","content":[{"type":"listItem","content":[` + inner + `]}]}`
		}
		return `{"type":"doc","content":[` + inner + `]}`
	}
	if _, reason := normalizedJSON(t, nest(2)); reason != "" {
		t.Fatalf("shallow nest: %q", reason)
	}
	if _, reason := normalizedJSON(t, nest(6)); reason != ReasonDocTooLarge {
		t.Fatalf("deep nest: %q", reason)
	}

	many := `{"type":"doc","content":[` + strings.TrimSuffix(strings.Repeat(p+",", 2001), ",") + `]}`
	if _, reason := normalizedJSON(t, many); reason != ReasonDocTooLarge {
		t.Fatalf("2001 nodes: %q", reason)
	}
	ok := `{"type":"doc","content":[` + strings.TrimSuffix(strings.Repeat(p+",", 2000), ",") + `]}`
	if _, reason := normalizedJSON(t, ok); reason != "" {
		t.Fatalf("2000 nodes: %q", reason)
	}

	big := `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"` + strings.Repeat("a", 101<<10) + `"}]}]}`
	if _, reason := normalizedJSON(t, big); reason != ReasonDocTooLarge {
		t.Fatalf("101 KB: %q", reason)
	}
}

func TestValidatedDocTextRules(t *testing.T) {
	text := func(s string) model.RichDoc {
		return model.RichDoc{Type: model.NodeDoc, Content: []model.RichNode{{Type: model.NodeParagraph, Content: []model.RichNode{{Type: model.NodeText, Text: s}}}}}
	}
	if _, reason := validatedDoc(model.EmptyDoc(), true, 100, ReasonQuestionContent); reason != ReasonQuestionContent {
		t.Fatalf("empty required: %q", reason)
	}
	if _, reason := validatedDoc(text("   "), true, 100, ReasonQuestionContent); reason != ReasonQuestionContent {
		t.Fatalf("blank required: %q", reason)
	}
	if doc, reason := validatedDoc(model.EmptyDoc(), false, 100, ReasonQuestionExpl); reason != "" || doc.Content == nil {
		t.Fatalf("empty optional: %q %v", reason, doc)
	}
	if _, reason := validatedDoc(text(strings.Repeat("é", 101)), true, 100, ReasonQuestionContent); reason != ReasonQuestionContent {
		t.Fatalf("too long: %q", reason)
	}
	if _, reason := validatedDoc(text(strings.Repeat("é", 100)), true, 100, ReasonQuestionContent); reason != "" {
		t.Fatalf("at limit: %q", reason)
	}
	shapeOnly := model.RichDoc{Type: model.NodeDoc, Content: []model.RichNode{{Type: model.NodeShape, Attrs: map[string]any{"kind": "kubus"}}}}
	if _, reason := validatedDoc(shapeOnly, true, 100, ReasonQuestionContent); reason != "" {
		t.Fatalf("shape-only content: %q", reason)
	}
	if _, reason := validatedDoc(model.RichDoc{Type: "x"}, true, 100, ReasonQuestionContent); reason != ReasonDocInvalid {
		t.Fatalf("bad root: %q", reason)
	}
}
