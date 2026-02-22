# FlipperCapital — Test-Übersicht

Alle Tests mit Kommandos, Erwartungen und Bereich. Dient als Review-Grundlage.

**Legende:**
- **Lokal** = Kein Netzwerk nötig (In-Memory DB, Mock-Daten)
- **Yahoo** = Braucht Yahoo Finance API (Internet)
- **Alpaca** = Braucht Alpaca API-Keys

---

## Schnellstart

```bash
# Alle lokalen Backend-Tests
cd backend && go test -v -timeout 120s ./...

# Nur schnelle Unit-Tests (ohne Yahoo)
cd backend && go test -v -run "Test[^B]" -timeout 60s

# Einzelne Test-Gruppen
cd backend && go test -v -run "TestArenaBatch" -timeout 300s
cd backend && go test -v -run "TestInteg_" -timeout 60s
cd backend && go test -v -run "TestArenaCache" -timeout 30s

# Frontend-Tests (alle Node)
cd frontend && node src/tests/signal.test.mjs
cd frontend && node src/tests/performance.test.mjs
cd frontend && node src/tests/performance_filter.test.mjs
cd frontend && node src/tests/live_trading.test.mjs
cd frontend && node src/tests/arena_backtest.test.mjs
cd frontend && node src/tests/daytradingStats.test.mjs
cd frontend && node src/tests/portfolio_rendite.test.mjs

# Build-Verifikation
cd backend && go build -o /dev/null .
cd frontend && npx vite build
```

---

## Backend Go Tests

### 1. `allowlist_test.go` — Bot-Allowlist & Admin-Close (29 Tests)

| Test | Erwartung | Netzwerk |
|------|-----------|----------|
| TestBotStockAllowlistModel | Allowlist-Entry in DB persistiert | Lokal |
| TestIsAdminClosedFieldOnTrades | IsAdminClosed auf allen 5 Trade-Modellen vorhanden | Lokal |
| TestIsAdminClosedFieldOnPositions | IsAdminClosed auf allen 5 Position-Modellen | Lokal |
| TestIsStockAllowedForBot_NoEntry | Kein Eintrag → erlaubt (default true) | Lokal |
| TestIsStockAllowedForBot_AllowedTrue | allowed=true → erlaubt | Lokal |
| TestIsStockAllowedForBot_AllowedFalse | allowed=false → blockiert | Lokal |
| TestIsStockAllowedForBot_DifferentBot | Block für Bot A betrifft Bot B nicht | Lokal |
| TestIsStockAllowedForBot_AllBots | Alle 5 Bots mit gleicher blockierter Aktie | Lokal |
| TestGetBotAllowlist_Empty | GET → 5 Bot-Keys mit leeren Arrays | Lokal |
| TestGetBotAllowlist_WithStocks | GET → korrekte allowed/blocked pro Bot | Lokal |
| TestGetBotAllowlist_Unauthorized | Ohne Auth → 401 | Lokal |
| TestUpdateBotAllowlist_BlockStock | PUT block → 200, DB blockiert | Lokal |
| TestUpdateBotAllowlist_AllowStock | PUT unblock → 200, DB erlaubt | Lokal |
| TestUpdateBotAllowlist_InvalidBot | PUT ungültiger Bot → 400 | Lokal |
| TestUpdateBotAllowlist_MissingFields | PUT ohne Symbol → 400 | Lokal |
| TestUpdateBotAllowlist_ToggleMultipleTimes | Block→Unblock→Block = 1 DB-Row (Upsert) | Lokal |
| TestClosePositionForBot_NoPosition | Keine offene Position → false | Lokal |
| TestClosePositionForBot_FlipperDBLogic | FlipperBot Position schließen → P&L korrekt | Lokal |
| TestClosePositionForBot_AllBotsDBLogic | Alle 5 Bots: SELL-Trade mit IsAdminClosed=true | Lokal |
| TestSimulatedPerformance_ExcludesAdminClosed | FlipperBot: Admin-Trades nicht in Performance | Lokal |
| TestSimulatedPerformance_QuantExcludesAdminClosed | QuantBot: Admin-Trades gefiltert | Lokal |
| TestSimulatedPerformance_DitzExcludesAdminClosed | DitzBot: Admin-Trades gefiltert | Lokal |
| TestSimulatedPerformance_TraderExcludesAdminClosed | TraderBot: Admin-Trades gefiltert | Lokal |
| TestSimulatedPerformance_LutzExcludesAdminClosed | LutzBot: Admin-Trades gefiltert | Lokal |
| TestAllowlistBlockPreventsNewTrades | Block AAPL → alle Bots false, MSFT true | Lokal |
| TestAllowlistPerBotIsolation | Block nur für flipper/quant → andere erlaubt | Lokal |
| TestUpdateAllowlist_ClosesPositionOnBlock | PUT block mit offener Position → closed | Lokal |
| TestAllowlistMultipleSymbols | 5 blockiert, 1 unblocked → Rest blockiert | Lokal |
| TestGetAllowlist_MultipleBotsWithDifferentStocks | Verschiedene Perf-Tabellen pro Bot | Lokal |

### 2. `backtest_lab_test.go` — Backtest Lab & BXtrender (18 Tests)

| Test | Erwartung | Netzwerk |
|------|-----------|----------|
| TestGetBarSignalState_Aggressive | Korrekte BUY/SELL basierend auf Xtrender-Werten | Lokal |
| TestGetBarSignalState_Quant | BUY wenn short+long positiv, SELL wenn short<0 | Lokal |
| TestGetBarConditionState_FirstLightRed | FIRST_LIGHT_RED nur bei erstem light-red Bar | Lokal |
| TestGetBarConditionState_PositionIndependent | Conditions unabhängig von inPosition | Lokal |
| TestGetBarConditionState_ANY | ANY-Wildcard matcht immer | Lokal |
| TestEvaluateBacktestLabRules_Testfall1 | Monthly FIRST_LIGHT_RED + Weekly BUY | Lokal |
| TestEvaluateBacktestLabRules_Testfall2 | Monthly SELL/WAIT + Weekly BUY_TO_HOLD | Lokal |
| TestEvaluateBacktestLabRules_MonthlySellWeeklyBuy | Trades mit validen Entry/Exit-Preisen | Lokal |
| TestEvaluateBacktestLabRules_WeeklyGranularity | Trade-Zeiten = Weekly-Bar-Zeiten | Lokal |
| TestFindMonthlyIndexForWeeklyBar | Weekly→Monthly Index korrekt, Zeiten monoton | Lokal |
| TestConvertServerTradesToArena | 2 Closed Trades, 4 Marker, Return korrekt | Lokal |
| TestCalculateBacktestLabMetrics | WinRate, Wins, Losses, TotalTrades korrekt | Lokal |
| TestWeeklyOHLCVCacheRoundtrip | JSON Marshal/Unmarshal ohne Datenverlust | Lokal |
| TestNWBollingerBandSignals | Signale generiert, korrekte Crossover-Erkennung | Lokal |
| TestNWBollingerBandOverlays | 8 Bänder (4 upper + 4 lower), Level 1 solid | Lokal |
| TestNWBollingerBandIndicators | 4 Serien, Oszillator in 0-100 | Lokal |
| TestNWSmoothingMatchesPineScript | Gauss-Kernel w(x,h) = exp(-x²/2h²) | Lokal |
| TestBacktestLabResponse_NoRules | Signale + Metriken ohne Custom-Rules | Lokal |

### 3. `arena_cache_test.go` — OHLCV-Cache (12 Tests)

| Test | Erwartung | Netzwerk |
|------|-----------|----------|
| TestArenaCache_SetAndGet | Write→Read korrekt | Lokal |
| TestArenaCache_MissReturnsEmpty | Cache-Miss → false | Lokal |
| TestArenaCache_IsolationFromLiveCache | Arena ≠ Live-Cache, keine Cross-Contamination | Lokal |
| TestArenaCache_FileWriteAndRead | Gzip Write→Read persistiert | Lokal |
| TestArenaCache_FilePathSeparation | Arena/Live in verschiedenen Dirs | Lokal |
| TestArenaCache_LazyLoadFromFile | File→Memory Lazy-Load funktioniert | Lokal |
| TestArenaCache_SaveOHLCVCacheWrapper | saveArenaOHLCVCache speichert & liest | Lokal |
| TestArenaCache_GetSymbols | Union von Memory + Disk Symbolen | Lokal |
| TestArenaCache_ConcurrentAccess | 8 Symbole parallel, kein Race | Lokal |
| TestArenaCache_FilePathSanitization | BRK.B → korrekter Pfad | Lokal |
| TestArenaCache_ReadNonExistentFile | Fehlende Datei → Error | Lokal |
| TestArenaCache_EmptyBarsNotCached | Leere Bars = Cache-Miss | Lokal |

### 4. `arena_batch_integration_test.go` — Arena Batch (8 Tests)

| Test | Erwartung | Netzwerk |
|------|-----------|----------|
| TestArenaBatch_SmallWatchlist | 20 Symbole, 80%+ Erfolg, <35s | Yahoo |
| TestArenaBatch_FullWatchlist | 200 Symbole, <120s Prefetch, <5s Backtest | Yahoo |
| TestArenaBatch_SSEEndpoint | HTTP 200, SSE Events mit Results | Yahoo |
| TestArenaBatch_PrefetchTiming | Min/Max/Avg Fetch-Zeiten | Yahoo |
| TestArenaBatch_BacktestSpeed | 200 Backtests <2s | Lokal (cached) |
| TestArenaBatch_ConcurrencyScaling | 1x vs 10x Ratio <5.0x | Yahoo |
| TestArenaBatch_SkippedSymbolReasons | Übersprungene Symbole haben Reason | Yahoo |
| TestArenaBatch_HannTrend | Batch mit Hann Trend, keine NaN/Inf | Yahoo |

### 5. `live_pipeline_test.go` — Live-Trading Pipeline (34 Tests)

| Test | Erwartung | Netzwerk |
|------|-----------|----------|
| TestMergeOHLCV_EmptyCache | nil Cache + Fresh → Fresh zurück (10 Bars) | Lokal |
| TestMergeOHLCV_EmptyFresh | Cached + nil → Cached zurück | Lokal |
| TestMergeOHLCV_NoOverlap | Nicht-überlappend → 15 Bars, ascending | Lokal |
| TestMergeOHLCV_WithOverlap | 10-Bar Overlap, Fresh überschreibt | Lokal |
| TestMergeOHLCV_CompleteOverlap | Fresh deckt alles ab → nur Fresh | Lokal |
| TestMergeOHLCV_SingleBarDelta | Poll-Update: letzte 2 Bars → 500 total | Lokal |
| TestStrategyMinBars_* (3) | RequiredBars korrekt, <required → 0 Signale | Lokal |
| TestSignalStability_* (3) | Full vs Cache+Merge → identische Signale | Lokal |
| TestSignalNoLookAhead_* (3) | Entry=Bar.Open, kein Look-Ahead-Bias | Lokal |
| TestProcessLiveSymbol_OpensPosition | Positionen mit korrektem Entry/Qty/SL/TP | Lokal |
| TestProcessLiveSymbol_NoDuplicateSignals | signal_index verhindert Duplikate | Lokal |
| TestProcessLiveSymbol_SLTPCloses | SL/TP/SIGNAL Reason korrekt | Lokal |
| TestProcessLiveSymbol_LongOnlyFilter | LongOnly=true → kein SHORT | Lokal |
| TestAlpacaOrderFlow_NoAlpaca_StillCreatesDB | Ohne Alpaca → DB-Positions trotzdem | Lokal |
| TestCloseLivePosition_Long/Short/Loss (3) | P&L% und Amount korrekt | Lokal |
| TestGetOHLCVDeltaPeriod | Interval→Delta-Period Mapping | Lokal |
| TestIncrementalSignalStability | +1 Bar → alte Signale bleiben | Lokal |
| TestTradeHistory_CompleteLifecycle | Open/Close: entry_time/close_time gesetzt | Lokal |
| TestPerformanceStats | WinRate 66.67%, P&L 80, R/R 2.0 | Lokal |
| TestAllStrategiesProduceValidSignals | 3 Strategien: valide Signale | Lokal |
| TestRequiredBarsMatchesStrategy | Alle Strategien ≥50 Bars | Lokal |
| TestBracketOrderConstruction | SL/TP = Signal-Werte | Lokal |
| TestGapRecovery_* (4) | Lücken im Cache korrekt gefüllt | Lokal |

### 6. `live_trading_test.go` — Live-Trading REST API (16 Tests)

| Test | Erwartung | Netzwerk |
|------|-----------|----------|
| TestSaveLiveTradingConfig | POST Config → Upsert, Interval updated | Lokal |
| TestGetLiveTradingConfig | GET → gespeicherte Config zurück | Lokal |
| TestStartLiveSession | POST start → Session erstellt, inaktiv | Lokal |
| TestStartSessionMultipleAdmin | Admin kann mehrere Sessions starten | Lokal |
| TestStopLiveSession | POST stop → IsActive=false, StoppedAt gesetzt | Lokal |
| TestStopClosesOpenPositions | Stop schließt alle Positions mit P&L | Lokal |
| TestPnLCalculationLong | LONG 100→110: P&L=+10%, Amount=+50 | Lokal |
| TestPnLCalculationShort | SHORT 100→90: P&L=+10%, Amount=+50 | Lokal |
| TestPnLCalculationLoss | LONG 100→95 (SL): P&L=-5%, Amount=-25 | Lokal |
| TestCreateStrategyFromJSON | Factory für alle Strategien → korrekter Typ | Lokal |
| TestIntervalToDuration | "5m"→5min, "1h"→1h, "1d"→24h | Lokal |
| TestCurrencyDetection | AAPL→USD, SAP.DE→EUR, BP.L→GBP | Lokal |
| TestGetLiveStatus | is_running=false/true je nach Session | Lokal |
| TestGetSessionHistory | Sessions sortiert nach ID DESC | Lokal |
| TestGetSessionDetail | Positions-Array mit Symbol/Direction/Entry | Lokal |
| TestStartSessionRequiresConfig | Ohne Config → 400 | Lokal |

### 7. `multi_strategy_test.go` — Multi-Strategie Sessions (14 Tests)

| Test | Erwartung | Netzwerk |
|------|-----------|----------|
| TestSessionCreatesInitialStrategy | 1 Strategy auto-erstellt, is_enabled=true | Lokal |
| TestAddStrategyStartsDisabled | Hot-Add → is_enabled=false | Lokal |
| TestToggleStrategyOnlyWhenStopped | Toggle nur bei gestoppter Session, sonst 400 | Lokal |
| TestSessionDetailIncludesStrategies | GET session/{id} → strategies Array | Lokal |
| TestSessionsListIncludesStrategiesCount | GET sessions → strategies_count | Lokal |
| TestPositionsAreStrategyIsolated | 2 Strategien, same Symbol → isoliert | Lokal |
| TestSignalDedupIsStrategyScoped | signal_index pro Strategy isoliert | Lokal |
| TestCloseLivePositionPnL | LONG/SHORT P&L korrekt | Lokal |
| TestMigrationCreatesStrategyForExistingSessions | Legacy-Session → Strategy-Row erstellt | Lokal |
| TestLogFilterByStrategy | ?strategy=strat_a → nur dessen Logs | Lokal |
| TestPositionsHaveStrategyNameInDetail | strategy_name in Response sichtbar | Lokal |
| TestSymbolMergeNoDuplicates | Überlappende Symbole → Union, keine Dups | Lokal |
| TestGetStrategiesEndpoint | GET strategies → 2 Strategien | Lokal |
| TestStopSessionClosesAllPositions | Stop → alle zu, CloseReason=MANUAL | Lokal |

### 8. `integration_test.go` — Voll-Integration (20 Tests)

| Test | Erwartung | Netzwerk |
|------|-----------|----------|
| TestInteg_SmartMoneyFlowSignals | Valide Indizes, Directions, SL<Entry<TP | Lokal |
| TestInteg_HannTrendSignals | Valide Signal-Properties | Lokal |
| TestInteg_BothStrategiesConflicts | SMF vs HannTrend Konflikte möglich | Lokal |
| TestInteg_SLTPScaling | Skalierte SL/TP behalten Risiko-% | Lokal |
| TestInteg_PosGuardConcurrent | Nur 1 von 10 Workers gewinnt LoadAndDelete | Lokal |
| TestInteg_InitPosGuardFromDB | Offene Positions → Guard, geschlossene nicht | Lokal |
| TestInteg_AlpacaMockServer | Mock-Alpaca: Account, Orders, Auth | Lokal |
| TestInteg_TIFFractionalVsWhole | Fractional→"day", Whole→"gtc" | Lokal |
| TestInteg_QuantityCalculation | 500/150 = 3.333 (fractional) oder 3 (whole) | Lokal |
| TestInteg_FractionableFlag | AAPL=true, BRK.A=false | Lokal |
| TestInteg_SmartMoneyFlowDefaults | TrendLength=34, BasisSmooth=3, RR=2.0 | Lokal |
| TestInteg_HannTrendDefaults | DMHLength=30, SARStart=0.02, RequiredBars=70 | Lokal |
| TestInteg_MarketHours | Wochenende → false | Lokal |
| TestInteg_SessionWithoutStartedAt | StartedAt=zero → keine Positions | Lokal |
| TestInteg_EmptyOHLCV | Leere Daten → ok=false | Lokal |
| TestInteg_SLBeforeTPPriority | SL+TP gleichzeitig → SL gewinnt | Lokal |
| TestInteg_LongOnlyFilter | LongOnly → kein SHORT | Lokal |
| TestInteg_DuplicateStrategyRejected | Doppelte Strategy → 400 | Lokal |
| TestInteg_DoubleStartPrevented | Doppelter Resume → 400 | Lokal |
| TestInteg_FullLifecycle | Arena→Strategy→Activate→Stop, P&L korrekt | Lokal |

### 9. `benchmark_yahoo_test.go` — Performance-Benchmarks (5 Tests)

| Test | Erwartung | Netzwerk |
|------|-----------|----------|
| TestBenchmark_YahooFetchSingle | 5 Symbole, ~200-500ms/Symbol | Yahoo |
| TestBenchmark_YahooCrumbInit | Erster Call ~1s, Cached instant | Yahoo |
| TestBenchmark_Yahoo20Concurrent | 20 parallel, 80%+ Erfolg, <30s | Yahoo |
| TestBenchmark_BacktestComputation | SMF+HannTrend Timing | Yahoo |
| TestBenchmark_BottleneckAnalysis | Crumb→Fetch→Concurrent→Backtest Profil | Yahoo |

### 10. `live_yahoo_integration_test.go` — Yahoo Delta-Merge (4 Tests)

| Test | Erwartung | Netzwerk |
|------|-----------|----------|
| TestYahoo_DeltaMergeMatchesFull_DBK | Full Signals == Merged Signals | Yahoo |
| TestYahoo_DeltaMergeMatchesFull_MultiStock | 5 Symbole: Signal-Count identisch nach Merge | Yahoo |
| TestYahoo_BarCountSufficient | Alle Strategien ≥ RequiredBars() | Yahoo |
| TestYahoo_SimulatePollCycle | Prefetch→Delta→Merge→Analyze stimmt überein | Yahoo |

---

## Frontend Node Tests

### 1. `signal.test.mjs` — Signal-Berechnung (8 Tests)

```bash
node frontend/src/tests/signal.test.mjs
```

| Test | Erwartung | Status |
|------|-----------|--------|
| Defensiv: zu wenig Daten → WAIT | signal='WAIT' bei <4 bars | **FAIL** — Code gibt 'NO_DATA', Test erwartet 'WAIT' |
| Quant: zu wenig Daten → WAIT | signal='WAIT' bei <2 bars | **FAIL** — gleicher Mismatch |
| Ditz: zu wenig Daten → WAIT | signal='WAIT' bei <3 bars | **FAIL** — gleicher Mismatch |
| Trader delegiert zu Ditz | Identisches Ergebnis |
| isAggressive hat keinen Einfluss | Signal unabhängig von Mode |
| Mehrere Trades — letzter zählt | Letzter geschlossener Trade bestimmt Signal |
| null trades → WAIT (Quant) | WAIT bei null trades |
| openTrade ohne entryDate → HOLD | HOLD bars=1 |

### 2. `performance.test.mjs` — Performance-Berechnung (20+ Tests)

```bash
node frontend/src/tests/performance.test.mjs
```

| Test-Gruppe | Erwartung |
|-------------|-----------|
| Cutoff 1y/3y/all | Korrekte Timestamp-Berechnung |
| Defensive 1y | 3 Trades, 2 Wins, WinRate=66.67%, TotalReturn=20% |
| Defensive 3y | 7 Trades, 4 Wins, WinRate=57.14%, TotalReturn=30% |
| Aggressive 1y | 3 Trades, 2 Wins |
| Risk/Reward | Korrekte Berechnung AvgWin/AvgLoss |

### 3. `performance_filter.test.mjs` — Performance-Filter (20 Tests)

```bash
node frontend/src/tests/performance_filter.test.mjs
```

| Test-Gruppe | Erwartung |
|-------------|-----------|
| minWinrate/maxWinrate | Filter korrekt, Range funktioniert |
| minRR/maxRR | Risk-Reward Filter |
| minAvgReturn/maxAvgReturn | Avg-Return Filter |
| minMarketCap | MarketCap=0 wird gefiltert, Boundary exakt |
| Kombinierte Filter | Alle Filter gleichzeitig anwendbar |
| Leere/keine Trades | Stats = 0, keine Fehler |

### 4. `live_trading.test.mjs` — Live-Trading Logik (12 Tests)

```bash
node frontend/src/tests/live_trading.test.mjs
```

| Test | Erwartung |
|------|-----------|
| P&L LONG profit: 500€ × +10% | = +50€ |
| P&L LONG loss: 500€ × -5% | = -25€ |
| P&L SHORT profit: 500€ × +10% | = +50€ |
| P&L SHORT loss: 500€ × -3% | = -15€ |
| P&L verschiedene Invest-Summen | 1000€ × +5% = +50€ |
| Session Summary | Wins/Losses/WinRate/TotalPnl korrekt |
| Session Summary: all winners | 100% WinRate |
| Session Summary: empty | 0 Trades |
| Symbol Aggregation | Gruppiert nach Symbol, sortiert nach PnL |
| Config Payload | Korrektes JSON mit Defaults |
| Config Payload: custom values | Überschreiben Defaults |

### 5. `arena_backtest.test.mjs` — Arena Strategien (20+ Tests)

```bash
node frontend/src/tests/arena_backtest.test.mjs
```

| Test-Gruppe | Erwartung |
|-------------|-----------|
| 9 Strategien definiert | Alle vorhanden (inkl. vwap_day_trading, gaussian_trend) |
| value + label vorhanden | Jede Strategie |
| Parameter-Definitionen | getDefaultParams liefert korrekte Keys |
| Default-Intervals gültig | In INTERVALS-Array enthalten |
| GMMA Pullback disabled | disabled=true |
| min < max, step > 0 | Für alle Parameter |
| Toggle-Parameter | min=0, max=1 |
| Metriken-Berechnung | WinRate, TotalTrades, TotalReturn, AvgReturn, R/R |

### Manuelle Verifikation: Arena Export V3 (AI/ML Pattern-Analyse)

**Datei:** `TradingArena.jsx` → `handleBacktestExport`, `arenaConfig.js` → `STRATEGY_ALGORITHMS`

| Prüfpunkt | Erwartung |
|-----------|-----------|
| `version` Feld | `3` im exportierten JSON |
| `strategy.algorithm` | Objekt mit `description`, `indicators`, `overlays`, `signal_logic`, `sl_tp`, `entry` |
| `param_definitions` | Array mit `key`, `label`, `value`, `default`, `min`, `max`, `step`, `is_toggle` pro Param |
| `backtest_engine` | Objekt mit `description`, `entry_rule`, `sl_tp_rule`, `metrics_note` |
| `indicator_columns` | Objekt: Spaltenname → Formel-String für jeden Indikator |
| `overlay_columns` | Objekt: Spaltenname → Formel-String für jedes Overlay |
| `timeseries.columns` | `["time","open","high","low","close","volume", ...indicator_names, ...overlay_names]` |
| `timeseries.data` | Array von Arrays, Länge = Anzahl chart_data Bars, Spalten = columns.length |
| `trades[].entry_indicators` | Objekt mit Indikator-Werten bei Entry-Bar (z.B. `{"MACD": 0.45}`) |
| `trades[].exit_indicators` | Objekt mit Indikator-Werten bei Exit-Bar, `null` bei offenen Trades |
| `trades[].entry_overlays` | Objekt mit Overlay-Werten bei Entry-Bar (z.B. `{"BB Upper": 152.0}`) |
| `trades[].exit_overlays` | Objekt mit Overlay-Werten bei Exit-Bar, `null` bei offenen Trades |
| `trades[].context_bars` | Array mit ≤5 OHLCV-Bars vor Entry (time, open, high, low, close, volume) |
| Kein Backtest → Export disabled | Button nur aktiv wenn `filteredMetrics` + `filteredTrades` vorhanden |
| `reference_line` Indikatoren | Werden aus Timeseries/Snapshots gefiltert (nur horizontale Referenzlinien) |
| Alle 10 Strategien in `STRATEGY_ALGORITHMS` | Jede hat `description`, `indicators`, `overlays`, `signal_logic`, `sl_tp` (inkl. gaussian_trend) |

**Verifizierung:** Backtest ausführen → Export-Button → JSON öffnen → Prüfen: `strategy.algorithm.signal_logic` vorhanden, `timeseries.data[0].length === timeseries.columns.length`, `indicator_columns` Keys matchen `timeseries.columns`.

### 6. `daytradingStats.test.mjs` — Daytrading-Statistiken (18 Tests)

```bash
node frontend/src/tests/daytradingStats.test.mjs
```

| Test | Erwartung |
|------|-----------|
| totalPnl | Summe profit_loss_amt aller geschlossenen |
| totalPnl ignoriert offene | Nur closed Positions |
| winRate basiert auf closed | Nicht auf offenen |
| avgWin / avgLoss | Korrekte Durchschnitte |
| rr = \|avgWin/avgLoss\| | Risk-Reward Ratio |
| profitFactor = grossWin/grossLoss | Profit-Faktor |
| rendite = Summe pct (additiv) | NICHT multiplikativ |
| rendite ≠ totalPnl/totalInvested | Alter Bug behoben |
| maxDD basic / consecutive / all wins | Max Drawdown korrekt |
| maxDD additiv, nicht multiplikativ | Korrekte Equity-Kurve |
| win streak / loss streak | Streaks korrekt gezählt |

### 7. `portfolio_rendite.test.mjs` — Portfolio-Rendite (14 Tests)

```bash
node frontend/src/tests/portfolio_rendite.test.mjs
```

| Test-Gruppe | Erwartung |
|-------------|-----------|
| Einzel-Position | Start 0%, Ende korrekt, Zwischenwerte |
| 2 Positionen gleichmäßig | Portfolio-Rendite = Durchschnitt |
| Ungleiche Positionen | Investitionsgewichtete Rendite |
| Rebase bei Join | Vor Join korrekt, nach Join prevPct beibehalten |
| Quantities | Investitionsgewichtete Berechnung |
| period_return | Offene + geschlossene korrekt kombiniert |

---

## Statistiken

| Bereich | Dateien | Tests | Davon Yahoo/Netzwerk |
|---------|---------|-------|---------------------|
| Backend Go | 10 | ~156 | ~17 |
| Frontend Node | 7 | ~112 | 0 |
| **Gesamt** | **17** | **~268** | **~17** |

---

## Letzter Testlauf

**Datum:** 2026-02-22
**Ergebnis:** Backend-Tests PASS, Frontend Build OK

```
TestArenaBatch (8 Tests)           PASS  (17.9s)
TestInteg_ (20 Tests)              PASS  (0.25s)
go build -o /dev/null .            OK
npx vite build                     OK (3.06s)
```

**Änderungen:** Arena Batch Cache-Fix + UX: Frontend nutzt V2-Endpoint mit korrekter Cache-Validierung (≥50 Bars). Dead code entfernt (~500 Zeilen: `backtestBatchHandler`, `backtestWatchlistHandler`, `getOHLCVMemCacheSymbols`). V2-Handler sendet jetzt `init`-Event sofort vor Cache-Check → Frontend zeigt "Prüfe Cache" statt minutenlang "Verbinde...". SSE-Events haben explizites `source: "Yahoo"` Feld.

---

## Bekannte Erwartungen / Business-Logik

### TSL (Trailing Stop Loss)
- `tsl_enabled=false` → **kein** TSL in Backtests (Frontend + Backend)
- TSL-Sells werden orange markiert, Signal-Sells rot
- Trade-Objekt hat `exitReason: 'TSL' | 'SIGNAL'`

### Signal-Reihenfolge
- SELL hat **immer** Priorität über BUY (gleicher Bar)
- SL wird **vor** TP geprüft (gleicher Bar)

### Bot-Trading
- 100 EUR/Trade, 20% TSL (wenn enabled)
- Re-Entry nur nach komplettem BUY→SELL Zyklus
- Allowlist blockiert pro Bot isoliert

### Live-Trading
- Position-Guard: `sync.Map` verhindert Doppel-Close
- DB-Writes via Channel (256 Buffer) serialisiert
- Fractional Shares → TIF "day", Whole Shares → TIF "gtc"

### Performance-Berechnung
- Rendite = **additiv** (Summe der pct), nicht multiplikativ
- MaxDrawdown basiert auf additiver Equity-Kurve
- Admin-geschlossene Trades werden aus Simulated Performance **ausgeschlossen**

### Admin "Alle Aktien aktualisieren" — Batch-Modus
- **Phase 1 (Prefetch):** Backend-Endpoint `POST /api/admin/prefetch-monthly-ohlcv` mit 5-concurrent Semaphore + Crumb-Auth
  - Symbole in 50er-Batches an Backend senden
  - Backend nutzt `fetchOHLCVFromYahoo` (authentifiziert, query2) mit Fallback auf `fetchHistoricalDataServer`
  - Daten werden **synchron** auf Disk geschrieben (nicht async), damit `getBotMonthlyOHLCVCached` sofort Cache-Hit liefert
  - Memory-Cache + Gzip-Datei → `getHistory` liefert instant
- **Phase 2 (Berechnung):** Frontend verarbeitet Aktien via `processStockWithConfigs`
  - Configs werden 1x vorgeladen (nicht pro Aktie)
  - **Fast Path** (Prefetch OK >50%): 10er Batches parallel, `getHistory` → Cache-Hit
  - **Slow Path** (Prefetch fehlgeschlagen): Sequenziell mit 1.5s Delay (sicher, kein Rate-Limit)
- **Vorher:** 1 Aktie/s sequenziell (500 Aktien = ~8 Min)
- **Nachher:** 50er Prefetch + 10er Calc-Batches (500 Aktien = ~1-2 Min) oder Fallback sequenziell
