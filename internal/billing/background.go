package billing

import (
	"context"
	"log"
	"net/http"
	"time"

	"koffy/internal/httpx"
)

func (s *Server) StartBackgroundJobs(ctx context.Context) {
	interval := time.Duration(s.cfg.EntitlementMaintenanceIntervalMinutes) * time.Minute
	if interval <= 0 {
		log.Printf("entitlement maintenance disabled")
		return
	}

	go func() {
		s.runEntitlementMaintenance(ctx)

		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.runEntitlementMaintenance(ctx)
			}
		}
	}()
}

func (s *Server) runEntitlementMaintenance(ctx context.Context) {
	runCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	result, err := s.store.RunEntitlementMaintenance(runCtx)
	if err != nil {
		log.Printf("entitlement maintenance failed: %v", err)
		return
	}
	if result.ExpiredSubscriptions > 0 || result.CreatedBalances > 0 || result.UpdatedBalances > 0 {
		log.Printf(
			"entitlement maintenance finished: expired_subscriptions=%d created_balances=%d updated_balances=%d",
			result.ExpiredSubscriptions,
			result.CreatedBalances,
			result.UpdatedBalances,
		)
	}
}

func (s *Server) adminRunEntitlementMaintenance(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	result, err := s.store.RunEntitlementMaintenance(r.Context())
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	httpx.JSON(w, http.StatusOK, result)
}
