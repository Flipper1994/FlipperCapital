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
- **BXtrender-Modi:** Defensiv, Aggressiv, Quant, Ditz, Trader — jeder hat eigene Trade-Logik
- **Signal-Bestimmung** (alle Modi einheitlich): BUY=frisch, HOLD=Position>1 Monat, SELL=frisch, WAIT=keine Position+SELL>1 Monat
- **Trade-Ausführung** (alle Modi): Signal basiert auf fertiger Monatskerze, Trade zum Open des Folgemonats
- **Bots:** FlipperBot (defensiv), Lutz (aggressiv), Quant, Ditz, Trader — alle nutzen `isStockAllowedForBot()` für Allowlist
- **Performance-Daten:** Frontend berechnet BXtrender → sendet an Backend → Backend speichert in DB. Backend hat eigene Server-Berechnung für Batch-Updates.

## Self-Update Regel
Wenn du während einer Session feststellst, dass diese CLAUDE.md **veraltet oder unvollständig** ist (z.B. neue Komponenten, geänderte Architektur, neue Pfade, neue Kommandos), dann **aktualisiere sie proaktiv** am Ende der Aufgabe — ohne dass ich es extra sagen muss. Halte sie dabei kompakt (<80 Zeilen).
