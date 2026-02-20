# FlipperCapital

Aktienanalyse-Webapp mit automatisierten Handelsbots.

## Stack
- **Backend:** Go + SQLite (`watchlist.db`) — einzelne Datei `backend/main.go` (~17k Zeilen)
- **Frontend:** React + Vite + Tailwind CSS + `lightweight-charts`
- **Infra:** Docker Compose, NGINX als Reverse Proxy
- **OS in Containern:** Linux

## Wichtige Pfade
- `backend/main.go` — gesamter Backend-Code (API, Bots, BXtrender-Berechnung)
- `frontend/src/components/` — React-Komponenten
- `frontend/src/utils/bxtrender.js` — BXtrender-Berechnung (Frontend)
- `frontend/src/tests/` — Test-Dateien (`.test.mjs`)
- `nginx/` — NGINX-Konfiguration
- `docker-compose.yml` — Service-Definition

## Kommandos
- **Alles starten:** `docker-compose up -d --build`
- **Frontend Dev:** `cd frontend && npm run dev`
- **Frontend Build:** `cd frontend && npx vite build`
- **Backend Dev:** `cd backend && go run main.go`
- **Tests:** `node frontend/src/tests/performance.test.mjs` und `node frontend/src/tests/performance_filter.test.mjs`
- **DB inspizieren:** `sqlite3 backend/watchlist.db`

## Regeln
- Antworte **kurz und direkt**, auf Deutsch.
- Höchstens 1–2 Rückfragen, nur wenn nötig.
- Bei Aufgaben mit >3 Dateien oder unklarem Scope: **zuerst Plan** (3-5 Bulletpoints), dann ausführen.
- Keine neuen Dateien (.md, docs, reports) außer explizit verlangt.
- Nur relevante Dateien ändern — keine "Sicherheits-Refactors".
- Nach Code-Änderungen: Frontend-Build prüfen (`npx vite build`), Tests laufen lassen wenn vorhanden.
- Commit nur wenn ich es sage.

## Architektur-Notizen
- **BXtrender-Berechnung:** Short EMA(5,20) → RSI(15)-50 = Short Xtrender; Long EMA(20) → RSI(15)-50 = Long Xtrender
- **Signal-Bestimmung** (alle Modi einheitlich): BUY=frisch (≤1 Monat), HOLD=Position>1 Monat, SELL=frisch (≤1 Monat), WAIT=keine Position+SELL>1 Monat
- **Trade-Ausführung** (alle Modi): Signal basiert auf fertiger Monatskerze, Trade zum Open des Folgemonats
- **Performance-Daten:** Frontend berechnet BXtrender → sendet an Backend → Backend speichert in DB. Backend hat eigene Server-Berechnung für Batch-Updates.

## Bot-Trading-Regeln ⚠️ Bei Änderungen an Bot-Logik MÜSSEN aktualisiert werden: dieser Abschnitt + `frontend/src/components/Help.jsx` (Sektionen `bxtrender-math` und `trading-strategies`)

5 Bots, sortiert nach Aggressivität: **Lutz** (aggressivster Entry) → **Trader** (schnellste Frequenz) → **FlipperBot** (defensiv) → **Quant** (Trend+MA) → **Ditz** (konservativster Exit)

### Gemeinsam (alle 5 Bots)
- **Investment:** 100 EUR pro Trade (EUR→USD konvertiert)
- **TSL:** 20% Trailing Stop Loss (default). `HighestPrice` täglich aktualisiert, Trigger: `price ≤ HighestPrice * 0.8`
- **Allowlist:** `isStockAllowedForBot(botName, symbol)` — kein DB-Eintrag = erlaubt
- **Bot-Filter:** Optional: MinWinrate, MinRR, MinAvgReturn, MinMarketCap → `IsFilterBlocked=true`
- **Re-Entry:** Nur nach vollständigem BUY→SELL Zyklus. Soft-deleted BUYs blockieren Re-Entry
- **User-IDs:** FlipperBot=999999, Lutz=999998, Quant=999997, Ditz=999996, Trader=999995

### FlipperBot (Defensiv) — wartet auf Bestätigung
- **BUY:** Red→Green ODER 4. aufeinanderfolgende "light red" Bar (`val < 0 && val > prev`)
- **SELL:** Erste "dark red" Bar (`val < 0 && val ≤ prev`) | Kein MA-Filter

### Lutz (Aggressiv) — kauft beim ersten Erholungszeichen
- **BUY:** 1. "light red" Bar ODER Red→Green — kauft früher als FlipperBot
- **SELL:** Erste "dark red" Bar (identisch zu FlipperBot) | Kein MA-Filter

### Quant — Trendfolger, doppelte Bestätigung, schneller Exit
- **BUY:** `short > 0 AND long > 0` + vorher mind. einer negativ + `price > EMA(200)`
- **SELL:** `short < 0 OR long < 0` ODER TSL | MA-Filter: EMA(200)

### Ditz — wie Quant, aber konservativer Exit (hält länger)
- **BUY:** `short > 0 AND long > 0` + vorher nicht beide positiv + `price > EMA(200)`
- **SELL:** `short < 0 AND long < 0` (BEIDE negativ) ODER TSL | MA-Filter: EMA(200)

### Trader — schnellster Bot, höchste Handelsfrequenz
- **BUY:** T3 Signal-Linie Rot→Grün (fallend→steigend) | Kein MA-Filter
- **SELL:** T3 Signal-Linie Grün→Rot ODER TSL
- **Signal-Linie:** T3(Short Xtrender, 5), Tillson T3 mit b=0.7

## Trading Arena ⚠️ Bei Änderungen an Arena-Logik MUSS dieser Abschnitt aktualisiert werden

### 7 Strategien (A–G)
| Key | Name | Default-TF | Kern-Logik |
|---|---|---|---|
| `regression_scalping` | Regression Scalping [BETA] | 5m | 3-Schritt: Close außerhalb Poly-Regression-Band → AO Farbflip → Heikin-Ashi Kerzenfarbe bestätigt |
| `hybrid_ai_trend` | NW Bollinger Bands | 5m | NW-geglättete 4-Level BB. Cross Level-1-Band → Entry. Optional: Hybrid-EMA-Filter (≥75 LONG/≤25 SHORT), Bestätigungskerze, Min-Band-Abstand |
| `diamond_signals` | Diamond Signals | — | Multi-Confluence-Score: Diamond-Pattern + RSI-Div + Vol-Div + Order-Blocks + Multi-TF (4x aggregiert). Score ≥ `confluence_min` (def 3) + Cooldown |
| `smart_money_flow` | Smart Money Flow [BETA] | 4h | Vol-Flow → adaptive ATR-Bänder → Regime (+1/-1). State-Machine: Regime-Wechsel → Swing tracken → Pullback zur Basis → Breakout über Swing |
| `hann_trend` | Hann Trend DMH+SAR | 1h | DMH (Hann-FIR, Ehlers TASC 2021.12) + Parabolic SAR. 4-Phasen: DMH Zero-Cross → SAR-Flip gegen Trend → SAR-Flip zurück → Close bricht Swing |
| `gmma_pullback` | GMMA Pullback | 1h | GMMA-Osc (Fast EMAs 3-15 vs Slow 30-60) Crossover + S/R-Zonen (Pivot-Fractals, on-the-fly invalidiert bei Breach) |
| `trippa_trade` | TrippaTrade Dual MACD [BETA] | 1h | Dual-MACD State-Machine: Slow(100/200/50) Histogramm → Trend, Fast(8/13/9) → Pullback-Detection + Re-Entry. MinTrendBars Choppy-Filter |

Alle implementieren: `TradingStrategy` (Analyze), `IndicatorProvider` (Sub-Chart), `OverlayProvider` (Price-Chart)

### Backtest Engine (`runArenaBacktest`)
- `strategy.Analyze(ohlcv)` → `[]StrategySignal{Index, Direction, Entry, SL, TP}` — Signal bei Bar i, Entry zum Open Bar i+1
- Bar-by-Bar: Phase 1 (Bar-Open): Signal → gegnerische Position schließen (SIGNAL) + neue öffnen. Phase 2 (Intrabar): SL/TP vs High/Low, **SL vor TP**
- Offene Position am Ende → Close zum letzten Close, Reason=END
- Metriken: WinRate, TotalReturn, AvgReturn, MaxDrawdown (compound Equity), NetProfit, RiskReward. Offene Trades ausgeschlossen

### Arena V2 Batch (SSE-Stream, `arenaV2BatchHandler`)
1. Watchlist laden, US-Only filtern → `cache_loaded` Event
2. Prefetch: Alpaca Batch (50er, 3 concurrent) → Yahoo Fallback (20 concurrent) → `prefetch_progress`
3. Backtest: 50 parallele Workers (Memory-Cache), je Symbol → `progress` Events
4. Post-Filter: MinWinrate, MinRR, MinAvgReturn, MinMarketCap
5. Persistierung in `arena_v2_batch_results` → `result` Event

### Live Trading Session-Lifecycle
- **Erstellen:** Arena "Neue Session" → `arenaV2StartSession` (Config + Session inaktiv) → Redirect `/live-trading/{id}`
- **Multi-Strategy:** "Zu Session" oder `addLiveSessionStrategy` → Strategie deaktiviert hinzufügen, Toggle nur bei gestoppter Session
- **Starten:** `resumeLiveTrading` → Alpaca-Keys vorhanden? **WebSocket-Modus** : **Polling-Modus**
- **WebSocket-Flow:** SharedWS (globaler Alpaca-Client, RefCount/Symbol) → 1m-Bars → `BarAggregator` (aggregiert zum Ziel-Interval) → `candleChan` (4096 Buffer) → 20 Worker → `processCandleEvent` → `processLiveSymbolWithData`
- **Polling-Flow:** Timer aligned auf Kerzen-Ende + 1.5s → `runLiveScan` → OHLCV-Cache refreshen (20 Worker) → je Symbol/Strategie `processLiveSymbolWithData`
- **Signal-Verarbeitung (`processLiveSymbolWithData`):** Offene Kerze abschneiden → `Analyze()` → Duplikat-Check → Entry-Preis = letzter Close (nicht Backtest-Preis!) → SL/TP proportional skaliert → Alpaca Market-Order (fractional) → DB. Gegensignal → Close. Intrabar SL/TP-Check
- **SL/TP-Monitor:** Goroutine alle 2min, Alpaca Realtime-Preise → Backup-Check
- **Stoppen:** `stopLiveTrading` → StopChan schließen, offene Positionen MANUAL-Close + Alpaca Sell, WS trennen
- **Alpaca:** Market-Orders, fractional=TIF day / ganze Shares=gtc. Close = exakte Qty (kein globaler DELETE)

### Frontend-Komponenten
- `TradingArena.jsx` — Hauptkomponente: Chart + Indikatoren + Backtest-Panel + Batch-Grid + Heatmap + Simulation
- `ArenaChart.jsx` — Candlestick (lightweight-charts) + Overlays (Line/Area) + Marker + Volumen
- `ArenaIndicatorChart.jsx` — Sub-Chart (Histogramme, Linien, Nulllinie)
- `ArenaBacktestPanel.jsx` — Metriken-Grid + Portfolio-Rendite (compound) + Trade-Tabelle
- `ArenaCalendarHeatmap.jsx` — Monat→Woche→Tag Drilldown, TradeOverlay-Modal
- `LiveTrading.jsx` — Session-Management, Positionen, Alpaca-Config, Debug-Logs, Analyse-Overlay, Notifications
- `BacktestLab.jsx` — BXtrender-Backtester (5 Bot-Modes + Custom-Regeln, Batch, Historie, Compare)
- `frontend/src/utils/arenaConfig.js` — Strategie-Definitionen, Parameter (key/default/min/max/step), Intervals

### DB-Tabellen (Arena)
`live_trading_sessions`, `live_session_strategies`, `live_trading_positions`, `live_trading_configs`, `live_trading_logs`, `arena_v2_batch_results`, `arena_strategy_settings`, `arena_backtest_histories`, `ohlcv_cache`, `trading_watchlist_items`

## Arena Redesign-Plan (abgeschlossen) ✅

### Architektur-Prinzip: Datenquellen-Trennung
- **Backtest/Prefetch → Yahoo Finance only** (historisch, kostenlos, 20+ concurrent, kein Rate-Limit)
- **Live-Trading → Alpaca only** (WebSocket für Realtime-Bars, REST für Orders/Quotes/SL-TP-Monitor)
- Alpaca-Account bleibt exklusiv für Live-Trading frei — kein Prefetch-Traffic

### Phase 1: Alpaca-only Watchlist ✅
### Phase 2: In-Memory OHLCV-Cache ✅
- `ohlcvMemCache` (map[string]map[string][]OHLCV) + RWMutex, geladen beim Startup
- `saveOHLCVCache()` schreibt in SQLite + Memory, `getOHLCVCached()` liest Memory-first
- Batch-Handler liest direkt aus Memory-Cache statt `bulkCacheJSON`

### Phase 3: Backtest-Isolation & Parallelisierung ✅
- Workers 15→50 (reine CPU mit Memory-Cache), kein JSON-Unmarshal pro Worker
- Batch-INSERT: `DELETE + CREATE` in einer Transaktion statt einzelne Upserts

### Phase 4: Live-Trading Entkopplung ✅
- `livePositionWriteCh`: serialisierte Position-Writes über Channel (kein SQLite-Lock-Contention)
- `liveAlpacaThrottle()`: separater Rate-Limiter für Live-Trading Alpaca-Calls
- `runLiveScan` liest Memory-Cache first, Fallback auf DB

### Phase 5: Frontend Table statt Cards ✅
- `StockPerformanceTable`: `@tanstack/react-table` mit sortierbar/filterbar
- Alle Spalten: Symbol, WinRate, Total, Ø/Trade, R/R, MaxDD, Trades
- `max-height: 80` + `overflow-y: auto` statt Cards-Grid

## Self-Update Regel
Wenn du während einer Session feststellst, dass diese CLAUDE.md **veraltet oder unvollständig** ist (z.B. neue Komponenten, geänderte Architektur, neue Pfade, neue Kommandos), dann **aktualisiere sie proaktiv** am Ende der Aufgabe — ohne dass ich es extra sagen muss. Halte sie dabei kompakt (<80 Zeilen).
