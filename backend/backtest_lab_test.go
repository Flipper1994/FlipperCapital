package main

import (
	"encoding/json"
	"fmt"
	"math"
	"testing"
)

// generateSyntheticOHLCV creates synthetic monthly price data with a trend pattern (~30 day intervals)
func generateSyntheticOHLCV(n int, startPrice float64) []OHLCV {
	ohlcv := make([]OHLCV, n)
	price := startPrice
	baseTime := int64(946684800) // 2000-01-01
	for i := 0; i < n; i++ {
		// Create a cyclical pattern: up for 20 bars, down for 15, repeat
		cycle := i % 35
		if cycle < 20 {
			price *= 1.02 // up 2%
		} else {
			price *= 0.97 // down 3%
		}
		ohlcv[i] = OHLCV{
			Time:   baseTime + int64(i*2592000), // ~30 days apart
			Open:   price * 0.99,
			High:   price * 1.02,
			Low:    price * 0.97,
			Close:  price,
			Volume: 1000000,
		}
	}
	return ohlcv
}

// generateSyntheticWeeklyOHLCV creates synthetic weekly price data (~7 day intervals)
// covering the same time range as monthly data with n monthly bars
func generateSyntheticWeeklyOHLCV(nMonthly int, startPrice float64) []OHLCV {
	// ~4.3 weeks per month
	nWeekly := nMonthly * 4
	ohlcv := make([]OHLCV, nWeekly)
	price := startPrice
	baseTime := int64(946684800) // 2000-01-01
	for i := 0; i < nWeekly; i++ {
		// Same cyclical pattern but at weekly scale (up for 80 weeks, down for 60)
		cycle := i % 140
		if cycle < 80 {
			price *= 1.005 // up 0.5% per week
		} else {
			price *= 0.993 // down 0.7% per week
		}
		ohlcv[i] = OHLCV{
			Time:   baseTime + int64(i*604800), // ~7 days apart
			Open:   price * 0.995,
			High:   price * 1.01,
			Low:    price * 0.985,
			Close:  price,
			Volume: 500000,
		}
	}
	return ohlcv
}

func TestGetBarSignalState_Aggressive(t *testing.T) {
	// Test with hand-crafted short/long values
	short := []float64{0, 5, 10, -5, -3, -1, 2, 5, -2, -4}
	long := []float64{0, 3, 8, -2, -1, 0, 1, 4, -1, -3}

	// idx=3: short goes from 10 to -5 (turns negative)
	// idx=4: short=-3, prev=-5 → light red (negative but rising)
	// idx=5: short=-1, prev=-3 → light red
	// idx=6: short=2, prev=-1 → turned green

	// Aggressive: 1st light red bar triggers BUY
	signal := getBarSignalState(4, short, long, "aggressive", false)
	if signal != "BUY" {
		t.Errorf("Expected BUY at first light red bar (aggressive), got %s", signal)
	}

	// Defensive: needs 4 consecutive light red or red→green
	signal = getBarSignalState(4, short, long, "defensive", false)
	if signal == "BUY" {
		t.Errorf("Defensive should NOT buy at first light red bar, got %s", signal)
	}

	// idx=6: red→green transition → BUY for both modes
	signal = getBarSignalState(6, short, long, "defensive", false)
	if signal != "BUY" {
		t.Errorf("Expected BUY at red→green (defensive), got %s", signal)
	}

	// SELL check: dark red while in position
	signal = getBarSignalState(9, short, long, "aggressive", true)
	if signal != "SELL" {
		t.Errorf("Expected SELL at dark red bar (aggressive, in position), got %s", signal)
	}

	// HOLD check: in position, no sell signal
	signal = getBarSignalState(7, short, long, "aggressive", true)
	if signal != "HOLD" {
		t.Errorf("Expected HOLD at idx 7 (in position, rising short), got %s", signal)
	}
}

func TestGetBarSignalState_Quant(t *testing.T) {
	short := []float64{0, -5, -3, 2, 5, 8, -2, -5}
	long := []float64{0, -3, -1, 1, 3, 5, 2, -1}

	// idx=3: both positive now, prev: short=-3<0 → BUY
	signal := getBarSignalState(3, short, long, "quant", false)
	if signal != "BUY" {
		t.Errorf("Quant: expected BUY when both turn positive, got %s", signal)
	}

	// idx=6: short<0 while in position → SELL
	signal = getBarSignalState(6, short, long, "quant", true)
	if signal != "SELL" {
		t.Errorf("Quant: expected SELL when short < 0, got %s", signal)
	}

	// Ditz: short<0 but long>0 → HOLD (needs BOTH negative)
	signal = getBarSignalState(6, short, long, "ditz", true)
	if signal != "HOLD" {
		t.Errorf("Ditz: expected HOLD when only short < 0 (long still positive), got %s", signal)
	}

	// Ditz: idx=7 both negative → SELL
	signal = getBarSignalState(7, short, long, "ditz", true)
	if signal != "SELL" {
		t.Errorf("Ditz: expected SELL when both negative, got %s", signal)
	}
}

func TestGetBarConditionState_FirstLightRed(t *testing.T) {
	short := []float64{0, 5, 10, -8, -5, -3, -1, 2}
	long := []float64{0, 3, 8, -2, -1, 0, 1, 4}

	// idx=4: short=-5, prev=-8 → first light red
	match := getBarConditionState("FIRST_LIGHT_RED", 4, short, long, "aggressive", false)
	if !match {
		t.Error("Expected FIRST_LIGHT_RED to match at idx 4")
	}

	// idx=5: short=-3, prev=-5 → still light red, but NOT first
	match = getBarConditionState("FIRST_LIGHT_RED", 5, short, long, "aggressive", false)
	if match {
		t.Error("Expected FIRST_LIGHT_RED to NOT match at idx 5 (second light red)")
	}

	// idx=3: short=-8, prev=10 → dark red, not light red
	match = getBarConditionState("FIRST_LIGHT_RED", 3, short, long, "aggressive", false)
	if match {
		t.Error("Expected FIRST_LIGHT_RED to NOT match at idx 3 (dark red)")
	}
}

func TestGetBarConditionState_PositionIndependent(t *testing.T) {
	// BUY/SELL/HOLD/WAIT conditions must be evaluated position-independently
	short := []float64{0, 5, 10, -8, -5, -3, -1, 2}
	long := []float64{0, 3, 8, -2, -1, 0, 1, 4}

	// idx=3: short=-8, prev=10 → dark red → this is a SELL bar (if in position)
	// Even when called with inPosition=false (entry check), SELL condition should match
	match := getBarConditionState("SELL", 3, short, long, "aggressive", false)
	if !match {
		t.Error("SELL condition should match at dark red bar (idx 3) regardless of inPosition parameter")
	}

	// BUY condition at first light red (aggressive)
	match = getBarConditionState("BUY", 4, short, long, "aggressive", true)
	if !match {
		t.Error("BUY condition should match at first light red bar (aggressive, idx 4) regardless of inPosition parameter")
	}

	// WAIT condition at idx=5 (not a BUY bar, not in aggressive-first-light-red-1 position)
	match = getBarConditionState("WAIT", 5, short, long, "aggressive", true)
	if !match {
		t.Error("WAIT condition should match at idx 5 (second light red, no buy trigger)")
	}

	// HOLD at green bar while in position
	// idx=7: short=2, prev=-1 → just turned green → BUY bar
	match = getBarConditionState("HOLD", 2, short, long, "aggressive", false)
	if !match {
		t.Error("HOLD condition should match at idx 2 (green, would be HOLD if in position)")
	}
}

func TestGetBarConditionState_ANY(t *testing.T) {
	short := []float64{0, 5, 10}
	long := []float64{0, 3, 8}

	match := getBarConditionState("ANY", 2, short, long, "aggressive", false)
	if !match {
		t.Error("ANY should always match")
	}
}

func TestEvaluateBacktestLabRules_Testfall1(t *testing.T) {
	// Testfall 1: Monthly Light Red + Weekly BUY
	// Base mode: aggressive
	// Entry: FIRST_LIGHT_RED (monthly) AND BUY (weekly)
	monthlyOHLCV := generateSyntheticOHLCV(100, 100.0)
	weeklyOHLCV := generateSyntheticWeeklyOHLCV(100, 100.0) // weekly bars covering same time range

	monthlyResult := calculateBXtrenderServer(monthlyOHLCV, true, BXtrenderConfig{
		ShortL1: 5, ShortL2: 20, ShortL3: 15, LongL1: 20, LongL2: 15,
	}, 0, 0)

	weeklyResult := calculateBXtrenderServer(weeklyOHLCV, true, BXtrenderConfig{
		ShortL1: 5, ShortL2: 20, ShortL3: 15, LongL1: 20, LongL2: 15,
	}, 0, 0)

	// Ensure we got data
	if monthlyResult.Signal == "NO_DATA" {
		t.Fatal("Monthly BXtrender returned NO_DATA")
	}
	if weeklyResult.Signal == "NO_DATA" {
		t.Fatal("Weekly BXtrender returned NO_DATA")
	}

	// Rules: Entry only when monthly first light red AND weekly BUY
	rules := []BacktestLabRule{
		{Type: "entry", MonthlyCondition: "FIRST_LIGHT_RED", WeeklyCondition: "BUY", Operator: "AND"},
	}

	trades, markers := evaluateBacktestLabRules(
		monthlyOHLCV, weeklyOHLCV,
		monthlyResult, weeklyResult,
		"aggressive", rules, 20.0,
	)

	customClosedTrades := 0
	for _, t := range trades {
		if !t.IsOpen {
			customClosedTrades++
		}
	}

	fmt.Printf("Testfall 1 - Custom (FIRST_LIGHT_RED monthly AND weekly BUY): %d closed trades, %d markers\n", customClosedTrades, len(markers))

	// Verify trades execute at weekly granularity (trade timestamps should align with weekly bar times)
	for i, trade := range trades {
		if !trade.IsOpen {
			if trade.EntryPrice <= 0 || trade.ExitPrice <= 0 {
				t.Errorf("Trade %d has invalid prices: entry=%.2f, exit=%.2f", i, trade.EntryPrice, trade.ExitPrice)
			}
			expectedReturn := (trade.ExitPrice - trade.EntryPrice) / trade.EntryPrice * 100
			if math.Abs(trade.ReturnPct-expectedReturn) > 0.01 {
				t.Errorf("Trade %d return mismatch: got %.2f%%, expected %.2f%%", i, trade.ReturnPct, expectedReturn)
			}
		}
	}
}

func TestEvaluateBacktestLabRules_Testfall2(t *testing.T) {
	// Testfall 2: Weekly Early Entry
	// Base mode: defensive
	// Entry when monthly=SELL AND weekly=BUY_TO_HOLD OR monthly=WAIT AND weekly=BUY_TO_HOLD
	monthlyOHLCV := generateSyntheticOHLCV(100, 100.0)
	weeklyOHLCV := generateSyntheticWeeklyOHLCV(100, 100.0)

	monthlyResult := calculateBXtrenderServer(monthlyOHLCV, false, BXtrenderConfig{
		ShortL1: 5, ShortL2: 20, ShortL3: 15, LongL1: 20, LongL2: 15,
	}, 0, 0)

	weeklyResult := calculateBXtrenderServer(weeklyOHLCV, false, BXtrenderConfig{
		ShortL1: 5, ShortL2: 20, ShortL3: 15, LongL1: 20, LongL2: 15,
	}, 0, 0)

	if monthlyResult.Signal == "NO_DATA" {
		t.Fatal("Monthly BXtrender returned NO_DATA")
	}

	rules := []BacktestLabRule{
		{Type: "entry", MonthlyCondition: "SELL", WeeklyCondition: "BUY_TO_HOLD", Operator: "AND"},
		{Type: "entry", MonthlyCondition: "WAIT", WeeklyCondition: "BUY_TO_HOLD", Operator: "AND"},
	}

	trades, markers := evaluateBacktestLabRules(
		monthlyOHLCV, weeklyOHLCV,
		monthlyResult, weeklyResult,
		"defensive", rules, 20.0,
	)

	customClosedTrades := 0
	for _, t := range trades {
		if !t.IsOpen {
			customClosedTrades++
		}
	}

	fmt.Printf("Testfall 2 - Custom (SELL/WAIT + weekly BUY_TO_HOLD) trades: %d, markers: %d\n", customClosedTrades, len(markers))

	// Verify trades have valid data
	for i, trade := range trades {
		if !trade.IsOpen {
			if trade.EntryPrice <= 0 || trade.ExitPrice <= 0 {
				t.Errorf("Trade %d has invalid prices: entry=%.2f, exit=%.2f", i, trade.EntryPrice, trade.ExitPrice)
			}
		}
	}
}

func TestEvaluateBacktestLabRules_MonthlySellWeeklyBuy(t *testing.T) {
	// User's exact scenario: Entry when Monthly=SELL AND Weekly=BUY, Exit when Monthly=SELL AND Weekly=SELL
	monthlyOHLCV := generateSyntheticOHLCV(100, 100.0)
	weeklyOHLCV := generateSyntheticWeeklyOHLCV(100, 100.0)

	monthlyResult := calculateBXtrenderServer(monthlyOHLCV, true, BXtrenderConfig{
		ShortL1: 5, ShortL2: 20, ShortL3: 15, LongL1: 20, LongL2: 15,
	}, 0, 0)
	weeklyResult := calculateBXtrenderServer(weeklyOHLCV, true, BXtrenderConfig{
		ShortL1: 5, ShortL2: 20, ShortL3: 15, LongL1: 20, LongL2: 15,
	}, 0, 0)

	if monthlyResult.Signal == "NO_DATA" {
		t.Fatal("Monthly BXtrender returned NO_DATA")
	}

	rules := []BacktestLabRule{
		{Type: "entry", MonthlyCondition: "SELL", WeeklyCondition: "BUY", Operator: "AND"},
		{Type: "exit", MonthlyCondition: "SELL", WeeklyCondition: "SELL", Operator: "AND"},
	}

	trades, markers := evaluateBacktestLabRules(
		monthlyOHLCV, weeklyOHLCV,
		monthlyResult, weeklyResult,
		"aggressive", rules, 20.0,
	)

	closedTrades := 0
	for _, tr := range trades {
		if !tr.IsOpen {
			closedTrades++
		}
	}

	fmt.Printf("Monthly SELL+Weekly BUY test: %d closed trades, %d total trades, %d markers\n", closedTrades, len(trades), len(markers))

	// With the fix, this should produce some trades (SELL condition now works in entry context)
	// Verify trades have valid data
	for i, trade := range trades {
		if !trade.IsOpen {
			if trade.EntryPrice <= 0 || trade.ExitPrice <= 0 {
				t.Errorf("Trade %d has invalid prices: entry=%.2f, exit=%.2f", i, trade.EntryPrice, trade.ExitPrice)
			}
		}
	}
}

func TestEvaluateBacktestLabRules_WeeklyGranularity(t *testing.T) {
	// Verify that trades execute at weekly timestamps, not monthly
	monthlyOHLCV := generateSyntheticOHLCV(100, 100.0)
	weeklyOHLCV := generateSyntheticWeeklyOHLCV(100, 100.0)

	monthlyResult := calculateBXtrenderServer(monthlyOHLCV, true, BXtrenderConfig{
		ShortL1: 5, ShortL2: 20, ShortL3: 15, LongL1: 20, LongL2: 15,
	}, 0, 0)
	weeklyResult := calculateBXtrenderServer(weeklyOHLCV, true, BXtrenderConfig{
		ShortL1: 5, ShortL2: 20, ShortL3: 15, LongL1: 20, LongL2: 15,
	}, 0, 0)

	// Use default rules: Monthly=WAIT AND Weekly=BUY entry, base mode SELL exit
	rules := []BacktestLabRule{
		{Type: "entry", MonthlyCondition: "WAIT", WeeklyCondition: "BUY", Operator: "AND"},
	}

	trades, _ := evaluateBacktestLabRules(
		monthlyOHLCV, weeklyOHLCV,
		monthlyResult, weeklyResult,
		"aggressive", rules, 0,
	)

	// Build a set of valid weekly bar timestamps
	weeklyTimes := map[int64]bool{}
	for _, bar := range weeklyOHLCV {
		weeklyTimes[bar.Time] = true
	}

	// All trade entry/exit times must match a weekly bar timestamp
	for i, trade := range trades {
		if trade.EntryTime > 0 && !weeklyTimes[trade.EntryTime] {
			t.Errorf("Trade %d entry time %d does not match any weekly bar timestamp", i, trade.EntryTime)
		}
		if !trade.IsOpen && trade.ExitTime > 0 && trade.ExitReason != "TSL" && !weeklyTimes[trade.ExitTime] {
			t.Errorf("Trade %d exit time %d does not match any weekly bar timestamp", i, trade.ExitTime)
		}
	}

	fmt.Printf("Weekly granularity test: %d trades, all timestamps match weekly bars\n", len(trades))
}

func TestFindMonthlyIndexForWeeklyBar(t *testing.T) {
	monthly := generateSyntheticOHLCV(12, 100.0)     // 12 monthly bars
	weekly := generateSyntheticWeeklyOHLCV(12, 100.0) // ~48 weekly bars

	// First weekly bars should map to first monthly bar
	idx := findMonthlyIndexForWeeklyBar(weekly[0].Time, monthly)
	if idx != 0 {
		t.Errorf("First weekly bar should map to monthly index 0, got %d", idx)
	}

	// Weekly bar in the middle should map to a valid monthly index
	midWeekly := len(weekly) / 2
	idx = findMonthlyIndexForWeeklyBar(weekly[midWeekly].Time, monthly)
	if idx < 0 || idx >= len(monthly) {
		t.Errorf("Mid weekly bar should map to a valid monthly index, got %d", idx)
	}

	// The monthly bar's time should be <= the weekly bar's time
	if monthly[idx].Time > weekly[midWeekly].Time {
		t.Errorf("Monthly bar time %d > weekly bar time %d", monthly[idx].Time, weekly[midWeekly].Time)
	}

	// And the next monthly bar (if exists) should have time > weekly bar's time
	if idx+1 < len(monthly) && monthly[idx+1].Time <= weekly[midWeekly].Time {
		t.Errorf("Next monthly bar time %d should be > weekly bar time %d", monthly[idx+1].Time, weekly[midWeekly].Time)
	}

	fmt.Printf("findMonthlyIndexForWeeklyBar: weekly[%d] (time=%d) → monthly[%d] (time=%d)\n",
		midWeekly, weekly[midWeekly].Time, idx, monthly[idx].Time)
}

func TestConvertServerTradesToArena(t *testing.T) {
	serverTrades := []ServerTrade{
		{Type: "BUY", Time: 1000, Price: 100.0},
		{Type: "SELL", Time: 2000, Price: 120.0, PrevPrice: 100.0, Return: 20.0},
		{Type: "BUY", Time: 3000, Price: 110.0},
		{Type: "SELL", Time: 4000, Price: 90.0, PrevPrice: 110.0, Return: -18.18},
	}

	trades, markers := convertServerTradesToArena(serverTrades)

	if len(trades) != 2 {
		t.Errorf("Expected 2 trades, got %d", len(trades))
	}
	if len(markers) != 4 {
		t.Errorf("Expected 4 markers, got %d", len(markers))
	}

	if trades[0].ReturnPct != 20.0 {
		t.Errorf("Trade 0 return should be 20.0, got %.2f", trades[0].ReturnPct)
	}
	if trades[0].Direction != "LONG" {
		t.Errorf("Trade 0 should be LONG, got %s", trades[0].Direction)
	}
}

func TestCalculateBacktestLabMetrics(t *testing.T) {
	trades := []ArenaBacktestTrade{
		{Direction: "LONG", EntryPrice: 100, ExitPrice: 120, ReturnPct: 20.0, ExitReason: "SIGNAL"},
		{Direction: "LONG", EntryPrice: 110, ExitPrice: 90, ReturnPct: -18.18, ExitReason: "TSL"},
		{Direction: "LONG", EntryPrice: 95, ExitPrice: 130, ReturnPct: 36.84, ExitReason: "SIGNAL"},
	}

	metrics := calculateBacktestLabMetrics(trades)

	if metrics.TotalTrades != 3 {
		t.Errorf("Expected 3 total trades, got %d", metrics.TotalTrades)
	}
	if metrics.Wins != 2 {
		t.Errorf("Expected 2 wins, got %d", metrics.Wins)
	}
	if metrics.Losses != 1 {
		t.Errorf("Expected 1 loss, got %d", metrics.Losses)
	}

	expectedWinRate := 2.0 / 3.0 * 100
	if math.Abs(metrics.WinRate-expectedWinRate) > 0.1 {
		t.Errorf("Expected win rate ~%.1f, got %.1f", expectedWinRate, metrics.WinRate)
	}

	// Sanity check: output metrics as JSON
	jsonBytes, _ := json.MarshalIndent(metrics, "", "  ")
	fmt.Printf("Metrics: %s\n", string(jsonBytes))
}

func TestWeeklyOHLCVCacheRoundtrip(t *testing.T) {
	// Verify OHLCV data survives JSON marshal/unmarshal (used in DB cache)
	original := generateSyntheticOHLCV(100, 50.0)

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Failed to marshal OHLCV: %v", err)
	}

	var restored []OHLCV
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatalf("Failed to unmarshal OHLCV: %v", err)
	}

	if len(restored) != len(original) {
		t.Fatalf("Length mismatch: %d vs %d", len(restored), len(original))
	}

	for i := 0; i < len(original); i++ {
		if original[i].Time != restored[i].Time || original[i].Close != restored[i].Close {
			t.Errorf("Bar %d mismatch: original={time:%d,close:%.2f} restored={time:%d,close:%.2f}",
				i, original[i].Time, original[i].Close, restored[i].Time, restored[i].Close)
			break
		}
	}
}

// ========== NW Bollinger Bands Strategy Tests ==========

// generateVolatileOHLCV creates synthetic data with sharp spikes that cross BB bands
func generateVolatileOHLCV(n int, startPrice float64) []OHLCV {
	ohlcv := make([]OHLCV, n)
	price := startPrice
	baseTime := int64(946684800)
	for i := 0; i < n; i++ {
		cycle := i % 50
		if cycle < 30 {
			price *= 1.005 // slow drift up
		} else if cycle < 35 {
			price *= 0.97 // sharp drop (crosses lower band)
		} else if cycle < 40 {
			price *= 1.005 // slow recovery
		} else {
			price *= 1.04 // sharp spike up (crosses upper band)
		}
		spread := price * 0.015
		ohlcv[i] = OHLCV{
			Time:   baseTime + int64(i*14400), // 4h apart
			Open:   price - spread*0.3,
			High:   price + spread,
			Low:    price - spread,
			Close:  price,
			Volume: 1000000,
		}
	}
	return ohlcv
}

func TestNWBollingerBandSignals(t *testing.T) {
	// Generate volatile data for NW lookback (499+100=599 bars)
	ohlcv := generateVolatileOHLCV(800, 100.0)

	strategy := &HybridAITrendStrategy{} // uses defaults
	strategy.defaults()

	// Verify defaults match Pine Script
	if strategy.BB1Period != 20 || strategy.BB1Stdev != 3.0 {
		t.Errorf("BB1 defaults wrong: period=%d stdev=%.1f", strategy.BB1Period, strategy.BB1Stdev)
	}
	if strategy.BB2Period != 75 || strategy.BB2Stdev != 3.0 {
		t.Errorf("BB2 defaults wrong: period=%d stdev=%.1f (should be 3.0, not 4!)", strategy.BB2Period, strategy.BB2Stdev)
	}
	if strategy.BB3Period != 100 || strategy.BB3Stdev != 4.0 {
		t.Errorf("BB3 defaults wrong: period=%d stdev=%.1f", strategy.BB3Period, strategy.BB3Stdev)
	}
	if strategy.BB4Period != 100 || strategy.BB4Stdev != 4.25 {
		t.Errorf("BB4 defaults wrong: period=%d stdev=%.2f", strategy.BB4Period, strategy.BB4Stdev)
	}
	if strategy.NWBandwidth != 6.0 {
		t.Errorf("NW Bandwidth default wrong: %.1f (should be 6.0)", strategy.NWBandwidth)
	}
	if strategy.NWLookback != 499 {
		t.Errorf("NW Lookback default wrong: %d (should be 499)", strategy.NWLookback)
	}

	closes := extractCloses(ohlcv)
	upper1, lower1 := calculateSingleBBLevel(ohlcv, strategy.BB1Period, strategy.BB1Stdev, strategy.NWBandwidth, strategy.NWLookback)

	// Test Analyze returns signals
	signals := strategy.Analyze(ohlcv)
	fmt.Printf("NW-BB Signals: %d total\n", len(signals))

	if len(signals) == 0 {
		t.Fatal("Expected at least some signals from synthetic data")
	}

	// Verify each signal: crossover at Index-1, entry at Index (next bar open)
	longCount, shortCount := 0, 0
	for _, sig := range signals {
		entryIdx := sig.Index
		crossIdx := entryIdx - 1 // crossover happened one bar earlier
		if crossIdx < 1 || entryIdx >= len(ohlcv) {
			t.Errorf("Signal at invalid index %d (crossover at %d)", entryIdx, crossIdx)
			continue
		}

		// Entry price must be the Open of the entry bar (no look-ahead bias)
		if sig.EntryPrice != ohlcv[entryIdx].Open {
			t.Errorf("Signal at %d: EntryPrice=%.2f but bar Open=%.2f (should use next bar open)",
				entryIdx, sig.EntryPrice, ohlcv[entryIdx].Open)
		}

		if sig.Direction == "LONG" {
			longCount++
			// BUY crossover at crossIdx: close <= lower1 AND prev_close > prev_lower1
			if !(closes[crossIdx] <= lower1[crossIdx] && closes[crossIdx-1] > lower1[crossIdx-1]) {
				t.Errorf("LONG signal (entry %d, cross %d) violates crossover: close=%.2f lower=%.2f, prev_close=%.2f prev_lower=%.2f",
					entryIdx, crossIdx, closes[crossIdx], lower1[crossIdx], closes[crossIdx-1], lower1[crossIdx-1])
			}
		} else if sig.Direction == "SHORT" {
			shortCount++
			// SELL crossover at crossIdx: close >= upper1 AND prev_close < prev_upper1
			if !(closes[crossIdx] >= upper1[crossIdx] && closes[crossIdx-1] < upper1[crossIdx-1]) {
				t.Errorf("SHORT signal (entry %d, cross %d) violates crossover: close=%.2f upper=%.2f, prev_close=%.2f prev_upper=%.2f",
					entryIdx, crossIdx, closes[crossIdx], upper1[crossIdx], closes[crossIdx-1], upper1[crossIdx-1])
			}
		}
	}

	fmt.Printf("NW-BB: %d LONG, %d SHORT signals\n", longCount, shortCount)
}

func TestNWBollingerBandOverlays(t *testing.T) {
	ohlcv := generateSyntheticOHLCV(700, 100.0)
	strategy := &HybridAITrendStrategy{}

	overlays := strategy.ComputeOverlays(ohlcv)

	// Should return 8 overlays (4 upper + 4 lower)
	if len(overlays) != 8 {
		t.Fatalf("Expected 8 overlays, got %d", len(overlays))
	}

	// Verify names and fill structure
	expectedNames := []string{
		"NW-BB Upper 1", "NW-BB Upper 2", "NW-BB Upper 3", "NW-BB Upper 4",
		"NW-BB Lower 1", "NW-BB Lower 2", "NW-BB Lower 3", "NW-BB Lower 4",
	}
	for i, name := range expectedNames {
		if overlays[i].Name != name {
			t.Errorf("Overlay %d: expected name %q, got %q", i, name, overlays[i].Name)
		}
	}

	// Level 1 bands should have visible lines (Style=0)
	if overlays[0].Style != 0 {
		t.Errorf("Upper 1 should have solid line (style=0), got %d", overlays[0].Style)
	}
	if overlays[4].Style != 0 {
		t.Errorf("Lower 1 should have solid line (style=0), got %d", overlays[4].Style)
	}

	// Lower bands should have InvertFill
	for i := 4; i < 7; i++ {
		if overlays[i].FillColor != "" && !overlays[i].InvertFill {
			t.Errorf("Overlay %d (%s) with FillColor should have InvertFill=true", i, overlays[i].Name)
		}
	}

	// Verify data has non-zero values after warmup
	nonZero := 0
	for _, pt := range overlays[0].Data {
		if pt.Value > 0 {
			nonZero++
		}
	}
	if nonZero == 0 {
		t.Error("Upper band 1 has no non-zero data points")
	}
	fmt.Printf("NW-BB Overlays: 8 bands, %d non-zero points in Upper 1\n", nonZero)
}

func TestNWBollingerBandIndicators(t *testing.T) {
	ohlcv := generateVolatileOHLCV(800, 100.0)
	strategy := &HybridAITrendStrategy{}
	strategy.defaults()

	indicators := strategy.ComputeIndicators(ohlcv)
	if indicators == nil || len(indicators) == 0 {
		t.Fatal("Expected Hybrid EMA AlgoLearner indicator series, got nil/empty")
	}

	// Should have 4 series: oscillator line + 3 reference lines (75, 50, 25)
	if len(indicators) != 4 {
		t.Errorf("Expected 4 indicator series (osc + 3 refs), got %d", len(indicators))
	}

	// First series should be the oscillator
	if indicators[0].Name != "Hybrid EMA AlgoLearner" {
		t.Errorf("Expected first series 'Hybrid EMA AlgoLearner', got %q", indicators[0].Name)
	}

	// Oscillator values should be in 0-100 range (after warmup)
	nonZero := 0
	for _, pt := range indicators[0].Data {
		if pt.Value > 0 {
			nonZero++
			if pt.Value > 100 {
				t.Errorf("Oscillator value %.2f exceeds 100", pt.Value)
			}
		}
	}
	if nonZero == 0 {
		t.Error("Oscillator has no non-zero data points")
	}
	fmt.Printf("Hybrid EMA AlgoLearner: %d series, %d non-zero oscillator points\n", len(indicators), nonZero)
}

func TestNWSmoothingMatchesPineScript(t *testing.T) {
	// Verify Gaussian kernel formula: w(x,h) = exp(-(x²)/(2h²))
	h := 6.0

	// At offset 0: weight should be 1.0
	w0 := math.Exp(-(0.0 * 0.0) / (2 * h * h))
	if math.Abs(w0-1.0) > 1e-10 {
		t.Errorf("Weight at offset 0: expected 1.0, got %f", w0)
	}

	// At offset h: weight should be exp(-0.5) ≈ 0.6065
	wh := math.Exp(-(h * h) / (2 * h * h))
	expected := math.Exp(-0.5)
	if math.Abs(wh-expected) > 1e-10 {
		t.Errorf("Weight at offset h: expected %f, got %f", expected, wh)
	}

	// At offset 2h: weight should be exp(-2) ≈ 0.1353
	w2h := math.Exp(-(2 * h * 2 * h) / (2 * h * h))
	expected2 := math.Exp(-2.0)
	if math.Abs(w2h-expected2) > 1e-10 {
		t.Errorf("Weight at offset 2h: expected %f, got %f", expected2, w2h)
	}

	// Test that NW smoothing with bandwidth=6 and lookback=499 produces smooth output
	data := make([]float64, 600)
	for i := range data {
		data[i] = 100.0 + 10.0*math.Sin(float64(i)*0.1) // smooth sine wave
	}
	smoothed := nadarayaWatsonSmooth(data, 6.0, 499)

	// Smoothed output should be close to original for smooth input
	maxDiff := 0.0
	for i := 100; i < len(data); i++ { // skip warmup
		diff := math.Abs(smoothed[i] - data[i])
		if diff > maxDiff {
			maxDiff = diff
		}
	}
	fmt.Printf("NW Smoothing max diff on sine wave: %.4f (should be small)\n", maxDiff)

	if maxDiff > 5.0 {
		t.Errorf("NW smoothing diverges too much from smooth input: max diff = %.4f", maxDiff)
	}
}

func TestBacktestLabResponse_NoRules(t *testing.T) {
	// Verify base mode backtest works (no custom rules)
	monthlyOHLCV := generateSyntheticOHLCV(100, 100.0)

	result := calculateBXtrenderServer(monthlyOHLCV, true, BXtrenderConfig{
		ShortL1: 5, ShortL2: 20, ShortL3: 15, LongL1: 20, LongL2: 15,
	}, 0, 0)

	if result.Signal == "NO_DATA" {
		t.Fatal("BXtrender returned NO_DATA")
	}

	trades, markers := convertServerTradesToArena(result.Trades)
	metrics := calculateBacktestLabMetrics(trades)

	closedTrades := 0
	for _, tr := range trades {
		if !tr.IsOpen {
			closedTrades++
		}
	}

	fmt.Printf("Base mode (no rules): %d closed trades, %d markers\n", closedTrades, len(markers))
	fmt.Printf("Signal: %s, Bars: %d\n", result.Signal, result.Bars)

	if closedTrades == 0 && len(result.Trades) > 0 {
		t.Error("Expected at least some closed trades from base mode")
	}

	if metrics.TotalTrades != closedTrades {
		t.Errorf("Metrics total trades (%d) doesn't match closed trades (%d)", metrics.TotalTrades, closedTrades)
	}
}
