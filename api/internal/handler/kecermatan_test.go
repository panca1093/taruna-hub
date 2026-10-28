package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/httpx"
)

// kecermatanBody is a valid 10 x 50 test; the caller names the type and its packages.
func kecermatanBody(program programResponse, itemType string, packageIDs ...string) map[string]any {
	return map[string]any{
		"program_id": program.ID.String(), "title": "Kecermatan " + uuid.NewString()[:8],
		"description": "Latihan kecermatan", "item_type": itemType, "item_length": 4, "options_per_soal": 5,
		"soal_per_kolom": 50, "kolom_count": 10, "kolom_duration_seconds": 120,
		"cover_from": "#f59e0b", "cover_to": "#ef4444", "package_ids": packageIDs,
	}
}

func decodeKecermatan(t *testing.T, rec *httptest.ResponseRecorder) kecermatanResponse {
	t.Helper()
	var body kecermatanResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body %s is not a kecermatan test: %v", rec.Body.String(), err)
	}
	return body
}

func decodeKecermatanPreview(t *testing.T, rec *httptest.ResponseRecorder) kecermatanPreviewResponse {
	t.Helper()
	if rec.Code != http.StatusOK {
		t.Fatalf("preview = %d %s, want 200", rec.Code, rec.Body.String())
	}
	var body kecermatanPreviewResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body %s is not a preview: %v", rec.Body.String(), err)
	}
	return body
}

func (f packageFixture) createKecermatan(t *testing.T, body map[string]any) kecermatanResponse {
	t.Helper()
	rec := sendTryoutJSON(f.e, http.MethodPost, f.hostA, "/api/v1/kecermatan-tests", body, f.tokenAdminA)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create kecermatan test = %d %s, want 201", rec.Code, rec.Body.String())
	}
	return decodeKecermatan(t, rec)
}

func kecermatanPath(id uuid.UUID) string { return "/api/v1/kecermatan-tests/" + id.String() }

func TestKecermatanRoles(t *testing.T) {
	f := newPackageFixture(t)
	program := questionCurriculum(t, f.e, f.hostA, f.tokenAdminA)
	body := func() map[string]any { return kecermatanBody(program, "char") }
	first := f.createKecermatan(t, body())
	operatorPut := f.createKecermatan(t, body())
	adminPut := f.createKecermatan(t, body())
	operatorDelete := f.createKecermatan(t, body())
	adminDelete := f.createKecermatan(t, body())
	path := kecermatanPath(first.ID)

	put := func(test kecermatanResponse) map[string]any {
		b := body()
		b["base_revision"] = test.Revision
		return b
	}
	cases := []struct {
		method, path string
		body         any
		who, token   string
		want         int
	}{
		{"GET", "/api/v1/kecermatan-tests", nil, "anonymous", "", http.StatusUnauthorized},
		{"GET", "/api/v1/kecermatan-tests", nil, "student", f.tokenA, http.StatusForbidden},
		{"GET", "/api/v1/kecermatan-tests", nil, "operator", f.tokenOperatorA, http.StatusOK},
		{"GET", "/api/v1/kecermatan-tests", nil, "admin", f.tokenAdminA, http.StatusOK},
		{"GET", path, nil, "anonymous", "", http.StatusUnauthorized},
		{"GET", path, nil, "student", f.tokenA, http.StatusForbidden},
		{"GET", path, nil, "operator", f.tokenOperatorA, http.StatusOK},
		{"POST", "/api/v1/kecermatan-tests", body(), "anonymous", "", http.StatusUnauthorized},
		{"POST", "/api/v1/kecermatan-tests", body(), "student", f.tokenA, http.StatusForbidden},
		{"POST", "/api/v1/kecermatan-tests", body(), "operator", f.tokenOperatorA, http.StatusCreated},
		{"POST", "/api/v1/kecermatan-tests", body(), "admin", f.tokenAdminA, http.StatusCreated},
		{"PUT", path, put(first), "anonymous", "", http.StatusUnauthorized},
		{"PUT", path, put(first), "student", f.tokenA, http.StatusForbidden},
		{"PUT", kecermatanPath(operatorPut.ID), put(operatorPut), "operator", f.tokenOperatorA, http.StatusOK},
		{"PUT", kecermatanPath(adminPut.ID), put(adminPut), "admin", f.tokenAdminA, http.StatusOK},
		{"POST", path + "/preview", map[string]any{}, "anonymous", "", http.StatusUnauthorized},
		{"POST", path + "/preview", map[string]any{}, "student", f.tokenA, http.StatusForbidden},
		{"POST", path + "/preview", map[string]any{}, "operator", f.tokenOperatorA, http.StatusOK},
		{"POST", path + "/preview", map[string]any{}, "admin", f.tokenAdminA, http.StatusOK},
		{"DELETE", kecermatanPath(operatorDelete.ID), nil, "anonymous", "", http.StatusUnauthorized},
		{"DELETE", kecermatanPath(operatorDelete.ID), nil, "student", f.tokenA, http.StatusForbidden},
		{"DELETE", kecermatanPath(operatorDelete.ID), nil, "operator", f.tokenOperatorA, http.StatusNoContent},
		{"DELETE", kecermatanPath(adminDelete.ID), nil, "admin", f.tokenAdminA, http.StatusNoContent},
	}
	for _, tc := range cases {
		rec := sendTryoutJSON(f.e, tc.method, f.hostA, tc.path, tc.body, tc.token)
		if rec.Code != tc.want {
			t.Errorf("%s %s as %s: status = %d body = %s, want %d", tc.method, tc.path, tc.who, rec.Code, rec.Body.String(), tc.want)
		}
	}
}

func TestKecermatanIsolatedBetweenTenants(t *testing.T) {
	f := newPackageFixture(t)
	programA := questionCurriculum(t, f.e, f.hostA, f.tokenAdminA)
	programB := questionCurriculum(t, f.e, f.hostB, f.tokenAdmB)
	packageA := f.create(t, f.hostA, f.tokenAdminA, "symbol", "Milik A")
	f.symbols(t, packageA.ID, strings.Join(strings.Split("♠♥♦♣★☆●○■□▲△▼▽◆◇←↑→↓", ""), " "))
	mine := f.createKecermatan(t, kecermatanBody(programA, "symbol", packageA.ID.String()))
	path := kecermatanPath(mine.ID)

	if list := sendTryoutJSON(f.e, http.MethodGet, f.hostB, "/api/v1/kecermatan-tests", nil, f.tokenAdmB); strings.Contains(list.Body.String(), mine.ID.String()) {
		t.Error("tenant B's list carries tenant A's test")
	}
	update := kecermatanBody(programB, "char")
	update["base_revision"] = mine.Revision
	for name, rec := range map[string]*httptest.ResponseRecorder{
		"GET":     sendTryoutJSON(f.e, http.MethodGet, f.hostB, path, nil, f.tokenAdmB),
		"PUT":     sendTryoutJSON(f.e, http.MethodPut, f.hostB, path, update, f.tokenAdmB),
		"DELETE":  sendTryoutJSON(f.e, http.MethodDelete, f.hostB, path, nil, f.tokenAdmB),
		"preview": sendTryoutJSON(f.e, http.MethodPost, f.hostB, path+"/preview", map[string]any{}, f.tokenAdmB),
	} {
		if rec.Code != http.StatusNotFound {
			t.Errorf("B %s on A's test = %d %s, want 404", name, rec.Code, rec.Body.String())
		}
	}

	// B cannot pin A's package, nor build on A's program, and neither answer reveals the other exists.
	wantError(t, "pin another tenant's package",
		sendTryoutJSON(f.e, http.MethodPost, f.hostB, "/api/v1/kecermatan-tests", kecermatanBody(programB, "symbol", packageA.ID.String()), f.tokenAdmB),
		http.StatusUnprocessableEntity, httpx.CodeValidation, "package_ids[0]")
	wantError(t, "use another tenant's program",
		sendTryoutJSON(f.e, http.MethodPost, f.hostB, "/api/v1/kecermatan-tests", kecermatanBody(programA, "char"), f.tokenAdmB),
		http.StatusUnprocessableEntity, httpx.CodeValidation, "program_id")

	if still := decodeKecermatan(t, sendTryoutJSON(f.e, http.MethodGet, f.hostA, path, nil, f.tokenAdminA)); still.Revision != mine.Revision || still.Title != mine.Title {
		t.Errorf("A's test after B's attempts = %+v, want it untouched", still)
	}
}

func TestKecermatanRejectsInvalidDefinitions(t *testing.T) {
	f := newPackageFixture(t)
	program := questionCurriculum(t, f.e, f.hostA, f.tokenAdminA)
	create := func(mutate func(map[string]any)) *httptest.ResponseRecorder {
		body := kecermatanBody(program, "char")
		mutate(body)
		return sendTryoutJSON(f.e, http.MethodPost, f.hostA, "/api/v1/kecermatan-tests", body, f.tokenAdminA)
	}
	cases := []struct {
		name   string
		mutate func(map[string]any)
		field  string
	}{
		{"odd kolom", func(b map[string]any) { b["kolom_count"] = 11 }, "kolom_count"},
		{"fewer than 10 kolom", func(b map[string]any) { b["kolom_count"] = 8 }, "kolom_count"},
		{"more than 50 kolom", func(b map[string]any) { b["kolom_count"] = 52 }, "kolom_count"},
		{"fewer than 50 soal", func(b map[string]any) { b["soal_per_kolom"] = 49 }, "soal_per_kolom"},
		{"more than 100 soal", func(b map[string]any) { b["soal_per_kolom"] = 101 }, "soal_per_kolom"},
		{"item length 0", func(b map[string]any) { b["item_length"] = 0 }, "item_length"},
		{"item length 6", func(b map[string]any) { b["item_length"] = 6 }, "item_length"},
		{"zero duration", func(b map[string]any) { b["kolom_duration_seconds"] = 0 }, "kolom_duration_seconds"},
		{"negative duration", func(b map[string]any) { b["kolom_duration_seconds"] = -60 }, "kolom_duration_seconds"},
		{"two options", func(b map[string]any) { b["options_per_soal"] = 2 }, "options_per_soal"},
		{"unknown type", func(b map[string]any) { b["item_type"] = "audio" }, "item_type"},
		{"blank title", func(b map[string]any) { b["title"] = "   " }, "title"},
		{"bad cover", func(b map[string]any) { b["cover_from"] = "red" }, "cover_from"},
		{"char with a package", func(b map[string]any) { b["package_ids"] = []string{uuid.NewString()} }, "package_ids"},
		{"pool too small: 10 digits for 5 x 4", func(b map[string]any) { b["item_type"] = "number" }, "pool"},
		{"pool too small: 7 shapes for 5 x 4", func(b map[string]any) { b["item_type"] = "shape" }, "pool"},
	}
	for _, tc := range cases {
		wantError(t, tc.name, create(tc.mutate), http.StatusUnprocessableEntity, httpx.CodeValidation, tc.field)
	}

	// The edge that is allowed: 26 letters cover 5 x 5 = 25 with one to spare, and 8 x 3 shapes fit nothing.
	edge := create(func(b map[string]any) { b["item_length"], b["options_per_soal"] = 5, 5 })
	if edge.Code != http.StatusCreated {
		t.Errorf("5 x 5 from 26 letters = %d %s, want 201", edge.Code, edge.Body.String())
	}
	if rec := create(func(b map[string]any) { b["item_length"], b["options_per_soal"] = 5, 6 }); rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("6 x 5 = 30 from 26 letters = %d, want 422", rec.Code)
	}
	if rec := create(func(b map[string]any) { b["kolom_count"], b["soal_per_kolom"] = 50, 100 }); rec.Code != http.StatusCreated {
		t.Errorf("50 kolom x 100 soal = %d %s, want 201 (D09 ceiling)", rec.Code, rec.Body.String())
	}
}

func TestKecermatanEditLocksAfterTheFirstAttemptAndOnAStaleRevision(t *testing.T) {
	f := newPackageFixture(t)
	program := questionCurriculum(t, f.e, f.hostA, f.tokenAdminA)
	test := f.createKecermatan(t, kecermatanBody(program, "char"))
	path := kecermatanPath(test.ID)
	if test.Revision != 1 || test.Locked {
		t.Fatalf("created = revision %d locked %v, want 1 and unlocked", test.Revision, test.Locked)
	}

	edit := func(revision int, title string) *httptest.ResponseRecorder {
		body := kecermatanBody(program, "char")
		body["title"], body["base_revision"] = title, revision
		return sendTryoutJSON(f.e, http.MethodPut, f.hostA, path, body, f.tokenAdminA)
	}
	saved := decodeKecermatan(t, edit(1, "Judul baru"))
	if saved.Revision != 2 || saved.Title != "Judul baru" {
		t.Fatalf("saved = %+v, want revision 2 and the new title", saved)
	}
	if rec := edit(1, "Basi"); rec.Code != http.StatusConflict {
		t.Errorf("save on a stale revision = %d %s, want 409", rec.Code, rec.Body.String())
	}
	if rec := edit(0, "Tanpa versi"); rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("save without base_revision = %d, want 422", rec.Code)
	}

	// P18's admission stamps first_attempt_at under the test row lock; this stands in for it.
	if _, err := f.pool.Exec(context.Background(), `UPDATE kecermatan_tests SET first_attempt_at = now() WHERE id = $1`, test.ID); err != nil {
		t.Fatalf("stamp the first attempt: %v", err)
	}
	if got := decodeKecermatan(t, sendTryoutJSON(f.e, http.MethodGet, f.hostA, path, nil, f.tokenAdminA)); !got.Locked {
		t.Error("a test with a first attempt does not report locked")
	}
	locked := edit(saved.Revision, "Setelah dikerjakan")
	if locked.Code != http.StatusConflict || !strings.Contains(locked.Body.String(), "sudah dikerjakan") {
		t.Errorf("edit after a first attempt = %d %s, want 409 saying it was attempted", locked.Code, locked.Body.String())
	}
	if rec := sendTryoutJSON(f.e, http.MethodDelete, f.hostA, path, nil, f.tokenAdminA); rec.Code != http.StatusConflict {
		t.Errorf("delete after a first attempt = %d %s, want 409", rec.Code, rec.Body.String())
	}
	after := decodeKecermatan(t, sendTryoutJSON(f.e, http.MethodGet, f.hostA, path, nil, f.tokenAdminA))
	if after.Title != "Judul baru" || after.Revision != saved.Revision {
		t.Errorf("after the refused edit and delete = %+v, want the definition untouched", after)
	}
	// Previewing is read-only, so a locked test still shows its mechanism.
	if rec := sendTryoutJSON(f.e, http.MethodPost, f.hostA, path+"/preview", map[string]any{}, f.tokenAdminA); rec.Code != http.StatusOK {
		t.Errorf("preview of a locked test = %d, want 200", rec.Code)
	}
}

func TestKecermatanPackagesArePinnedAndGuardedByTheTest(t *testing.T) {
	f := newPackageFixture(t)
	program := questionCurriculum(t, f.e, f.hostA, f.tokenAdminA)
	symbols := f.create(t, f.hostA, f.tokenAdminA, "symbol", "Simbol uji")
	f.symbols(t, symbols.ID, strings.Join(strings.Split("♠♥♦♣★☆●○■□▲△▼▽◆◇←↑→↓", ""), " "))
	small := f.create(t, f.hostA, f.tokenAdminA, "symbol", "Simbol kecil")
	f.symbols(t, small.ID, "♠ ♥ ♦")
	emoji := f.create(t, f.hostA, f.tokenAdminA, "emoji", "Emoji uji")
	shape := f.create(t, f.hostA, f.tokenAdminA, "shape", "Bentuk uji")
	elementsCall(t, f.e, f.hostA, f.tokenAdminA, commitPath(shape.ID), map[string][]string{"ids": {"kotak", "segitiga", "lingkaran"}})

	wantError(t, "a pool of 3 for 5 x 4", sendTryoutJSON(f.e, http.MethodPost, f.hostA, "/api/v1/kecermatan-tests", kecermatanBody(program, "symbol", small.ID.String()), f.tokenAdminA),
		http.StatusUnprocessableEntity, httpx.CodeValidation, "pool")
	wantError(t, "an emoji package for a symbol test", sendTryoutJSON(f.e, http.MethodPost, f.hostA, "/api/v1/kecermatan-tests", kecermatanBody(program, "symbol", emoji.ID.String()), f.tokenAdminA),
		http.StatusUnprocessableEntity, httpx.CodeValidation, "package_ids[0]")
	wantError(t, "a symbol test without a package", sendTryoutJSON(f.e, http.MethodPost, f.hostA, "/api/v1/kecermatan-tests", kecermatanBody(program, "symbol"), f.tokenAdminA),
		http.StatusUnprocessableEntity, httpx.CodeValidation, "package_ids")
	wantError(t, "the same package twice in a mix", sendTryoutJSON(f.e, http.MethodPost, f.hostA, "/api/v1/kecermatan-tests", kecermatanBody(program, "mix", symbols.ID.String(), symbols.ID.String()), f.tokenAdminA),
		http.StatusUnprocessableEntity, httpx.CodeValidation, "package_ids[1]")
	wantError(t, "a package id that is not a uuid", sendTryoutJSON(f.e, http.MethodPost, f.hostA, "/api/v1/kecermatan-tests", kecermatanBody(program, "symbol", "nope"), f.tokenAdminA),
		http.StatusUnprocessableEntity, httpx.CodeValidation, "package_ids[0]")

	test := f.createKecermatan(t, kecermatanBody(program, "symbol", symbols.ID.String()))
	if len(test.Packages) != 1 || test.Packages[0].PackageID != symbols.ID || test.Packages[0].Revision != 2 || test.Packages[0].ElementCount != 20 {
		t.Fatalf("packages = %+v, want the symbol package pinned at revision 2 with its 20 elements", test.Packages)
	}
	if raw := sendTryoutJSON(f.e, http.MethodGet, f.hostA, kecermatanPath(test.ID), nil, f.tokenAdminA).Body.String(); strings.Contains(raw, "seed") {
		t.Errorf("GET exposes the generation seed: %s", raw)
	}

	// P09's guards now see a real test: the pinned package cannot be deleted or shrunk below the need.
	if rec := sendTryoutJSON(f.e, http.MethodDelete, f.hostA, packagePath(symbols.ID), nil, f.tokenAdminA); rec.Code != http.StatusConflict || errorBody(t, rec).Code != httpx.Code("package_in_use") {
		t.Errorf("delete of a pinned package = %d %s, want 409 package_in_use", rec.Code, rec.Body.String())
	}
	elements := f.detail(t, f.hostA, f.tokenAdminA, symbols.ID, "").Elements
	if rec := sendTryoutJSON(f.e, http.MethodDelete, f.hostA, elementPath(symbols.ID, elements[0].ID), nil, f.tokenAdminA); rec.Code != http.StatusConflict ||
		errorBody(t, rec).Code != httpx.Code("package_too_small") {
		t.Errorf("shrinking a 20-element package a 20-element test needs = %d %s, want 409 package_too_small", rec.Code, rec.Body.String())
	}

	// Growing the package writes a revision the test does not use until it is saved again.
	f.symbols(t, symbols.ID, "✓ ✗")
	same := decodeKecermatan(t, sendTryoutJSON(f.e, http.MethodGet, f.hostA, kecermatanPath(test.ID), nil, f.tokenAdminA))
	if same.Packages[0].Revision != 2 || same.Packages[0].ElementCount != 20 {
		t.Errorf("after the package grew: %+v, want the test still on revision 2 with 20 elements", same.Packages[0])
	}
	body := kecermatanBody(program, "symbol", symbols.ID.String())
	body["base_revision"] = same.Revision
	resaved := decodeKecermatan(t, sendTryoutJSON(f.e, http.MethodPut, f.hostA, kecermatanPath(test.ID), body, f.tokenAdminA))
	if resaved.Packages[0].Revision != 3 || resaved.Packages[0].ElementCount != 22 {
		t.Errorf("after saving again: %+v, want the package re-pinned at revision 3 with 22 elements", resaved.Packages[0])
	}

	// Deleting the test releases the package.
	if rec := sendTryoutJSON(f.e, http.MethodDelete, f.hostA, kecermatanPath(test.ID), nil, f.tokenAdminA); rec.Code != http.StatusNoContent {
		t.Fatalf("delete the test = %d %s, want 204", rec.Code, rec.Body.String())
	}
	if rec := sendTryoutJSON(f.e, http.MethodDelete, f.hostA, packagePath(symbols.ID), nil, f.tokenAdminA); rec.Code != http.StatusNoContent && rec.Code != http.StatusOK {
		t.Errorf("delete of the released package = %d %s, want it allowed", rec.Code, rec.Body.String())
	}

	// A shape package is optional; mix takes built-ins and any package type.
	f.createKecermatan(t, func() map[string]any {
		b := kecermatanBody(program, "shape", shape.ID.String())
		b["item_length"], b["options_per_soal"] = 1, 3
		return b
	}())
	mix := kecermatanBody(program, "mix")
	mix["builtin_sources"] = []string{"char", "number"}
	if created := f.createKecermatan(t, mix); len(created.BuiltinSources) != 2 || created.BuiltinSources[0] != "char" {
		t.Errorf("mix builtin sources = %v, want char and number", created.BuiltinSources)
	}
}

func TestKecermatanPreviewShowsTheRepetitionAndStoresNothing(t *testing.T) {
	f := newPackageFixture(t)
	program := questionCurriculum(t, f.e, f.hostA, f.tokenAdminA)
	test := f.createKecermatan(t, kecermatanBody(program, "char"))
	path := kecermatanPath(test.ID) + "/preview"

	var storedBefore string
	read := func() string {
		var seed string
		if err := f.pool.QueryRow(context.Background(), `SELECT generation_seed FROM kecermatan_tests WHERE id = $1`, test.ID).Scan(&seed); err != nil {
			t.Fatal(err)
		}
		return seed
	}
	storedBefore = read()
	var rowsBefore int
	_ = f.pool.QueryRow(context.Background(), `SELECT count(*) FROM kecermatan_tests WHERE tenant_id = $1`, f.tenantA.ID).Scan(&rowsBefore)

	first := decodeKecermatanPreview(t, sendTryoutJSON(f.e, http.MethodPost, f.hostA, path, map[string]any{"kolom": 1}, f.tokenAdminA))
	if first.Kolom != 1 || first.SourceKolom != 1 || len(first.Reference) != 5 || len(first.Soal) != 50 || len(first.Seed) != 32 {
		t.Fatalf("preview = kolom %d source %d, %d reference, %d soal, seed %q; want kolom 1, 5 items, 50 soal", first.Kolom, first.SourceKolom, len(first.Reference), len(first.Soal), first.Seed)
	}
	if first.Seed == storedBefore {
		t.Error("the preview reused the test's stored seed")
	}
	seen := map[string]bool{}
	for _, item := range first.Reference {
		if len(item.Elements) != 4 {
			t.Fatalf("reference item %s has %d elements, want 4", item.Label, len(item.Elements))
		}
		for _, e := range item.Elements {
			if seen[e.Value] {
				t.Errorf("element %q appears twice in the reference set", e.Value)
			}
			seen[e.Value] = true
		}
	}

	// The same seed asked for kolom 6 (N/2 + 1) replays kolom 1, and a new seed gives different content.
	replay := decodeKecermatanPreview(t, sendTryoutJSON(f.e, http.MethodPost, f.hostA, path, map[string]any{"kolom": 6, "seed": first.Seed}, f.tokenAdminA))
	if replay.Kolom != 6 || replay.SourceKolom != 1 {
		t.Errorf("kolom 6 = source %d, want it to replay generated column 1", replay.SourceKolom)
	}
	replay.Kolom, replay.SourceKolom = first.Kolom, first.SourceKolom
	a, _ := json.Marshal(first)
	b, _ := json.Marshal(replay)
	if string(a) != string(b) {
		t.Error("kolom 6 differs from kolom 1 for the same seed")
	}
	other := decodeKecermatanPreview(t, sendTryoutJSON(f.e, http.MethodPost, f.hostA, path, map[string]any{"kolom": 1}, f.tokenAdminA))
	if other.Seed == first.Seed {
		t.Error("two previews without a seed used the same seed")
	}

	for name, body := range map[string]map[string]any{
		"kolom 0 is the default": nil, "kolom 11": {"kolom": 11}, "negative kolom": {"kolom": -1}, "bad seed": {"seed": "xyz"},
	} {
		want := http.StatusUnprocessableEntity
		if body == nil {
			body, want = map[string]any{}, http.StatusOK
		}
		if rec := sendTryoutJSON(f.e, http.MethodPost, f.hostA, path, body, f.tokenAdminA); rec.Code != want {
			t.Errorf("preview %s = %d %s, want %d", name, rec.Code, rec.Body.String(), want)
		}
	}

	var rowsAfter int
	_ = f.pool.QueryRow(context.Background(), `SELECT count(*) FROM kecermatan_tests WHERE tenant_id = $1`, f.tenantA.ID).Scan(&rowsAfter)
	if read() != storedBefore || rowsAfter != rowsBefore {
		t.Error("a preview changed the stored test")
	}
}
