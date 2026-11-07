package handler

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/httpx"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/model"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/service"
)

const (
	reportDateLayout = "2006-01-02"
	reportRangeRule  = "Rentang tanggal YYYY-MM-DD, awal tidak setelah akhir, maksimal 366 hari."
)

type revenueBucketResponse struct {
	Key   string `json:"key"`
	Count int    `json:"count"`
	Total string `json:"total"`
}

type revenueTotalResponse struct {
	Count int    `json:"count"`
	Total string `json:"total"`
}

type revenueDayResponse struct {
	Date        string `json:"date"`
	Orders      int    `json:"orders"`
	OrdersTotal string `json:"orders_total"`
	Topups      int    `json:"topups"`
	TopupsTotal string `json:"topups_total"`
}

type revenueRecapResponse struct {
	From            string                  `json:"from"`
	To              string                  `json:"to"`
	Orders          revenueTotalResponse    `json:"orders"`
	ByPaymentMethod []revenueBucketResponse `json:"by_payment_method"`
	ByProductType   []revenueBucketResponse `json:"by_product_type"`
	TopupsApproved  revenueTotalResponse    `json:"topups_approved"`
	Days            []revenueDayResponse    `json:"days"`
	DaysTotal       int                     `json:"days_total"`
}

func (h *Handler) RevenueRecap(c echo.Context) error {
	scope, principal, ok := walletCaller(c)
	if !ok {
		return httpx.Unauthorized(sessionInvalid)
	}
	limit, offset, err := walletPageOf(c)
	if err != nil {
		return err
	}
	recap, err := h.reports.Revenue(c.Request().Context(), scope, principal, service.RevenueInput{
		From: c.QueryParam("from"), To: c.QueryParam("to"), Limit: limit, Offset: offset,
	})
	if err != nil {
		return userAdminError(err)
	}
	return c.JSON(http.StatusOK, revenueRecapOf(recap))
}

func revenueRecapOf(r model.RevenueRecap) revenueRecapResponse {
	body := revenueRecapResponse{
		From: r.From.Format(reportDateLayout), To: r.To.Format(reportDateLayout),
		Orders:          revenueTotalResponse{Count: r.Orders.Count, Total: r.Orders.Total},
		TopupsApproved:  revenueTotalResponse{Count: r.Topups.Count, Total: r.Topups.Total},
		ByPaymentMethod: bucketsOf(r.ByMethod), ByProductType: bucketsOf(r.ByProduct),
		Days: make([]revenueDayResponse, 0, len(r.Days)), DaysTotal: r.DaysTotal,
	}
	for _, d := range r.Days {
		body.Days = append(body.Days, revenueDayResponse{
			Date: d.Date.Format(reportDateLayout), Orders: d.Orders, OrdersTotal: d.OrdersTotal,
			Topups: d.Topups, TopupsTotal: d.TopupsTotal,
		})
	}
	return body
}

func bucketsOf(in []model.RevenueBucket) []revenueBucketResponse {
	out := make([]revenueBucketResponse, 0, len(in))
	for _, b := range in {
		out = append(out, revenueBucketResponse{Key: b.Key, Count: b.Count, Total: b.Total})
	}
	return out
}
