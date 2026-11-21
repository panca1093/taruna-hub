package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/model"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/repository"
)

func (h questionHarness) versionCount(t *testing.T, questionID uuid.UUID) int {
	t.Helper()
	var n int
	if err := h.pool.QueryRow(context.Background(), `SELECT count(*) FROM question_versions WHERE question_id = $1`, questionID).Scan(&n); err != nil {
		t.Fatalf("count versions: %v", err)
	}
	return n
}

// asInput is what the editor posts back after opening the question and changing nothing.
func asInput(q model.Question) QuestionInput {
	in := QuestionInput{
		ProgramID: q.ProgramID, SubjectID: q.SubjectID, ChapterID: q.ChapterID, Type: q.Type, Content: q.Content,
		Difficulty: q.Difficulty, Explanation: q.Explanation, OptionWeighted: q.OptionWeighted,
		CaseSensitive: q.CaseSensitive, Correct: q.Correct, BaseVersion: q.Version,
	}
	for _, option := range q.Options {
		in.Options = append(in.Options, QuestionOptionInput{ID: option.ID, Label: option.Doc, Points: option.Points})
	}
	return in
}

func inputErrorOn(t *testing.T, err error, path, reason string) {
	t.Helper()
	var input *InputError
	if !errors.As(err, &input) || input.Fields[path] != reason {
		t.Fatalf("err = %v, want %s on %s", err, reason, path)
	}
}

func TestUneditedLegacyRowResavesWithoutAVersion(t *testing.T) {
	h := newQuestionHarness(t)
	tenant := h.a
	cases := map[string]string{
		"formula":                "Hitung $x^2$ lalu **tebal**\n- a\n- b",
		"blank formula":          "sebelum $ $ sesudah",
		"padded formula":         "nilai $ x $ saja",
		"CRLF text":              "baris satu\r\nbaris dua\r\n- a\r\n- b\r\nakhir",
		"long alt":               "![" + strings.Repeat("a", 300) + "](https://x.test/a.png)\nteks",
		"emphasis around math":   "*Jika $x=2$ maka* dan **$y$**",
		"mixed list run":         "- a\n- b\n1. c",
		"trailing blank lines":   "isi\n- a\n- b\n\n\n",
		"image at the end":       "teks\n![peta](https://x.test/a.png)\n",
		"image then blank lines": "![](http://x.test/a.png)\r\n\r\n",
	}
	for name, text := range cases {
		t.Run(name, func(t *testing.T) {
			id, _ := insertLegacy(t, h, tenant, text, "Karena *begitu*")
			got, err := h.svc.Get(context.Background(), h.scope(tenant), h.principal(tenant, model.RoleAdmin), id)
			if err != nil {
				t.Fatalf("get: %v", err)
			}
			if strings.Contains(string(mustJSON(t, got.Content)), `\r`) {
				t.Fatalf("a carriage return reached a node: %s", mustJSON(t, got.Content))
			}
			saved, err := h.update(t, tenant, id, asInput(got))
			if err != nil {
				t.Fatalf("resave of an unedited legacy row: %v", err)
			}
			if saved.Version != got.Version || h.versionCount(t, id) != 1 {
				t.Errorf("resave = version %d with %d rows, want no new version", saved.Version, h.versionCount(t, id))
			}
		})
	}
}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestLegacyConversionIsNormalisedLikeInput(t *testing.T) {
	long := strings.Repeat("é", 300)
	doc := docOfLegacyText("a $ x $ b $ $ c\n![" + long + "](https://x.test/a.png)\n")
	if got := doc.Content[0].Content; len(got) != 3 || got[1].Attrs[model.AttrLatex] != "x" || got[2].Text != " b $ $ c" {
		t.Errorf("paragraph = %+v, want trimmed LaTeX and the blank formula merged into its neighbours' text", got)
	}
	if len(doc.Content) != 2 {
		t.Fatalf("content = %d blocks, want the trailing empty paragraph dropped", len(doc.Content))
	}
	if alt, _ := doc.Content[1].Attrs[model.AttrAlt].(string); len([]rune(alt)) != maxAltLen {
		t.Errorf("alt has %d runes, want it cut to %d", len([]rune(alt)), maxAltLen)
	}
}

func TestLegacyTextThatCannotBeValidIsShownAsConverted(t *testing.T) {
	h := newQuestionHarness(t)
	tenant := h.a
	id, _ := insertLegacy(t, h, tenant, strings.Repeat("baris\n", maxDocNodes+10), "")
	got, err := h.svc.Get(context.Background(), h.scope(tenant), h.principal(tenant, model.RoleAdmin), id)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if len(got.Content.Content) < maxDocNodes {
		t.Errorf("content has %d blocks, want the raw conversion kept for reading", len(got.Content.Content))
	}
	_, err = h.update(t, tenant, id, asInput(got))
	inputErrorOn(t, err, "content", ReasonDocTooLarge)
}

func TestLegacyRowsReadAsDocumentsOnEveryReader(t *testing.T) {
	h := newResultHarness(t)
	tenant := h.tenant()
	q := h.question(t)
	const content, explanation = "Legacy $x^2$ **tebal**\n- a\n- b", "Penjelasan *lama*"
	if _, err := h.pool.Exec(context.Background(),
		`UPDATE question_versions
		    SET content = $2, explanation = $3, content_doc = NULL, explanation_doc = NULL,
		        options = (SELECT jsonb_agg(o.obj - 'doc' ORDER BY o.ord) FROM jsonb_array_elements(options) WITH ORDINALITY AS o(obj, ord))
		  WHERE id = $1`, q.VersionID, content, explanation); err != nil {
		t.Fatalf("make the version legacy: %v", err)
	}
	wantContent, wantExplanation, wantOption := docOfLegacyText(content), docOfLegacyText(explanation), docOfLegacyText(q.Options[0].Label)

	tr := h.tryoutOf(t, nil, q)
	got := h.get(t, tenant, tr.ID)
	if c := got.Subtests[0].Questions[0].Content; !sameDoc(c, wantContent) {
		t.Errorf("tryout assignment content = %s, want %s", mustJSON(t, c), mustJSON(t, wantContent))
	}
	print, err := h.svc.Print(context.Background(), h.scopeA(), h.staff(model.RoleAdmin), tr.ID)
	if err != nil {
		t.Fatalf("print: %v", err)
	}
	pq := print.Subtests[0].Questions[0]
	if !sameDoc(pq.Content, wantContent) || !sameDoc(pq.Options[0], wantOption) {
		t.Errorf("print = %s / %s, want converted documents", mustJSON(t, pq.Content), mustJSON(t, pq.Options[0]))
	}

	_, p := h.enrolledStudentOf(t, tr)
	view, err := h.rt.Start(context.Background(), h.scopeA(), p, tr.ID)
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	h.startPart(t, p, view.Attempt.ID, tr.Subtests[0].ID)
	sub, err := h.rt.Subtest(context.Background(), h.scopeA(), p, view.Attempt.ID, tr.Subtests[0].ID)
	if err != nil {
		t.Fatalf("subtest: %v", err)
	}
	aq := sub.Questions[0]
	if !sameDoc(aq.Content, wantContent) || aq.Options[0].Doc.Type != model.NodeDoc || aq.Options[0].Doc.Legacy != "" {
		t.Errorf("runtime question = %s / %+v, want converted documents", mustJSON(t, aq.Content), aq.Options[0].Doc)
	}
	h.submitPart(t, p, view.Attempt.ID, tr.Subtests[0].ID)
	review, err := h.rs.Review(context.Background(), h.scopeA(), p, view.Attempt.ID)
	if err != nil {
		t.Fatalf("review: %v", err)
	}
	rq := review.Subtests[0].Questions[0].Question
	if !sameDoc(rq.Content, wantContent) || !sameDoc(rq.Explanation, wantExplanation) || rq.Options[0].Doc.Type != model.NodeDoc {
		t.Errorf("review question = %s / %s, want converted documents", mustJSON(t, rq.Content), mustJSON(t, rq.Explanation))
	}

	list, total := h.q.list(t, tenant, QuestionFilter{Search: "Legacy"})
	if total != 1 || !sameDoc(list[0].Content, wantContent) {
		t.Errorf("list = %d rows, want the question with converted content", total)
	}
}

func TestStoredTextKeepsFormattingForRollbackAndStaysSearchable(t *testing.T) {
	h := newQuestionHarness(t)
	tenant := h.a
	in := h.pgInput(tenant)
	in.Content = richContent()
	in.Explanation = d("Karena **begitu**\n- satu\n- dua")
	in.Options = []QuestionOptionInput{{ID: "a", Label: d("$3$")}, {ID: "b", Label: d("*empat*")}}
	created := h.create(t, tenant, in)

	var content, explanation string
	if err := h.pool.QueryRow(context.Background(), `SELECT content, explanation FROM question_versions WHERE id = $1`, created.VersionID).Scan(&content, &explanation); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"**tabel**", "$x^2$", "A | " + tableCellText, "1. langkah"} {
		if !strings.Contains(content, want) {
			t.Errorf("content column %q lacks %q", content, want)
		}
	}
	if strings.Contains(content, "kubus") {
		t.Errorf("content column %q mentions the shape, which has no text form", content)
	}
	if explanation != "Karena **begitu**\n- satu\n- dua" {
		t.Errorf("explanation column = %q, want the legacy text", explanation)
	}
	if created.Options[1].Label != "*empat*" {
		t.Errorf("option label = %q, want *empat*", created.Options[1].Label)
	}

	for _, search := range []string{tableCellText, "Perhatikan", "x^2"} {
		if _, total := h.list(t, tenant, QuestionFilter{Search: search}); total != 1 {
			t.Errorf("search %q = %d results, want the question", search, total)
		}
	}
}

func TestStagedQuestionImageIsOnlyTheUploadersToAttach(t *testing.T) {
	h := newQuestionHarness(t)
	tenant := h.a
	ctx := context.Background()
	uploader := h.author[tenant.ID].ID
	email := "second-" + uuid.NewString()[:8] + "@example.com"
	second, err := repository.NewUserRepository(h.pool).Create(ctx, model.User{
		TenantID: &tenant.ID, Name: "Operator Soal", Email: &email, PasswordHash: "x", Role: model.RoleOperator,
	})
	if err != nil {
		t.Fatalf("create second staff user: %v", err)
	}
	other := model.Principal{UserID: second.ID, TenantID: &tenant.ID, Role: model.RoleOperator}

	staged := h.insertAsset(t, tenant, model.PurposeQuestionImage, uploader)
	in := h.pgInput(tenant)
	in.Content = imageNode(staged)
	_, err = h.svc.Create(ctx, h.scope(tenant), other, in)
	inputErrorOn(t, err, "content", ReasonDocAsset)
	if status := assetStatus(t, h, tenant, staged); status != model.AssetStaged {
		t.Fatalf("someone else's pending upload is %q after a refused save, want it still staged", status)
	}

	created, err := h.svc.Create(ctx, h.scope(tenant), h.principal(tenant, model.RoleAdmin), in)
	if err != nil {
		t.Fatalf("uploader attaching their own staged image: %v", err)
	}
	if status := assetStatus(t, h, tenant, staged); status != model.AssetFinal {
		t.Errorf("status = %q, want final", status)
	}
	if _, err := h.svc.Create(ctx, h.scope(tenant), other, in); err != nil {
		t.Errorf("another staff member reusing a final image: %v", err)
	}
	if created.ID == uuid.Nil {
		t.Fatal("no question")
	}
}

func TestImageSrcIsRefusedOnCreateAndOnlyKeptOnUpdate(t *testing.T) {
	h := newQuestionHarness(t)
	tenant := h.a
	src := map[string]any{"src": "https://x.test/a.png"}

	for path, mutate := range map[string]func(*QuestionInput){
		"content":          func(in *QuestionInput) { in.Content = imageDoc(src) },
		"explanation":      func(in *QuestionInput) { in.Explanation = imageDoc(src) },
		"options[1].label": func(in *QuestionInput) { in.Options[1].Label = imageDoc(src) },
	} {
		in := h.pgInput(tenant)
		mutate(&in)
		_, err := h.svc.Create(context.Background(), h.scope(tenant), h.principal(tenant, model.RoleAdmin), in)
		inputErrorOn(t, err, path, ReasonDocAttr)
	}

	id, _ := insertLegacy(t, h, tenant, "lihat\n![peta](https://x.test/a.png)", "")
	got, err := h.svc.Get(context.Background(), h.scope(tenant), h.principal(tenant, model.RoleAdmin), id)
	if err != nil {
		t.Fatal(err)
	}
	keep := asInput(got)
	keep.Content = model.RichDoc{Type: model.NodeDoc, Content: append([]model.RichNode{{Type: model.NodeParagraph, Content: []model.RichNode{{Type: model.NodeText, Text: "ubah"}}}}, got.Content.Content[1:]...)}
	updated, err := h.update(t, tenant, id, keep)
	if err != nil || updated.Version != 2 {
		t.Fatalf("update keeping the existing src = version %d, err %v, want a new version", updated.Version, err)
	}

	for path, mutate := range map[string]func(*QuestionInput){
		"content":          func(in *QuestionInput) { in.Content = imageDoc(map[string]any{"src": "https://x.test/b.png"}) },
		"explanation":      func(in *QuestionInput) { in.Explanation = imageDoc(map[string]any{"src": "https://x.test/b.png"}) },
		"options[0].label": func(in *QuestionInput) { in.Options[0].Label = imageDoc(map[string]any{"src": "https://x.test/b.png"}) },
	} {
		in := asInput(updated)
		mutate(&in)
		_, err := h.update(t, tenant, id, in)
		inputErrorOn(t, err, path, ReasonDocAttr)
	}
}

// editorDoc is a document the way Tiptap emits it: null attributes, header cells, a shape in a cell, empty
// paragraphs without content.
const editorDoc = `{"type":"doc","content":[
	{"type":"paragraph","attrs":{"textAlign":null},"content":[{"type":"text","text":"Lihat "},{"type":"text","text":"gambar","marks":[{"type":"italic"}]},{"type":"math","attrs":{"latex":" a+b "}}]},
	{"type":"image","attrs":{"asset_id":"%s","alt":null,"src":null,"title":null}},
	{"type":"table","content":[
		{"type":"tableRow","content":[
			{"type":"tableHeader","attrs":{"colspan":1,"rowspan":1,"colwidth":null},"content":[{"type":"paragraph"}]},
			{"type":"tableHeader","attrs":{"colspan":1,"rowspan":1,"colwidth":null},"content":[{"type":"paragraph","content":[{"type":"text","text":"B"}]}]}]},
		{"type":"tableRow","content":[
			{"type":"tableCell","attrs":{"colspan":1,"rowspan":1,"colwidth":null},"content":[{"type":"shape","attrs":{"kind":"kubus","caption":null}}]},
			{"type":"tableCell","attrs":{"colspan":1,"rowspan":1,"colwidth":null},"content":[{"type":"paragraph"}]}]}]},
	{"type":"orderedList","attrs":{"start":1,"type":null},"content":[{"type":"listItem","content":[{"type":"paragraph","content":[{"type":"text","text":"satu"}]}]}]},
	{"type":"paragraph"}]}`

func TestEditorShapedDocumentIsAcceptedAndNormalised(t *testing.T) {
	asset := uuid.New()
	doc := docFromJSON(strings.Replace(editorDoc, "%s", asset.String(), 1))
	out, reason := validatedDoc(doc, true, maxContentLen, ReasonQuestionContent)
	if reason != "" {
		t.Fatalf("editor document refused: %s", reason)
	}
	const want = `{"type":"doc","content":[` +
		`{"type":"paragraph","content":[{"type":"text","text":"Lihat "},{"type":"text","text":"gambar","marks":[{"type":"italic"}]},{"type":"math","attrs":{"latex":"a+b"}}]},` +
		`{"type":"image","attrs":{"asset_id":"%s"}},` +
		`{"type":"table","content":[` +
		`{"type":"tableRow","content":[{"type":"tableHeader","content":[{"type":"paragraph"}]},{"type":"tableHeader","content":[{"type":"paragraph","content":[{"type":"text","text":"B"}]}]}]},` +
		`{"type":"tableRow","content":[{"type":"tableCell","content":[{"type":"shape","attrs":{"kind":"kubus"}}]},{"type":"tableCell","content":[{"type":"paragraph"}]}]}]},` +
		`{"type":"orderedList","content":[{"type":"listItem","content":[{"type":"paragraph","content":[{"type":"text","text":"satu"}]}]}]}]}`
	if got := string(mustJSON(t, out)); got != strings.Replace(want, "%s", asset.String(), 1) {
		t.Errorf("normalised = %s\nwant       %s", got, want)
	}
}

func TestBackfillSkipsAnUndecodableRowAndNeverRepeatsIt(t *testing.T) {
	h := newQuestionHarness(t)
	tenant := h.a
	ctx := context.Background()
	_, goodVersion := insertLegacy(t, h, tenant, "Hitung $x^2$", "Karena *begitu*")
	_, badVersion := insertLegacy(t, h, tenant, "Rusak", "")
	if _, err := h.pool.Exec(ctx, `UPDATE question_versions SET options = '{"not":"an array"}' WHERE id = $1`, badVersion); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		insertLegacy(t, h, tenant, "Soal lain", "")
	}

	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, nil))
	converted, err := h.svc.BackfillDocs(ctx, logger)
	if err != nil || converted != 4 {
		t.Fatalf("backfill = %d, %v, want the 4 decodable rows converted and no error", converted, err)
	}
	if !strings.Contains(logs.String(), badVersion.String()) {
		t.Errorf("log %q lacks the skipped version id", logs.String())
	}
	var goodDoc, badDoc []byte
	if err := h.pool.QueryRow(ctx, `SELECT content_doc FROM question_versions WHERE id = $1`, goodVersion).Scan(&goodDoc); err != nil || goodDoc == nil {
		t.Errorf("good row content_doc = %s, %v, want a document", goodDoc, err)
	}
	if err := h.pool.QueryRow(ctx, `SELECT content_doc FROM question_versions WHERE id = $1`, badVersion).Scan(&badDoc); err != nil || badDoc != nil {
		t.Errorf("undecodable row content_doc = %s, %v, want it left as text", badDoc, err)
	}

	if n, err := h.svc.BackfillDocs(ctx, logger); err != nil || n != 0 {
		t.Errorf("second backfill = %d, %v, want nothing converted", n, err)
	}
}
