package infra

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/service"
)

const (
	gotenbergConvertPath = "/forms/chromium/convert/html"
	gotenbergMaxPDFBytes = 64 << 20
	gotenbergMaxErrBytes = 2 << 10
	// The page script sets this once every formula is typeset; Chromium prints only after it is true.
	gotenbergReadyExpression = "window.__mathDone === true"
)

// Gotenberg converts self-contained HTML to PDF through a Gotenberg sidecar. It only ever sends the files it is
// given, so the renderer controls every byte Chromium can read.
type Gotenberg struct {
	endpoint string
	client   *http.Client
}

// NewGotenberg takes the sidecar's base URL; timeout bounds one whole conversion, upload to last byte.
func NewGotenberg(baseURL string, timeout time.Duration) (*Gotenberg, error) {
	u, err := url.Parse(strings.TrimRight(baseURL, "/"))
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, fmt.Errorf("GOTENBERG_URL must be an http(s) URL, got %q", baseURL)
	}
	return &Gotenberg{endpoint: u.String() + gotenbergConvertPath, client: &http.Client{Timeout: timeout}}, nil
}

// HTMLToPDF names the first file index.html. A transport failure, a timeout and a 502/503/504 wrap
// service.ErrPDFUnavailable so the caller can tell a sidecar outage from a document it cannot render.
func (g *Gotenberg) HTMLToPDF(ctx context.Context, files []service.PDFFile) ([]byte, error) {
	var body bytes.Buffer
	form := multipart.NewWriter(&body)
	for _, f := range files {
		part, err := form.CreateFormFile("files", f.Name)
		if err != nil {
			return nil, fmt.Errorf("build gotenberg form: %w", err)
		}
		if _, err := part.Write(f.Data); err != nil {
			return nil, fmt.Errorf("build gotenberg form: %w", err)
		}
	}
	fields := map[string]string{
		"paperWidth": "8.27", "paperHeight": "11.7",
		"marginTop": "0.6", "marginBottom": "0.8", "marginLeft": "0.6", "marginRight": "0.6",
		"printBackground": "true", "preferCssPageSize": "false",
		"waitForExpression": gotenbergReadyExpression, "failOnConsoleExceptions": "true",
	}
	for name, value := range fields {
		if err := form.WriteField(name, value); err != nil {
			return nil, fmt.Errorf("build gotenberg form: %w", err)
		}
	}
	if err := form.Close(); err != nil {
		return nil, fmt.Errorf("build gotenberg form: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, g.endpoint, &body)
	if err != nil {
		return nil, fmt.Errorf("build gotenberg request: %w", err)
	}
	req.Header.Set("Content-Type", form.FormDataContentType())
	resp, err := g.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", service.ErrPDFUnavailable, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		detail, _ := io.ReadAll(io.LimitReader(resp.Body, gotenbergMaxErrBytes))
		err := fmt.Errorf("gotenberg answered %d: %s", resp.StatusCode, strings.TrimSpace(string(detail)))
		switch resp.StatusCode {
		case http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
			return nil, fmt.Errorf("%w: %v", service.ErrPDFUnavailable, err)
		}
		return nil, err
	}
	pdf, err := io.ReadAll(io.LimitReader(resp.Body, gotenbergMaxPDFBytes+1))
	if err != nil {
		return nil, fmt.Errorf("%w: read gotenberg response: %v", service.ErrPDFUnavailable, err)
	}
	if len(pdf) > gotenbergMaxPDFBytes {
		return nil, errors.New("gotenberg pdf exceeds the size limit")
	}
	return pdf, nil
}
