package handler

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"
)

func isDoc(v any) bool {
	m, ok := v.(map[string]any)
	if !ok || m["type"] != "doc" {
		return false
	}
	_, isArray := m["content"].([]any)
	return isArray
}

func TestStudentRuntimeAndReviewCarryDocumentsWithoutLeaks(t *testing.T) {
	f := newRuntimeHTTPFixture(t)
	f.enroll(t, f.studentA)
	future := time.Now().Add(time.Hour)
	f.setRelease(t, &future)

	session := f.startSession(t, f.tokenA)
	f.call(http.MethodPost, sessionPath(session.ID, "/subtests/"+f.sub(0)+"/start"), nil, f.tokenA)
	live := f.call(http.MethodGet, sessionPath(session.ID, "/subtests/"+f.sub(0)), nil, f.tokenA)
	var runtime struct {
		Questions []map[string]any `json:"questions"`
	}
	if err := json.Unmarshal(live.Body.Bytes(), &runtime); err != nil || len(runtime.Questions) != 1 {
		t.Fatalf("runtime body = %s, err = %v", live.Body.String(), err)
	}
	question := runtime.Questions[0]
	if !isDoc(question["content"]) {
		t.Errorf("runtime content = %v, want a document", question["content"])
	}
	for _, option := range question["options"].([]any) {
		if label := option.(map[string]any)["label"]; !isDoc(label) {
			t.Errorf("runtime option label = %v, want a document", label)
		}
	}
	for _, leaked := range []string{`"correct"`, `"explanation"`, `"points"`} {
		if strings.Contains(live.Body.String(), leaked) {
			t.Errorf("runtime payload contains %s: %s", leaked, live.Body.String())
		}
	}

	f.call(http.MethodPost, sessionPath(session.ID, "/subtests/"+f.sub(0)+"/submit"), nil, f.tokenA)
	f.call(http.MethodPost, sessionPath(session.ID, "/subtests/"+f.sub(1)+"/start"), nil, f.tokenA)
	f.call(http.MethodPost, sessionPath(session.ID, "/subtests/"+f.sub(1)+"/submit"), nil, f.tokenA)

	if rec := f.call(http.MethodGet, sessionPath(session.ID, "/review"), nil, f.tokenA); rec.Code != http.StatusConflict {
		t.Fatalf("review before release = %d, want 409", rec.Code)
	} else if strings.Contains(rec.Body.String(), `"content"`) {
		t.Errorf("withheld review carries question data: %s", rec.Body.String())
	}

	past := time.Now().Add(-time.Hour)
	f.setRelease(t, &past)
	released := f.call(http.MethodGet, sessionPath(session.ID, "/review"), nil, f.tokenA)
	var review struct {
		Subtests []struct {
			Questions []map[string]any `json:"questions"`
		} `json:"subtests"`
	}
	if err := json.Unmarshal(released.Body.Bytes(), &review); err != nil || len(review.Subtests) == 0 || len(review.Subtests[0].Questions) != 1 {
		t.Fatalf("review body = %s, err = %v", released.Body.String(), err)
	}
	reviewed := review.Subtests[0].Questions[0]
	if !isDoc(reviewed["content"]) || !isDoc(reviewed["explanation"]) {
		t.Errorf("review content = %v explanation = %v, want documents", reviewed["content"], reviewed["explanation"])
	}
	for _, option := range reviewed["options"].([]any) {
		if label := option.(map[string]any)["label"]; !isDoc(label) {
			t.Errorf("review option label = %v, want a document", label)
		}
	}
}
