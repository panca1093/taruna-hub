package infra

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/service"
)

var gotenbergFiles = []service.PDFFile{
	{Name: "index.html", Data: []byte("<html>a</html>")},
	{Name: "footer.html", Data: []byte("<html>f</html>")},
	{Name: "katex.min.js", Data: []byte("var k;")},
}

func TestGotenbergSendsOnlyTheGivenFiles(t *testing.T) {
	var path string
	got := map[string]string{}
	fields := map[string]string{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Errorf("parse form: %v", err)
		}
		for _, hdrs := range r.MultipartForm.File {
			for _, h := range hdrs {
				f, _ := h.Open()
				b, _ := io.ReadAll(f)
				got[h.Filename] = string(b)
			}
		}
		for k, v := range r.MultipartForm.Value {
			fields[k] = v[0]
		}
		_, _ = w.Write([]byte("%PDF-1.7 ok"))
	}))
	defer srv.Close()

	g, err := NewGotenberg(srv.URL+"/", time.Second)
	if err != nil {
		t.Fatal(err)
	}
	pdf, err := g.HTMLToPDF(context.Background(), gotenbergFiles)
	if err != nil || string(pdf) != "%PDF-1.7 ok" {
		t.Fatalf("pdf = %q, %v", pdf, err)
	}
	if path != "/forms/chromium/convert/html" {
		t.Errorf("path = %s", path)
	}
	if len(got) != 3 || got["index.html"] != "<html>a</html>" || got["footer.html"] == "" || got["katex.min.js"] == "" {
		t.Errorf("files = %v, want exactly the three given", got)
	}
	if fields["waitForExpression"] != "window.__mathDone === true" || fields["failOnConsoleExceptions"] != "true" {
		t.Errorf("fields = %v, want the math wait and console failure switches", fields)
	}
}

func TestGotenbergErrorsAreClassified(t *testing.T) {
	for status, wantUnavailable := range map[int]bool{http.StatusInternalServerError: false, http.StatusBadRequest: false,
		http.StatusServiceUnavailable: true, http.StatusBadGateway: true} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(status)
			_, _ = w.Write([]byte("boom"))
		}))
		g, _ := NewGotenberg(srv.URL, time.Second)
		_, err := g.HTMLToPDF(context.Background(), gotenbergFiles)
		srv.Close()
		if err == nil || !strings.Contains(err.Error(), "boom") || errors.Is(err, service.ErrPDFUnavailable) != wantUnavailable {
			t.Errorf("status %d: err = %v, want unavailable=%v with the body", status, err, wantUnavailable)
		}
	}
}

func TestGotenbergTimeoutAndUnreachableAreUnavailable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(2 * time.Second):
		}
	}))
	defer srv.Close()
	g, _ := NewGotenberg(srv.URL, 50*time.Millisecond)
	start := time.Now()
	if _, err := g.HTMLToPDF(context.Background(), gotenbergFiles); !errors.Is(err, service.ErrPDFUnavailable) || time.Since(start) > time.Second {
		t.Errorf("slow server: err = %v after %s, want a prompt ErrPDFUnavailable", err, time.Since(start))
	}
	down, _ := NewGotenberg("http://127.0.0.1:1", time.Second)
	if _, err := down.HTMLToPDF(context.Background(), gotenbergFiles); !errors.Is(err, service.ErrPDFUnavailable) {
		t.Errorf("unreachable: err = %v, want ErrPDFUnavailable", err)
	}
}

func TestNewGotenbergRejectsBadURL(t *testing.T) {
	for _, raw := range []string{"", "gotenberg:3000", "ftp://x"} {
		if _, err := NewGotenberg(raw, time.Second); err == nil {
			t.Errorf("NewGotenberg(%q) accepted", raw)
		}
	}
}
