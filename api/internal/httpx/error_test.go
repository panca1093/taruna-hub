package httpx

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/labstack/echo/v4"
)

func TestTranslateAppError(t *testing.T) {
	err := Validation("Data tidak valid.", map[string]string{"email": "wajib diisi"})

	status, body := translate(err)

	if status != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d", status, http.StatusUnprocessableEntity)
	}
	if body.Code != CodeValidation {
		t.Errorf("code = %q, want %q", body.Code, CodeValidation)
	}
	if body.Fields["email"] != "wajib diisi" {
		t.Errorf("fields[email] = %q, want %q", body.Fields["email"], "wajib diisi")
	}
}

// A wrapped app error must keep its status and code, otherwise every service that
// annotates errors on the way up would silently downgrade them to 500.
func TestTranslateWrappedAppError(t *testing.T) {
	err := fmt.Errorf("find user: %w", NotFound("Pengguna tidak ditemukan."))

	status, body := translate(err)

	if status != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", status, http.StatusNotFound)
	}
	if body.Code != CodeNotFound {
		t.Errorf("code = %q, want %q", body.Code, CodeNotFound)
	}
}

func TestTranslateEchoError(t *testing.T) {
	status, body := translate(echo.NewHTTPError(http.StatusNotFound, "Not Found"))

	if status != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", status, http.StatusNotFound)
	}
	if body.Code != CodeNotFound {
		t.Errorf("code = %q, want %q", body.Code, CodeNotFound)
	}
}

// Unknown errors must never leak their text to the client.
func TestTranslateUnknownErrorIsOpaque(t *testing.T) {
	secret := "pq: password authentication failed for user \"tryout\""

	status, body := translate(errors.New(secret))

	if status != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", status, http.StatusInternalServerError)
	}
	if body.Code != CodeInternal {
		t.Errorf("code = %q, want %q", body.Code, CodeInternal)
	}
	if body.Message == secret {
		t.Error("internal error message leaked to client")
	}
}

func TestErrorUnwrapPreservesCause(t *testing.T) {
	cause := errors.New("boom")
	err := NotFound("tidak ditemukan").WithCause(cause)

	if !errors.Is(err, cause) {
		t.Error("errors.Is could not find the wrapped cause")
	}
}
