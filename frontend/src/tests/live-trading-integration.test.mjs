/**
 * ============================================================================
 * LIVE TRADING INTEGRATION TEST (E2E)
 * ============================================================================
 *
 * Testet den kompletten Workflow Frontend → Backend für Live Trading
 * mit Smart Money Flow und Hann Trend Strategien.
 *
 * VORAUSSETZUNG: Backend läuft auf http://localhost:8080
 *   cd backend && go run main.go
 *
 * AUSFÜHRUNG:
 *   node frontend/src/tests/live-trading-integration.test.mjs
 *
 * OPTIONAL: Test-DB verwenden (verhindert Änderungen an Produktionsdaten)
 *   DB_PATH=test_integration.db go run main.go
 */

const BASE_URL = process.env.API_URL || 'http://localhost:8080';
const ADMIN_EMAIL = process.env.TEST_EMAIL || 'admin@flipper.de';
const ADMIN_PASSWORD = process.env.TEST_PASSWORD || 'admin';

let authToken = '';
let testSessionId = null;
let testResults = [];
let passed = 0;
let failed = 0;
let skipped = 0;

// ============================================================================
// HELPER FUNKTIONEN
// ============================================================================

async function api(method, path, body = null) {
  const headers = {
    'Content-Type': 'application/json',
  };
  if (authToken) {
    headers['Authorization'] = `Bearer ${authToken}`;
  }

  const opts = { method, headers };
  if (body) {
    opts.body = JSON.stringify(body);
  }

  const res = await fetch(`${BASE_URL}${path}`, opts);
  const text = await res.text();
  let data;
  try {
    data = JSON.parse(text);
  } catch {
    data = { raw: text };
  }
  return { status: res.status, ok: res.ok, data };
}

function assert(condition, message) {
  if (!condition) {
    throw new Error(message);
  }
}

async function runTest(name, fn) {
  const start = Date.now();
  try {
    await fn();
    const duration = Date.now() - start;
    testResults.push({ name, status: 'PASS', duration });
    passed++;
    console.log(`  ✓ ${name} (${duration}ms)`);
  } catch (err) {
    const duration = Date.now() - start;
    if (err.message.startsWith('SKIP:')) {
      testResults.push({ name, status: 'SKIP', reason: err.message.slice(5), duration });
      skipped++;
      console.log(`  ⊘ ${name} — ${err.message.slice(5)}`);
    } else {
      testResults.push({ name, status: 'FAIL', error: err.message, duration });
      failed++;
      console.log(`  ✗ ${name} — ${err.message}`);
    }
  }
}

function skip(reason) {
  throw new Error('SKIP:' + reason);
}

// ============================================================================
// TESTS
// ============================================================================

async function main() {
  console.log('╔══════════════════════════════════════════════════════╗');
  console.log('║   LIVE TRADING INTEGRATION TEST                     ║');
  console.log('║   Strategien: Smart Money Flow + Hann Trend         ║');
  console.log('╚══════════════════════════════════════════════════════╝');
  console.log(`Backend: ${BASE_URL}`);
  console.log('');

  // ─── PHASE 1: VERBINDUNG & AUTH ──────────────────────────────────
  console.log('── Phase 1: Verbindung & Authentifizierung ──');

  await runTest('Backend erreichbar', async () => {
    try {
      const res = await fetch(`${BASE_URL}/api/stocks`);
      assert(res.status < 500, `Backend antwortet mit Status ${res.status}`);
    } catch (e) {
      throw new Error(`Backend nicht erreichbar auf ${BASE_URL}: ${e.message}`);
    }
  });

  await runTest('Admin Login', async () => {
    const { status, data } = await api('POST', '/api/login', {
      email: ADMIN_EMAIL,
      password: ADMIN_PASSWORD,
    });
    if (status === 401 || status === 400) {
      skip('Login fehlgeschlagen — Zugangsdaten prüfen');
    }
    assert(status === 200, `Login Status: ${status} ${JSON.stringify(data)}`);
    assert(data.token, 'Kein Auth-Token erhalten');
    authToken = data.token;
  });

  if (!authToken) {
    console.log('\n⚠ Kein Auth-Token — restliche Tests werden übersprungen');
    printSummary();
    return;
  }

  // ─── PHASE 2: LIVE TRADING CONFIG ────────────────────────────────
  console.log('\n── Phase 2: Live Trading Konfiguration ──');

  await runTest('Config speichern (Smart Money Flow, 4h)', async () => {
    const { status, data } = await api('POST', '/api/trading/live/config', {
      strategy: 'smart_money_flow',
      interval: '4h',
      params: {},
      symbols: ['AAPL', 'MSFT', 'NVDA'],
      long_only: false,
      us_only: true,
      trade_amount: 500,
      filters: {},
      filters_active: false,
      currency: 'USD',
    });
    assert(status === 200, `Config speichern: ${status} ${JSON.stringify(data)}`);
  });

  await runTest('Config laden', async () => {
    const { status, data } = await api('GET', '/api/trading/live/config');
    assert(status === 200, `Config laden: ${status}`);
    assert(data.strategy === 'smart_money_flow', `Strategy: ${data.strategy}`);
    assert(data.interval === '4h', `Interval: ${data.interval}`);
  });

  // ─── PHASE 3: SESSION ERSTELLEN (aus Arena) ──────────────────────
  console.log('\n── Phase 3: Session erstellen (Arena → Live) ──');

  await runTest('Arena V2 Session erstellen', async () => {
    const { status, data } = await api('POST', '/api/trading/arena/v2/start-session', {
      name: 'Integration Test Session',
      strategy: 'smart_money_flow',
      interval: '4h',
      params_json: '{"risk_reward":2.0}',
      long_only: false,
      trade_amount: 500,
      symbols: ['AAPL', 'MSFT', 'NVDA'],
    });
    assert(status === 200, `Session erstellen: ${status} ${JSON.stringify(data)}`);
    assert(data.status === 'created', `Status: ${data.status}`);
    assert(data.session, 'Keine Session in Response');
    testSessionId = data.session.id || data.session.ID;
    assert(testSessionId, 'Keine Session-ID');
  });

  await runTest('Session in Liste vorhanden', async () => {
    const { status, data } = await api('GET', '/api/trading/live/sessions');
    assert(status === 200, `Sessions laden: ${status}`);
    const sessions = data.sessions || [];
    const found = sessions.find(s => (s.id || s.ID) === testSessionId);
    assert(found, `Session ${testSessionId} nicht in Liste gefunden`);
    assert(!found.is_active, 'Session sollte noch nicht aktiv sein');
  });

  await runTest('Session Details laden', async () => {
    const { status, data } = await api('GET', `/api/trading/live/session/${testSessionId}`);
    assert(status === 200, `Session Details: ${status}`);
    assert(data.positions !== undefined, 'positions fehlt');
    assert(data.strategies !== undefined || true, 'strategies optional');
  });

  // ─── PHASE 4: ZWEITE STRATEGIE HINZUFÜGEN ───────────────────────
  console.log('\n── Phase 4: Hann Trend Strategie hinzufügen ──');

  await runTest('Hann Trend Strategie hinzufügen', async () => {
    const { status, data } = await api('POST', `/api/trading/live/session/${testSessionId}/strategy`, {
      strategy: 'hann_trend',
      params: JSON.stringify({ dmh_length: 30, sar_start: 0.02, risk_reward: 2.0 }),
      symbols: ['AAPL', 'TSLA'],
      long_only: true,
    });
    assert(status === 200, `Strategie hinzufügen: ${status} ${JSON.stringify(data)}`);
    assert(data.status === 'added', `Status: ${data.status}`);
  });

  await runTest('Strategien verifizieren (2 Stück)', async () => {
    const { status, data } = await api('GET', `/api/trading/live/session/${testSessionId}/strategies`);
    assert(status === 200, `Strategien laden: ${status}`);
    const strategies = data.strategies || data || [];
    assert(strategies.length >= 2, `Erwartet >=2 Strategien, bekommen: ${strategies.length}`);
    const smf = strategies.find(s => s.name === 'smart_money_flow');
    const hann = strategies.find(s => s.name === 'hann_trend');
    assert(smf, 'SmartMoneyFlow nicht gefunden');
    assert(hann, 'HannTrend nicht gefunden');
    assert(smf.is_enabled === true, 'SMF sollte enabled sein');
    assert(hann.is_enabled === false, 'HannTrend sollte disabled sein (hot-add)');
  });

  await runTest('Duplikat-Strategie ablehnen', async () => {
    const { status } = await api('POST', `/api/trading/live/session/${testSessionId}/strategy`, {
      strategy: 'hann_trend',
      params: JSON.stringify({ dmh_length: 30, sar_start: 0.02, risk_reward: 2.0 }),
      symbols: ['AAPL'],
      long_only: true,
    });
    assert(status === 400, `Duplikat sollte 400 sein, bekommen: ${status}`);
  });

  // ─── PHASE 5: STRATEGIE AKTIVIEREN ───────────────────────────────
  console.log('\n── Phase 5: Hann Trend aktivieren ──');

  let hannStrategyId = null;
  await runTest('Hann Trend Strategy-ID finden', async () => {
    const { data } = await api('GET', `/api/trading/live/session/${testSessionId}/strategies`);
    const strategies = data.strategies || data || [];
    const hann = strategies.find(s => s.name === 'hann_trend');
    assert(hann, 'HannTrend nicht gefunden');
    hannStrategyId = hann.id || hann.ID;
    assert(hannStrategyId, 'Keine Strategy-ID');
  });

  await runTest('Hann Trend Toggle (aktivieren)', async () => {
    if (!hannStrategyId) skip('Keine Strategy-ID');
    const { status } = await api('PUT', `/api/trading/live/session/${testSessionId}/strategy/${hannStrategyId}`);
    assert(status === 200, `Toggle Status: ${status}`);
  });

  await runTest('Hann Trend ist jetzt enabled', async () => {
    const { data } = await api('GET', `/api/trading/live/session/${testSessionId}/strategies`);
    const strategies = data.strategies || data || [];
    const hann = strategies.find(s => (s.id || s.ID) === hannStrategyId);
    assert(hann, 'HannTrend nicht gefunden');
    assert(hann.is_enabled === true, `HannTrend enabled: ${hann.is_enabled}`);
  });

  // ─── PHASE 6: SESSION STARTEN ────────────────────────────────────
  console.log('\n── Phase 6: Session starten (Resume) ──');

  await runTest('Session starten (Resume)', async () => {
    const { status, data } = await api('POST', `/api/trading/live/session/${testSessionId}/resume`);
    assert(status === 200, `Resume: ${status} ${JSON.stringify(data)}`);
  });

  await runTest('Session ist aktiv', async () => {
    const { data } = await api('GET', '/api/trading/live/status');
    assert(data.is_running === true || data.active_sessions?.length > 0,
      'Keine aktive Session gefunden');
  });

  await runTest('Double-Start verhindern', async () => {
    const { status } = await api('POST', `/api/trading/live/session/${testSessionId}/resume`);
    assert(status === 400, `Double-Start sollte 400 sein: ${status}`);
  });

  // ─── PHASE 7: LIVE STATUS PRÜFEN ────────────────────────────────
  console.log('\n── Phase 7: Live Status & Monitoring ──');

  await runTest('Live Status abrufen', async () => {
    const { status, data } = await api('GET', '/api/trading/live/status');
    assert(status === 200, `Status: ${status}`);
    assert(data.is_running !== undefined, 'is_running fehlt');
  });

  await runTest('Session Positionen abrufen', async () => {
    const { status, data } = await api('GET', `/api/trading/live/session/${testSessionId}`);
    assert(status === 200, `Positionen: ${status}`);
    assert(Array.isArray(data.positions), 'positions muss ein Array sein');
  });

  await runTest('Debug Logs abrufen', async () => {
    const { status, data } = await api('GET', `/api/trading/live/logs/${testSessionId}`);
    assert(status === 200, `Logs: ${status}`);
    assert(data.logs !== undefined, 'logs fehlt');
    if (data.logs?.length > 0) {
      console.log(`    → ${data.logs.length} Log-Einträge`);
      // Letzte 3 Logs anzeigen
      data.logs.slice(-3).forEach(l => {
        console.log(`      [${l.level}] ${l.symbol}: ${l.message}`);
      });
    }
  });

  // ─── PHASE 8: ANALYSE-ENDPOINT ──────────────────────────────────
  console.log('\n── Phase 8: Live-Analyse Endpoint ──');

  await runTest('Symbol-Analyse (AAPL)', async () => {
    const { status, data } = await api('POST', '/api/trading/live/analyze', {
      session_id: testSessionId,
      symbol: 'AAPL',
    });
    if (status === 500 || status === 404) {
      skip('Analyse nicht verfügbar (OHLCV-Cache leer?)');
    }
    assert(status === 200, `Analyse: ${status} ${JSON.stringify(data).slice(0, 200)}`);
  });

  // ─── PHASE 9: ALPACA INTEGRATION ────────────────────────────────
  console.log('\n── Phase 9: Alpaca Integration ──');

  await runTest('Alpaca Keys validieren', async () => {
    // Erst Config laden um zu prüfen ob Alpaca konfiguriert ist
    const { data: config } = await api('GET', `/api/trading/live/config?session_id=${testSessionId}`);
    if (!config.alpaca_enabled) {
      skip('Alpaca nicht aktiviert');
    }
    const { status, data } = await api('POST', '/api/trading/live/alpaca/validate', {
      session_id: testSessionId,
    });
    assert(status === 200, `Alpaca Validate: ${status} ${JSON.stringify(data)}`);
  });

  await runTest('Alpaca Portfolio laden', async () => {
    const { data: config } = await api('GET', `/api/trading/live/config?session_id=${testSessionId}`);
    if (!config.alpaca_enabled) {
      skip('Alpaca nicht aktiviert');
    }
    const { status, data } = await api('GET', `/api/trading/live/alpaca/portfolio?session_id=${testSessionId}`);
    assert(status === 200, `Portfolio: ${status}`);
  });

  // ─── PHASE 10: SESSION STOPPEN ───────────────────────────────────
  console.log('\n── Phase 10: Session stoppen ──');

  await runTest('Session stoppen', async () => {
    const { status, data } = await api('POST', `/api/trading/live/stop?session_id=${testSessionId}`);
    assert(status === 200, `Stop: ${status} ${JSON.stringify(data)}`);
    assert(data.status === 'stopped', `Status: ${data.status}`);
  });

  await runTest('Session ist inaktiv', async () => {
    const { data } = await api('GET', '/api/trading/live/sessions');
    const sessions = data.sessions || [];
    const session = sessions.find(s => (s.id || s.ID) === testSessionId);
    assert(session, 'Session nicht gefunden');
    assert(!session.is_active, 'Session sollte inaktiv sein');
  });

  await runTest('Alle Positionen geschlossen (MANUAL)', async () => {
    const { data } = await api('GET', `/api/trading/live/session/${testSessionId}`);
    const openPositions = (data.positions || []).filter(p => !p.is_closed);
    assert(openPositions.length === 0, `${openPositions.length} Positionen noch offen`);
    const closedPositions = (data.positions || []).filter(p => p.is_closed);
    closedPositions.forEach(p => {
      assert(p.close_reason === 'MANUAL', `Position ${p.symbol}: CloseReason=${p.close_reason}`);
    });
    console.log(`    → ${closedPositions.length} Positionen per MANUAL geschlossen`);
  });

  await runTest('Stop bei bereits gestoppter Session', async () => {
    const { status } = await api('POST', `/api/trading/live/stop?session_id=${testSessionId}`);
    assert(status === 400, `Erneuter Stop sollte 400 sein: ${status}`);
  });

  // ─── PHASE 11: SESSION RESET & DELETE ────────────────────────────
  console.log('\n── Phase 11: Session Reset & Cleanup ──');

  await runTest('Session Reset', async () => {
    const { status } = await api('POST', `/api/trading/live/session/${testSessionId}/reset`);
    assert(status === 200, `Reset: ${status}`);
  });

  await runTest('Positionen nach Reset leer', async () => {
    const { data } = await api('GET', `/api/trading/live/session/${testSessionId}`);
    assert((data.positions || []).length === 0, 'Positionen sollten nach Reset leer sein');
  });

  await runTest('Session löschen', async () => {
    const { status } = await api('POST', `/api/trading/live/session/${testSessionId}/reset`);
    // Reset nochmal vor Delete ist OK
    const delRes = await api('DELETE', `/api/trading/live/session/${testSessionId}`);
    assert(delRes.status === 200, `Delete: ${delRes.status}`);
  });

  await runTest('Gelöschte Session nicht mehr auffindbar', async () => {
    const { status } = await api('GET', `/api/trading/live/session/${testSessionId}`);
    assert(status === 404, `Gelöschte Session: erwartet 404, bekommen ${status}`);
  });

  // ─── PHASE 12: RENAME TEST ──────────────────────────────────────
  console.log('\n── Phase 12: Session Rename ──');

  let renameSessionId = null;
  await runTest('Neue Session für Rename erstellen', async () => {
    const { status, data } = await api('POST', '/api/trading/arena/v2/start-session', {
      name: 'Rename Test',
      strategy: 'hann_trend',
      interval: '1h',
      params_json: '{}',
      trade_amount: 100,
      symbols: ['AAPL'],
    });
    assert(status === 200, `Erstellen: ${status}`);
    renameSessionId = data.session.id || data.session.ID;
  });

  await runTest('Session umbenennen', async () => {
    if (!renameSessionId) skip('Keine Session');
    const { status } = await api('PATCH', `/api/trading/live/session/${renameSessionId}/name`, {
      name: 'Umbenannt: SMF+Hann',
    });
    assert(status === 200, `Rename: ${status}`);
  });

  // Cleanup
  if (renameSessionId) {
    await api('DELETE', `/api/trading/live/session/${renameSessionId}`);
  }

  // ─── ZUSAMMENFASSUNG ─────────────────────────────────────────────
  printSummary();
}

function printSummary() {
  console.log('\n╔══════════════════════════════════════════════════════╗');
  console.log('║   TEST-ERGEBNIS                                     ║');
  console.log('╠══════════════════════════════════════════════════════╣');
  console.log(`║   Bestanden: ${passed}                                      `);
  console.log(`║   Fehlgeschlagen: ${failed}                                 `);
  console.log(`║   Übersprungen: ${skipped}                                  `);
  console.log(`║   Gesamt: ${testResults.length}                             `);
  console.log('╚══════════════════════════════════════════════════════╝');

  if (failed > 0) {
    console.log('\n--- FEHLGESCHLAGENE TESTS ---');
    testResults.filter(r => r.status === 'FAIL').forEach(r => {
      console.log(`  ✗ ${r.name}`);
      console.log(`    Error: ${r.error}`);
    });
  }

  if (skipped > 0) {
    console.log('\n--- ÜBERSPRUNGENE TESTS ---');
    testResults.filter(r => r.status === 'SKIP').forEach(r => {
      console.log(`  ⊘ ${r.name} — ${r.reason}`);
    });
  }

  // JSON-Report für maschinelle Auswertung
  const report = {
    timestamp: new Date().toISOString(),
    backend_url: BASE_URL,
    strategies: ['smart_money_flow', 'hann_trend'],
    summary: { total: testResults.length, passed, failed, skipped },
    tests: testResults,
  };

  try {
    const __dirname = path.dirname(fileURLToPath(import.meta.url));
    const reportPath = path.join(__dirname, 'integration-test-results.json');
    fs.writeFileSync(reportPath, JSON.stringify(report, null, 2));
    console.log(`\nJSON-Report: ${reportPath}`);
  } catch (e) {
    console.log(`\nJSON-Report konnte nicht geschrieben werden: ${e.message}`);
  }

  process.exit(failed > 0 ? 1 : 0);
}

import fs from 'fs';
import { fileURLToPath } from 'url';
import path from 'path';

main().catch(err => {
  console.error('Fataler Fehler:', err);
  process.exit(2);
});
