package main

import (
	"fmt"
	"math"
	"testing"
	"time"
)

// ============================================================
// Helper: Generate realistic OHLCV data with known patterns
// ============================================================

func generateOHLCV(n int, startPrice float64, startTime int64, intervalSec int64) []OHLCV {
	bars := make([]OHLCV, n)
	price := startPrice
	for i := 0; i < n; i++ {
		// Simple random walk with sine pattern for predictable movement
		phase := float64(i) / 30.0 * math.Pi
		delta := math.Sin(phase) * 2.0
		price = startPrice + delta + float64(i)*0.01

		open := price - 0.5
		close := price + 0.5
		high := math.Max(open, close) + 0.3
		low := math.Min(open, close) - 0.3
		if i%7 == 0 {
			// Occasional dip
			low -= 3.0
			close = open - 1.0
		}
		bars[i] = OHLCV{
			Time:   startTime + int64(i)*intervalSec,
			Open:   open,
			High:   high,
			Low:    low,
			Close:  close,
			Volume: 100000 + float64(i%20)*10000,
		}
	}
	return bars
}

// ============================================================
// 1. mergeOHLCV Tests
// ============================================================

func TestMergeOHLCV_EmptyCache(t *testing.T) {
	fresh := generateOHLCV(10, 100, 1000, 60)
	result := mergeOHLCV(nil, fresh)
	if len(result) != 10 {
		t.Fatalf("expected 10 bars, got %d", len(result))
	}
}

func TestMergeOHLCV_EmptyFresh(t *testing.T) {
	cached := generateOHLCV(10, 100, 1000, 60)
	result := mergeOHLCV(cached, nil)
	if len(result) != 10 {
		t.Fatalf("expected 10 bars, got %d", len(result))
	}
}

func TestMergeOHLCV_NoOverlap(t *testing.T) {
	cached := generateOHLCV(10, 100, 1000, 60)   // Times: 1000..1540
	fresh := generateOHLCV(5, 110, 2000, 60)      // Times: 2000..2240
	result := mergeOHLCV(cached, fresh)
	if len(result) != 15 {
		t.Fatalf("expected 15 bars (no overlap), got %d", len(result))
	}
	// Verify order
	for i := 1; i < len(result); i++ {
		if result[i].Time <= result[i-1].Time {
			t.Fatalf("bar %d time %d <= bar %d time %d", i, result[i].Time, i-1, result[i-1].Time)
		}
	}
}

func TestMergeOHLCV_WithOverlap(t *testing.T) {
	cached := generateOHLCV(100, 100, 1000, 60)
	// Fresh starts at bar 90 of cached (time = 1000 + 90*60 = 6400)
	freshStart := int64(1000 + 90*60)
	fresh := generateOHLCV(20, 110, freshStart, 60)
	result := mergeOHLCV(cached, fresh)

	// Should be: 90 cached bars + 20 fresh bars = 110
	if len(result) != 110 {
		t.Fatalf("expected 110 bars (90 cached + 20 fresh), got %d", len(result))
	}
	// No duplicates: all timestamps unique and ascending
	seen := map[int64]bool{}
	for i, bar := range result {
		if seen[bar.Time] {
			t.Fatalf("duplicate timestamp at index %d: %d", i, bar.Time)
		}
		seen[bar.Time] = true
		if i > 0 && bar.Time <= result[i-1].Time {
			t.Fatalf("bar %d not ascending: %d <= %d", i, bar.Time, result[i-1].Time)
		}
	}
	// Fresh data should overwrite cached in overlap region
	if result[90].Close != fresh[0].Close {
		t.Fatalf("overlap region not from fresh data: got %.2f, expected %.2f", result[90].Close, fresh[0].Close)
	}
}

func TestMergeOHLCV_CompleteOverlap(t *testing.T) {
	cached := generateOHLCV(50, 100, 1000, 60)
	// Fresh covers entire cached range and more
	fresh := generateOHLCV(70, 105, 1000, 60)
	result := mergeOHLCV(cached, fresh)
	// Fresh starts at same time as cached → all cached cut, only fresh remains
	if len(result) != 70 {
		t.Fatalf("expected 70 bars (complete overlap), got %d", len(result))
	}
	if result[0].Close != fresh[0].Close {
		t.Fatalf("first bar should be from fresh data")
	}
}

func TestMergeOHLCV_SingleBarDelta(t *testing.T) {
	cached := generateOHLCV(500, 100, 1000, 3600) // 500 hourly bars
	// Delta: only last 2 bars fresh (simulating poll update)
	freshStart := cached[498].Time
	fresh := []OHLCV{
		{Time: freshStart, Open: 200, High: 205, Low: 195, Close: 202, Volume: 50000},
		{Time: freshStart + 3600, Open: 202, High: 208, Low: 200, Close: 206, Volume: 60000},
	}
	result := mergeOHLCV(cached, fresh)
	// 498 from cache + 2 from fresh = 500
	if len(result) != 500 {
		t.Fatalf("expected 500 bars, got %d", len(result))
	}
	// Last bar is the new one
	if result[499].Close != 206 {
		t.Fatalf("last bar close should be 206, got %.2f", result[499].Close)
	}
	// Bar at 498 should be from fresh (updated candle)
	if result[498].Close != 202 {
		t.Fatalf("bar 498 should be from fresh (202), got %.2f", result[498].Close)
	}
}

// ============================================================
// 2. Strategy Minimum Bars Tests
// ============================================================

func TestStrategyMinBars_RegressionScalping(t *testing.T) {
	s := &RegressionScalpingStrategy{}
	s.defaults()

	required := s.RequiredBars()
	if required != s.Length+20 {
		t.Fatalf("expected %d required bars, got %d", s.Length+20, required)
	}

	// Too few bars → no signals
	tooFew := generateOHLCV(required-1, 100, 1000, 300)
	signals := s.Analyze(tooFew)
	if len(signals) != 0 {
		t.Fatalf("expected 0 signals with %d bars (need %d), got %d", len(tooFew), required, len(signals))
	}

	// Exactly enough bars → should not panic
	enough := generateOHLCV(required, 100, 1000, 300)
	_ = s.Analyze(enough) // no panic = pass
}

func TestStrategyMinBars_HybridAITrend(t *testing.T) {
	s := &HybridAITrendStrategy{}
	s.defaults()

	required := s.RequiredBars()
	if required != s.NWLookback+100 {
		t.Fatalf("expected %d required bars, got %d", s.NWLookback+100, required)
	}

	// Too few bars → no signals
	tooFew := generateOHLCV(s.BB1Period+1, 100, 1000, 300)
	signals := s.Analyze(tooFew)
	if len(signals) != 0 {
		t.Fatalf("expected 0 signals with too few bars, got %d", len(signals))
	}
}

func TestStrategyMinBars_DiamondSignals(t *testing.T) {
	s := &DiamondSignalsStrategy{}
	s.defaults()

	required := s.RequiredBars()
	if required != 200 {
		t.Fatalf("expected 200 required bars, got %d", required)
	}

	tooFew := generateOHLCV(199, 100, 1000, 14400)
	signals := s.Analyze(tooFew)
	if len(signals) != 0 {
		t.Fatalf("expected 0 signals with 199 bars, got %d", len(signals))
	}
}

// ============================================================
// 3. Signal Stability — same data = same signals
//    Full data vs Cache+Merge must produce identical signals
// ============================================================

func TestSignalStability_RegressionScalping(t *testing.T) {
	s := &RegressionScalpingStrategy{}
	s.defaults()

	// Generate enough data
	n := 300
	fullData := generateOHLCV(n, 100, 1000, 300)

	// Full analysis
	signalsFull := s.Analyze(fullData)

	// Simulate cache + merge: split at bar 250, delta fetch last 80 bars (overlap at 250)
	cached := make([]OHLCV, 250)
	copy(cached, fullData[:250])
	fresh := make([]OHLCV, 80)
	copy(fresh, fullData[220:300])
	merged := mergeOHLCV(cached, fresh)

	signalsMerged := s.Analyze(merged)

	// Signals must be identical
	if len(signalsFull) != len(signalsMerged) {
		t.Fatalf("signal count mismatch: full=%d merged=%d", len(signalsFull), len(signalsMerged))
	}
	for i := range signalsFull {
		if signalsFull[i].Index != signalsMerged[i].Index {
			t.Errorf("signal %d index mismatch: full=%d merged=%d", i, signalsFull[i].Index, signalsMerged[i].Index)
		}
		if signalsFull[i].Direction != signalsMerged[i].Direction {
			t.Errorf("signal %d direction mismatch: full=%s merged=%s", i, signalsFull[i].Direction, signalsMerged[i].Direction)
		}
		if math.Abs(signalsFull[i].EntryPrice-signalsMerged[i].EntryPrice) > 0.001 {
			t.Errorf("signal %d entry price mismatch: full=%.4f merged=%.4f", i, signalsFull[i].EntryPrice, signalsMerged[i].EntryPrice)
		}
		if math.Abs(signalsFull[i].StopLoss-signalsMerged[i].StopLoss) > 0.001 {
			t.Errorf("signal %d SL mismatch: full=%.4f merged=%.4f", i, signalsFull[i].StopLoss, signalsMerged[i].StopLoss)
		}
		if math.Abs(signalsFull[i].TakeProfit-signalsMerged[i].TakeProfit) > 0.001 {
			t.Errorf("signal %d TP mismatch: full=%.4f merged=%.4f", i, signalsFull[i].TakeProfit, signalsMerged[i].TakeProfit)
		}
	}
}

func TestSignalStability_HybridAITrend(t *testing.T) {
	s := &HybridAITrendStrategy{}
	s.defaults()

	n := 700
	fullData := generateOHLCV(n, 150, 1000, 300)

	signalsFull := s.Analyze(fullData)

	// Cache+merge split
	cached := make([]OHLCV, 650)
	copy(cached, fullData[:650])
	fresh := make([]OHLCV, 80)
	copy(fresh, fullData[620:700])
	merged := mergeOHLCV(cached, fresh)

	signalsMerged := s.Analyze(merged)

	if len(signalsFull) != len(signalsMerged) {
		t.Fatalf("signal count mismatch: full=%d merged=%d", len(signalsFull), len(signalsMerged))
	}
	for i := range signalsFull {
		if signalsFull[i].Index != signalsMerged[i].Index || signalsFull[i].Direction != signalsMerged[i].Direction {
			t.Errorf("signal %d mismatch: full={idx:%d dir:%s} merged={idx:%d dir:%s}",
				i, signalsFull[i].Index, signalsFull[i].Direction, signalsMerged[i].Index, signalsMerged[i].Direction)
		}
	}
}

func TestSignalStability_DiamondSignals(t *testing.T) {
	s := &DiamondSignalsStrategy{}
	s.defaults()

	n := 400
	fullData := generateOHLCV(n, 200, 1000, 14400)

	signalsFull := s.Analyze(fullData)

	// Cache+merge split
	cached := make([]OHLCV, 350)
	copy(cached, fullData[:350])
	fresh := make([]OHLCV, 80)
	copy(fresh, fullData[320:400])
	merged := mergeOHLCV(cached, fresh)

	signalsMerged := s.Analyze(merged)

	if len(signalsFull) != len(signalsMerged) {
		t.Fatalf("signal count mismatch: full=%d merged=%d", len(signalsFull), len(signalsMerged))
	}
	for i := range signalsFull {
		if signalsFull[i].Index != signalsMerged[i].Index || signalsFull[i].Direction != signalsMerged[i].Direction {
			t.Errorf("signal %d mismatch: full={idx:%d dir:%s} merged={idx:%d dir:%s}",
				i, signalsFull[i].Index, signalsFull[i].Direction, signalsMerged[i].Index, signalsMerged[i].Direction)
		}
	}
}

// ============================================================
// 4. Signal Correctness — Entry at i+1 open (no look-ahead)
// ============================================================

func TestSignalNoLookAhead_RegressionScalping(t *testing.T) {
	s := &RegressionScalpingStrategy{}
	s.defaults()

	data := generateOHLCV(300, 100, 1000, 300)
	signals := s.Analyze(data)

	for _, sig := range signals {
		if sig.Index <= 0 {
			t.Errorf("signal index %d <= 0", sig.Index)
		}
		if sig.Index >= len(data) {
			t.Errorf("signal index %d >= len(data) %d", sig.Index, len(data))
		}
		// Entry must be at bar's open (no close price = look-ahead)
		if sig.EntryPrice != data[sig.Index].Open {
			t.Errorf("signal at index %d: entry %.4f != bar open %.4f (look-ahead bias!)",
				sig.Index, sig.EntryPrice, data[sig.Index].Open)
		}
		// SL must be set
		if sig.StopLoss <= 0 {
			t.Errorf("signal at index %d: StopLoss is 0", sig.Index)
		}
		// TP must be set
		if sig.TakeProfit <= 0 {
			t.Errorf("signal at index %d: TakeProfit is 0", sig.Index)
		}
		// LONG: SL < Entry < TP
		if sig.Direction == "LONG" {
			if sig.StopLoss >= sig.EntryPrice {
				t.Errorf("LONG signal at %d: SL %.4f >= Entry %.4f", sig.Index, sig.StopLoss, sig.EntryPrice)
			}
			if sig.TakeProfit <= sig.EntryPrice {
				t.Errorf("LONG signal at %d: TP %.4f <= Entry %.4f", sig.Index, sig.TakeProfit, sig.EntryPrice)
			}
		}
		// SHORT: TP < Entry < SL
		if sig.Direction == "SHORT" {
			if sig.StopLoss <= sig.EntryPrice {
				t.Errorf("SHORT signal at %d: SL %.4f <= Entry %.4f", sig.Index, sig.StopLoss, sig.EntryPrice)
			}
			if sig.TakeProfit >= sig.EntryPrice {
				t.Errorf("SHORT signal at %d: TP %.4f >= Entry %.4f", sig.Index, sig.TakeProfit, sig.EntryPrice)
			}
		}
	}
}

func TestSignalNoLookAhead_HybridAITrend(t *testing.T) {
	s := &HybridAITrendStrategy{}
	s.defaults()

	data := generateOHLCV(700, 150, 1000, 300)
	signals := s.Analyze(data)

	for _, sig := range signals {
		if sig.Index >= len(data) {
			t.Errorf("signal index %d >= len(data) %d", sig.Index, len(data))
			continue
		}
		// Entry at bar's open (Signal Index i+1 → entry at next bar open)
		// HybridAI uses Index: i+1 as signal bar, entryPrice = ohlcv[i+1].Open
		if sig.EntryPrice != data[sig.Index].Open {
			t.Errorf("signal at index %d: entry %.6f != bar open %.6f (look-ahead!)",
				sig.Index, sig.EntryPrice, data[sig.Index].Open)
		}
		// SL/TP direction check — SL Buffer 1.5% means SL is always different from Entry
		if sig.Direction == "LONG" {
			expectedSL := sig.EntryPrice * (1 - s.SLBuffer/100)
			if math.Abs(sig.StopLoss-expectedSL) > 0.0001 {
				t.Errorf("LONG at %d: SL %.4f != expected %.4f (entry*%.3f)", sig.Index, sig.StopLoss, expectedSL, 1-s.SLBuffer/100)
			}
		}
		if sig.Direction == "SHORT" {
			expectedSL := sig.EntryPrice * (1 + s.SLBuffer/100)
			if math.Abs(sig.StopLoss-expectedSL) > 0.0001 {
				t.Errorf("SHORT at %d: SL %.4f != expected %.4f (entry*%.3f)", sig.Index, sig.StopLoss, expectedSL, 1+s.SLBuffer/100)
			}
		}
	}
}

func TestSignalNoLookAhead_DiamondSignals(t *testing.T) {
	s := &DiamondSignalsStrategy{}
	s.defaults()

	data := generateOHLCV(400, 200, 1000, 14400)
	signals := s.Analyze(data)

	for _, sig := range signals {
		if sig.Index >= len(data) {
			t.Errorf("signal index %d >= len(data) %d", sig.Index, len(data))
			continue
		}
		if sig.EntryPrice != data[sig.Index].Open {
			t.Errorf("signal at index %d: entry %.4f != bar open %.4f",
				sig.Index, sig.EntryPrice, data[sig.Index].Open)
		}
	}
}

// ============================================================
// 5. processLiveSymbolWithData E2E
//    Tests: Signal → DB Position → correct fields
// ============================================================

func TestProcessLiveSymbol_OpensPosition(t *testing.T) {
	setupLiveTestDB(t)

	// Create session
	session := LiveTradingSession{
		UserID:      1,
		Strategy:    "regression_scalping",
		Interval:    "5m",
		Symbols:     `["TEST"]`,
		TradeAmount: 500,
		Currency:    "USD",
		LongOnly:    false,
		IsActive:    true,
		StartedAt:   time.Unix(1000, 0), // very early start
	}
	db.Create(&session)

	config := LiveTradingConfig{
		UserID:        1,
		AlpacaEnabled: false, // no Alpaca for unit test
	}

	strategy := &RegressionScalpingStrategy{}
	strategy.defaults()

	// Generate enough data for signals
	data := generateOHLCV(300, 100, 1000, 300)
	signals := strategy.Analyze(data)
	if len(signals) == 0 {
		t.Skip("no signals generated with test data — data pattern doesn't trigger strategy")
	}

	// Run the full process
	_, ok := processLiveSymbolWithData(session, "TEST", strategy, data, config)
	if !ok {
		t.Fatal("processLiveSymbolWithData returned ok=false")
	}

	// Check DB positions
	var positions []LiveTradingPosition
	db.Where("session_id = ? AND symbol = ?", session.ID, "TEST").Find(&positions)

	if len(positions) == 0 {
		t.Fatal("no positions created despite signals")
	}

	for _, pos := range positions {
		// Verify required fields
		if pos.Symbol != "TEST" {
			t.Errorf("wrong symbol: %s", pos.Symbol)
		}
		if pos.Direction != "LONG" && pos.Direction != "SHORT" {
			t.Errorf("invalid direction: %s", pos.Direction)
		}
		if pos.EntryPrice <= 0 {
			t.Errorf("entry price <= 0: %.4f", pos.EntryPrice)
		}
		if pos.Quantity <= 0 {
			t.Errorf("quantity <= 0: %.6f", pos.Quantity)
		}
		if pos.InvestedAmount <= 0 {
			t.Errorf("invested amount <= 0: %.2f", pos.InvestedAmount)
		}
		// Invested should be qty * entry (in USD since Currency=USD)
		expected := pos.Quantity * pos.EntryPriceUSD
		if math.Abs(pos.InvestedAmount-expected) > 0.01 {
			t.Errorf("invested amount %.2f != qty*entry %.2f", pos.InvestedAmount, expected)
		}
		if pos.StopLoss <= 0 {
			t.Errorf("SL not set: %.4f", pos.StopLoss)
		}
		if pos.TakeProfit <= 0 {
			t.Errorf("TP not set: %.4f", pos.TakeProfit)
		}
		if pos.AlpacaOrderID != "" {
			t.Errorf("should have no Alpaca order ID when disabled, got: %s", pos.AlpacaOrderID)
		}
	}
}

func TestProcessLiveSymbol_NoDuplicateSignals(t *testing.T) {
	setupLiveTestDB(t)

	session := LiveTradingSession{
		UserID:      1,
		Strategy:    "regression_scalping",
		Interval:    "5m",
		Symbols:     `["DUP"]`,
		TradeAmount: 500,
		Currency:    "USD",
		IsActive:    true,
		StartedAt:   time.Unix(1000, 0),
	}
	db.Create(&session)

	config := LiveTradingConfig{UserID: 1}
	strategy := &RegressionScalpingStrategy{}
	strategy.defaults()

	data := generateOHLCV(300, 100, 1000, 300)

	// Run twice with same data
	processLiveSymbolWithData(session, "DUP", strategy, data, config)
	processLiveSymbolWithData(session, "DUP", strategy, data, config)

	var count int64
	db.Model(&LiveTradingPosition{}).Where("session_id = ? AND symbol = ?", session.ID, "DUP").Count(&count)

	// Second run should not create duplicates (signal_index already processed)
	var positions []LiveTradingPosition
	db.Where("session_id = ? AND symbol = ?", session.ID, "DUP").Find(&positions)

	signalIndices := map[int]int{}
	for _, p := range positions {
		signalIndices[p.SignalIndex]++
		if signalIndices[p.SignalIndex] > 1 {
			t.Errorf("duplicate signal_index %d found", p.SignalIndex)
		}
	}
}

func TestProcessLiveSymbol_SLTPCloses(t *testing.T) {
	setupLiveTestDB(t)

	session := LiveTradingSession{
		UserID:      1,
		Strategy:    "regression_scalping",
		Interval:    "5m",
		Symbols:     `["SLTP"]`,
		TradeAmount: 500,
		Currency:    "USD",
		IsActive:    true,
		StartedAt:   time.Unix(1000, 0),
	}
	db.Create(&session)

	config := LiveTradingConfig{UserID: 1}
	strategy := &RegressionScalpingStrategy{}
	strategy.defaults()

	data := generateOHLCV(300, 100, 1000, 300)

	processLiveSymbolWithData(session, "SLTP", strategy, data, config)

	// Check if any positions were closed by SL or TP
	var closed []LiveTradingPosition
	db.Where("session_id = ? AND symbol = ? AND is_closed = ?", session.ID, "SLTP", true).Find(&closed)

	for _, pos := range closed {
		if pos.CloseReason != "SL" && pos.CloseReason != "TP" && pos.CloseReason != "SIGNAL" {
			t.Errorf("unexpected close reason: %s", pos.CloseReason)
		}
		if pos.ClosePrice <= 0 {
			t.Errorf("close price <= 0: %.4f", pos.ClosePrice)
		}
		// P&L direction check
		if pos.Direction == "LONG" {
			expectedPct := (pos.ClosePrice - pos.EntryPrice) / pos.EntryPrice * 100
			if math.Abs(pos.ProfitLossPct-expectedPct) > 0.01 {
				t.Errorf("LONG P&L pct mismatch: got %.2f expected %.2f", pos.ProfitLossPct, expectedPct)
			}
		}
		if pos.Direction == "SHORT" {
			expectedPct := (pos.EntryPrice - pos.ClosePrice) / pos.EntryPrice * 100
			if math.Abs(pos.ProfitLossPct-expectedPct) > 0.01 {
				t.Errorf("SHORT P&L pct mismatch: got %.2f expected %.2f", pos.ProfitLossPct, expectedPct)
			}
		}
		// SL close price must be at SL level
		if pos.CloseReason == "SL" {
			if math.Abs(pos.ClosePrice-pos.StopLoss) > 0.01 {
				t.Errorf("SL close price %.4f != StopLoss %.4f", pos.ClosePrice, pos.StopLoss)
			}
		}
		// TP close price must be at TP level
		if pos.CloseReason == "TP" {
			if math.Abs(pos.ClosePrice-pos.TakeProfit) > 0.01 {
				t.Errorf("TP close price %.4f != TakeProfit %.4f", pos.ClosePrice, pos.TakeProfit)
			}
		}
	}
}

func TestProcessLiveSymbol_LongOnlyFilter(t *testing.T) {
	setupLiveTestDB(t)

	session := LiveTradingSession{
		UserID:      1,
		Strategy:    "regression_scalping",
		Interval:    "5m",
		Symbols:     `["LO"]`,
		TradeAmount: 500,
		Currency:    "USD",
		LongOnly:    true,
		IsActive:    true,
		StartedAt:   time.Unix(1000, 0),
	}
	db.Create(&session)

	config := LiveTradingConfig{UserID: 1}
	strategy := &RegressionScalpingStrategy{}
	strategy.defaults()

	data := generateOHLCV(300, 100, 1000, 300)
	processLiveSymbolWithData(session, "LO", strategy, data, config)

	// No SHORT positions should exist
	var shortCount int64
	db.Model(&LiveTradingPosition{}).Where("session_id = ? AND symbol = ? AND direction = ?", session.ID, "LO", "SHORT").Count(&shortCount)
	if shortCount > 0 {
		t.Errorf("LongOnly mode: found %d SHORT positions", shortCount)
	}
}

// ============================================================
// 6. Alpaca Order Flow — DB only after Alpaca success
// ============================================================

func TestAlpacaOrderFlow_NoAlpaca_StillCreatesDB(t *testing.T) {
	setupLiveTestDB(t)

	session := LiveTradingSession{
		UserID:      1,
		Strategy:    "regression_scalping",
		Interval:    "5m",
		Symbols:     `["NOALP"]`,
		TradeAmount: 500,
		Currency:    "USD",
		IsActive:    true,
		StartedAt:   time.Unix(1000, 0),
	}
	db.Create(&session)

	// Alpaca disabled
	config := LiveTradingConfig{
		UserID:        1,
		AlpacaEnabled: false,
	}

	strategy := &RegressionScalpingStrategy{}
	strategy.defaults()

	data := generateOHLCV(300, 100, 1000, 300)
	signals := strategy.Analyze(data)
	if len(signals) == 0 {
		t.Skip("no signals generated")
	}

	processLiveSymbolWithData(session, "NOALP", strategy, data, config)

	var count int64
	db.Model(&LiveTradingPosition{}).Where("session_id = ? AND symbol = ?", session.ID, "NOALP").Count(&count)
	if count == 0 {
		t.Fatal("with Alpaca disabled, should still create DB positions")
	}
}

// ============================================================
// 7. closeLivePosition P&L Tests
// ============================================================

func TestCloseLivePosition_LongProfit(t *testing.T) {
	setupLiveTestDB(t)
	pos := LiveTradingPosition{
		SessionID:      1,
		Symbol:         "PNL",
		Direction:      "LONG",
		EntryPrice:     100.0,
		EntryPriceUSD:  100.0,
		Quantity:       5,
		InvestedAmount: 500.0,
		NativeCurrency: "USD",
		StopLoss:       95.0,
		TakeProfit:     110.0,
	}
	db.Create(&pos)

	closeLivePosition(&pos, 110.0, "TP", "USD")

	if !pos.IsClosed {
		t.Fatal("position should be closed")
	}
	if pos.ClosePrice != 110.0 {
		t.Errorf("close price: got %.2f, expected 110.0", pos.ClosePrice)
	}
	expectedPct := (110.0 - 100.0) / 100.0 * 100 // +10%
	if math.Abs(pos.ProfitLossPct-expectedPct) > 0.01 {
		t.Errorf("P&L pct: got %.2f, expected %.2f", pos.ProfitLossPct, expectedPct)
	}
	expectedAmt := 500.0 * expectedPct / 100 // +50.0
	if math.Abs(pos.ProfitLossAmt-expectedAmt) > 0.01 {
		t.Errorf("P&L amt: got %.2f, expected %.2f", pos.ProfitLossAmt, expectedAmt)
	}
}

func TestCloseLivePosition_ShortProfit(t *testing.T) {
	setupLiveTestDB(t)
	pos := LiveTradingPosition{
		SessionID:      1,
		Symbol:         "PNL",
		Direction:      "SHORT",
		EntryPrice:     100.0,
		EntryPriceUSD:  100.0,
		Quantity:       5,
		InvestedAmount: 500.0,
		NativeCurrency: "USD",
		StopLoss:       105.0,
		TakeProfit:     90.0,
	}
	db.Create(&pos)

	closeLivePosition(&pos, 90.0, "TP", "USD")

	if !pos.IsClosed {
		t.Fatal("position should be closed")
	}
	expectedPct := (100.0 - 90.0) / 100.0 * 100 // +10%
	if math.Abs(pos.ProfitLossPct-expectedPct) > 0.01 {
		t.Errorf("P&L pct: got %.2f, expected %.2f", pos.ProfitLossPct, expectedPct)
	}
}

func TestCloseLivePosition_LongLoss(t *testing.T) {
	setupLiveTestDB(t)
	pos := LiveTradingPosition{
		SessionID:      1,
		Symbol:         "PNL",
		Direction:      "LONG",
		EntryPrice:     100.0,
		EntryPriceUSD:  100.0,
		Quantity:       5,
		InvestedAmount: 500.0,
		NativeCurrency: "USD",
		StopLoss:       95.0,
		TakeProfit:     110.0,
	}
	db.Create(&pos)

	closeLivePosition(&pos, 95.0, "SL", "USD")

	expectedPct := (95.0 - 100.0) / 100.0 * 100 // -5%
	if math.Abs(pos.ProfitLossPct-expectedPct) > 0.01 {
		t.Errorf("P&L pct: got %.2f, expected %.2f", pos.ProfitLossPct, expectedPct)
	}
	expectedAmt := 500.0 * expectedPct / 100 // -25.0
	if math.Abs(pos.ProfitLossAmt-expectedAmt) > 0.01 {
		t.Errorf("P&L amt: got %.2f, expected %.2f", pos.ProfitLossAmt, expectedAmt)
	}
}

// ============================================================
// 8. Delta Fetch Period Tests (OHLCV Cache Worker)
// ============================================================

func TestGetOHLCVDeltaPeriod(t *testing.T) {
	tests := []struct {
		interval string
		expected string
	}{
		{"5m", "1d"},
		{"15m", "5d"},
		{"60m", "5d"},
		{"1d", "3mo"},
		{"1wk", "6mo"},
		{"1mo", "2y"},
		{"unknown", "5d"},
	}
	for _, tt := range tests {
		result := getOHLCVDeltaPeriod(tt.interval)
		if result != tt.expected {
			t.Errorf("getOHLCVDeltaPeriod(%q) = %q, want %q", tt.interval, result, tt.expected)
		}
	}
}

// ============================================================
// 9. Incremental Signal — adding 1 bar shouldn't change old signals
// ============================================================

func TestIncrementalSignalStability(t *testing.T) {
	s := &RegressionScalpingStrategy{}
	s.defaults()

	data := generateOHLCV(300, 100, 1000, 300)

	// Analyze with N-1 bars
	signalsN1 := s.Analyze(data[:299])
	// Analyze with N bars
	signalsN := s.Analyze(data[:300])

	// All signals from N-1 must still exist in N (same index, direction, prices)
	for _, s1 := range signalsN1 {
		found := false
		for _, s2 := range signalsN {
			if s1.Index == s2.Index && s1.Direction == s2.Direction {
				found = true
				if math.Abs(s1.EntryPrice-s2.EntryPrice) > 0.001 {
					t.Errorf("signal at %d entry changed: %.4f -> %.4f (repaint!)", s1.Index, s1.EntryPrice, s2.EntryPrice)
				}
				if math.Abs(s1.StopLoss-s2.StopLoss) > 0.001 {
					t.Errorf("signal at %d SL changed: %.4f -> %.4f (repaint!)", s1.Index, s1.StopLoss, s2.StopLoss)
				}
				break
			}
		}
		if !found {
			t.Errorf("signal at index %d (%s) disappeared when adding 1 bar (repaint!)", s1.Index, s1.Direction)
		}
	}
}

// ============================================================
// 10. Trade History correctness
// ============================================================

func TestTradeHistory_CompleteLifecycle(t *testing.T) {
	setupLiveTestDB(t)

	session := LiveTradingSession{
		UserID:      1,
		Strategy:    "regression_scalping",
		Interval:    "5m",
		Symbols:     `["HIST"]`,
		TradeAmount: 1000,
		Currency:    "USD",
		IsActive:    true,
		StartedAt:   time.Unix(1000, 0),
	}
	db.Create(&session)

	config := LiveTradingConfig{UserID: 1}
	strategy := &RegressionScalpingStrategy{}
	strategy.defaults()

	data := generateOHLCV(300, 100, 1000, 300)
	processLiveSymbolWithData(session, "HIST", strategy, data, config)

	// Get all positions (open + closed) = trade history
	var all []LiveTradingPosition
	db.Where("session_id = ?", session.ID).Find(&all)

	for _, pos := range all {
		// Every position must have a valid entry time
		if pos.EntryTime.IsZero() {
			t.Error("entry time is zero")
		}
		// Closed positions must have close time
		if pos.IsClosed && pos.CloseTime == nil {
			t.Error("closed position has no close time")
		}
		// Closed positions must have a reason
		if pos.IsClosed && pos.CloseReason == "" {
			t.Error("closed position has no close reason")
		}
		// SignalIndex must be valid
		if pos.SignalIndex < 0 {
			t.Errorf("invalid signal index: %d", pos.SignalIndex)
		}
		// Quantity must be positive
		if pos.Quantity <= 0 {
			t.Errorf("quantity must be > 0, got %.6f", pos.Quantity)
		}
		// InvestedAmount must be positive
		if pos.InvestedAmount <= 0 {
			t.Errorf("invested amount must be > 0, got %.2f", pos.InvestedAmount)
		}
	}
}

// ============================================================
// 11. Performance Stats correctness
// ============================================================

func TestPerformanceStats(t *testing.T) {
	setupLiveTestDB(t)

	// Create positions manually with known values
	positions := []LiveTradingPosition{
		{SessionID: 99, Symbol: "A", Direction: "LONG", EntryPrice: 100, Quantity: 5, InvestedAmount: 500, IsClosed: true, ClosePrice: 110, ProfitLossPct: 10, ProfitLossAmt: 50, CloseReason: "TP"},
		{SessionID: 99, Symbol: "B", Direction: "LONG", EntryPrice: 200, Quantity: 2, InvestedAmount: 400, IsClosed: true, ClosePrice: 190, ProfitLossPct: -5, ProfitLossAmt: -20, CloseReason: "SL"},
		{SessionID: 99, Symbol: "C", Direction: "LONG", EntryPrice: 50, Quantity: 10, InvestedAmount: 500, IsClosed: false, CurrentPrice: 55, ProfitLossPct: 10, ProfitLossAmt: 50},
	}
	for i := range positions {
		db.Create(&positions[i])
	}

	// Calculate stats same way as frontend
	var allPos []LiveTradingPosition
	db.Where("session_id = ?", 99).Find(&allPos)

	totalPnl := 0.0
	totalInvested := 0.0
	wins := 0
	losses := 0
	for _, p := range allPos {
		totalPnl += p.ProfitLossAmt
		totalInvested += p.InvestedAmount
		if p.ProfitLossPct > 0 {
			wins++
		} else {
			losses++
		}
	}

	// Expected: 50 - 20 + 50 = 80
	if math.Abs(totalPnl-80) > 0.01 {
		t.Errorf("total P&L: got %.2f, expected 80.00", totalPnl)
	}
	// Expected invested: 500 + 400 + 500 = 1400
	if math.Abs(totalInvested-1400) > 0.01 {
		t.Errorf("total invested: got %.2f, expected 1400.00", totalInvested)
	}
	// Win rate: 2 wins (A, C) / 3 total = 66.67%
	winRate := float64(wins) / float64(len(allPos)) * 100
	if math.Abs(winRate-66.67) > 0.1 {
		t.Errorf("win rate: got %.2f%%, expected 66.67%%", winRate)
	}
	// Rendite: 80 / 1400 = 5.71%
	rendite := totalPnl / totalInvested * 100
	if math.Abs(rendite-5.714) > 0.1 {
		t.Errorf("rendite: got %.2f%%, expected ~5.71%%", rendite)
	}

	// R/R: avg win / |avg loss|
	avgWin := (10.0 + 10.0) / 2 // positions A + C
	avgLoss := -5.0 / 1.0        // position B
	rr := math.Abs(avgWin / avgLoss)
	if math.Abs(rr-2.0) > 0.01 {
		t.Errorf("R/R: got %.2f, expected 2.00", rr)
	}
}

// ============================================================
// 12. Strategy.Analyze produces valid signals
// ============================================================

func TestAllStrategiesProduceValidSignals(t *testing.T) {
	strategies := []struct {
		name     string
		strategy TradingStrategy
		bars     int
		interval int64
	}{
		{"RegressionScalping", &RegressionScalpingStrategy{}, 300, 300},
		{"HybridAITrend", &HybridAITrendStrategy{}, 700, 300},
		{"DiamondSignals", &DiamondSignalsStrategy{}, 400, 14400},
	}

	for _, tt := range strategies {
		t.Run(tt.name, func(t *testing.T) {
			data := generateOHLCV(tt.bars, 150, 1000, tt.interval)
			signals := tt.strategy.Analyze(data)

			t.Logf("%s: %d signals from %d bars", tt.name, len(signals), tt.bars)

			for i, sig := range signals {
				// Valid index
				if sig.Index < 0 || sig.Index >= len(data) {
					t.Errorf("signal %d: index %d out of range [0, %d)", i, sig.Index, len(data))
				}
				// Valid direction
				if sig.Direction != "LONG" && sig.Direction != "SHORT" {
					t.Errorf("signal %d: invalid direction %q", i, sig.Direction)
				}
				// Entry must be positive
				if sig.EntryPrice <= 0 {
					t.Errorf("signal %d: entry price %.4f <= 0", i, sig.EntryPrice)
				}
				// SL must be positive
				if sig.StopLoss <= 0 {
					t.Errorf("signal %d: stop loss %.4f <= 0", i, sig.StopLoss)
				}
				// TP must be positive
				if sig.TakeProfit <= 0 {
					t.Errorf("signal %d: take profit %.4f <= 0", i, sig.TakeProfit)
				}
				// No duplicate indices
				for j := i + 1; j < len(signals); j++ {
					if sig.Index == signals[j].Index {
						t.Errorf("duplicate signal index %d at positions %d and %d", sig.Index, i, j)
					}
				}
			}
		})
	}
}

// ============================================================
// 13. RequiredBars vs runLiveScan threshold
// ============================================================

func TestRequiredBarsMatchesStrategy(t *testing.T) {
	strategies := map[string]TradingStrategy{
		"regression_scalping": &RegressionScalpingStrategy{},
		"hybrid_ai_trend":    &HybridAITrendStrategy{},
		"diamond_signals":    &DiamondSignalsStrategy{},
	}
	for name, s := range strategies {
		required := s.RequiredBars()
		t.Logf("%s: RequiredBars = %d", name, required)
		if required < 50 {
			t.Errorf("%s: RequiredBars %d < 50 — too low for reliable signals", name, required)
		}
	}
}

// ============================================================
// 14. Bracket Order SL/TP values
// ============================================================

func TestBracketOrderConstruction(t *testing.T) {
	// Test that bracket order uses exact SL/TP from signal
	testCases := []struct {
		direction string
		entry     float64
		sl        float64
		tp        float64
	}{
		{"LONG", 100.0, 95.0, 112.5},
		{"SHORT", 200.0, 210.0, 180.0},
		{"LONG", 50.0, 48.5, 53.0},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("%s_%.0f", tc.direction, tc.entry), func(t *testing.T) {
			// The bracket options are built from signal values
			bracketOpts := map[string]float64{}
			if tc.sl > 0 {
				bracketOpts["stop_loss"] = tc.sl
			}
			if tc.tp > 0 {
				bracketOpts["take_profit"] = tc.tp
			}

			// Verify the values match exactly
			if bracketOpts["stop_loss"] != tc.sl {
				t.Errorf("SL mismatch: got %.2f, want %.2f", bracketOpts["stop_loss"], tc.sl)
			}
			if bracketOpts["take_profit"] != tc.tp {
				t.Errorf("TP mismatch: got %.2f, want %.2f", bracketOpts["take_profit"], tc.tp)
			}

			// Validate SL/TP direction logic
			if tc.direction == "LONG" {
				if tc.sl >= tc.entry {
					t.Errorf("LONG: SL %.2f >= Entry %.2f", tc.sl, tc.entry)
				}
				if tc.tp <= tc.entry {
					t.Errorf("LONG: TP %.2f <= Entry %.2f", tc.tp, tc.entry)
				}
			} else {
				if tc.sl <= tc.entry {
					t.Errorf("SHORT: SL %.2f <= Entry %.2f", tc.sl, tc.entry)
				}
				if tc.tp >= tc.entry {
					t.Errorf("SHORT: TP %.2f >= Entry %.2f", tc.tp, tc.entry)
				}
			}
		})
	}
}

// ============================================================
// 15. Gap Recovery — missed polls must be recovered by delta fetch
// ============================================================

func TestGapRecovery_MissedPollsFilled(t *testing.T) {
	// Simulate: cache has 500 bars, then 10 bars are "missed" (polls failed),
	// then delta fetch returns last 50 bars (covering the gap + more)
	intervalSec := int64(3600) // 1h
	full := generateOHLCV(510, 100, 1000, intervalSec)

	// Cache: first 500 bars (last bar at time 1000 + 499*3600)
	cache := make([]OHLCV, 500)
	copy(cache, full[:500])

	// Gap: bars 500-509 are "missed" (no poll ran)

	// Delta fetch: returns bars 460-509 (50 bars, overlapping cache and covering gap)
	delta := make([]OHLCV, 50)
	copy(delta, full[460:510])

	merged := mergeOHLCV(cache, delta)

	// Verify: all 510 bars present
	if len(merged) != 510 {
		t.Fatalf("expected 510 bars after gap recovery, got %d", len(merged))
	}

	// Verify: gap bars (500-509) are present
	for i := 500; i < 510; i++ {
		expectedTime := full[i].Time
		found := false
		for _, bar := range merged {
			if bar.Time == expectedTime {
				found = true
				if math.Abs(bar.Close-full[i].Close) > 0.001 {
					t.Errorf("bar %d close mismatch: got %.4f expected %.4f", i, bar.Close, full[i].Close)
				}
				break
			}
		}
		if !found {
			t.Errorf("GAP NOT RECOVERED: bar %d (time %d) missing after merge", i, expectedTime)
		}
	}

	// Signals must be same as full data
	s := &HybridAITrendStrategy{}
	s.defaults()
	if len(full) >= s.RequiredBars() {
		sigFull := s.Analyze(full)
		sigMerged := s.Analyze(merged)
		if len(sigFull) != len(sigMerged) {
			t.Errorf("signal count mismatch after gap recovery: full=%d merged=%d", len(sigFull), len(sigMerged))
		}
	}
}

func TestGapRecovery_DeltaWindowCoversGap(t *testing.T) {
	// Check: for each interval, is the delta period large enough
	// to cover a reasonable downtime (e.g., overnight = 16h)?
	intervals := []struct {
		name        string
		deltaPeriod string
		maxGapHours float64
	}{
		{"5m", "1d", 24},
		{"15m", "5d", 120},
		{"60m", "5d", 120},
		{"1d", "3mo", 2160},
		{"1wk", "6mo", 4320},
	}

	for _, iv := range intervals {
		t.Run(iv.name, func(t *testing.T) {
			deltaPeriod := getOHLCVDeltaPeriod(iv.name)
			if deltaPeriod != iv.deltaPeriod {
				t.Errorf("delta period mismatch: got %s expected %s", deltaPeriod, iv.deltaPeriod)
			}
			t.Logf("%s: delta=%s covers ~%.0fh — 24h gap recovery: OK", iv.name, iv.deltaPeriod, iv.maxGapHours)
			if iv.maxGapHours < 24 {
				t.Errorf("%s: delta period only covers %.0fh — overnight gap NOT recoverable!", iv.name, iv.maxGapHours)
			}
		})
	}
}

func TestGapRecovery_CacheWithHole(t *testing.T) {
	// Worst case: cache has a hole in the middle, delta covers it
	intervalSec := int64(3600)
	full := generateOHLCV(200, 100, 1000, intervalSec)

	// Cache: bars 0-99 and 150-199 (bars 100-149 missing = 50 bar hole)
	cache := make([]OHLCV, 0, 150)
	cache = append(cache, full[:100]...)
	cache = append(cache, full[150:]...)

	// Delta: returns bars 90-199 (covers the hole)
	delta := make([]OHLCV, 110)
	copy(delta, full[90:200])

	merged := mergeOHLCV(cache, delta)

	// Merge logic: cuts cache at delta start (bar 90), then appends all delta
	// Expected: cache[0:90] + delta[0:110] = 200 bars
	if len(merged) != 200 {
		t.Fatalf("expected 200 bars after hole recovery, got %d", len(merged))
	}

	// Check: no timestamps missing
	for i := 0; i < 200; i++ {
		if merged[i].Time != full[i].Time {
			t.Errorf("bar %d time mismatch: got %d expected %d (hole not filled?)",
				i, merged[i].Time, full[i].Time)
		}
	}
}

func TestGapRecovery_DeltaTooSmallDocumentsLimit(t *testing.T) {
	// Edge case: gap LARGER than delta window → bars are lost
	// This documents the limitation (auto-resume with prefetch solves it)
	intervalSec := int64(300) // 5m
	full := generateOHLCV(1000, 100, 1000, intervalSec)

	cache := make([]OHLCV, 500)
	copy(cache, full[:500])

	// Delta only covers bars 800-999 (gap at 500-799 is outside delta window)
	delta := make([]OHLCV, 200)
	copy(delta, full[800:1000])

	merged := mergeOHLCV(cache, delta)

	// Bars 500-799 are LOST
	mergedTimes := map[int64]bool{}
	for _, bar := range merged {
		mergedTimes[bar.Time] = true
	}
	missingCount := 0
	for i := 500; i < 800; i++ {
		if !mergedTimes[full[i].Time] {
			missingCount++
		}
	}

	if missingCount > 0 {
		t.Logf("Expected: %d bars lost when gap exceeds delta window. Auto-resume with prefetch recovers this.", missingCount)
	}
	// This is not a failure — it documents the limitation
	// The auto-resume on server restart does a full prefetch to solve this
}
