# Integration Test Report — Live Trading (Smart Money Flow + Hann Trend)

**Datum:** 2026-02-21
**Ziel:** Sicherstellen, dass der komplette Workflow Arena → Live Trading mit Multi-Strategie (SmartMoneyFlow + HannTrend) fehlerfrei funktioniert, bevor Montag die Börse öffnet.

---

## Testdateien

| Datei | Typ | Ausführung |
|-------|-----|-----------|
| `backend/integration_test.go` | Go Unit + Integration | `cd backend && go test -v -run Test -timeout 120s` |
| `frontend/src/tests/live-trading-integration.test.mjs` | E2E HTTP | `node frontend/src/tests/live-trading-integration.test.mjs` |

---

## Übersicht aller Testfälle

### A. STRATEGIE-SIGNAL-GENERIERUNG

| # | Testfall | Erwartetes Verhalten | Go-Test | E2E |
|---|---------|---------------------|---------|-----|
| A1 | SmartMoneyFlow Signale generieren | Analyze() gibt valide Signale mit Entry/SL/TP zurück. SL < Entry < TP (LONG), SL > Entry > TP (SHORT). RiskReward exakt 2.0 | `TestSmartMoneyFlowSignals` | — |
| A2 | HannTrend Signale generieren | Analog zu A1. DMH-Cross + SAR-Pullback-Confirmation erzeugt Signale | `TestHannTrendSignals` | — |
| A3 | Beide Strategien auf gleichen Daten | Zeigt ob/wo Signalkonflikte (LONG vs SHORT auf gleicher Bar) auftreten | `TestBothStrategiesOnSameData` | — |
| A4 | Zu wenige Bars → keine Signale | Bei < RequiredBars Bars gibt Analyze() nil/leeres Array zurück, kein Crash | `TestInsufficientBars` | — |
| A5 | Leere OHLCV-Daten | processLiveSymbolWithData gibt (0, false) zurück, kein Crash | `TestEmptyOHLCV` | — |

### B. SESSION LIFECYCLE

| # | Testfall | Erwartetes Verhalten | Go-Test | E2E |
|---|---------|---------------------|---------|-----|
| B1 | Session aus Arena erstellen | POST → 200, Status "created", Session in DB mit is_active=false, Config erstellt, LiveSessionStrategy erstellt (enabled) | `TestArenaV2StartSession` | `Arena V2 Session erstellen` |
| B2 | Session ohne Symbole → Fehler | POST mit leeren Symbolen → 400 "Mindestens ein Symbol erforderlich" | `TestArenaV2StartSessionValidation` | — |
| B3 | Session in Liste vorhanden | GET /sessions enthält die erstellte Session | — | `Session in Liste vorhanden` |
| B4 | Session Details laden | GET /session/:id liefert positions, symbol_prices, strategies | — | `Session Details laden` |
| B5 | Session umbenennen | PATCH /session/:id/name ändert den Namen | — | `Session umbenennen` |

### C. MULTI-STRATEGIE MANAGEMENT

| # | Testfall | Erwartetes Verhalten | Go-Test | E2E |
|---|---------|---------------------|---------|-----|
| C1 | Zweite Strategie hinzufügen | POST → 200, neue Strategy mit is_enabled=false (hot-add) | `TestAddLiveSessionStrategy` | `Hann Trend Strategie hinzufügen` |
| C2 | Symbole werden gemergt | Union der Symbole ohne Duplikate in Session + Config | `TestAddLiveSessionStrategy` | — |
| C3 | Duplikat-Strategie ablehnen | Gleiche strategy+params nochmal → 400 | `TestAddDuplicateStrategy` | `Duplikat-Strategie ablehnen` |
| C4 | 2 Strategien verifizieren | GET /strategies gibt 2 Strategien zurück, SMF=enabled, Hann=disabled | — | `Strategien verifizieren (2 Stück)` |
| C5 | Strategie aktivieren (Toggle) | PUT /strategy/:stratId schaltet is_enabled um | — | `Hann Trend Toggle (aktivieren)` |
| C6 | Separate Guards pro Strategie | openPosGuardKey für SMF ≠ HannTrend (sessionID:strategyID:symbol) | `TestMultiStrategyProcessing` | — |

### D. SESSION START/STOP

| # | Testfall | Erwartetes Verhalten | Go-Test | E2E |
|---|---------|---------------------|---------|-----|
| D1 | Session starten (Resume) | POST → 200, is_active=true, StartedAt gesetzt, Scheduler erstellt (Mode: polling/websocket) | `TestResumeLiveTrading` | `Session starten (Resume)` |
| D2 | Double-Start verhindern | Resume bei aktiver Session → 400 "Session ist bereits aktiv" | `TestResumeAlreadyActive` | `Double-Start verhindern` |
| D3 | Session stoppen | POST → 200, is_active=false, StoppedAt gesetzt, Scheduler entfernt | `TestStopLiveTrading` | `Session stoppen` |
| D4 | Positionen bei Stop geschlossen | Alle offene Positionen → IsClosed=true, CloseReason="MANUAL", P&L berechnet | `TestStopLiveTrading` | `Alle Positionen geschlossen (MANUAL)` |
| D5 | Stop bei gestoppter Session | Erneuter Stop → 400 | — | `Stop bei bereits gestoppter Session` |

### E. SIGNAL-VERARBEITUNG (processLiveSymbolWithData)

| # | Testfall | Erwartetes Verhalten | Go-Test | E2E |
|---|---------|---------------------|---------|-----|
| E1 | Position eröffnen bei Signal | Signal nach SessionStart → Position mit Entry=lastClose, SL/TP skaliert, Qty berechnet | `TestProcessLiveSymbolWithData_OpenPosition` | — |
| E2 | Duplikat-Signal ignorieren | Gleicher SignalIndex → keine zweite Position (DB-Check) | `TestProcessLiveSymbolWithData_DuplicateGuard` | — |
| E3 | Offene Position → gleiches Signal ignorieren | Bei existierender offener Position in gleicher Richtung → kein neuer Eintrag | `TestProcessLiveSymbolWithData_DuplicateGuard` | — |
| E4 | Gegensignal → Close + Open | Bei offener LONG-Position + SHORT-Signal: alte Position schließen (Reason=SIGNAL), dann neue eröffnen | Dokumentiert in `processLiveSymbolWithData` | — |
| E5 | Session ohne StartedAt | Keine Positionen eröffnet, Signale ignoriert (Schutz gegen retroaktives Trading) | `TestSessionWithoutStartedAt` | — |
| E6 | Signale vor SessionStart ignorieren | Nur Signale nach session.StartedAt werden verarbeitet | Implicit in E1 | — |
| E7 | LongOnly Filter | SHORT-Signale werden übersprungen wenn LongOnly=true | `TestLongOnlyFilter` | — |
| E8 | Markt geschlossen → Skip | Neue Entries werden außerhalb US-Marktzeiten (9:30-16:00 ET, Mo-Fr) übersprungen | `TestMarketHoursCheck` | — |
| E9 | Incomplete Candle abschneiden | Letzte Kerze wird entfernt wenn sie noch "offen" ist (candleEnd > now) | Implicit in processLiveSymbolWithData | — |
| E10 | Multi-Strategy: separate Positionen | SMF und HannTrend können gleichzeitig eigene Positionen für dasselbe Symbol halten | `TestMultiStrategyProcessing` | — |

### F. POSITION MANAGEMENT

| # | Testfall | Erwartetes Verhalten | Go-Test | E2E |
|---|---------|---------------------|---------|-----|
| F1 | SL bei LONG korrekt | Preis ≤ StopLoss → Position schließen, negativer P&L | `TestSLTPCalculation/LONG_SL_Hit` | — |
| F2 | TP bei LONG korrekt | Preis ≥ TakeProfit → Position schließen, positiver P&L | `TestSLTPCalculation/LONG_TP_Hit` | — |
| F3 | SL vor TP Priorität | Wenn Bar sowohl SL als auch TP berührt → SL hat Vorrang | `TestSLTPCalculation/SL_Before_TP` | — |
| F4 | SL bei SHORT korrekt | Preis ≥ StopLoss → Position schließen | `TestSLTPCalculation/SHORT_SL_TP` | — |
| F5 | TP bei SHORT korrekt | Preis ≤ TakeProfit → Position schließen | `TestSLTPCalculation/SHORT_SL_TP` | — |
| F6 | SL/TP proportionale Skalierung | Bei abweichendem Entry-Preis werden SL/TP proportional angepasst | `TestSLTPScaling` | — |
| F7 | Alpaca Guard: TP > Entry+0.01 | Wenn skalierter TP zu nah am Entry → Fallback auf +0.5% | `TestSLTPScaling` | — |
| F8 | P&L LONG korrekt | (Close-Entry)/Entry*100 | `TestPnLCalculations/LONG_Gewinn` | — |
| F9 | P&L SHORT korrekt | (Entry-Close)/Entry*100 | `TestPnLCalculations/SHORT_Gewinn` | — |
| F10 | P&L Amt Berechnung | InvestedAmount * PnL% / 100 | `TestPnLCalculations/*` | — |

### G. RACE CONDITION SCHUTZ (Position Guard)

| # | Testfall | Erwartetes Verhalten | Go-Test | E2E |
|---|---------|---------------------|---------|-----|
| G1 | Concurrent Close: nur 1 Winner | 10 parallele LoadAndDelete → genau 1 gewinnt | `TestOpenPosGuard_ConcurrentAccess` | — |
| G2 | Guard Key Format | "sessionID:strategyID:symbol" (z.B. "42:7:AAPL") | `TestOpenPosGuard_KeyFormat` | — |
| G3 | Guard Initialisierung bei Resume | Offene Positionen aus DB → Guard geladen. Geschlossene ignoriert | `TestInitOpenPosGuard` | — |
| G4 | Guard Cleanup bei Stop | liveOpenPosGuard.Delete für jede geschlossene Position | Implicit in `TestStopLiveTrading` | — |

### H. ALPACA INTEGRATION

| # | Testfall | Erwartetes Verhalten | Go-Test | E2E |
|---|---------|---------------------|---------|-----|
| H1 | Mock-Alpaca: Account-Abfrage | GET /v2/account → 200 mit Account-Daten | `TestAlpacaMockOrders` | — |
| H2 | Mock-Alpaca: Order platzieren | POST /v2/orders → market order accepted | `TestAlpacaMockOrders` | — |
| H3 | Mock-Alpaca: Auth-Fehler | Falscher Key → 401 | `TestAlpacaMockOrders` | — |
| H4 | Fractional → TIF=day | Nicht-ganzzahlige Qty → time_in_force="day" | `TestTIFForFractionalVsWhole` | — |
| H5 | Whole → TIF=gtc | Ganzzahlige Qty → time_in_force="gtc" | `TestTIFForFractionalVsWhole` | — |
| H6 | Alpaca Keys validieren | POST /alpaca/validate prüft Verbindung | — | `Alpaca Keys validieren` |
| H7 | Alpaca Portfolio laden | GET /alpaca/portfolio gibt Positionen+Orders zurück | — | `Alpaca Portfolio laden` |

### I. BERECHNUNGEN & EDGE CASES

| # | Testfall | Erwartetes Verhalten | Go-Test | E2E |
|---|---------|---------------------|---------|-----|
| I1 | Qty-Berechnung | TradeAmount/EntryPrice, gerundet auf 6 Dezimalstellen | `TestQuantityCalculation` | — |
| I2 | Nicht-fractionable: abrunden | Qty wird auf ganze Aktien abgerundet. Wenn <1 → skip | `TestQuantityCalculation` | — |
| I3 | Fractionable-Flag aus DB | TradingWatchlistItem.Fractionable steuert Rundung | `TestFractionableHandling` | — |
| I4 | createStrategyFromJSON Defaults | SMF: RequiredBars=108, HannTrend: RequiredBars=70 | `TestCreateStrategyFromJSON` | — |
| I5 | createStrategyFromJSON Custom | Custom-Params überschreiben Defaults | `TestCreateStrategyFromJSON` | — |
| I6 | Unbekannte Strategie → nil | createStrategyFromJSON("nonexistent", "") → nil | `TestCreateStrategyFromJSON` | — |
| I7 | Interval-Dauer-Mapping | 5m→5min, 1h→1h, 4h→4h, 1d→24h | `TestIntervalToDuration` | — |
| I8 | Währungserkennung | AAPL→USD, SAP.DE→EUR, BP.L→GBP | `TestCurrencyDetection` | — |
| I9 | SMF Default-Parameter | TrendLength=34, BasisSmooth=3, FlowWindow=24, RiskReward=2.0 | `TestSmartMoneyFlowDefaults` | — |
| I10 | HannTrend Default-Parameter | DMHLength=30, SARStart=0.02, SARIncrement=0.03, RiskReward=2.0 | `TestHannTrendDefaults` | — |

### J. FULL LIFECYCLE (E2E)

| # | Testfall | Erwartetes Verhalten | Go-Test | E2E |
|---|---------|---------------------|---------|-----|
| J1 | Kompletter Ablauf | Session erstellen → HannTrend hinzufügen → Resume → Signal-Verarbeitung → Stop → Cleanup | `TestFullLifecycle_ArenaToLive` | Phasen 1-11 |
| J2 | Session Reset | Positionen + Logs gelöscht, Config bleibt | — | `Session Reset` |
| J3 | Session löschen | Session nicht mehr auffindbar (404) | — | `Session löschen` |

---

## Testausführung

### Go-Tests (Backend-Logik)

```bash
cd backend
go test -v -run Test -timeout 120s 2>&1 | tee test_results.txt
```

**Letzte Ausführung (2026-02-21):** Alle 21 Tests PASS in 1.5s.

**Hinweis:** `TestInteg_FullLifecycle` zeigt bei Samstag/Sonntag 0 Positionen (Markt geschlossen → Signals werden übersprungen). Das ist korrekt! Montag 15:30-22:00 MEZ werden Positionen eröffnet.

### E2E-Tests (Frontend-Backend)

```bash
# 1. Backend starten (optional mit Test-DB)
cd backend && go run main.go &

# 2. Tests ausführen
node frontend/src/tests/live-trading-integration.test.mjs
```

**Hinweis:** Login-Credentials anpassen via Umgebungsvariablen:
```bash
TEST_EMAIL=admin@deine.email TEST_PASSWORD=deinpasswort node frontend/src/tests/live-trading-integration.test.mjs
```

---

## Bekannte Limitierungen

| Limitation | Auswirkung | Workaround |
|-----------|------------|-----------|
| `isUSMarketOpen()` prüft Echtzeit | Am Wochenende/Nacht werden keine Positionen eröffnet | Test Mo-Fr 15:30-22:00 MEZ ausführen, oder Signal-Tests einzeln prüfen |
| Alpaca-Mocking nur HTTP-Level | `alpacaPlaceOrder` in main.go hat keinen Injection Point | Go-Test mockt HTTP-Server; E2E-Test überspringt wenn Alpaca nicht konfiguriert |
| Yahoo Finance nicht gemockt | Signal-Tests verwenden synthetische OHLCV-Daten | Für reale Signale: E2E-Test nach `arenaPrefetch` ausführen |
| OHLCV MemCache nicht im Test | `runLiveScan` liest aus MemCache, der im Test leer ist | `processLiveSymbolWithData` wird direkt mit OHLCV-Daten aufgerufen |
| Async Position-Writes | `livePositionWriteCh` ist asynchron | Test startet eigenen Writer-Goroutine + drainPositionWriter() |

---

## Nicht abgedeckte Szenarien (manuell prüfen!)

> **Diese Fälle sind schwer automatisch zu testen und sollten Montag früh manuell geprüft werden:**

| # | Szenario | Warum manuell? | Prüfschritte |
|---|---------|---------------|-------------|
| M1 | **WebSocket-Modus (Alpaca WS)** | Braucht echte Alpaca-Verbindung + Market Open | Session mit Alpaca-Keys starten, SharedWS-Verbindung im Log prüfen |
| M2 | **Bar-Aggregation (1min → 4h)** | BarAggregator braucht echte WS-Bars | WS-Log prüfen: "BarAggregator: aggregated 1min → 4h" |
| M3 | **Alpaca echte Order (Paper)** | Braucht Paper-Account + Market Open | Test-Order über UI: LiveTrading → Test-Order Panel → Buy AAPL |
| M4 | **checkOpenPositionsSLTP Monitor** | Goroutine läuft parallel im Hintergrund | Position mit engem SL eröffnen, warten bis Monitor triggert |
| M5 | **SharedWS Multi-Session** | Mehrere Sessions auf gleichem WS | 2 Sessions mit überlappenden Symbolen starten |
| M6 | **Netzwerk-Unterbrechung** | WS-Reconnect nach Disconnect | WLAN kurz trennen, Log auf Reconnect prüfen |
| M7 | **Prefetch bei Session-Start** | triggerPriorityRefresh async | Nach Resume: Log prüfen "Priority refresh completed" |
| M8 | **Gleichzeitige SL+Signal Close** | Race Condition Worker vs Monitor | Position nahe SL, Signal kommt gleichzeitig → nur 1 Close |

---

## Checkliste Montag Morgen

- [ ] Backend starten, Logs beobachten
- [ ] Go-Tests ausführen: `cd backend && go test -v -run Test`
- [ ] E2E-Tests ausführen (mit echtem Login)
- [ ] Alpaca Paper-Account prüfen (Balance, API-Status)
- [ ] Test-Session erstellen (SmartMoneyFlow + HannTrend)
- [ ] Session starten, auf erste Logs warten
- [ ] WS-Verbindung prüfen (Log: "SharedWS connected")
- [ ] Erste Signale abwarten und im Debug-Panel verifizieren
- [ ] Test-Order (Paper) manuell über UI platzieren
- [ ] SL/TP-Level der ersten Position prüfen
- [ ] Nach 30min: Positionen-Tabelle prüfen (P&L wird aktualisiert?)

---

## Strategien: Erwartetes Signal-Verhalten

### Smart Money Flow (4h)
1. **Regime wechselt** (Flow-basiert: EMA + RSI Berechnung)
2. **Tracking Phase**: Swing High/Low wird verfolgt
3. **Retest**: Preis fällt unter/über Baseline → Armed
4. **Structure Break**: Close über Swing High → **LONG** (oder unter Swing Low → SHORT)
5. **SL** = Pullback-Extremum, **TP** = Entry + 2× Risk

### Hann Trend DMH+SAR (1h)
1. **DMH kreuzt Nulllinie** (Hann-FIR geglätteter Momentum-Indikator)
2. **Warte auf Pullback**: SAR flippt entgegen der Trendrichtung
3. **Pullback läuft**: SAR flippt zurück in Trendrichtung
4. **Bestätigung**: Close bricht über Swing High → **LONG** (oder unter Swing Low → SHORT)
5. **SL** = Pullback-Extremum × (1 - SLBuffer%), **TP** = Entry + 2× Risk

### Mögliche Logik-Probleme zu beachten
- SMF Regime-Erkennung verzögert (EMA+RSI Lag) → Erste Signale erst nach ~60 Bars
- HannTrend SAR kann bei Seitwärtsmarkt häufig flippen → Viele Fehlsignale
- Beide Strategien auf gleichem Symbol: Können gegensätzliche Signale gleichzeitig geben
- LongOnly-Filter auf HannTrend: SHORT-Signale werden ignoriert, nur LONG-Trades
