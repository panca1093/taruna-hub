package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/model"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/repository"
)

const tableCellText = "sel-unik-9137"

// richContent is a document with every block type, shaped the way Tiptap sends it (extra attributes included).
func richContent() model.RichDoc {
	return docFromJSON(`{"type":"doc","content":[
		{"type":"paragraph","content":[{"type":"text","text":"Perhatikan "},{"type":"text","text":"tabel","marks":[{"type":"bold"}]},{"type":"math","attrs":{"latex":"x^2"}}]},
		{"type":"table","content":[{"type":"tableRow","content":[
			{"type":"tableHeader","attrs":{"colspan":1,"rowspan":1,"colwidth":null},"content":[{"type":"paragraph","content":[{"type":"text","text":"A"}]}]},
			{"type":"tableCell","attrs":{"colspan":1,"rowspan":1,"colwidth":null},"content":[{"type":"paragraph","content":[{"type":"text","text":"` + tableCellText + `"}]}]}
		]}]},
		{"type":"shape","attrs":{"kind":"kubus","caption":"gambar 1"}},
		{"type":"orderedList","attrs":{"start":1,"type":null},"content":[{"type":"listItem","content":[{"type":"paragraph","content":[{"type":"text","text":"langkah"}]}]}]}
	]}`)
}

func docFromJSON(raw string) model.RichDoc {
	var doc model.RichDoc
	if err := json.Unmarshal([]byte(raw), &doc); err != nil {
		panic(err)
	}
	return doc
}

func imageNode(assetID uuid.UUID) model.RichDoc {
	return imageDoc(map[string]any{"asset_id": assetID.String()})
}

func (h questionHarness) insertAsset(t *testing.T, tenant model.Tenant, purpose string, uploader uuid.UUID) uuid.UUID {
	t.Helper()
	id := uuid.New()
	_, err := repository.NewAssetRepository(h.pool).Insert(context.Background(), tenant.ID, model.Asset{
		ID: id, Purpose: purpose, ObjectKey: tenant.ID.String() + "/" + purpose + "/" + id.String(),
		ContentType: "image/png", SizeBytes: 10, UploadedBy: &uploader,
	})
	if err != nil {
		t.Fatalf("insert asset: %v", err)
	}
	t.Cleanup(func() {
		_, _ = h.pool.Exec(context.Background(), `DELETE FROM assets WHERE id = $1`, id)
	})
	return id
}

func assetStatus(t *testing.T, h questionHarness, tenant model.Tenant, id uuid.UUID) string {
	t.Helper()
	var status string
	if err := h.pool.QueryRow(context.Background(), `SELECT status FROM assets WHERE tenant_id = $1 AND id = $2`, tenant.ID, id).Scan(&status); err != nil {
		t.Fatalf("read asset status: %v", err)
	}
	return status
}

func TestQuestionDocumentsRoundTripNormalisedAndSearchable(t *testing.T) {
	h := newQuestionHarness(t)
	tenant := h.a
	in := h.pgInput(tenant)
	in.Content = richContent()
	in.Explanation = d("Karena **begitu**.")
	in.Options = []QuestionOptionInput{{ID: "a", Label: d("$3$")}, {ID: "b", Label: d("4")}}
	created := h.create(t, tenant, in)

	table := created.Content.Content[1]
	if got := table.Content[0].Content[0].Attrs; len(got) != 0 {
		t.Errorf("header attrs = %v, want the default colspan/rowspan/colwidth dropped", got)
	}
	if got := created.Content.Content[3].Attrs; len(got) != 0 {
		t.Errorf("ordered list attrs = %v, want the default start dropped", got)
	}
	if got := created.Options[0].Doc.Content[0].Content[0]; got.Type != model.NodeMath || got.Attrs[model.AttrLatex] != "3" {
		t.Errorf("option doc = %+v, want a math node", got)
	}
	if created.Options[0].Label != "$3$" {
		t.Errorf("stored option label = %q, want the legacy text projection", created.Options[0].Label)
	}

	again, err := h.svc.Get(context.Background(), h.scope(tenant), h.principal(tenant, model.RoleAdmin), created.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if !sameDoc(again.Content, created.Content) || !sameDoc(again.Explanation, created.Explanation) {
		t.Error("documents changed between create and get")
	}

	found, total := h.list(t, tenant, QuestionFilter{Search: tableCellText})
	if total != 1 || len(found) != 1 || found[0].ID != created.ID {
		t.Errorf("search for text inside a table cell = %d results, want the question", total)
	}
	var text string
	if err := h.pool.QueryRow(context.Background(), `SELECT content FROM question_versions WHERE id = $1`, created.VersionID).Scan(&text); err != nil {
		t.Fatal(err)
	}
	if want := model.DocLegacyText(created.Content); text != want {
		t.Errorf("content column = %q, want the projection %q", text, want)
	}
}

func TestUnchangedDocumentUpdateCreatesNoVersion(t *testing.T) {
	h := newQuestionHarness(t)
	tenant := h.a
	in := h.pgInput(tenant)
	in.Content = richContent()
	created := h.create(t, tenant, in)

	same := QuestionInput{
		ProgramID: created.ProgramID, SubjectID: created.SubjectID, Type: created.Type, Difficulty: created.Difficulty,
		Content: richContent(), Explanation: created.Explanation, BaseVersion: created.Version, Correct: created.Correct,
	}
	for _, option := range created.Options {
		same.Options = append(same.Options, QuestionOptionInput{ID: option.ID, Label: option.Doc})
	}
	got, err := h.update(t, tenant, created.ID, same)
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if got.Version != created.Version {
		t.Errorf("version after an unchanged save = %d, want %d", got.Version, created.Version)
	}

	same.Content = d("Isi lain.")
	if got, err = h.update(t, tenant, created.ID, same); err != nil || got.Version != created.Version+1 {
		t.Errorf("changed save = version %d, err %v, want a new version", got.Version, err)
	}
}

func TestQuestionImagesAreFinalizedInTheVersionTransaction(t *testing.T) {
	h := newQuestionHarness(t)
	tenant := h.a
	author := h.author[tenant.ID].ID
	image := h.insertAsset(t, tenant, model.PurposeQuestionImage, author)

	in := h.pgInput(tenant)
	in.Content = imageNode(image)
	created := h.create(t, tenant, in)
	if got := model.DocAssetIDs(created.Content); len(got) != 1 || got[0] != image {
		t.Fatalf("content asset ids = %v, want %s", got, image)
	}
	if status := assetStatus(t, h, tenant, image); status != model.AssetFinal {
		t.Errorf("asset status = %q, want final", status)
	}

	// A different staff user saving a question that carries the already final image succeeds.
	email := "second-" + uuid.NewString()[:8] + "@example.com"
	second, err := repository.NewUserRepository(h.pool).Create(context.Background(), model.User{
		TenantID: &tenant.ID, Name: "Operator Soal", Email: &email, PasswordHash: "x", Role: model.RoleOperator,
	})
	if err != nil {
		t.Fatalf("create second staff user: %v", err)
	}
	other := model.Principal{UserID: second.ID, TenantID: &tenant.ID, Role: model.RoleOperator}
	if _, err := h.svc.Create(context.Background(), h.scope(tenant), other, in); err != nil {
		t.Errorf("another staff user re-using a final image: %v", err)
	}

	optionImage := h.insertAsset(t, tenant, model.PurposeQuestionImage, author)
	edit := in
	edit.BaseVersion = created.Version
	edit.Explanation = imageNode(optionImage)
	edit.Options = []QuestionOptionInput{{ID: created.Options[0].ID, Label: created.Options[0].Doc}, {ID: created.Options[1].ID, Label: created.Options[1].Doc}}
	edit.Correct = created.Correct
	if _, err := h.update(t, tenant, created.ID, edit); err != nil {
		t.Fatalf("update with a new explanation image: %v", err)
	}
	if status := assetStatus(t, h, tenant, optionImage); status != model.AssetFinal {
		t.Errorf("explanation image status = %q, want final", status)
	}

	proof := h.insertAsset(t, tenant, model.PurposePaymentProof, author)
	foreign := h.insertAsset(t, h.b, model.PurposeQuestionImage, h.author[h.b.ID].ID)
	for name, tc := range map[string]struct {
		mutate func(*QuestionInput)
		path   string
	}{
		"wrong purpose in content":       {func(in *QuestionInput) { in.Content = imageNode(proof) }, "content"},
		"foreign tenant in content":      {func(in *QuestionInput) { in.Content = imageNode(foreign) }, "content"},
		"unknown asset in explanation":   {func(in *QuestionInput) { in.Explanation = imageNode(uuid.New()) }, "explanation"},
		"foreign tenant in option label": {func(in *QuestionInput) { in.Options[1].Label = imageNode(foreign) }, "options[1].label"},
	} {
		bad := h.pgInput(tenant)
		tc.mutate(&bad)
		_, err := h.svc.Create(context.Background(), h.scope(tenant), h.principal(tenant, model.RoleAdmin), bad)
		var input *InputError
		if !errors.As(err, &input) || input.Fields[tc.path] != ReasonDocAsset {
			t.Errorf("%s: err = %v, want %s on %s", name, err, ReasonDocAsset, tc.path)
		}
	}
	if status := assetStatus(t, h, tenant, proof); status != model.AssetStaged {
		t.Errorf("rejected asset status = %q, want it left staged", status)
	}
	if status := assetStatus(t, h, h.b, foreign); status != model.AssetStaged {
		t.Errorf("other tenant's asset status = %q, want it untouched", status)
	}
}

// insertLegacy stores a version the way the app did before documents: text columns and plain options only.
func insertLegacy(t *testing.T, h questionHarness, tenant model.Tenant, content, explanation string) (questionID, versionID uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	author := h.author[tenant.ID].ID
	if err := h.pool.QueryRow(ctx,
		`INSERT INTO questions (tenant_id, current_version, created_by) VALUES ($1, 1, $2) RETURNING id`,
		tenant.ID, author).Scan(&questionID); err != nil {
		t.Fatalf("insert legacy question: %v", err)
	}
	options := `[{"id":"a","label":"3"},{"id":"b","label":"**4**"}]`
	if err := h.pool.QueryRow(ctx,
		`INSERT INTO question_versions (question_id, tenant_id, version, program_id, subject_id, type, content, difficulty,
		        explanation, options, correct, created_by)
		 VALUES ($1, $2, 1, $3, $4, 'pg', $5, 'easy', $6, $7::jsonb, '["b"]', $8) RETURNING id`,
		questionID, tenant.ID, h.program(tenant).ID, h.subject(tenant).ID, content, explanation, options, author).Scan(&versionID); err != nil {
		t.Fatalf("insert legacy version: %v", err)
	}
	return questionID, versionID
}

func TestRowsWithOnlyLegacyTextReadAsDocuments(t *testing.T) {
	h := newQuestionHarness(t)
	tenant := h.a
	id, _ := insertLegacy(t, h, tenant, "Hitung $x^2$ lalu **tebal**\n- a\n- b", "Karena *begitu*")

	got, err := h.svc.Get(context.Background(), h.scope(tenant), h.principal(tenant, model.RoleAdmin), id)
	if err != nil {
		t.Fatalf("get legacy row: %v", err)
	}
	if !sameDoc(got.Content, model.DocFromLegacyText("Hitung $x^2$ lalu **tebal**\n- a\n- b")) {
		t.Errorf("content = %+v, want the converted legacy text", got.Content)
	}
	if text := model.DocText(got.Explanation); text != "Karena begitu" {
		t.Errorf("explanation text = %q, want Karena begitu", text)
	}
	if bold := got.Options[1].Doc.Content[0].Content[0]; len(bold.Marks) != 1 || bold.Marks[0].Type != model.MarkBold {
		t.Errorf("option doc = %+v, want a bold 4", got.Options[1].Doc)
	}
}

func TestBackfillDocsConvertsLegacyRowsOnceAndKeepsTheText(t *testing.T) {
	h := newQuestionHarness(t)
	tenant := h.a
	_, versionID := insertLegacy(t, h, tenant, "Hitung $x^2$ lalu **tebal**", "Karena *begitu*")
	ctx := context.Background()

	if n, err := h.svc.BackfillDocs(ctx, nil); err != nil || n < 1 {
		t.Fatalf("backfill = %d, %v, want the legacy row converted", n, err)
	}
	var content, explanation, options string
	var contentDoc, explanationDoc []byte
	if err := h.pool.QueryRow(ctx,
		`SELECT content, explanation, options::text, content_doc, explanation_doc FROM question_versions WHERE id = $1`,
		versionID).Scan(&content, &explanation, &options, &contentDoc, &explanationDoc); err != nil {
		t.Fatal(err)
	}
	if content != "Hitung $x^2$ lalu **tebal**" || explanation != "Karena *begitu*" {
		t.Errorf("text columns = %q / %q, want them untouched", content, explanation)
	}
	if contentDoc == nil || explanationDoc == nil {
		t.Fatal("documents were not written")
	}
	var stored []model.QuestionOption
	if err := json.Unmarshal([]byte(options), &stored); err != nil {
		t.Fatal(err)
	}
	if len(stored) != 2 || stored[0].Label != "3" || stored[1].Label != "**4**" || stored[1].Doc.Type != model.NodeDoc {
		t.Errorf("options = %s, want labels kept and a doc added to each", options)
	}
	doc, err := model.DocFromColumn(contentDoc, "")
	if err != nil || !sameDoc(doc, model.DocFromLegacyText(content)) {
		t.Errorf("stored content doc = %+v, %v", doc, err)
	}

	if n, err := h.svc.BackfillDocs(ctx, nil); err != nil || n != 0 {
		t.Errorf("second backfill = %d, %v, want nothing left to convert", n, err)
	}
}

func TestImportedTextBecomesDocumentsAndKeepsRefusingDataImages(t *testing.T) {
	h := newQuestionHarness(t)
	tenant := h.a
	cell := "Nilai $x^2$ adalah **tebal**\n- satu\n- dua"

	rows := importSheetRows(
		importPG(map[string]string{"content": cell, "explanation": "Lihat *ini*", "option_a": "$3$", "option_b": "4"}),
		importPG(map[string]string{"content": "![x](data:image/png;base64,AAAA)"}),
		importPG(map[string]string{"explanation": "![x](javascript:alert(1))"}),
	)
	data := importCSV(rows)
	preview := h.preview(t, tenant, "docs.csv", data)
	if preview.Valid != 1 || preview.Invalid != 2 {
		t.Fatalf("preview = %d valid, %d invalid, want 1 and 2", preview.Valid, preview.Invalid)
	}
	if preview.Rows[0].Preview.Content != cell {
		t.Errorf("preview content = %q, want the plain cell text", preview.Rows[0].Preview.Content)
	}
	for at, field := range map[int]string{1: "content", 2: "explanation"} {
		if errs := preview.Rows[at].Errors; len(errs) != 1 || errs[0].Field != field || errs[0].Reason != ReasonQuestionImage {
			t.Errorf("row %d errors = %+v, want %s on %s", at+1, errs, ReasonQuestionImage, field)
		}
	}

	commit, err := h.commit(t, tenant, "docs.csv", data, []int{1})
	if err != nil || commit.Created != 1 {
		t.Fatalf("commit = %+v, %v", commit, err)
	}
	question, err := h.svc.Get(context.Background(), h.scope(tenant), h.principal(tenant, model.RoleAdmin), commit.QuestionIDs[0])
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if !sameDoc(question.Content, model.DocFromLegacyText(cell)) {
		t.Errorf("content = %+v, want the converted cell", question.Content)
	}
	if math := question.Options[0].Doc.Content[0].Content[0]; math.Type != model.NodeMath {
		t.Errorf("option doc = %+v, want a math node", question.Options[0].Doc)
	}
}
