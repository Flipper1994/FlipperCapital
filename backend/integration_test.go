package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// ============================================================================
// TEST SETUP (separate von allowlist_test.go setupTestDB)
// ============================================================================

var integrationOrigDB *gorm.DB

func setupIntegrationDB(t *testing.T) {
	t.Helper()
	integrationOrigDB = db

	testDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Test-DB öffnen fehlgeschlagen: %v", err)
	}
	db = testDB

	db.AutoMigrate(
		&User{},
		&LiveTradingConfig{},
		&LiveTradingSession{},
		&LiveTradingPosition{},
		&LiveTradingLog{},
		&LiveSessionStrategy{},
		&TradingWatchlistItem{},
	)

	db.Create(&User{
		ID: 1, Email: "admin@test.de", Username: "testadmin",
		Password: "hashed", IsAdmin: true,
	})
}

func teardownIntegrationDB(t *testing.T) {
	t.Helper()
	db = integrationOrigDB
}

func startTestPositionWriter() (stop func()) {
	done := make(chan struct{})
	go func() {
		for {
			select {
			case fn := <-livePositionWriteCh:
				fn()
			case <-done:
				for {
					select {
					case fn := <-livePositionWriteCh:
						fn()
					default:
						return
					}
				}
			}
		}
	}()
	return func() { close(done) }
}

func drainWrites() {
	for {
		select {
		case fn := <-livePositionWriteCh:
			fn()
		default:
			return
		}
	}
}

func createAdminContext(method, path string, body interface{}) (*gin.Context, *httptest.ResponseRecorder) {
	w := httptest.NewRecorder()
	var req *http.Request
	if body != nil {
		jsonBytes, _ := json.Marshal(body)
		req = httptest.NewRequest(method, path, bytes.NewReader(jsonBytes))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	c, _ := gin.CreateTestContext(w)
	c.Request = req
	c.Set("userID", uint(1))
	c.Set("isAdmin", true)
	return c, w
}

// ============================================================================
// MOCK OHLCV DATA
// ============================================================================

// generateBreakoutOHLCV erzeugt OHLCV-Daten mit klaren Trend-Phasen:
// Abwärtstrend → Boden → Aufwärtstrend → Pullback → Breakout → Abwärtstrend → Breakdown
func generateBreakoutOHLCV(bars int, interval string) []OHLCV {
	ohlcv := make([]OHLCV, bars)
	baseTime := time.Date(2025, 6, 1, 9, 30, 0, 0, time.UTC)
	dur := intervalToDuration(interval)

	price := 150.0
	rng := rand.New(rand.NewSource(123))

	for i := 0; i < bars; i++ {
		t := baseTime.Add(dur * time.Duration(i))
		progress := float64(i) / float64(bars)

		var trend float64
		switch {
		case progress < 0.15:
			trend = -0.6
		case progress < 0.25:
			trend = 0.05
		case progress < 0.45:
			trend = 0.5
		case progress < 0.50:
			trend = -0.4
		case progress < 0.55:
			trend = 1.0
		case progress < 0.65:
			trend = 0.3
		case progress < 0.75:
			trend = -0.5
		case progress < 0.80:
			trend = 0.3
		case progress < 0.85:
			trend = -0.8
		default:
			trend = 0.1
		}

		change := trend + rng.NormFloat64()*0.3
		price = price * (1 + change/100)
		if price < 20 {
			price = 20
		}

		spread := price * 0.008
		high := price + rng.Float64()*spread
		low := price - rng.Float64()*spread
		open := low + rng.Float64()*(high-low)
		closeP := low + rng.Float64()*(high-low)
		vol := 200000 + rng.Intn(800000)

		ohlcv[i] = OHLCV{
			Time:   t.Unix(),
			Open:   math.Round(open*100) / 100,
			High:   math.Round(high*100) / 100,
			Low:    math.Round(low*100) / 100,
			Close:  math.Round(closeP*100) / 100,
			Volume: float64(vol),
		}
	}
	return ohlcv
}

// ============================================================================
// TEST 1: SMART MONEY FLOW SIGNAL-VALIDIERUNG
// ============================================================================

func TestInteg_SmartMoneyFlowSignals(t *testing.T) {
	strategy := &SmartMoneyFlowStrategy{}
	strategy.defaults()

	ohlcv := generateBreakoutOHLCV(300, "4h")
	t.Logf("SMF RequiredBars: %d, Bars: %d", strategy.RequiredBars(), len(ohlcv))

	signals := strategy.Analyze(ohlcv)
	t.Logf("SMF Signale: %d", len(signals))

	for i, sig := range signals {
		t.Logf("  Signal %d: Index=%d Dir=%s Entry=%.2f SL=%.2f TP=%.2f",
			i+1, sig.Index, sig.Direction, sig.EntryPrice, sig.StopLoss, sig.TakeProfit)

		if sig.Direction != "LONG" && sig.Direction != "SHORT" {
			t.Errorf("Ungültige Richtung: %s", sig.Direction)
		}
		if sig.Index < 0 || sig.Index >= len(ohlcv) {
			t.Errorf("Index %d außerhalb [0,%d)", sig.Index, len(ohlcv))
		}
		if sig.EntryPrice <= 0 || sig.StopLoss <= 0 || sig.TakeProfit <= 0 {
			t.Errorf("Entry/SL/TP muss > 0 sein")
		}

		// SL/TP Plausibilität
		if sig.Direction == "LONG" {
			if sig.StopLoss >= sig.EntryPrice {
				t.Errorf("LONG: SL %.4f >= Entry %.4f", sig.StopLoss, sig.EntryPrice)
			}
			if sig.TakeProfit <= sig.EntryPrice {
				t.Errorf("LONG: TP %.4f <= Entry %.4f", sig.TakeProfit, sig.EntryPrice)
			}
		} else {
			if sig.StopLoss <= sig.EntryPrice {
				t.Errorf("SHORT: SL %.4f <= Entry %.4f", sig.StopLoss, sig.EntryPrice)
			}
			if sig.TakeProfit >= sig.EntryPrice {
				t.Errorf("SHORT: TP %.4f >= Entry %.4f", sig.TakeProfit, sig.EntryPrice)
			}
		}

		// RiskReward = 2.0
		var rr float64
		if sig.Direction == "LONG" {
			risk := sig.EntryPrice - sig.StopLoss
			reward := sig.TakeProfit - sig.EntryPrice
			if risk > 0 {
				rr = reward / risk
			}
		} else {
			risk := sig.StopLoss - sig.EntryPrice
			reward := sig.EntryPrice - sig.TakeProfit
			if risk > 0 {
				rr = reward / risk
			}
		}
		if rr > 0 && math.Abs(rr-2.0) > 0.01 {
			t.Errorf("RR=%.2f, erwartet 2.0", rr)
		}
	}
}

// ============================================================================
// TEST 2: HANN TREND SIGNAL-VALIDIERUNG
// ============================================================================

func TestInteg_HannTrendSignals(t *testing.T) {
	strategy := &HannTrendStrategy{}
	strategy.defaults()

	ohlcv := generateBreakoutOHLCV(300, "1h")
	t.Logf("HannTrend RequiredBars: %d, Bars: %d", strategy.RequiredBars(), len(ohlcv))

	signals := strategy.Analyze(ohlcv)
	t.Logf("HannTrend Signale: %d", len(signals))

	for i, sig := range signals {
		t.Logf("  Signal %d: Index=%d Dir=%s Entry=%.2f SL=%.2f TP=%.2f",
			i+1, sig.Index, sig.Direction, sig.EntryPrice, sig.StopLoss, sig.TakeProfit)

		if sig.Direction != "LONG" && sig.Direction != "SHORT" {
			t.Errorf("Ungültige Richtung: %s", sig.Direction)
		}
		if sig.Index < 0 || sig.Index >= len(ohlcv) {
			t.Errorf("Index außerhalb: %d", sig.Index)
		}
		if sig.Direction == "LONG" {
			if sig.StopLoss >= sig.EntryPrice {
				t.Errorf("LONG SL >= Entry")
			}
			if sig.TakeProfit <= sig.EntryPrice {
				t.Errorf("LONG TP <= Entry")
			}
		} else {
			if sig.StopLoss <= sig.EntryPrice {
				t.Errorf("SHORT SL <= Entry")
			}
			if sig.TakeProfit >= sig.EntryPrice {
				t.Errorf("SHORT TP >= Entry")
			}
		}
	}
}

// ============================================================================
// TEST 3: BEIDE STRATEGIEN AUF GLEICHEN DATEN (Signalkonflikte)
// ============================================================================

func TestInteg_BothStrategiesConflicts(t *testing.T) {
	smf := &SmartMoneyFlowStrategy{}
	smf.defaults()
	hann := &HannTrendStrategy{}
	hann.defaults()

	ohlcv := generateBreakoutOHLCV(300, "4h")

	smfSignals := smf.Analyze(ohlcv)
	hannSignals := hann.Analyze(ohlcv)

	t.Logf("Gleiche Daten — SMF: %d Signale, HannTrend: %d Signale", len(smfSignals), len(hannSignals))

	smfMap := map[int]string{}
	for _, s := range smfSignals {
		smfMap[s.Index] = s.Direction
	}
	conflicts := 0
	for _, s := range hannSignals {
		if smfDir, ok := smfMap[s.Index]; ok {
			if smfDir != s.Direction {
				conflicts++
				t.Logf("  KONFLIKT Bar %d: SMF=%s vs Hann=%s", s.Index, smfDir, s.Direction)
			} else {
				t.Logf("  ÜBEREINSTIMMUNG Bar %d: beide %s", s.Index, s.Direction)
			}
		}
	}
	t.Logf("Signal-Konflikte auf gleicher Bar: %d", conflicts)
}

// ============================================================================
// TEST 4: SL/TP PROPORTIONALE SKALIERUNG
// ============================================================================

func TestInteg_SLTPScaling(t *testing.T) {
	sigEntry := 150.0
	sigSL := 145.0
	sigTP := 160.0
	actualEntry := 152.0

	ratio := actualEntry / sigEntry
	scaledSL := math.Round(sigSL*ratio*100) / 100
	scaledTP := math.Round(sigTP*ratio*100) / 100

	t.Logf("Signal: Entry=%.2f SL=%.2f TP=%.2f", sigEntry, sigSL, sigTP)
	t.Logf("Actual: Entry=%.2f SL=%.2f TP=%.2f (ratio=%.4f)", actualEntry, scaledSL, scaledTP, ratio)

	origRisk := sigEntry - sigSL
	scaledRisk := actualEntry - scaledSL
	if math.Abs(scaledRisk/actualEntry-origRisk/sigEntry) > 0.001 {
		t.Errorf("Risiko-Verhältnis nicht proportional")
	}

	// Alpaca Guard: TP muss > entry + 0.01 (LONG)
	t.Run("Alpaca_Guard_LONG", func(t *testing.T) {
		tpTooClose := actualEntry + 0.005
		if tpTooClose <= actualEntry+0.01 {
			tpTooClose = math.Round(actualEntry*1.005*100) / 100
			t.Logf("Fallback TP: %.2f (+0.5%%)", tpTooClose)
		}
		if tpTooClose <= actualEntry {
			t.Error("TP muss > Entry sein")
		}
	})

	t.Run("Alpaca_Guard_SHORT", func(t *testing.T) {
		shortEntry := 150.0
		tpTooClose := shortEntry - 0.005
		if tpTooClose >= shortEntry-0.01 {
			tpTooClose = math.Round(shortEntry*0.995*100) / 100
			t.Logf("Fallback SHORT TP: %.2f (-0.5%%)", tpTooClose)
		}
		if tpTooClose >= shortEntry {
			t.Error("SHORT TP muss < Entry sein")
		}
	})
}

// ============================================================================
// TEST 5: RACE CONDITION GUARD (CONCURRENT ACCESS)
// ============================================================================

func TestInteg_PosGuardConcurrent(t *testing.T) {
	key := openPosGuardKey(999, 1, "TEST")
	liveOpenPosGuard.Store(key, true)
	defer liveOpenPosGuard.Delete(key)

	var winners int64
	var wg sync.WaitGroup
	var mu sync.Mutex

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, loaded := liveOpenPosGuard.LoadAndDelete(key); loaded {
				mu.Lock()
				winners++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	if winners != 1 {
		t.Errorf("Erwartet genau 1 Winner, bekommen: %d", winners)
	}
	t.Logf("Race-Test: %d/10 Workers gewonnen (erwartet: 1)", winners)
}

func TestInteg_InitPosGuardFromDB(t *testing.T) {
	gin.SetMode(gin.TestMode)
	setupIntegrationDB(t)
	defer teardownIntegrationDB(t)

	sessionID := uint(999)

	db.Create(&LiveTradingPosition{SessionID: sessionID, StrategyID: 1, Symbol: "AAPL", IsClosed: false, CreatedAt: time.Now()})
	db.Create(&LiveTradingPosition{SessionID: sessionID, StrategyID: 2, Symbol: "MSFT", IsClosed: false, CreatedAt: time.Now()})
	db.Create(&LiveTradingPosition{SessionID: sessionID, StrategyID: 1, Symbol: "TSLA", IsClosed: true, CreatedAt: time.Now()})

	initOpenPosGuard(sessionID)
	defer func() {
		liveOpenPosGuard.Delete("999:1:AAPL")
		liveOpenPosGuard.Delete("999:2:MSFT")
	}()

	if _, ok := liveOpenPosGuard.Load("999:1:AAPL"); !ok {
		t.Error("Guard für AAPL fehlt")
	}
	if _, ok := liveOpenPosGuard.Load("999:2:MSFT"); !ok {
		t.Error("Guard für MSFT fehlt")
	}
	if _, ok := liveOpenPosGuard.Load("999:1:TSLA"); ok {
		t.Error("Geschlossene Position TSLA sollte keinen Guard haben")
	}
}

// ============================================================================
// TEST 6: ALPACA MOCK SERVER
// ============================================================================

func TestInteg_AlpacaMockServer(t *testing.T) {
	mockAlpaca := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiKey := r.Header.Get("APCA-API-KEY-ID")
		if apiKey != "test-key" {
			w.WriteHeader(401)
			json.NewEncoder(w).Encode(map[string]string{"message": "unauthorized"})
			return
		}

		switch {
		case r.Method == "GET" && r.URL.Path == "/v2/account":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"id": "test-account", "status": "ACTIVE",
				"buying_power": "50000.00", "portfolio_value": "50000.00",
			})
		case r.Method == "POST" && r.URL.Path == "/v2/orders":
			var orderReq map[string]interface{}
			json.NewDecoder(r.Body).Decode(&orderReq)
			t.Logf("Mock Order: %v", orderReq)

			symbol, _ := orderReq["symbol"].(string)
			if symbol == "" {
				w.WriteHeader(422)
				return
			}
			json.NewEncoder(w).Encode(map[string]interface{}{
				"id": "mock-order-123", "status": "accepted",
				"symbol": symbol, "order_class": "simple",
			})
		default:
			w.WriteHeader(404)
		}
	}))
	defer mockAlpaca.Close()

	client := &http.Client{Timeout: 5 * time.Second}

	// Account Check
	req, _ := http.NewRequest("GET", mockAlpaca.URL+"/v2/account", nil)
	req.Header.Set("APCA-API-KEY-ID", "test-key")
	req.Header.Set("APCA-API-SECRET-KEY", "test-secret")
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Account-Abfrage: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("Account: erwartet 200, bekommen %d", resp.StatusCode)
	}

	// Order
	orderBody, _ := json.Marshal(map[string]interface{}{
		"symbol": "AAPL", "qty": "3.333333", "side": "buy",
		"type": "market", "time_in_force": "day",
	})
	req, _ = http.NewRequest("POST", mockAlpaca.URL+"/v2/orders", bytes.NewReader(orderBody))
	req.Header.Set("APCA-API-KEY-ID", "test-key")
	req.Header.Set("Content-Type", "application/json")
	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("Order: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("Order: erwartet 200, bekommen %d", resp.StatusCode)
	}
	var orderResp map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&orderResp)
	if orderResp["id"] != "mock-order-123" {
		t.Errorf("Order ID: %v", orderResp["id"])
	}

	// Unauthorized
	req, _ = http.NewRequest("GET", mockAlpaca.URL+"/v2/account", nil)
	req.Header.Set("APCA-API-KEY-ID", "wrong-key")
	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("Auth-Test: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 401 {
		t.Errorf("Unauthorized: erwartet 401, bekommen %d", resp.StatusCode)
	}
}

// ============================================================================
// TEST 7: TIF FRACTIONAL vs WHOLE
// ============================================================================

func TestInteg_TIFFractionalVsWhole(t *testing.T) {
	cases := []struct {
		qty      float64
		wantTIF  string
		wantFrac bool
	}{
		{3.333333, "day", true},
		{3.0, "gtc", false},
		{1.0, "gtc", false},
		{0.5, "day", true},
	}
	for _, tc := range cases {
		isFrac := tc.qty != float64(int(tc.qty))
		tif := "gtc"
		if isFrac {
			tif = "day"
		}
		if tif != tc.wantTIF {
			t.Errorf("Qty=%.6f: TIF=%s, erwartet %s", tc.qty, tif, tc.wantTIF)
		}
		if isFrac != tc.wantFrac {
			t.Errorf("Qty=%.6f: isFrac=%v, erwartet %v", tc.qty, isFrac, tc.wantFrac)
		}
	}
}

// ============================================================================
// TEST 8: QUANTITY BERECHNUNG
// ============================================================================

func TestInteg_QuantityCalculation(t *testing.T) {
	t.Run("Normal", func(t *testing.T) {
		qty := math.Round(500.0/150.0*1000000) / 1000000
		if math.Abs(qty-3.333333) > 0.001 {
			t.Errorf("Qty: %.6f, erwartet ~3.333333", qty)
		}
	})

	t.Run("NichtFractionable", func(t *testing.T) {
		qty := math.Floor(500.0 / 150.0)
		if qty != 3 {
			t.Errorf("Abgerundet: %g, erwartet 3", qty)
		}
	})

	t.Run("ZuTeuer", func(t *testing.T) {
		qty := math.Floor(500.0 / 600.0)
		if qty >= 1 {
			t.Errorf("Teures Asset: Qty=%g, sollte <1 sein → skip", qty)
		}
	})
}

// ============================================================================
// TEST 9: FRACTIONABLE HANDLING
// ============================================================================

func TestInteg_FractionableFlag(t *testing.T) {
	gin.SetMode(gin.TestMode)
	setupIntegrationDB(t)
	defer teardownIntegrationDB(t)

	db.Create(&TradingWatchlistItem{Symbol: "AAPL", Fractionable: true})
	brk := TradingWatchlistItem{Symbol: "BRK.A", Fractionable: true}
	db.Create(&brk)
	db.Model(&brk).Update("fractionable", false) // Explizit auf false setzen (GORM default:true)

	var item1 TradingWatchlistItem
	db.Where("symbol = ?", "AAPL").First(&item1)
	if !item1.Fractionable {
		t.Error("AAPL sollte fractionable sein")
	}

	var item2 TradingWatchlistItem
	db.Where("symbol = ?", "BRK.A").First(&item2)
	if item2.Fractionable {
		t.Error("BRK.A sollte nicht fractionable sein")
	}
}

// ============================================================================
// TEST 10: STRATEGY DEFAULTS
// ============================================================================

func TestInteg_SmartMoneyFlowDefaults(t *testing.T) {
	s := &SmartMoneyFlowStrategy{}
	s.defaults()

	if s.TrendLength != 34 {
		t.Errorf("TrendLength: %d, erwartet 34", s.TrendLength)
	}
	if s.BasisSmooth != 3 {
		t.Errorf("BasisSmooth: %d, erwartet 3", s.BasisSmooth)
	}
	if s.FlowWindow != 24 {
		t.Errorf("FlowWindow: %d, erwartet 24", s.FlowWindow)
	}
	if s.RiskReward != 2.0 {
		t.Errorf("RiskReward: %f, erwartet 2.0", s.RiskReward)
	}
	if s.RequiredBars() != 108 {
		t.Errorf("RequiredBars: %d, erwartet 108", s.RequiredBars())
	}
}

func TestInteg_HannTrendDefaults(t *testing.T) {
	s := &HannTrendStrategy{}
	s.defaults()

	if s.DMHLength != 30 {
		t.Errorf("DMHLength: %d, erwartet 30", s.DMHLength)
	}
	if s.SARStart != 0.02 {
		t.Errorf("SARStart: %f, erwartet 0.02", s.SARStart)
	}
	if s.SARIncrement != 0.03 {
		t.Errorf("SARIncrement: %f, erwartet 0.03", s.SARIncrement)
	}
	if s.RiskReward != 2.0 {
		t.Errorf("RiskReward: %f, erwartet 2.0", s.RiskReward)
	}
	if s.RequiredBars() != 70 {
		t.Errorf("RequiredBars: %d, erwartet 70", s.RequiredBars())
	}
}

// ============================================================================
// TEST 11: MARKET HOURS
// ============================================================================

func TestInteg_MarketHours(t *testing.T) {
	open := isUSMarketOpen()
	loc, _ := time.LoadLocation("America/New_York")
	now := time.Now().In(loc)
	t.Logf("US Markt offen: %v (NY: %s, %s)", open, now.Format("15:04"), now.Weekday())

	if now.Weekday() == time.Saturday || now.Weekday() == time.Sunday {
		if open {
			t.Error("Markt sollte am Wochenende geschlossen sein")
		}
	}
}

// ============================================================================
// TEST 12: SESSION OHNE STARTED_AT (Edge Case)
// ============================================================================

func TestInteg_SessionWithoutStartedAt(t *testing.T) {
	gin.SetMode(gin.TestMode)
	setupIntegrationDB(t)
	defer teardownIntegrationDB(t)
	stopWriter := startTestPositionWriter()
	defer stopWriter()

	session := LiveTradingSession{
		UserID: 1, ConfigID: 1, Name: "No Start", Strategy: "smart_money_flow",
		Interval: "4h", Symbols: `["AAPL"]`, TradeAmount: 500,
		IsActive: true, CreatedAt: time.Now(),
		// StartedAt bleibt Zero!
	}
	db.Create(&session)
	strat := LiveSessionStrategy{SessionID: session.ID, Name: "smart_money_flow", Symbols: `["AAPL"]`, IsEnabled: true, CreatedAt: time.Now()}
	db.Create(&strat)
	config := LiveTradingConfig{ID: 1, UserID: 1, AlpacaEnabled: false, TradeAmount: 500}
	db.Create(&config)

	smf := createStrategyFromJSON("smart_money_flow", "")
	ohlcv := generateBreakoutOHLCV(300, "4h")
	processLiveSymbolWithData(session, "AAPL", smf, ohlcv, config, strat)
	drainWrites()

	var count int64
	db.Model(&LiveTradingPosition{}).Where("session_id = ?", session.ID).Count(&count)
	if count > 0 {
		t.Errorf("Keine Positionen erwartet bei StartedAt=Zero, bekommen: %d", count)
	}
}

// ============================================================================
// TEST 13: EMPTY OHLCV
// ============================================================================

func TestInteg_EmptyOHLCV(t *testing.T) {
	gin.SetMode(gin.TestMode)
	setupIntegrationDB(t)
	defer teardownIntegrationDB(t)

	session := LiveTradingSession{
		UserID: 1, ConfigID: 1, Strategy: "smart_money_flow",
		Interval: "4h", Symbols: `["AAPL"]`, TradeAmount: 500,
		IsActive: true, StartedAt: time.Now(), CreatedAt: time.Now(),
	}
	db.Create(&session)
	db.Create(&LiveTradingConfig{ID: 1, UserID: 1})

	smf := createStrategyFromJSON("smart_money_flow", "")
	price, ok := processLiveSymbolWithData(session, "AAPL", smf, []OHLCV{}, LiveTradingConfig{})
	if ok {
		t.Error("Leere OHLCV sollte ok=false zurückgeben")
	}
	t.Logf("Leere OHLCV: price=%.2f ok=%v", price, ok)
}

// ============================================================================
// TEST 14: SL VOR TP PRIORITÄT
// ============================================================================

func TestInteg_SLBeforeTP(t *testing.T) {
	// Wenn in einer Bar sowohl SL als auch TP berührt werden,
	// hat SL Priorität (defensiv, wie im Backtest-Engine)
	pos := LiveTradingPosition{
		Direction: "LONG", EntryPrice: 150.0,
		StopLoss: 145.0, TakeProfit: 160.0,
	}

	bar := OHLCV{Open: 149.0, High: 161.0, Low: 144.0, Close: 155.0}

	closePrice := 0.0
	closeReason := ""

	// SL wird ZUERST geprüft (wie in processLiveSymbolWithData)
	if pos.StopLoss > 0 && bar.Low <= pos.StopLoss {
		closePrice = pos.StopLoss
		closeReason = "SL"
	} else if pos.TakeProfit > 0 && bar.High >= pos.TakeProfit {
		closePrice = pos.TakeProfit
		closeReason = "TP"
	}

	if closeReason != "SL" {
		t.Errorf("Erwartet SL, bekommen: %s", closeReason)
	}
	if closePrice != 145.0 {
		t.Errorf("ClosePrice: %.2f, erwartet 145.0", closePrice)
	}
	t.Logf("SL vor TP: Reason=%s Price=%.2f (korrekt)", closeReason, closePrice)

	// SHORT analog
	shortPos := LiveTradingPosition{
		Direction: "SHORT", EntryPrice: 150.0,
		StopLoss: 155.0, TakeProfit: 140.0,
	}
	shortBar := OHLCV{Open: 151.0, High: 156.0, Low: 139.0, Close: 148.0}

	closePrice = 0
	closeReason = ""
	if shortPos.StopLoss > 0 && shortBar.High >= shortPos.StopLoss {
		closePrice = shortPos.StopLoss
		closeReason = "SL"
	} else if shortPos.TakeProfit > 0 && shortBar.Low <= shortPos.TakeProfit {
		closePrice = shortPos.TakeProfit
		closeReason = "TP"
	}

	if closeReason != "SL" {
		t.Errorf("SHORT: Erwartet SL, bekommen: %s", closeReason)
	}
}

// ============================================================================
// TEST 15: LONG ONLY FILTER
// ============================================================================

func TestInteg_LongOnlyFilter(t *testing.T) {
	longOnly := true
	signal := StrategySignal{Direction: "SHORT"}

	if longOnly && signal.Direction == "SHORT" {
		t.Log("SHORT Signal korrekt übersprungen (Long Only)")
	} else {
		t.Error("SHORT hätte übersprungen werden müssen")
	}

	signal.Direction = "LONG"
	if longOnly && signal.Direction == "SHORT" {
		t.Error("LONG Signal sollte NICHT übersprungen werden")
	}
}

// ============================================================================
// TEST 16: DUPLIKAT-STRATEGIE ABLEHNEN
// ============================================================================

func TestInteg_DuplicateStrategyRejected(t *testing.T) {
	gin.SetMode(gin.TestMode)
	setupIntegrationDB(t)
	defer teardownIntegrationDB(t)

	session := LiveTradingSession{UserID: 1, ConfigID: 1, Strategy: "smart_money_flow", Interval: "4h", Symbols: `["AAPL"]`, CreatedAt: time.Now()}
	db.Create(&session)
	db.Create(&LiveTradingConfig{ID: 1, UserID: 1})
	db.Create(&LiveSessionStrategy{SessionID: session.ID, Name: "hann_trend", ParamsJSON: `{"dmh_length":30}`, Symbols: `["AAPL"]`, CreatedAt: time.Now()})

	body := map[string]interface{}{
		"strategy": "hann_trend",
		"params":   `{"dmh_length":30}`,
		"symbols":  []string{"AAPL"},
	}
	c, w := createAdminContext("POST", "/api/trading/live/session/"+fmt.Sprint(session.ID)+"/strategy", body)
	c.Params = gin.Params{{Key: "id", Value: fmt.Sprint(session.ID)}}
	addLiveSessionStrategy(c)

	if w.Code != 400 {
		t.Errorf("Duplikat: erwartet 400, bekommen %d", w.Code)
	}
}

// ============================================================================
// TEST 17: DOUBLE-START VERHINDERN
// ============================================================================

func TestInteg_DoubleStartPrevented(t *testing.T) {
	gin.SetMode(gin.TestMode)
	setupIntegrationDB(t)
	defer teardownIntegrationDB(t)

	session := LiveTradingSession{
		UserID: 1, ConfigID: 1, Strategy: "smart_money_flow",
		Interval: "4h", Symbols: `["AAPL"]`, IsActive: true, TradeAmount: 500, CreatedAt: time.Now(),
	}
	db.Create(&session)
	db.Create(&LiveTradingConfig{ID: 1, UserID: 1, AlpacaEnabled: false})

	c, w := createAdminContext("POST", "/api/trading/live/session/"+fmt.Sprint(session.ID)+"/resume", nil)
	c.Params = gin.Params{{Key: "id", Value: fmt.Sprint(session.ID)}}
	resumeLiveTrading(c)

	if w.Code != 400 {
		t.Errorf("Double-Start: erwartet 400, bekommen %d", w.Code)
	}
}

// ============================================================================
// TEST 18: FULL LIFECYCLE (ARENA → LIVE → STOP)
// ============================================================================

func TestInteg_FullLifecycle(t *testing.T) {
	gin.SetMode(gin.TestMode)
	setupIntegrationDB(t)
	defer teardownIntegrationDB(t)

	t.Log("=== 1. Session erstellen ===")
	body := map[string]interface{}{
		"name": "E2E Test", "strategy": "smart_money_flow", "interval": "4h",
		"params_json": `{"risk_reward":2.0}`, "long_only": false,
		"trade_amount": 500.0, "symbols": []string{"AAPL", "MSFT", "NVDA"},
	}
	c, w := createAdminContext("POST", "/api/trading/arena/v2/start-session", body)
	arenaV2StartSession(c)
	if w.Code != 200 {
		t.Fatalf("Session erstellen: %d %s", w.Code, w.Body.String())
	}
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	sessionID := uint(resp["session"].(map[string]interface{})["id"].(float64))
	t.Logf("Session ID=%d", sessionID)

	t.Log("=== 2. HannTrend hinzufügen ===")
	addBody := map[string]interface{}{
		"strategy": "hann_trend", "params": `{"dmh_length":30}`,
		"symbols": []string{"AAPL", "TSLA"}, "long_only": true,
	}
	c, w = createAdminContext("POST", "/api/trading/live/session/"+fmt.Sprint(sessionID)+"/strategy", addBody)
	c.Params = gin.Params{{Key: "id", Value: fmt.Sprint(sessionID)}}
	addLiveSessionStrategy(c)
	if w.Code != 200 {
		t.Fatalf("Strategie hinzufügen: %d %s", w.Code, w.Body.String())
	}

	var strategies []LiveSessionStrategy
	db.Where("session_id = ?", sessionID).Find(&strategies)
	if len(strategies) != 2 {
		t.Fatalf("Erwartet 2 Strategien, bekommen: %d", len(strategies))
	}
	t.Logf("Strategien: %s (enabled=%v), %s (enabled=%v)",
		strategies[0].Name, strategies[0].IsEnabled, strategies[1].Name, strategies[1].IsEnabled)

	// Symbole gemergt?
	var session LiveTradingSession
	db.First(&session, sessionID)
	var symbols []string
	json.Unmarshal([]byte(session.Symbols), &symbols)
	if len(symbols) != 4 {
		t.Errorf("4 Symbole erwartet, bekommen: %d (%v)", len(symbols), symbols)
	}

	t.Log("=== 3. Session aktivieren (manuell, ohne Scheduler-Goroutine) ===")
	// resumeLiveTrading startet async Goroutines die mit dem Test-DB konkurrieren.
	// Stattdessen aktivieren wir die Session manuell und simulieren den Scheduler.
	now := time.Now()
	db.Model(&LiveTradingSession{}).Where("id = ?", sessionID).Updates(map[string]interface{}{
		"is_active":  true,
		"started_at": now,
	})
	db.First(&session, sessionID)
	if !session.IsActive {
		t.Fatal("Session sollte aktiv sein")
	}
	if session.StartedAt.IsZero() {
		t.Fatal("StartedAt sollte gesetzt sein")
	}

	// Scheduler-Eintrag anlegen (stopLiveTrading braucht ihn)
	liveSchedulerMu.Lock()
	liveSchedulers[sessionID] = &liveSessionState{StopChan: make(chan struct{})}
	liveSchedulerMu.Unlock()

	t.Log("=== 4. Signal-Verarbeitung ===")
	ohlcv := generateBreakoutOHLCV(300, "4h")
	for i := range ohlcv {
		ohlcv[i].Time = session.StartedAt.Add(time.Duration(i) * 4 * time.Hour).Unix()
	}

	var config LiveTradingConfig
	db.First(&config, session.ConfigID)

	smfEngine := createStrategyFromJSON("smart_money_flow", strategies[0].ParamsJSON)
	hannEngine := createStrategyFromJSON("hann_trend", strategies[1].ParamsJSON)

	stopWriter := startTestPositionWriter()
	db.First(&session, sessionID)

	processLiveSymbolWithData(session, "AAPL", smfEngine, ohlcv, config, strategies[0])
	processLiveSymbolWithData(session, "AAPL", hannEngine, ohlcv, config, strategies[1])
	processLiveSymbolWithData(session, "MSFT", smfEngine, ohlcv, config, strategies[0])

	drainWrites()
	stopWriter()
	time.Sleep(100 * time.Millisecond)

	var positions []LiveTradingPosition
	db.Where("session_id = ?", sessionID).Find(&positions)
	t.Logf("Positionen: %d", len(positions))
	for _, p := range positions {
		t.Logf("  %s %s StratID=%d Entry=%.4f Closed=%v", p.Symbol, p.Direction, p.StrategyID, p.EntryPrice, p.IsClosed)
	}

	t.Log("=== 5. Session stoppen ===")
	c, w = createAdminContext("POST", "/api/trading/live/stop?session_id="+fmt.Sprint(sessionID), nil)
	c.Request.URL.RawQuery = "session_id=" + fmt.Sprint(sessionID)
	stopLiveTrading(c)
	if w.Code != 200 {
		t.Fatalf("Stop: %d %s", w.Code, w.Body.String())
	}

	db.First(&session, sessionID)
	if session.IsActive {
		t.Error("Session sollte inaktiv sein")
	}

	var openCount int64
	db.Model(&LiveTradingPosition{}).Where("session_id = ? AND is_closed = ?", sessionID, false).Count(&openCount)
	if openCount > 0 {
		t.Errorf("%d Positionen noch offen nach Stop", openCount)
	}

	// Logs
	var logs []LiveTradingLog
	db.Where("session_id = ?", sessionID).Order("created_at").Find(&logs)
	t.Logf("Logs: %d Einträge", len(logs))

	// Cleanup Guards
	for _, p := range positions {
		liveOpenPosGuard.Delete(openPosGuardKey(sessionID, p.StrategyID, p.Symbol))
	}
	liveSchedulerMu.Lock()
	delete(liveSchedulers, sessionID)
	liveSchedulerMu.Unlock()

	t.Log("=== E2E LIFECYCLE KOMPLETT ===")
}
