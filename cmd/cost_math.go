package cmd

// Pure cost-math helpers. Kept out of cost.go so unit tests can exercise
// the arithmetic without stubbing stdout, and so the constants are named
// (grep-able) rather than buried in a printf.

// AvgDaysPerMonth is the standard cloud-billing constant: 365.25 / 12 =
// 30.4375. AWS and GCP both bill monthly as `hourly × 730` when
// running 24/7, which is `24 × 30.4375 ≈ 730.5`. The previous code
// used a flat `30`, which under-reports by ~1.44% (24 × 30.4375 = 730.5
// vs 24 × 30 = 720; delta 10.5 hours/month per replica per GPU).
// That's small individually but compounds with replica count and GPU
// count. Using the correct constant makes the numbers here line up
// with what AWS's own cost calculator produces.
const AvgDaysPerMonth = 30.4375

// IllustrativeSpotDiscount is a FIXED discount used only to give users
// a rough sense of the shape. Real spot markets vary by region,
// instance type, and hour of day:
//   * L4  on AWS us-east-1: p50 discount ~60-65%, p95 ~40-70%
//   * A10 on AWS us-east-1: p50 discount ~55-65%
//   * H100 anywhere:        p50 discount ~30-50%, sometimes 0%
// The 0.65 constant is a defensible midpoint for the small/mid GPUs
// this tool most often targets, but users should verify against their
// provider's actual spot history before treating this number as a
// commitment. See the "illustrative" annotation in the printed output.
const IllustrativeSpotDiscount = 0.65

// MonthlyGPUCost returns the estimated monthly cost of a deployment,
// using the AWS/GCP-standard 30.4375 days/month constant. Inputs:
//
//   hourlyRate:  on-demand $/hr for a single GPU of this type
//   gpuCount:    GPUs per replica (from spec.Resources.GPUCount)
//   hoursPerDay: user-provided operating hours (default 24 = always on)
//   replicas:    replica count multiplier
//
// All inputs are treated as independent multipliers; no capping, no
// step function. Returning float64 (not a currency type) is deliberate
// — the output is a printed estimate, not a billed amount.
func MonthlyGPUCost(hourlyRate float64, gpuCount int, hoursPerDay float64, replicas int) float64 {
	return hourlyRate * float64(gpuCount) * hoursPerDay * AvgDaysPerMonth * float64(replicas)
}

// SpotHourly returns the illustrative spot-market hourly rate for an
// on-demand rate. See IllustrativeSpotDiscount above for why this is a
// rough estimate.
func SpotHourly(onDemand float64) float64 {
	return onDemand * (1.0 - IllustrativeSpotDiscount)
}

// BreakEvenMinutes returns how many minutes of usage per month would
// make self-hosting break even against a per-minute hosted API. If the
// user's actual usage exceeds this, self-host wins; below it, the API
// is cheaper. Guards against divide-by-zero — a $0/min API is either
// a data-entry error or a giveaway and returns +Inf so the "cheaper
// above N min" text stays truthful.
func BreakEvenMinutes(selfHostMonthlyUSD, apiPerMinuteUSD float64) float64 {
	if apiPerMinuteUSD <= 0 {
		return 1e18 // sentinel: "you cannot beat free"
	}
	return selfHostMonthlyUSD / apiPerMinuteUSD
}
