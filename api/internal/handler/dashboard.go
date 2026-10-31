package handler

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/httpx"
)

type dashboardSummaryResponse struct {
	OwnedTryouts     int      `json:"owned_tryouts"`
	CompletedTryouts int      `json:"completed_tryouts"`
	AverageScore     *float64 `json:"average_score"`
	Balance          string   `json:"balance"`
}

func (h *Handler) StudentDashboard(c echo.Context) error {
	scope, principal, ok := runtimeCaller(c)
	if !ok {
		return httpx.Unauthorized(sessionInvalid)
	}
	summary, err := h.runtime.Dashboard(c.Request().Context(), scope, principal)
	if err != nil {
		return runtimeError(err)
	}
	return c.JSON(http.StatusOK, dashboardSummaryResponse{
		OwnedTryouts: summary.OwnedTryouts, CompletedTryouts: summary.CompletedTryouts,
		AverageScore: summary.AverageScore, Balance: summary.Balance,
	})
}
