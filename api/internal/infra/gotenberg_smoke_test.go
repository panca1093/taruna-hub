package infra

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/model"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/service"
)

// TestGotenbergRendersTryoutPDF needs a running sidecar: GOTENBERG_URL=http://localhost:3020. PDF_SMOKE_OUT names
// the file to keep, otherwise the PDF is written to the test's temp dir.
func TestGotenbergRendersTryoutPDF(t *testing.T) {
	url := os.Getenv("GOTENBERG_URL")
	if url == "" {
		t.Skip("GOTENBERG_URL is not set")
	}
	conv, err := NewGotenberg(url, 90*time.Second)
	if err != nil {
		t.Fatal(err)
	}

	img := image.NewRGBA(image.Rect(0, 0, 480, 240))
	for x := 0; x < 480; x++ {
		for y := 0; y < 240; y++ {
			img.Set(x, y, color.RGBA{uint8(x / 2), uint8(y), 160, 255})
		}
	}
	var enc bytes.Buffer
	if err := png.Encode(&enc, img); err != nil {
		t.Fatal(err)
	}
	dataURI := "data:image/png;base64," + base64.StdEncoding.EncodeToString(enc.Bytes())

	var questions []model.PrintQuestion
	for i := 1; i <= 30; i++ {
		content := fmt.Sprintf("Jika $2x + %d = 17$, maka nilai $\\frac{x}{2} + \\sqrt{x}$ adalah ...", i)
		if i%7 == 0 {
			content += "\n![diagram](" + dataURI + ")"
		}
		questions = append(questions, model.PrintQuestion{Content: content, Options: []string{"$\\frac{1}{2}$", "$x^2$", "$\\sum_{k=1}^{n} k$", "Tidak ada"}})
	}
	files, err := service.BuildTryoutPDFFiles(model.PrintTryout{
		TenantName: "Taruna Eduka", Title: "Tryout Smoke UTBK", Revision: 2,
		Subtests: []model.PrintSubtest{
			{Name: "Penalaran Matematika", DurationSeconds: 3600, Questions: questions},
			{Name: "Literasi", DurationSeconds: 1800, Questions: questions[:5]},
		},
	}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	pdf, err := conv.HTMLToPDF(context.Background(), files)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if !bytes.HasPrefix(pdf, []byte("%PDF")) || len(pdf) < 1024 {
		t.Fatalf("pdf is %d bytes starting %q", len(pdf), pdf[:min(8, len(pdf))])
	}

	out := os.Getenv("PDF_SMOKE_OUT")
	if out == "" {
		out = filepath.Join(t.TempDir(), "smoke.pdf")
	}
	if err := os.WriteFile(out, pdf, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Logf("pdf: %s (%d bytes)", out, len(pdf))

	if _, err := exec.LookPath("pdftotext"); err != nil {
		t.Log("pdftotext not found; skipping text assertions")
		return
	}
	text, err := exec.Command("pdftotext", "-layout", out, "-").Output()
	if err != nil {
		t.Fatalf("pdftotext: %v", err)
	}
	got := string(text)
	for _, want := range []string{"Tryout Smoke UTBK", "Penalaran Matematika", "Durasi 1 jam", "Taruna Eduka", "Halaman"} {
		if !strings.Contains(got, want) {
			t.Errorf("pdf text lacks %q", want)
		}
	}
	if strings.Contains(got, `\frac`) || strings.Contains(got, `\sqrt`) || strings.Contains(got, "$2x") {
		t.Error("pdf shows raw LaTeX instead of typeset math")
	}
	if strings.Count(got, "Halaman") < 3 {
		t.Errorf("only %d pages carry the footer, want a multi-page document", strings.Count(got, "Halaman"))
	}
}
