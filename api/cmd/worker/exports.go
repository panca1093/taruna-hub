package main

import (
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/service"
)

type exportDeps struct {
	pool   *pgxpool.Pool
	store  service.ObjectStore
	logger *slog.Logger
}

func registerExportRenderers(svc *service.ExportService, deps exportDeps) {
}
