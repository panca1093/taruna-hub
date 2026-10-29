package service

import (
	"sync"
	"testing"
)

func TestConcurrentNotificationsOfOnePaymentGrantOnce(t *testing.T) {
	h := newPaymentHarness(t)
	tenant := h.t.q.a
	user, p := h.student(t, tenant, "")
	order, _, _ := h.checkout(tenant, p, gatewayProduct(h, t, tenant), "")
	id := h.providerID(t, order.ID)
	h.gw.set(id, "settlement", "", "100000.00")

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := h.notify(id, "settlement", "", "100000.00"); err != nil {
				t.Errorf("concurrent notification: %v", err)
			}
		}()
	}
	wg.Wait()
	if h.enrollments(t, user.ID) != 1 || h.grants(t, user.ID) != 1 || h.notifications(t, tenant, TemplateOrderConfirmed) != 2 {
		t.Errorf("enrollments %d grants %d notifications %d, want 1, 1 and 2",
			h.enrollments(t, user.ID), h.grants(t, user.ID), h.notifications(t, tenant, TemplateOrderConfirmed))
	}
}
