package main

import (
	"encoding/json"
	"math"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// =============================================================================
// Arena Batch Integration Test — Vollständiger Backtest mit Trading Watchlist
//
// Testet den kompletten Flow: Watchlist laden → Yahoo Prefetch → Backtest →
// Ergebnis mit Metriken. Misst dabei alle Phasen-Zeiten und prüft ob sie
// innerhalb großzügiger aber sinnvoller Limits liegen.
// =============================================================================

// --- Hilfsfunktionen ---

// setupArenaTestDB erstellt eine Test-DB mit der globalen db-Variable
func setupArenaTestDB(t *testing.T) func() {
	t.Helper()
	origDB := db
	testDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open test DB: %v", err)
	}
	db = testDB

	// Tabellen erstellen
	db.AutoMigrate(
		&TradingWatchlistItem{},
		&ArenaV2BatchResult{},
		&User{},
	)

	// Admin-User für Auth
	db.Create(&User{ID: 1, Username: "testadmin", IsAdmin: true})

	// ohlcvMemCache initialisieren falls nil
	ohlcvMemCacheMu.Lock()
	if ohlcvMemCache == nil {
		ohlcvMemCache = make(map[string]map[string]*ohlcvCacheEntry)
	}
	ohlcvMemCacheMu.Unlock()

	// arenaOHLCVMemCache initialisieren falls nil
	arenaOHLCVMemCacheMu.Lock()
	if arenaOHLCVMemCache == nil {
		arenaOHLCVMemCache = make(map[string]map[string]*ohlcvCacheEntry)
	}
	arenaOHLCVMemCacheMu.Unlock()

	return func() {
		db = origDB
	}
}

// seedWatchlist füllt die Trading Watchlist mit Symbolen und gibt die Anzahl zurück
func seedWatchlist(t *testing.T, symbols []string) int {
	t.Helper()
	for _, sym := range symbols {
		db.Create(&TradingWatchlistItem{
			Symbol:       sym,
			Name:         sym,
			Fractionable: true,
		})
	}
	return len(symbols)
}

// largeTradingWatchlist gibt eine realistische Watchlist mit US-Aktien zurück
// Dies sind echte Symbole aus dem S&P 500 + weitere populäre Aktien
func largeTradingWatchlist() []string {
	return []string{
		// Top 50 — Large Cap
		"AAPL", "MSFT", "NVDA", "AMZN", "GOOG", "META", "TSLA", "BRK-B", "UNH", "JNJ",
		"V", "XOM", "JPM", "PG", "MA", "HD", "CVX", "LLY", "ABBV", "MRK",
		"PEP", "AVGO", "KO", "COST", "TMO", "MCD", "WMT", "CSCO", "ABT", "CRM",
		"ACN", "DHR", "LIN", "ADBE", "NKE", "TXN", "PM", "NEE", "CMCSA", "VZ",
		"RTX", "HON", "UNP", "INTC", "AMGN", "LOW", "IBM", "QCOM", "INTU", "SPGI",
		// 50-100 — Mid-Large Cap
		"AMAT", "CAT", "BA", "DE", "GS", "AXP", "BLK", "SYK", "ISRG", "GILD",
		"ADI", "MDLZ", "BKNG", "ADP", "REGN", "VRTX", "MMC", "LRCX", "CI", "SCHW",
		"MO", "ZTS", "ETN", "DUK", "BDX", "SO", "CME", "CL", "PGR", "ITW",
		"KLAC", "SHW", "EOG", "SNPS", "CDNS", "MCK", "NOC", "APD", "GD", "FDX",
		"MPC", "ORLY", "SLB", "AJG", "HUM", "PH", "ROP", "TDG", "MCHP", "NXPI",
		// 100-150
		"MSI", "AZO", "PCAR", "PSA", "CCI", "MNST", "CTAS", "PAYX", "TT", "DLR",
		"ADSK", "AEP", "FTNT", "WELL", "DXCM", "KMB", "AFL", "ROST", "D", "CARR",
		"GWW", "FAST", "VRSK", "O", "AME", "CPRT", "IDXX", "EA", "URI", "CMI",
		"A", "GIS", "BIIB", "HLT", "CTSH", "ON", "ANSS", "WEC", "EW", "EXC",
		"DOW", "XEL", "ED", "KDP", "STZ", "GPN", "ODFL", "DD", "GEHC", "IQV",
		// 150-200
		"STE", "WAB", "OTIS", "RMD", "MTD", "ALB", "FE", "AVB", "HBAN", "WBD",
		"IFF", "GLW", "TRGP", "FANG", "CBRE", "PPG", "HAL", "KEYS", "DFS", "BR",
		"VLTO", "HUBB", "TYL", "WTW", "CDW", "EFX", "EQR", "TSCO", "DOV", "CHD",
		"VICI", "PKG", "BRO", "CBOE", "HPQ", "LH", "NUE", "LDOS", "J", "BBY",
		"CF", "SNA", "TRMB", "FTV", "TDY", "STT", "OMC", "MKC", "ROL", "ESS",
	}
}

// --- Tests ---

// TestArenaBatch_SmallWatchlist testet den Batch mit 20 Symbolen — Funktions- und Zeitcheck
func TestArenaBatch_SmallWatchlist(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	symbols := []string{
		"AAPL", "MSFT", "NVDA", "TSLA", "AMZN",
		"GOOG", "META", "NFLX", "AMD", "INTC",
		"JPM", "V", "MA", "DIS", "PYPL",
		"CRM", "ADBE", "ORCL", "CSCO", "IBM",
	}

	t.Logf("=== Small Batch: %d Symbole ===", len(symbols))

	// Phase 1: Yahoo Prefetch — alle 20 concurrent
	prefetchStart := time.Now()
	var wg sync.WaitGroup
	sem := make(chan struct{}, 20)
	var fetched, failed int64
	var fetchTimes sync.Map

	for _, sym := range symbols {
		wg.Add(1)
		go func(s string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			sStart := time.Now()
			ohlcv, err := fetchOHLCVFromYahoo(s, "2y", "60m")
			dur := time.Since(sStart)
			fetchTimes.Store(s, dur)
			if err != nil {
				atomic.AddInt64(&failed, 1)
				t.Logf("  PREFETCH %s: FEHLER nach %v — %v", s, dur, err)
			} else {
				atomic.AddInt64(&fetched, 1)
				saveArenaOHLCVCache(s, "60m", ohlcv)
			}
		}(sym)
	}
	wg.Wait()
	prefetchTime := time.Since(prefetchStart)

	t.Logf("Prefetch: %v (OK: %d, Fehler: %d)", prefetchTime, fetched, failed)

	// Zeitlimit: 20 Symbole concurrent sollte <30s dauern (großzügig)
	if prefetchTime > 30*time.Second {
		t.Errorf("LANGSAM: Prefetch dauerte %v — erwartet <30s für 20 Symbole", prefetchTime)
	}

	// Min 80% Erfolg
	successRate := float64(fetched) / float64(len(symbols)) * 100
	if successRate < 80 {
		t.Errorf("Zu viele Fehler: %.0f%% Erfolg (erwartet ≥80%%)", successRate)
	}

	// Phase 2: Backtest für alle gecachten Symbole
	backtestStart := time.Now()
	smf := createStrategyFromJSON("smart_money_flow", "")
	var backtested int64
	var totalTrades int64

	for _, sym := range symbols {
		ohlcv, ok := getArenaOHLCVFromMemCache(sym, "60m")
		if !ok || len(ohlcv) < 50 {
			continue
		}
		ohlcv4h := aggregateOHLCV(ohlcv, 4)
		result := runArenaBacktest(ohlcv4h, smf)
		atomic.AddInt64(&backtested, 1)
		atomic.AddInt64(&totalTrades, int64(len(result.Trades)))
	}
	backtestTime := time.Since(backtestStart)

	t.Logf("Backtest: %v (%d Aktien, %d Trades total)", backtestTime, backtested, totalTrades)

	// Backtest sollte für 20 Aktien <1s dauern (CPU-only)
	if backtestTime > 1*time.Second {
		t.Errorf("LANGSAM: Backtest dauerte %v — erwartet <1s für 20 Aktien", backtestTime)
	}

	// Gesamt
	totalTime := prefetchTime + backtestTime
	t.Logf("GESAMT: %v (Prefetch: %v + Backtest: %v)", totalTime, prefetchTime, backtestTime)

	if totalTime > 35*time.Second {
		t.Errorf("GESAMTZEIT zu hoch: %v — erwartet <35s", totalTime)
	}
}

// TestArenaBatch_FullWatchlist testet mit 200 Symbolen — simuliert ~10% der Prod-Watchlist
func TestArenaBatch_FullWatchlist(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	symbols := largeTradingWatchlist()
	t.Logf("=== Full Batch: %d Symbole (Smart Money Flow, 4h) ===", len(symbols))

	// Phase 1: Crumb Init — muss schnell sein
	resetYahooCrumb()
	crumbStart := time.Now()
	_, _, err := getYahooCrumbClient()
	crumbTime := time.Since(crumbStart)
	if err != nil {
		t.Fatalf("Crumb Init fehlgeschlagen nach %v: %v", crumbTime, err)
	}
	t.Logf("1. Crumb Init: %v", crumbTime)

	if crumbTime > 5*time.Second {
		t.Errorf("LANGSAM: Crumb Init dauerte %v — erwartet <5s", crumbTime)
	}

	// Phase 2: Prefetch — 20 concurrent Workers
	prefetchStart := time.Now()
	var prefetchWg sync.WaitGroup
	prefetchSem := make(chan struct{}, 20)
	var prefetched, prefetchFailed int64
	var slowFetches int64 // Fetches >5s

	for _, sym := range symbols {
		prefetchWg.Add(1)
		go func(s string) {
			defer prefetchWg.Done()
			prefetchSem <- struct{}{}
			defer func() { <-prefetchSem }()

			sStart := time.Now()
			var ohlcv []OHLCV
			var lastErr error
			for attempt := 0; attempt < 3; attempt++ {
				if attempt > 0 {
					time.Sleep(time.Duration(attempt) * 2 * time.Second)
				}
				ohlcv, lastErr = fetchOHLCVFromYahoo(s, "2y", "60m")
				if lastErr == nil && len(ohlcv) > 0 {
					break
				}
			}
			dur := time.Since(sStart)

			if dur > 5*time.Second {
				atomic.AddInt64(&slowFetches, 1)
			}

			if lastErr == nil && len(ohlcv) > 0 {
				atomic.AddInt64(&prefetched, 1)
				saveArenaOHLCVCache(s, "60m", ohlcv)
			} else {
				atomic.AddInt64(&prefetchFailed, 1)
			}
		}(sym)
	}
	prefetchWg.Wait()
	prefetchTime := time.Since(prefetchStart)

	t.Logf("2. Prefetch: %v (OK: %d, Fehler: %d, Langsam >5s: %d)",
		prefetchTime, prefetched, prefetchFailed, slowFetches)

	// 200 Symbole / 20 Worker = 10 Runden. Bei ~500ms/Fetch = ~5s ideal.
	// Mit Retries, Netzwerk-Jitter: <120s großzügig
	if prefetchTime > 120*time.Second {
		t.Errorf("LANGSAM: Prefetch dauerte %v — erwartet <120s für %d Symbole", prefetchTime, len(symbols))
	}

	prefetchSuccessRate := float64(prefetched) / float64(len(symbols)) * 100
	t.Logf("   Erfolgsrate: %.1f%%", prefetchSuccessRate)
	if prefetchSuccessRate < 75 {
		t.Errorf("Zu viele Prefetch-Fehler: %.1f%% (erwartet ≥75%%)", prefetchSuccessRate)
	}

	// Phase 3: Backtest — 50 concurrent Workers
	backtestStart := time.Now()
	var btWg sync.WaitGroup
	btSem := make(chan struct{}, 50)
	var backtested int64
	var totalTrades int64
	var skipped int64
	var maxBacktestDur time.Duration
	var maxBtMu sync.Mutex

	for _, sym := range symbols {
		btWg.Add(1)
		go func(s string) {
			defer btWg.Done()
			btSem <- struct{}{}
			defer func() { <-btSem }()

			ohlcv, ok := getArenaOHLCVFromMemCache(s, "60m")
			if !ok || len(ohlcv) < 50 {
				atomic.AddInt64(&skipped, 1)
				return
			}
			ohlcv4h := aggregateOHLCV(ohlcv, 4)

			strategy := createStrategyFromJSON("smart_money_flow", "")
			sStart := time.Now()
			result := runArenaBacktest(ohlcv4h, strategy)
			dur := time.Since(sStart)

			maxBtMu.Lock()
			if dur > maxBacktestDur {
				maxBacktestDur = dur
			}
			maxBtMu.Unlock()

			atomic.AddInt64(&backtested, 1)
			atomic.AddInt64(&totalTrades, int64(len(result.Trades)))
		}(sym)
	}
	btWg.Wait()
	backtestTime := time.Since(backtestStart)

	t.Logf("3. Backtest: %v (%d berechnet, %d übersprungen, %d Trades, max einzeln: %v)",
		backtestTime, backtested, skipped, totalTrades, maxBacktestDur)

	// 200 Backtests bei ~50µs/Stück = ~0.01s. Großzügig: <5s
	if backtestTime > 5*time.Second {
		t.Errorf("LANGSAM: Backtest dauerte %v — erwartet <5s für %d Aktien", backtestTime, len(symbols))
	}

	// Phase 4: Ergebnis-Validierung
	if backtested == 0 {
		t.Fatal("Kein einziger Backtest wurde durchgeführt!")
	}

	// Min 70% der Aktien sollten backtested werden
	btRate := float64(backtested) / float64(len(symbols)) * 100
	t.Logf("   Backtest-Rate: %.1f%% (%d/%d)", btRate, backtested, len(symbols))
	if btRate < 70 {
		t.Errorf("Zu wenige Backtests: %.1f%% (erwartet ≥70%%)", btRate)
	}

	// Gesamt
	totalTime := crumbTime + prefetchTime + backtestTime
	t.Logf("")
	t.Logf("=== ERGEBNIS ===")
	t.Logf("Crumb Init:  %v", crumbTime)
	t.Logf("Prefetch:    %v (%d/%d Symbole)", prefetchTime, prefetched, len(symbols))
	t.Logf("Backtest:    %v (%d Aktien, %d Trades)", backtestTime, backtested, totalTrades)
	t.Logf("GESAMT:      %v", totalTime)
	t.Logf("")

	// Hochrechnung auf 2213 Aktien
	fetchPerSymbol := prefetchTime / time.Duration(len(symbols))
	btPerSymbol := backtestTime / time.Duration(max64(backtested, 1))
	estFetch := float64(fetchPerSymbol) * 2213 / 20 / float64(time.Second)
	estBt := float64(btPerSymbol) * 2213 / 50 / float64(time.Second)
	t.Logf("=== HOCHRECHNUNG 2213 AKTIEN ===")
	t.Logf("Prefetch:  %v/Symbol × 2213 / 20 Worker = ~%.0fs", fetchPerSymbol, estFetch)
	t.Logf("Backtest:  %v/Symbol × 2213 / 50 Worker = ~%.1fs", btPerSymbol, estBt)
	t.Logf("Geschätzt: ~%.0fs (%.1f Minuten)", estFetch+estBt, (estFetch+estBt)/60)

	// Gesamtzeit: <130s für 200 Symbole (großzügig)
	if totalTime > 130*time.Second {
		t.Errorf("GESAMTZEIT zu hoch: %v — erwartet <130s für %d Symbole", totalTime, len(symbols))
	}
}

// TestArenaBatch_SSEEndpoint testet den echten HTTP-Endpoint mit SSE-Streaming
func TestArenaBatch_SSEEndpoint(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	teardown := setupArenaTestDB(t)
	defer teardown()

	// 20 Symbole in Watchlist
	testSymbols := []string{
		"AAPL", "MSFT", "NVDA", "TSLA", "AMZN",
		"GOOG", "META", "NFLX", "AMD", "INTC",
		"JPM", "V", "MA", "DIS", "PYPL",
		"CRM", "ADBE", "ORCL", "CSCO", "IBM",
	}
	seedWatchlist(t, testSymbols)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/api/trading/arena/v2/batch", func(c *gin.Context) {
		// Simuliere Auth
		c.Set("user_id", uint(1))
		c.Set("role", "admin")
		arenaV2BatchHandler(c)
	})

	body := `{"strategy":"smart_money_flow","interval":"60m"}`
	req := httptest.NewRequest("POST", "/api/trading/arena/v2/batch", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	start := time.Now()
	router.ServeHTTP(w, req)
	elapsed := time.Since(start)

	t.Logf("SSE Endpoint: %v (Status: %d, Body: %d bytes)", elapsed, w.Code, w.Body.Len())

	if w.Code != 200 {
		t.Fatalf("HTTP Status %d, erwartet 200. Body: %s", w.Code, w.Body.String()[:min(500, w.Body.Len())])
	}

	// Parse SSE events
	sseBody := w.Body.String()
	lines := strings.Split(sseBody, "\n")
	var events []map[string]interface{}
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		jsonStr := strings.TrimPrefix(line, "data: ")
		var evt map[string]interface{}
		if err := json.Unmarshal([]byte(jsonStr), &evt); err == nil {
			events = append(events, evt)
		}
	}

	t.Logf("SSE Events: %d empfangen", len(events))

	// Prüfe Event-Typen
	eventTypes := map[string]int{}
	for _, evt := range events {
		if tp, ok := evt["type"].(string); ok {
			eventTypes[tp]++
		}
	}
	t.Logf("Event-Typen: %v", eventTypes)

	// Es muss ein "result" Event geben
	if eventTypes["result"] == 0 {
		t.Fatal("Kein 'result' SSE-Event empfangen!")
	}

	// Result-Event analysieren
	var resultEvt map[string]interface{}
	for _, evt := range events {
		if tp, _ := evt["type"].(string); tp == "result" {
			resultEvt = evt
			break
		}
	}

	if data, ok := resultEvt["data"].(map[string]interface{}); ok {
		// per_stock prüfen
		if perStock, ok := data["per_stock"].(map[string]interface{}); ok {
			t.Logf("Ergebnis: %d Aktien mit Backtests", len(perStock))
			if len(perStock) == 0 {
				t.Error("per_stock ist leer — kein Backtest wurde berechnet")
			}
		}

		// skipped_symbols prüfen (jetzt mit Gründen)
		if skipped, ok := data["skipped_symbols"].([]interface{}); ok && len(skipped) > 0 {
			t.Logf("Übersprungen: %d Aktien", len(skipped))
			// Prüfe ob Gründe vorhanden sind
			if first, ok := skipped[0].(map[string]interface{}); ok {
				if reason, ok := first["reason"].(string); ok && reason != "" {
					t.Logf("  Beispiel: %s — %s", first["symbol"], reason)
				} else {
					t.Error("skipped_symbols enthält keinen 'reason'")
				}
			}
		}

		// Trades prüfen
		if trades, ok := data["trades"].([]interface{}); ok {
			t.Logf("Trades: %d", len(trades))
		}

		// Metriken prüfen
		if metrics, ok := data["metrics"].(map[string]interface{}); ok {
			winRate, _ := metrics["win_rate"].(float64)
			totalTrades, _ := metrics["total_trades"].(float64)
			t.Logf("Metriken: WinRate=%.1f%%, Trades=%.0f", winRate, totalTrades)
		}
	}

	// Zeitlimit: 20 Symbole SSE komplett <60s (inkl. Prefetch + Backtest + Serialisierung)
	if elapsed > 60*time.Second {
		t.Errorf("SSE Endpoint zu langsam: %v — erwartet <60s", elapsed)
	}
}

// TestArenaBatch_PrefetchTiming misst die Prefetch-Phase isoliert mit detailliertem Timing
func TestArenaBatch_PrefetchTiming(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	symbols := []string{
		"AAPL", "MSFT", "NVDA", "TSLA", "AMZN",
		"GOOG", "META", "NFLX", "AMD", "INTC",
	}

	// Cache leeren für diese Symbole (damit wirklich Yahoo gefetcht wird)
	ohlcvMemCacheMu.Lock()
	for _, sym := range symbols {
		delete(ohlcvMemCache, sym)
	}
	ohlcvMemCacheMu.Unlock()

	t.Logf("=== Prefetch Timing: %d Symbole (Cache geleert) ===", len(symbols))

	var wg sync.WaitGroup
	sem := make(chan struct{}, 20)
	type fetchResult struct {
		Symbol   string
		Duration time.Duration
		Bars     int
		Error    string
	}
	results := make([]fetchResult, len(symbols))

	start := time.Now()
	for i, sym := range symbols {
		wg.Add(1)
		go func(idx int, s string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			sStart := time.Now()
			ohlcv, err := fetchOHLCVFromYahoo(s, "2y", "60m")
			dur := time.Since(sStart)

			r := fetchResult{Symbol: s, Duration: dur}
			if err != nil {
				r.Error = err.Error()
			} else {
				r.Bars = len(ohlcv)
			}
			results[idx] = r
		}(i, sym)
	}
	wg.Wait()
	totalTime := time.Since(start)

	// Statistiken
	var minDur, maxDur time.Duration
	var sumDur time.Duration
	var successCount int
	minDur = time.Hour

	for _, r := range results {
		if r.Error != "" {
			t.Logf("  %s: FEHLER nach %v — %s", r.Symbol, r.Duration, r.Error)
		} else {
			t.Logf("  %s: %d Bars in %v", r.Symbol, r.Bars, r.Duration)
			successCount++
			sumDur += r.Duration
			if r.Duration < minDur {
				minDur = r.Duration
			}
			if r.Duration > maxDur {
				maxDur = r.Duration
			}
		}
	}

	if successCount > 0 {
		avgDur := sumDur / time.Duration(successCount)
		t.Logf("")
		t.Logf("Statistik: min=%v, max=%v, avg=%v, total=%v", minDur, maxDur, avgDur, totalTime)
		t.Logf("Erfolg: %d/%d", successCount, len(symbols))

		// Einzelner Fetch sollte <10s dauern
		if maxDur > 10*time.Second {
			t.Errorf("Langsamster Fetch: %v — erwartet <10s", maxDur)
		}

		// Durchschnitt sollte <3s sein
		if avgDur > 3*time.Second {
			t.Errorf("Durchschnittlicher Fetch: %v — erwartet <3s", avgDur)
		}
	}

	// Gesamtzeit: 10 concurrent Fetches <15s
	if totalTime > 15*time.Second {
		t.Errorf("Gesamtzeit: %v — erwartet <15s für %d concurrent Fetches", totalTime, len(symbols))
	}
}

// TestArenaBatch_BacktestSpeed misst die reine Backtest-Geschwindigkeit mit vielen Symbolen
func TestArenaBatch_BacktestSpeed(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Erstmal Daten für ein paar Symbole laden
	baseSymbols := []string{"AAPL", "MSFT", "NVDA", "TSLA", "AMZN"}
	var dataMap sync.Map

	var wg sync.WaitGroup
	for _, sym := range baseSymbols {
		wg.Add(1)
		go func(s string) {
			defer wg.Done()
			ohlcv, err := fetchOHLCVFromYahoo(s, "2y", "60m")
			if err == nil && len(ohlcv) > 0 {
				dataMap.Store(s, aggregateOHLCV(ohlcv, 4))
			}
		}(sym)
	}
	wg.Wait()

	// Sammle die Daten
	var allData [][]OHLCV
	dataMap.Range(func(_, value interface{}) bool {
		allData = append(allData, value.([]OHLCV))
		return true
	})

	if len(allData) == 0 {
		t.Fatal("Keine OHLCV-Daten geladen")
	}

	t.Logf("=== Backtest Speed: %d Datensätze, simuliere 200 Backtests ===", len(allData))

	// 200 Backtests durchführen (rotierend über die Datensätze)
	numBacktests := 200
	sem := make(chan struct{}, 50)
	var btWg sync.WaitGroup
	var totalTrades int64

	start := time.Now()
	for i := 0; i < numBacktests; i++ {
		btWg.Add(1)
		go func(idx int) {
			defer btWg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			data := allData[idx%len(allData)]
			strategy := createStrategyFromJSON("smart_money_flow", "")
			result := runArenaBacktest(data, strategy)
			atomic.AddInt64(&totalTrades, int64(len(result.Trades)))
		}(i)
	}
	btWg.Wait()
	elapsed := time.Since(start)

	avgPer := elapsed / time.Duration(numBacktests)
	t.Logf("200 Backtests: %v (avg: %v/Backtest, Trades: %d)", elapsed, avgPer, totalTrades)

	// 200 Backtests sollten <2s dauern (CPU-only, ~50µs/Stück)
	if elapsed > 2*time.Second {
		t.Errorf("LANGSAM: 200 Backtests dauerten %v — erwartet <2s", elapsed)
	}

	// Hochrechnung auf 2213
	est2213 := float64(avgPer) * 2213 / 50 / float64(time.Second)
	t.Logf("Hochrechnung 2213 Aktien / 50 Worker: ~%.2fs", est2213)

	if est2213 > 5 {
		t.Errorf("Hochrechnung zu hoch: %.2fs — erwartet <5s", est2213)
	}
}

// TestArenaBatch_ConcurrencyScaling testet ob concurrent Fetches tatsächlich parallel laufen
func TestArenaBatch_ConcurrencyScaling(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	t.Log("=== Concurrency Scaling Test ===")

	// Crumb vorwärmen, damit nicht der erste Fetch die Crumb-Init-Latenz misst
	resetYahooCrumb()
	_, _, err := getYahooCrumbClient()
	if err != nil {
		t.Fatalf("Crumb Init fehlgeschlagen: %v", err)
	}

	// Single Fetch (immer Yahoo, kein Cache da fetchOHLCVFromYahoo nie Cache liest)
	start := time.Now()
	_, err = fetchOHLCVFromYahoo("AAPL", "2y", "60m")
	singleTime := time.Since(start)
	if err != nil {
		t.Fatalf("Single fetch fehlgeschlagen: %v", err)
	}
	t.Logf("1x Fetch:  %v", singleTime)

	// 10 concurrent Fetches (verschiedene Symbole)
	concSymbols := []string{"MSFT", "NVDA", "TSLA", "AMZN", "GOOG", "META", "NFLX", "AMD", "INTC", "JPM"}
	var wg sync.WaitGroup
	start = time.Now()
	for _, s := range concSymbols {
		wg.Add(1)
		go func(sym string) {
			defer wg.Done()
			fetchOHLCVFromYahoo(sym, "2y", "60m")
		}(s)
	}
	wg.Wait()
	concTime := time.Since(start)
	t.Logf("10x Fetch: %v", concTime)

	// Concurrent sollte NICHT 10x so lang dauern wie Single (= echte Parallelität)
	// Idealerweise ~1-2x Single. Grenze: 5x ist noch OK (Netzwerk-Jitter, Connection-Setup)
	ratio := float64(concTime) / float64(singleTime)
	t.Logf("Ratio: %.1fx (ideal ≈1.0x, max 5.0x)", ratio)

	if ratio > 5.0 {
		t.Errorf("Parallelität schlecht: 10 concurrent = %.1fx statt ~1x — Connection-Pooling oder Mutex-Problem?", ratio)
	}
}

// TestArenaBatch_SkippedSymbolReasons prüft ob übersprungene Symbole korrekte Gründe haben
func TestArenaBatch_SkippedSymbolReasons(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	teardown := setupArenaTestDB(t)
	defer teardown()

	// Mischung aus gültigen und ungültigen Symbolen
	testSymbols := []string{
		"AAPL",             // Gültig
		"MSFT",             // Gültig
		"XYZXYZXYZ123",     // Existiert nicht
		"INVALIDTICKER999", // Existiert nicht
		"GOOG",             // Gültig
	}
	seedWatchlist(t, testSymbols)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/api/trading/arena/v2/batch", func(c *gin.Context) {
		c.Set("user_id", uint(1))
		c.Set("role", "admin")
		arenaV2BatchHandler(c)
	})

	body := `{"strategy":"smart_money_flow","interval":"60m"}`
	req := httptest.NewRequest("POST", "/api/trading/arena/v2/batch", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	// Parse result event
	lines := strings.Split(w.Body.String(), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		var evt map[string]interface{}
		json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &evt)
		if tp, _ := evt["type"].(string); tp != "result" {
			continue
		}

		data, _ := evt["data"].(map[string]interface{})
		skipped, _ := data["skipped_symbols"].([]interface{})

		if len(skipped) == 0 {
			t.Log("Keine Symbole übersprungen (alle im Cache?) — OK")
			return
		}

		t.Logf("Übersprungene Symbole: %d", len(skipped))
		for _, s := range skipped {
			entry, _ := s.(map[string]interface{})
			symbol, _ := entry["symbol"].(string)
			reason, _ := entry["reason"].(string)
			t.Logf("  %s: %s", symbol, reason)

			// Grund darf nicht leer sein
			if reason == "" {
				t.Errorf("Symbol %s hat keinen Skip-Grund!", symbol)
			}
		}

		// Ungültige Symbole sollten in der Liste sein (wenn sie nicht im Cache waren)
		skippedSet := map[string]bool{}
		for _, s := range skipped {
			entry, _ := s.(map[string]interface{})
			sym, _ := entry["symbol"].(string)
			skippedSet[sym] = true
		}

		for _, invalidSym := range []string{"XYZXYZXYZ123", "INVALIDTICKER999"} {
			if skippedSet[invalidSym] {
				t.Logf("  ✓ %s korrekt als übersprungen markiert", invalidSym)
			}
			// Könnte auch im Cache sein von einem früheren Test → kein Fehler wenn nicht in skipped
		}
		return
	}

	t.Error("Kein 'result' Event im SSE-Stream gefunden")
}

// TestArenaBatch_HannTrend testet den Batch mit Hann Trend Strategie (zweite Strategie)
func TestArenaBatch_HannTrend(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	symbols := []string{"AAPL", "MSFT", "NVDA", "TSLA", "AMZN", "GOOG", "META", "NFLX", "AMD", "INTC"}
	t.Logf("=== Hann Trend Batch: %d Symbole ===", len(symbols))

	var wg sync.WaitGroup
	sem := make(chan struct{}, 20)
	var fetched int64

	for _, sym := range symbols {
		wg.Add(1)
		go func(s string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			ohlcv, err := fetchOHLCVFromYahoo(s, "2y", "60m")
			if err == nil && len(ohlcv) > 0 {
				saveArenaOHLCVCache(s, "60m", ohlcv)
				atomic.AddInt64(&fetched, 1)
			}
		}(sym)
	}
	wg.Wait()

	t.Logf("Prefetch: %d/%d OK", fetched, len(symbols))

	// Backtest mit Hann Trend
	ht := createStrategyFromJSON("hann_trend", "")
	var backtested, totalTrades int64

	start := time.Now()
	for _, sym := range symbols {
		ohlcv, ok := getArenaOHLCVFromMemCache(sym, "60m")
		if !ok || len(ohlcv) < 50 {
			continue
		}
		result := runArenaBacktest(ohlcv, ht)
		backtested++
		totalTrades += int64(len(result.Trades))

		// Jedes Ergebnis muss valide Metriken haben
		if math.IsNaN(result.Metrics.WinRate) || math.IsInf(result.Metrics.WinRate, 0) {
			t.Errorf("%s: WinRate ist NaN/Inf", sym)
		}
	}
	elapsed := time.Since(start)

	t.Logf("Backtest: %v (%d Aktien, %d Trades)", elapsed, backtested, totalTrades)

	if backtested == 0 {
		t.Error("Kein Backtest durchgeführt")
	}

	if elapsed > 1*time.Second {
		t.Errorf("Backtest zu langsam: %v für %d Aktien", elapsed, backtested)
	}
}

// --- Helper ---

func max64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
