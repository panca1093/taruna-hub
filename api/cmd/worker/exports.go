package main

import (
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/model"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/repository"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/service"
)

type exportDeps struct {
	pool   *pgxpool.Pool
	store  service.ObjectStore
	logger *slog.Logger
}

func registerExportRenderers(svc *service.ExportService, deps exportDeps) {
	results := repository.NewResultRepository(deps.pool)
	svc.Register(model.ExportTryoutResultsCSV, service.NewResultExportRenderer(deps.pool, results, service.ExportFormatCSV, service.IRTMinStudents))
	svc.Register(model.ExportTryoutResultsXLSX, service.NewResultExportRenderer(deps.pool, results, service.ExportFormatXLSX, service.IRTMinStudents))
}
