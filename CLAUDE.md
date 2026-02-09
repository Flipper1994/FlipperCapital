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

## Bot-Trading-Regeln ⚠️ Bei Änderungen an Bot-Logik MUSS dieser Abschnitt aktualisiert werden!

### Gemeinsam (alle 5 Bots)
- **Investment:** 100 EUR pro Trade (EUR→USD konvertiert)
- **Stop-Loss:** 20% Trailing Stop Loss (default). `HighestPrice` wird täglich aktualisiert, Trigger: `currentPrice ≤ HighestPrice * 0.8`. Konfigurierbar (TSL oder Fixed).
- **Allowlist:** `isStockAllowedForBot(botName, symbol)` — kein DB-Eintrag = erlaubt, sonst `Allowed`-Flag
- **Bot-Filter:** Optional pro Bot: MinWinrate, MinRR, MinAvgReturn, MinMarketCap — blockierter Trade wird als `IsFilterBlocked=true` gespeichert
- **Re-Entry:** Nur nach vollständigem Zyklus (BUY→SELL). Soft-deleted BUYs blockieren Re-Entry.
- **Max Positionen:** Nicht limitiert
- **User-IDs:** FlipperBot=999999, Lutz=999998, Quant=999997, Ditz=999996, Trader=999995

### FlipperBot (Defensiv)
- **BUY:** Red→Green Übergang ODER 4. aufeinanderfolgende "light red" Bar (negativ aber steigend: `value < 0 && value > prev`)
- **SELL:** Erste "dark red" Bar (negativ und fallend: `value < 0 && value ≤ prev`)
- **MA-Filter:** Nein
- **Charakter:** Konservativ, wartet auf Bestätigung bevor Kauf

### Lutz (Aggressiv)
- **BUY:** 1. "light red" Bar ODER Red→Green Übergang — kauft früher als FlipperBot
- **SELL:** Erste "dark red" Bar (identisch zu FlipperBot)
- **MA-Filter:** Nein
- **Charakter:** Aggressiv, steigt beim ersten Erholungszeichen ein

### Quant
- **BUY:** `short > 0 AND long > 0` (beide Indikatoren positiv) + vorher mindestens einer negativ + `price > EMA(200)` (MA-Filter ON)
- **SELL:** `short < 0 OR long < 0` (mindestens ein Indikator negativ) ODER TSL
- **MA-Filter:** Ja, 200-EMA (default, konfigurierbar EMA/SMA)
- **Charakter:** Trendfolger, doppelte Bestätigung, verkauft schnell bei Schwäche

### Ditz
- **BUY:** `short > 0 AND long > 0` + vorher nicht beide positiv + `price > EMA(200)` (MA-Filter ON)
- **SELL:** `short < 0 AND long < 0` (BEIDE negativ) ODER TSL
- **MA-Filter:** Ja, 200-EMA (default)
- **Charakter:** Wie Quant aber konservativer Exit — hält Positionen länger (SELL erst wenn beide negativ)

### Trader
- **BUY:** T3 Signal-Linie wechselt von fallend→steigend (Rot→Grün) — **KEIN MA-Filter**
- **SELL:** T3 Signal-Linie wechselt von steigend→fallend (Grün→Rot) ODER TSL
- **Signal-Linie:** T3(Short Xtrender, 5) — geglätteter Short Xtrender (Tillson T3, b=0.7)
- **MA-Filter:** Nein
- **Charakter:** Schnellster Bot — handelt jeden Farbwechsel der Signal-Linie, mehr Trades als Ditz/Quant

## Self-Update Regel
Wenn du während einer Session feststellst, dass diese CLAUDE.md **veraltet oder unvollständig** ist (z.B. neue Komponenten, geänderte Architektur, neue Pfade, neue Kommandos), dann **aktualisiere sie proaktiv** am Ende der Aufgabe — ohne dass ich es extra sagen muss. Halte sie dabei kompakt (<80 Zeilen).
