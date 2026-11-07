package main

import (
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/infra"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/model"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/repository"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/service"
)

// pdfConvertTimeout bounds one conversion, so a stuck Chromium frees the worker slot instead of holding it.
const pdfConvertTimeout = 90 * time.Second

type exportDeps struct {
	pool   *pgxpool.Pool
	store  service.ObjectStore
	logger *slog.Logger
	// gotenbergURL is empty when no converter is configured; the PDF kind is then left unregistered.
	gotenbergURL string
}

func registerExportRenderers(svc *service.ExportService, deps exportDeps) {
	if deps.gotenbergURL != "" {
		conv, err := infra.NewGotenberg(deps.gotenbergURL, pdfConvertTimeout)
		if err != nil {
			deps.logger.Error("tryout pdf renderer disabled", slog.String("error", err.Error()))
		} else {
			svc.Register(model.ExportTryoutPDF, service.NewTryoutPDFRenderer(repository.NewTryoutRepository(deps.pool), conv))
		}
	}
}
