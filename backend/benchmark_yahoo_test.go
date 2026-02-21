package main

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestBenchmark_YahooFetchSingle(t *testing.T) {
	symbols := []string{"AAPL", "MSFT", "NVDA", "TSLA", "AMZN"}

	for _, sym := range symbols {
		start := time.Now()
		ohlcv, err := fetchOHLCVFromYahoo(sym, "2y", "60m")
		elapsed := time.Since(start)
		if err != nil {
			t.Logf("  %s: FEHLER nach %v — %v", sym, elapsed, err)
		} else {
			t.Logf("  %s: %d Bars in %v", sym, len(ohlcv), elapsed)
		}
	}
}

func TestBenchmark_YahooCrumbInit(t *testing.T) {
	// Crumb zurücksetzen
	resetYahooCrumb()

	start := time.Now()
	client, crumb, err := getYahooCrumbClient()
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("Crumb-Init: FEHLER nach %v — %v", elapsed, err)
	}
	t.Logf("Crumb-Init: %v (crumb=%s, client=%v)", elapsed, crumb[:min(10, len(crumb))], client != nil)

	// Zweiter Aufruf sollte sofort sein (cached)
	start2 := time.Now()
	_, _, _ = getYahooCrumbClient()
	t.Logf("Crumb-Cached: %v", time.Since(start2))
}

func TestBenchmark_Yahoo20Concurrent(t *testing.T) {
	symbols := []string{
		"AAPL", "MSFT", "NVDA", "TSLA", "AMZN",
		"GOOG", "META", "NFLX", "AMD", "INTC",
		"JPM", "V", "MA", "DIS", "PYPL",
		"CRM", "ADBE", "ORCL", "CSCO", "IBM",
	}

	sem := make(chan struct{}, 20)
	var wg sync.WaitGroup
	var success, failed int64

	start := time.Now()
	for _, sym := range symbols {
		wg.Add(1)
		go func(s string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			sStart := time.Now()
			ohlcv, err := fetchOHLCVFromYahoo(s, "2y", "60m")
			dur := time.Since(sStart)
			if err != nil {
				atomic.AddInt64(&failed, 1)
				t.Logf("  %s: FEHLER nach %v — %v", s, dur, err)
			} else {
				atomic.AddInt64(&success, 1)
				t.Logf("  %s: %d Bars in %v", s, len(ohlcv), dur)
			}
		}(sym)
	}
	wg.Wait()
	total := time.Since(start)
	t.Logf("20 Symbole concurrent: %v (OK: %d, Fehler: %d, avg: %v/Symbol)",
		total, success, failed, total/time.Duration(len(symbols)))
}

func TestBenchmark_BacktestComputation(t *testing.T) {
	// OHLCV einmal laden
	ohlcv, err := fetchOHLCVFromYahoo("AAPL", "2y", "60m")
	if err != nil {
		t.Fatalf("OHLCV laden: %v", err)
	}
	// 4h aggregieren
	ohlcv4h := aggregateOHLCV(ohlcv, 4)
	t.Logf("AAPL: %d 1h-Bars → %d 4h-Bars", len(ohlcv), len(ohlcv4h))

	strategies := []struct {
		name   string
		engine TradingStrategy
	}{
		{"smart_money_flow", createStrategyFromJSON("smart_money_flow", "")},
		{"hann_trend", createStrategyFromJSON("hann_trend", "")},
	}

	for _, strat := range strategies {
		// Analyze
		start := time.Now()
		signals := strat.engine.Analyze(ohlcv4h)
		analyzeTime := time.Since(start)

		// Backtest
		start = time.Now()
		result := runArenaBacktest(ohlcv4h, strat.engine)
		backtestTime := time.Since(start)

		t.Logf("  %s: Analyze=%v (%d Signale), Backtest=%v (WinRate=%.1f%%, Trades=%d)",
			strat.name, analyzeTime, len(signals), backtestTime,
			result.Metrics.WinRate, len(result.Trades))
	}

	// Bulk-Berechnung: 100x Backtest
	smf := createStrategyFromJSON("smart_money_flow", "")
	start := time.Now()
	for i := 0; i < 100; i++ {
		runArenaBacktest(ohlcv4h, smf)
	}
	bulk := time.Since(start)
	t.Logf("100x SMF Backtest: %v (avg: %v/Aktie)", bulk, bulk/100)
	t.Logf("Hochrechnung 2213 Aktien (50 Workers): ~%.1fs",
		float64(bulk/100)*2213/50/float64(time.Second))
}

func TestBenchmark_BottleneckAnalysis(t *testing.T) {
	t.Log("=== BOTTLENECK ANALYSE ===")
	t.Log("")

	// 1. Crumb Init
	resetYahooCrumb()
	start := time.Now()
	_, _, err := getYahooCrumbClient()
	crumbTime := time.Since(start)
	if err != nil {
		t.Logf("1. Crumb Init: FEHLER — %v", err)
		t.Log("   → PROBLEM: Ohne Crumb funktioniert kein Yahoo-Fetch!")
		return
	}
	t.Logf("1. Crumb Init: %v", crumbTime)

	// 2. Single Fetch
	start = time.Now()
	ohlcv, err := fetchOHLCVFromYahoo("AAPL", "2y", "60m")
	singleFetch := time.Since(start)
	if err != nil {
		t.Logf("2. Single Fetch: FEHLER — %v", err)
		return
	}
	t.Logf("2. Single Fetch (AAPL): %v (%d Bars)", singleFetch, len(ohlcv))

	// 3. 5 concurrent fetches
	syms5 := []string{"MSFT", "NVDA", "TSLA", "AMZN", "GOOG"}
	var wg sync.WaitGroup
	start = time.Now()
	for _, s := range syms5 {
		wg.Add(1)
		go func(sym string) {
			defer wg.Done()
			fetchOHLCVFromYahoo(sym, "2y", "60m")
		}(s)
	}
	wg.Wait()
	concurrent5 := time.Since(start)
	t.Logf("3. 5x Concurrent Fetch: %v (avg: %v)", concurrent5, concurrent5/5)

	// 4. Backtest
	ohlcv4h := aggregateOHLCV(ohlcv, 4)
	smf := createStrategyFromJSON("smart_money_flow", "")
	start = time.Now()
	runArenaBacktest(ohlcv4h, smf)
	btTime := time.Since(start)
	t.Logf("4. Backtest (SMF, 4h): %v", btTime)

	// 5. Hochrechnung
	fetchPer := singleFetch
	if concurrent5/5 < fetchPer {
		fetchPer = concurrent5 / 5
	}
	t.Log("")
	t.Log("=== HOCHRECHNUNG 2213 AKTIEN ===")
	t.Logf("Yahoo Fetch: %v/Aktie × 2213 / 20 Worker = ~%.0fs",
		fetchPer, float64(fetchPer)*2213/20/float64(time.Second))
	t.Logf("Backtest:    %v/Aktie × 2213 / 50 Worker = ~%.1fs",
		btTime, float64(btTime)*2213/50/float64(time.Second))
	totalEst := float64(fetchPer)*2213/20/float64(time.Second) + float64(btTime)*2213/50/float64(time.Second)
	t.Logf("GESAMT geschätzt: ~%.0fs (%.1f Minuten)", totalEst, totalEst/60)

	if crumbTime > 3*time.Second {
		t.Log("")
		t.Log("⚠ WARNUNG: Crumb-Init ist langsam — könnte 'Aktie 1/2213 hängt' erklären")
	}
	if singleFetch > 5*time.Second {
		t.Log("")
		t.Log("⚠ WARNUNG: Yahoo Fetch ist generell langsam — Netzwerk-Latenz?")
	}

	fmt.Println()
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
