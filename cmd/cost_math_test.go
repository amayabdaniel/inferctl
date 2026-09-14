package cmd

import (
	"math"
	"testing"
)

// approxEqual — pure-arithmetic tolerance. Cost math never needs more
// than 1e-6 precision; a wider bar would let real bugs hide.
func approxEqual(t *testing.T, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 1e-6 {
		t.Errorf("got %v, want %v (delta %v)", got, want, math.Abs(got-want))
	}
}

// The previous implementation used a flat 30 days/month, which
// undercounts every estimate by ~1.44%. This test pins the 30.4375
// constant explicitly — a regression back to 30 would trip it, as
// would a switch to 30 (calendar days for a short month). See
// AvgDaysPerMonth in cost_math.go for the rationale.
func TestMonthlyGPUCost_UsesCloudBillingConstant(t *testing.T) {
	// 24 hrs/day × 30.4375 days = 730.5 hours (canonical always-on)
	got := MonthlyGPUCost(1.0, 1, 24, 1)
	approxEqual(t, got, 24*AvgDaysPerMonth)
	// Sanity: this is what AWS/GCP quote as the "730 hour" figure.
	// The prior 30 constant would produce 720 here — off by 10.5.
	if math.Abs(got-720.0) < 1.0 {
		t.Errorf("suspicious: result %v is within 1.0 of the OLD (30 days/mo) constant — did the constant regress?", got)
	}
}

// Linearity: doubling replicas doubles cost. This is the class the
// log-app payroll bugs were in — the multiplier stopped being applied
// somewhere in the chain.
func TestMonthlyGPUCost_LinearInReplicas(t *testing.T) {
	single := MonthlyGPUCost(2.5, 1, 24, 1)
	quintuple := MonthlyGPUCost(2.5, 1, 24, 5)
	approxEqual(t, quintuple, single*5)
}

// Same shape for GPU count. gpuCount is per-replica so it multiplies
// independently of replicas.
func TestMonthlyGPUCost_LinearInGPUCount(t *testing.T) {
	one := MonthlyGPUCost(1.0, 1, 24, 1)
	four := MonthlyGPUCost(1.0, 4, 24, 1)
	approxEqual(t, four, one*4)
}

// Partial-day operation must scale linearly with hoursPerDay. A
// nighttime-only deployment at 8 hrs/day should be exactly 1/3 the
// cost of always-on.
func TestMonthlyGPUCost_LinearInHoursPerDay(t *testing.T) {
	full := MonthlyGPUCost(1.0, 1, 24, 1)
	third := MonthlyGPUCost(1.0, 1, 8, 1)
	approxEqual(t, third, full/3)
}

// Zero cases: any zero input in the multiplier chain produces zero
// output. Catches a "default to 1 somewhere" bug where a missing
// hoursPerDay value silently becomes 24.
func TestMonthlyGPUCost_ZeroInputsProduceZero(t *testing.T) {
	cases := []struct {
		name  string
		hr    float64
		gpu   int
		hrs   float64
		reps  int
	}{
		{"zero_rate", 0, 1, 24, 1},
		{"zero_gpu", 1, 0, 24, 1},
		{"zero_hours", 1, 1, 0, 1},
		{"zero_replicas", 1, 1, 24, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := MonthlyGPUCost(c.hr, c.gpu, c.hrs, c.reps); got != 0 {
				t.Errorf("expected 0, got %v", got)
			}
		})
	}
}

// Pins the illustrative spot rate. A user quoting spot numbers from
// this tool is trusting the constant hasn't drifted between releases.
// If someone updates IllustrativeSpotDiscount, this test forces them to
// also update the "~65%" annotation in the printed output.
func TestSpotHourly_AppliesIllustrativeDiscount(t *testing.T) {
	approxEqual(t, SpotHourly(1.00), 0.35)
	approxEqual(t, SpotHourly(2.50), 0.875)
	// Zero on-demand → zero spot; a real spot market can't be
	// negative, and neither should our estimate.
	approxEqual(t, SpotHourly(0), 0)
}

// Break-even math + the divide-by-zero guard. A $0/min API is a data
// bug or a giveaway; the sentinel keeps the calling printf truthful
// ("cheaper above N min" would be nonsense with an infinity, so the
// sentinel is intentionally an astronomical number rather than Inf).
func TestBreakEvenMinutes(t *testing.T) {
	// Obvious case: self-host $730/mo vs $0.05/min API
	// → break-even at 730 / 0.05 = 14600 minutes/month.
	approxEqual(t, BreakEvenMinutes(730.0, 0.05), 14600)

	// Divide-by-zero guard.
	sentinel := BreakEvenMinutes(100, 0)
	if sentinel < 1e17 {
		t.Errorf("expected sentinel for zero API rate, got %v", sentinel)
	}
	if BreakEvenMinutes(100, -0.01) < 1e17 {
		t.Error("negative API rate must also hit the guard")
	}
}
