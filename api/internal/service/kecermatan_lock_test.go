package service

import (
	"context"
	"sync"
	"testing"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/model"
)

// A wallet purchase locks the user row and then the enrollment; a start must take them in that same order or the
// two can wait on each other. Each round races one of each for the same student; a deadlock would surface as an error.
func TestKecermatanStartRacesWalletCheckoutWithoutDeadlock(t *testing.T) {
	h := newKecHarness(t)
	tenant := h.t.q.a
	test := h.testIn(t, tenant)
	product := h.kecProduct(t, tenant, test, "20000", 2)

	for round := 0; round < 12; round++ {
		user, p := h.student(t, tenant, "100000")
		h.grant(t, tenant, user, test, 1)
		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			if _, _, err := h.rt.Start(context.Background(), h.scope(tenant), p, test.ID); err != nil {
				t.Errorf("round %d start: %v", round, err)
			}
		}()
		go func() {
			defer wg.Done()
			if _, _, err := h.wallet(t, tenant, p, product); err != nil {
				t.Errorf("round %d checkout: %v", round, err)
			}
		}()
		wg.Wait()
		if c := h.credit(t, user, test); c.purchased != 3 || c.used != 0 {
			t.Fatalf("round %d credit = %+v, want 3 purchased and none used", round, c)
		}
		if n := h.count(t, `SELECT count(*) FROM kecermatan_sessions WHERE user_id = $1 AND status = $2`, user.ID, model.KecSessionInProgress); n != 1 {
			t.Fatalf("round %d attempts in progress = %d, want 1", round, n)
		}
	}
}
