package handler

import (
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestDocumentReasonsNameNoParticularField(t *testing.T) {
	f := newAssetHTTPFixture(t)
	e := f.server(t, time.Minute)
	program := questionCurriculum(t, e, f.hostA, f.tokenAdminA)
	notADoc := map[string]any{"type": "paragraph", "content": []any{}}
	srcImage := map[string]any{"type": "doc", "content": []any{
		map[string]any{"type": "image", "attrs": map[string]any{"src": "https://x.test/a.png"}},
	}}

	inExplanation := pgQuestionBody(program)
	inExplanation["explanation"] = notADoc
	inOption := pgQuestionBody(program)
	inOption["options"] = []any{
		map[string]any{"id": "a", "label": notADoc},
		map[string]any{"id": "b", "label": d("4")},
	}
	for path, body := range map[string]map[string]any{"explanation": inExplanation, "options[0].label": inOption} {
		rec := sendQuestionJSON(e, http.MethodPost, f.hostA, "/api/v1/questions", body, f.tokenAdminA)
		if rec.Code != http.StatusUnprocessableEntity || errorBody(t, rec).Fields[path] != docInvalidRule {
			t.Errorf("%s = %d %s, want 422 with %q", path, rec.Code, rec.Body.String(), docInvalidRule)
		}
	}
	for _, rule := range []string{docInvalidRule, docNodeRule, docMarkRule, docAttrRule, docTooLargeRule, docAssetRule} {
		if strings.Contains(strings.ToLower(rule), "soal") {
			t.Errorf("rule %q names the question, but it is shown for options and explanations too", rule)
		}
	}

	withSrc := pgQuestionBody(program)
	withSrc["content"] = srcImage
	rec := sendQuestionJSON(e, http.MethodPost, f.hostA, "/api/v1/questions", withSrc, f.tokenAdminA)
	if rec.Code != http.StatusUnprocessableEntity || errorBody(t, rec).Fields["content"] != docAttrRule {
		t.Errorf("create with an image src = %d %s, want 422 with the attribute rule on content", rec.Code, rec.Body.String())
	}
}
