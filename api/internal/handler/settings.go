package handler

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/httpx"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/model"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/service"
)

type weightsBody struct {
	Easy   float64 `json:"easy"`
	Medium float64 `json:"medium"`
	Hard   float64 `json:"hard"`
}

type settingsBody struct {
	ScoringWeights weightsBody `json:"scoring_weights"`
	MinTopup       string      `json:"min_topup"`
}

func settingsResponseOf(s model.Settings) settingsBody {
	return settingsBody{
		ScoringWeights: weightsBody{Easy: s.Weights.Easy, Medium: s.Weights.Medium, Hard: s.Weights.Hard},
		MinTopup:       s.MinTopup,
	}
}

func (h *Handler) Settings(c echo.Context) error {
	scope, principal, ok := runtimeCaller(c)
	if !ok {
		return httpx.Unauthorized(sessionInvalid)
	}
	w, err := h.runtime.Settings(c.Request().Context(), scope, principal)
	if err != nil {
		return runtimeError(err)
	}
	return c.JSON(http.StatusOK, settingsResponseOf(w))
}

func (h *Handler) UpdateSettings(c echo.Context) error {
	scope, principal, ok := runtimeCaller(c)
	if !ok {
		return httpx.Unauthorized(sessionInvalid)
	}
	c.Request().Body = http.MaxBytesReader(c.Response(), c.Request().Body, maxAnswerBody)
	var req settingsBody
	if err := bindStrict(c, &req); err != nil {
		return err
	}
	w, err := h.runtime.UpdateSettings(c.Request().Context(), scope, principal, service.SettingsInput{
		Weights:  model.Weights{Easy: req.ScoringWeights.Easy, Medium: req.ScoringWeights.Medium, Hard: req.ScoringWeights.Hard},
		MinTopup: req.MinTopup,
	})
	if err != nil {
		return runtimeError(err)
	}
	return c.JSON(http.StatusOK, settingsResponseOf(w))
}
