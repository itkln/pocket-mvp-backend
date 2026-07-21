package billing

import "testing"

func TestPlanLimits(t *testing.T) {
	if limit := limitForPlan("start"); limit == nil || *limit != 1 {
		t.Fatalf("unexpected start limit: %v", limit)
	}
	if limit := limitForPlan("business"); limit == nil || *limit != 3 {
		t.Fatalf("unexpected business limit: %v", limit)
	}
	if limit := limitForPlan("pro"); limit != nil {
		t.Fatalf("pro must be unlimited: %v", *limit)
	}
}
