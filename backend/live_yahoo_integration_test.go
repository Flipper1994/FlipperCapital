//go:build integration

package main

import (
	"fmt"
	"math"
	"testing"
	"time"
)

// ============================================================
// Live Yahoo Integration Tests
//
// These tests hit the REAL Yahoo Finance API.
// Run manually: go test -run TestYahoo -v -timeout 300s
//
// Best run during market hours or shortly after close
// so that delta-fetch returns fresh bars to merge.
// ============================================================

// testIntervals defines the intervals to test for a given symbol.
// Each entry has: interval (our format), yahooInterval, prefetchPeriod, deltaPeriod
type testInterval struct {
	Name           string
	YahooInterval  string
	PrefetchPeriod string
	DeltaPeriod    string
	Strategy       string
}

var testIntervals = []testInterval{
	{"1h", "60m", "2y", "7d", "hybrid_ai_trend"},
	{"4h", "4h", "2y", "1mo", "diamond_signals"},
	{"1d", "1d", "2y", "3mo", "hybrid_ai_trend"},
}

// TestYahoo_DeltaMergeMatchesFull is the main integration test.
// It simulates exactly what runLiveScan does:
//   1. Prefetch: fetch full history from Yahoo
//   2. Delta: fetch only recent bars (like a poll would)
//   3. Merge: combine cache + delta
//   4. Compare: merged signals must match full-fetch signals
//
// This catches any issues with:
//   - mergeOHLCV losing or duplicating bars
//   - Yahoo returning different data for different period params
//   - Signal divergence due to data gaps
func TestYahoo_DeltaMergeMatchesFull_DBK(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping Yahoo integration test in short mode")
	}

	symbol := "DBK.DE" // Deutsche Bank

	for _, iv := range testIntervals {
		t.Run(fmt.Sprintf("%s_%s", symbol, iv.Name), func(t *testing.T) {
			t.Logf("=== Testing %s %s ===", symbol, iv.Name)
			t.Logf("Prefetch period: %s, Delta period: %s", iv.PrefetchPeriod, iv.DeltaPeriod)

			// Step 1: Full fetch (this is the reference — "ground truth")
			t.Log("Step 1: Fetching full data from Yahoo...")
			fullData, err := fetchOHLCVFromYahoo(symbol, iv.PrefetchPeriod, iv.YahooInterval)
			if err != nil {
				t.Fatalf("Full fetch failed: %v", err)
			}
			t.Logf("  Full data: %d bars, range %s — %s",
				len(fullData),
				time.Unix(fullData[0].Time, 0).Format("2006-01-02 15:04"),
				time.Unix(fullData[len(fullData)-1].Time, 0).Format("2006-01-02 15:04"))

			// Step 2: Simulate prefetch (same as full fetch — this becomes our "cache")
			// In production: prefetchLiveOHLCV does this once on Go Live
			cache := make([]OHLCV, len(fullData))
			copy(cache, fullData)

			// Step 3: Delta fetch (small recent window)
			t.Log("Step 2: Fetching delta data from Yahoo...")
			time.Sleep(500 * time.Millisecond) // small delay to not hammer Yahoo
			deltaData, err := fetchOHLCVFromYahoo(symbol, iv.DeltaPeriod, iv.YahooInterval)
			if err != nil {
				t.Fatalf("Delta fetch failed: %v", err)
			}
			t.Logf("  Delta data: %d bars, range %s — %s",
				len(deltaData),
				time.Unix(deltaData[0].Time, 0).Format("2006-01-02 15:04"),
				time.Unix(deltaData[len(deltaData)-1].Time, 0).Format("2006-01-02 15:04"))

			// Step 4: Merge cache + delta (exactly what runLiveScan does)
			merged := mergeOHLCV(cache, deltaData)
			t.Logf("  Merged: %d bars", len(merged))

			// =============================================
			// CHECK 1: No duplicate timestamps
			// =============================================
			t.Log("Check 1: No duplicate timestamps...")
			seen := map[int64]int{}
			for i, bar := range merged {
				if prev, ok := seen[bar.Time]; ok {
					t.Errorf("DUPLICATE timestamp %d at indices %d and %d (%s)",
						bar.Time, prev, i, time.Unix(bar.Time, 0).Format("2006-01-02 15:04"))
				}
				seen[bar.Time] = i
			}

			// =============================================
			// CHECK 2: Timestamps ascending
			// =============================================
			t.Log("Check 2: Timestamps ascending...")
			for i := 1; i < len(merged); i++ {
				if merged[i].Time <= merged[i-1].Time {
					t.Errorf("NOT ASCENDING at index %d: %d <= %d", i, merged[i].Time, merged[i-1].Time)
				}
			}

			// =============================================
			// CHECK 3: No data loss — merged must cover same range as full
			// =============================================
			t.Log("Check 3: Data coverage...")
			if len(merged) < len(fullData) {
				t.Errorf("LOST BARS: merged %d < full %d (lost %d bars!)",
					len(merged), len(fullData), len(fullData)-len(merged))
			}
			if merged[0].Time != fullData[0].Time {
				t.Errorf("START MISMATCH: merged starts %s, full starts %s",
					time.Unix(merged[0].Time, 0).Format("2006-01-02 15:04"),
					time.Unix(fullData[0].Time, 0).Format("2006-01-02 15:04"))
			}

			// =============================================
			// CHECK 4: Overlap region has same data
			// =============================================
			t.Log("Check 4: Overlap region integrity...")
			deltaStart := deltaData[0].Time
			// Find this timestamp in both full and merged
			for _, fb := range fullData {
				if fb.Time < deltaStart {
					continue
				}
				// Find same timestamp in merged
				for _, mb := range merged {
					if mb.Time == fb.Time {
						if math.Abs(mb.Close-fb.Close) > 0.01 {
							t.Errorf("CLOSE MISMATCH at %s: merged=%.4f full=%.4f (diff=%.4f)",
								time.Unix(fb.Time, 0).Format("2006-01-02 15:04"),
								mb.Close, fb.Close, mb.Close-fb.Close)
						}
						break
					}
				}
			}

			// =============================================
			// CHECK 5: Signals match
			// =============================================
			t.Log("Check 5: Signal comparison...")
			strategy := createStrategyFromJSON(iv.Strategy, "")
			if strategy == nil {
				t.Fatalf("unknown strategy: %s", iv.Strategy)
			}

			minBars := strategy.RequiredBars()
			if len(fullData) < minBars {
				t.Skipf("Not enough bars for %s: have %d, need %d", iv.Strategy, len(fullData), minBars)
			}

			signalsFull := strategy.Analyze(fullData)
			signalsMerged := strategy.Analyze(merged)

			t.Logf("  Full signals: %d, Merged signals: %d", len(signalsFull), len(signalsMerged))

			if len(signalsFull) != len(signalsMerged) {
				t.Errorf("SIGNAL COUNT MISMATCH: full=%d merged=%d", len(signalsFull), len(signalsMerged))
				// Show differences
				fullMap := map[int]StrategySignal{}
				for _, s := range signalsFull {
					fullMap[s.Index] = s
				}
				mergedMap := map[int]StrategySignal{}
				for _, s := range signalsMerged {
					mergedMap[s.Index] = s
				}
				for idx, fs := range fullMap {
					if _, ok := mergedMap[idx]; !ok {
						t.Logf("  MISSING in merged: index %d %s @ %.4f", idx, fs.Direction, fs.EntryPrice)
					}
				}
				for idx, ms := range mergedMap {
					if _, ok := fullMap[idx]; !ok {
						t.Logf("  EXTRA in merged: index %d %s @ %.4f", idx, ms.Direction, ms.EntryPrice)
					}
				}
			}

			// Compare each signal
			for i := 0; i < len(signalsFull) && i < len(signalsMerged); i++ {
				sf := signalsFull[i]
				sm := signalsMerged[i]
				if sf.Index != sm.Index {
					t.Errorf("Signal %d INDEX mismatch: full=%d merged=%d", i, sf.Index, sm.Index)
					continue
				}
				if sf.Direction != sm.Direction {
					t.Errorf("Signal %d DIRECTION mismatch at index %d: full=%s merged=%s", i, sf.Index, sf.Direction, sm.Direction)
				}
				if math.Abs(sf.EntryPrice-sm.EntryPrice) > 0.01 {
					t.Errorf("Signal %d ENTRY mismatch at index %d: full=%.4f merged=%.4f", i, sf.Index, sf.EntryPrice, sm.EntryPrice)
				}
				if math.Abs(sf.StopLoss-sm.StopLoss) > 0.01 {
					t.Errorf("Signal %d SL mismatch at index %d: full=%.4f merged=%.4f", i, sf.Index, sf.StopLoss, sm.StopLoss)
				}
				if math.Abs(sf.TakeProfit-sm.TakeProfit) > 0.01 {
					t.Errorf("Signal %d TP mismatch at index %d: full=%.4f merged=%.4f", i, sf.Index, sf.TakeProfit, sm.TakeProfit)
				}
			}

			if len(signalsFull) == len(signalsMerged) {
				t.Logf("  ALL %d SIGNALS MATCH", len(signalsFull))
			}
		})
	}
}

// TestYahoo_MultipleSymbols tests the same logic for several German/US stocks
func TestYahoo_DeltaMergeMatchesFull_MultiStock(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping Yahoo integration test in short mode")
	}

	symbols := []string{
		"DBK.DE",  // Deutsche Bank
		"SAP.DE",  // SAP
		"SIE.DE",  // Siemens
		"AAPL",    // Apple (US)
		"MSFT",    // Microsoft (US)
	}

	iv := testInterval{"1h", "60m", "2y", "7d", "hybrid_ai_trend"}

	for _, symbol := range symbols {
		t.Run(symbol, func(t *testing.T) {
			t.Logf("Testing %s with %s interval...", symbol, iv.Name)

			// Full fetch
			fullData, err := fetchOHLCVFromYahoo(symbol, iv.PrefetchPeriod, iv.YahooInterval)
			if err != nil {
				t.Fatalf("Full fetch failed for %s: %v", symbol, err)
			}
			t.Logf("  Full: %d bars", len(fullData))

			time.Sleep(300 * time.Millisecond)

			// Delta fetch
			deltaData, err := fetchOHLCVFromYahoo(symbol, iv.DeltaPeriod, iv.YahooInterval)
			if err != nil {
				t.Fatalf("Delta fetch failed for %s: %v", symbol, err)
			}
			t.Logf("  Delta: %d bars", len(deltaData))

			// Merge
			merged := mergeOHLCV(fullData, deltaData)
			t.Logf("  Merged: %d bars", len(merged))

			// No duplicates
			seen := map[int64]bool{}
			dupes := 0
			for _, bar := range merged {
				if seen[bar.Time] {
					dupes++
				}
				seen[bar.Time] = true
			}
			if dupes > 0 {
				t.Errorf("%s: %d duplicate timestamps!", symbol, dupes)
			}

			// No data loss
			if len(merged) < len(fullData) {
				t.Errorf("%s: LOST %d bars after merge (full=%d, merged=%d)",
					symbol, len(fullData)-len(merged), len(fullData), len(merged))
			}

			// Signal comparison
			strategy := createStrategyFromJSON(iv.Strategy, "")
			minBars := strategy.RequiredBars()
			if len(fullData) < minBars {
				t.Skipf("%s: not enough bars (%d < %d)", symbol, len(fullData), minBars)
			}

			sigFull := strategy.Analyze(fullData)
			sigMerged := strategy.Analyze(merged)

			if len(sigFull) != len(sigMerged) {
				t.Errorf("%s: SIGNAL COUNT MISMATCH full=%d merged=%d", symbol, len(sigFull), len(sigMerged))
			} else {
				mismatches := 0
				for i := range sigFull {
					if sigFull[i].Index != sigMerged[i].Index || sigFull[i].Direction != sigMerged[i].Direction {
						mismatches++
					}
				}
				if mismatches > 0 {
					t.Errorf("%s: %d signal mismatches out of %d", symbol, mismatches, len(sigFull))
				} else {
					t.Logf("  %s: ALL %d signals match", symbol, len(sigFull))
				}
			}
		})
	}
}

// TestYahoo_BarCountSufficient checks that prefetch delivers enough bars
// for each strategy's RequiredBars() per interval.
func TestYahoo_BarCountSufficient(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping Yahoo integration test in short mode")
	}

	symbol := "DBK.DE"

	configs := []struct {
		interval       string
		yahooInterval  string
		prefetchPeriod string
		strategy       string
	}{
		{"5m", "5m", "60d", "regression_scalping"},
		{"15m", "15m", "60d", "regression_scalping"},
		{"1h", "60m", "2y", "hybrid_ai_trend"},
		{"4h", "4h", "2y", "diamond_signals"},
		{"1d", "1d", "2y", "hybrid_ai_trend"},
	}

	for _, c := range configs {
		t.Run(fmt.Sprintf("%s_%s", c.interval, c.strategy), func(t *testing.T) {
			data, err := fetchOHLCVFromYahoo(symbol, c.prefetchPeriod, c.yahooInterval)
			if err != nil {
				t.Fatalf("Fetch failed: %v", err)
			}

			strategy := createStrategyFromJSON(c.strategy, "")
			required := strategy.RequiredBars()

			t.Logf("%s %s: got %d bars, strategy '%s' needs %d",
				symbol, c.interval, len(data), c.strategy, required)

			if len(data) < required {
				t.Errorf("NOT ENOUGH BARS: got %d, need %d (deficit: %d)",
					len(data), required, required-len(data))
			} else {
				surplus := len(data) - required
				t.Logf("  OK — %d bars surplus (%.0f%% headroom)", surplus, float64(surplus)/float64(required)*100)
			}

			// Also verify strategy can produce signals with this data
			signals := strategy.Analyze(data)
			t.Logf("  Produced %d signals", len(signals))

			time.Sleep(300 * time.Millisecond)
		})
	}
}

// TestYahoo_SimulatePollCycle simulates a complete poll cycle:
//   1. Prefetch (like Go Live)
//   2. Wait
//   3. Delta fetch + merge (like a poll)
//   4. Analyze + compare
// This is the closest simulation to what actually happens in production.
func TestYahoo_SimulatePollCycle(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping Yahoo integration test in short mode")
	}

	symbol := "DBK.DE"
	yahooInterval := "60m"
	prefetchPeriod := "2y"
	deltaPeriod := "7d" // getPollFetchPeriod("60m") = "7d"

	t.Log("=== Simulating Live Trading Poll Cycle for DBK.DE 1h ===")
	t.Log("")

	// Phase 1: Prefetch (Go Live moment)
	t.Log("PHASE 1: Prefetch (simulating Go Live)...")
	cache, err := fetchOHLCVFromYahoo(symbol, prefetchPeriod, yahooInterval)
	if err != nil {
		t.Fatalf("Prefetch failed: %v", err)
	}
	t.Logf("  Cache: %d bars (%s — %s)",
		len(cache),
		time.Unix(cache[0].Time, 0).Format("2006-01-02 15:04"),
		time.Unix(cache[len(cache)-1].Time, 0).Format("2006-01-02 15:04"))

	// Take a snapshot for later comparison
	referenceData := make([]OHLCV, len(cache))
	copy(referenceData, cache)

	// Phase 2: Simulate time passing (in production: wait for next candle)
	t.Log("")
	t.Log("PHASE 2: Simulating poll (delta fetch)...")
	time.Sleep(1 * time.Second)

	// Phase 3: Delta fetch + merge (what runLiveScan does)
	delta, err := fetchOHLCVFromYahoo(symbol, deltaPeriod, yahooInterval)
	if err != nil {
		t.Fatalf("Delta fetch failed: %v", err)
	}
	t.Logf("  Delta: %d bars (%s — %s)",
		len(delta),
		time.Unix(delta[0].Time, 0).Format("2006-01-02 15:04"),
		time.Unix(delta[len(delta)-1].Time, 0).Format("2006-01-02 15:04"))

	merged := mergeOHLCV(cache, delta)
	t.Logf("  After merge: %d bars", len(merged))

	// Phase 4: Fresh full fetch (ground truth)
	t.Log("")
	t.Log("PHASE 3: Fresh full fetch (ground truth)...")
	time.Sleep(500 * time.Millisecond)
	freshFull, err := fetchOHLCVFromYahoo(symbol, prefetchPeriod, yahooInterval)
	if err != nil {
		t.Fatalf("Fresh full fetch failed: %v", err)
	}
	t.Logf("  Fresh full: %d bars", len(freshFull))

	// Phase 5: Compare merged vs fresh full
	t.Log("")
	t.Log("PHASE 4: Comparing merged vs fresh full...")

	// Build timestamp maps
	mergedMap := map[int64]OHLCV{}
	for _, b := range merged {
		mergedMap[b.Time] = b
	}
	freshMap := map[int64]OHLCV{}
	for _, b := range freshFull {
		freshMap[b.Time] = b
	}

	// Check: every bar in freshFull should be in merged
	missing := 0
	closeDiffs := 0
	for _, fb := range freshFull {
		mb, ok := mergedMap[fb.Time]
		if !ok {
			missing++
			if missing <= 5 {
				t.Logf("  MISSING in merged: %s", time.Unix(fb.Time, 0).Format("2006-01-02 15:04"))
			}
			continue
		}
		if math.Abs(mb.Close-fb.Close) > 0.01 {
			closeDiffs++
			if closeDiffs <= 5 {
				t.Logf("  CLOSE DIFF at %s: merged=%.4f fresh=%.4f",
					time.Unix(fb.Time, 0).Format("2006-01-02 15:04"), mb.Close, fb.Close)
			}
		}
	}

	if missing > 0 {
		t.Errorf("MISSING BARS: %d bars in fresh-full not found in merged", missing)
	} else {
		t.Log("  No missing bars")
	}
	if closeDiffs > 0 {
		t.Logf("  %d bars have different close prices (expected for last/current candle)", closeDiffs)
	}

	// Signal comparison
	t.Log("")
	t.Log("PHASE 5: Signal comparison...")
	strategy := &HybridAITrendStrategy{}
	strategy.defaults()

	if len(freshFull) < strategy.RequiredBars() {
		t.Skipf("Not enough bars: %d < %d", len(freshFull), strategy.RequiredBars())
	}

	sigMerged := strategy.Analyze(merged)
	sigFresh := strategy.Analyze(freshFull)

	t.Logf("  Merged signals: %d", len(sigMerged))
	t.Logf("  Fresh signals:  %d", len(sigFresh))

	if len(sigMerged) != len(sigFresh) {
		t.Errorf("SIGNAL COUNT MISMATCH: merged=%d fresh=%d", len(sigMerged), len(sigFresh))
	} else {
		allMatch := true
		for i := range sigMerged {
			if sigMerged[i].Index != sigFresh[i].Index ||
				sigMerged[i].Direction != sigFresh[i].Direction ||
				math.Abs(sigMerged[i].EntryPrice-sigFresh[i].EntryPrice) > 0.01 {
				allMatch = false
				t.Errorf("  Signal %d MISMATCH: merged={idx:%d %s @%.2f} fresh={idx:%d %s @%.2f}",
					i, sigMerged[i].Index, sigMerged[i].Direction, sigMerged[i].EntryPrice,
					sigFresh[i].Index, sigFresh[i].Direction, sigFresh[i].EntryPrice)
			}
		}
		if allMatch {
			t.Logf("  ALL %d SIGNALS MATCH — Delta merge is correct!", len(sigMerged))
		}
	}

	t.Log("")
	t.Log("=== Poll Cycle Simulation Complete ===")
}
