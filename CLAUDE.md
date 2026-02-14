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

## Self-Update Regel
Wenn du während einer Session feststellst, dass diese CLAUDE.md **veraltet oder unvollständig** ist (z.B. neue Komponenten, geänderte Architektur, neue Pfade, neue Kommandos), dann **aktualisiere sie proaktiv** am Ende der Aufgabe — ohne dass ich es extra sagen muss. Halte sie dabei kompakt (<80 Zeilen).
