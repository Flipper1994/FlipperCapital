/**
 * Performance Filter Tests
 * Tests filter logic in computeModeData for all filter types
 * Run: node frontend/src/tests/performance_filter.test.mjs
 */

import assert from 'node:assert/strict'

// ─── Extracted from Performance.jsx (same as performance.test.mjs) ───

const MODES = [
  { key: 'defensive', title: 'Defensiv' },
  { key: 'aggressive', title: 'Aggressiv' },
  { key: 'quant', title: 'Quant' },
  { key: 'ditz', title: 'Ditz' },
  { key: 'trader', title: 'Trader' },
]

function computeModeData(trades, cutoffDate, filters = {}) {
  const result = {}
  for (const m of MODES) {
    const filtered = trades
      .filter(t => t.mode === m.key && t.entry_date >= cutoffDate)
      .filter(t => {
        if (filters.minWinrate && t.win_rate < parseFloat(filters.minWinrate)) return false
        if (filters.maxWinrate && t.win_rate > parseFloat(filters.maxWinrate)) return false
        if (filters.minRR && t.risk_reward < parseFloat(filters.minRR)) return false
        if (filters.maxRR && t.risk_reward > parseFloat(filters.maxRR)) return false
        if (filters.minAvgReturn && t.avg_return < parseFloat(filters.minAvgReturn)) return false
        if (filters.maxAvgReturn && t.avg_return > parseFloat(filters.maxAvgReturn)) return false
        if (filters.minMarketCap && t.market_cap < parseFloat(filters.minMarketCap) * 1e9) return false
        return true
      })
    const wins = filtered.filter(t => (t.return_pct || 0) > 0)
    const losses = filtered.filter(t => (t.return_pct || 0) < 0)
    const totalReturn = filtered.reduce((s, t) => s + (t.return_pct || 0), 0)
    const aw = wins.length > 0 ? wins.reduce((s, t) => s + t.return_pct, 0) / wins.length : 0
    const al = losses.length > 0 ? Math.abs(losses.reduce((s, t) => s + t.return_pct, 0) / losses.length) : 0
    result[m.key] = {
      trades: filtered,
      stats: {
        tradeCount: filtered.length,
        winRate: filtered.length > 0 ? (wins.length / filtered.length) * 100 : 0,
        riskReward: al > 0 ? aw / al : aw > 0 ? Infinity : 0,
        totalReturn,
        avgReturn: filtered.length > 0 ? totalReturn / filtered.length : 0,
        wins: wins.length,
        losses: losses.length,
      }
    }
  }
  return result
}

// ─── Test helpers ───

const NOW = 1738886400
const DAY = 86400
const MONTH = 30 * DAY

let passed = 0, failed = 0, errors = []

function test(name, fn) {
  try {
    fn()
    passed++
    console.log(`  ✓ ${name}`)
  } catch (e) {
    failed++
    errors.push({ name, error: e.message })
    console.log(`  ✗ ${name}`)
    console.log(`    ${e.message}`)
  }
}

function approx(actual, expected, tolerance = 0.01, msg = '') {
  const diff = Math.abs(actual - expected)
  if (diff > tolerance) {
    throw new Error(`${msg}Expected ~${expected}, got ${actual} (diff ${diff.toFixed(6)}, tol ${tolerance})`)
  }
}

function makeTrade(mode, symbol, returnPct, extra = {}) {
  return {
    mode, symbol, name: symbol + ' Inc',
    entry_date: NOW - 2 * MONTH, exit_date: NOW - MONTH,
    entry_price: 100, exit_price: 100 + returnPct,
    current_price: 100 + returnPct,
    return_pct: returnPct, status: 'CLOSED',
    win_rate: 50, risk_reward: 1, avg_return: 0, market_cap: 100e9,
    ...extra,
  }
}

// ─── TESTS ───

console.log('\n═══ Performance Filter Tests ═══\n')

// ────────────────────────────────────────
console.log('── 1. Winrate filter ──')
// ────────────────────────────────────────

test('minWinrate filters out low win_rate trades', () => {
  const trades = [
    makeTrade('defensive', 'A', 10, { win_rate: 70 }),
    makeTrade('defensive', 'B', 5, { win_rate: 40 }),
    makeTrade('defensive', 'C', 8, { win_rate: 55 }),
  ]
  const data = computeModeData(trades, 0, { minWinrate: '50' })
  assert.equal(data.defensive.stats.tradeCount, 2)
  approx(data.defensive.stats.totalReturn, 18, 0.01) // A(10) + C(8)
})

test('maxWinrate filters out high win_rate trades', () => {
  const trades = [
    makeTrade('defensive', 'A', 10, { win_rate: 70 }),
    makeTrade('defensive', 'B', 5, { win_rate: 40 }),
  ]
  const data = computeModeData(trades, 0, { maxWinrate: '60' })
  assert.equal(data.defensive.stats.tradeCount, 1)
  approx(data.defensive.stats.totalReturn, 5, 0.01)
})

test('minWinrate + maxWinrate range filter', () => {
  const trades = [
    makeTrade('defensive', 'A', 10, { win_rate: 80 }),
    makeTrade('defensive', 'B', 5, { win_rate: 50 }),
    makeTrade('defensive', 'C', -3, { win_rate: 30 }),
    makeTrade('defensive', 'D', 7, { win_rate: 60 }),
  ]
  const data = computeModeData(trades, 0, { minWinrate: '40', maxWinrate: '70' })
  assert.equal(data.defensive.stats.tradeCount, 2) // B(50) + D(60)
  approx(data.defensive.stats.totalReturn, 12, 0.01)
})

// ────────────────────────────────────────
console.log('\n── 2. RiskReward filter ──')
// ────────────────────────────────────────

test('minRR filters out low risk_reward trades', () => {
  const trades = [
    makeTrade('defensive', 'A', 10, { risk_reward: 2.5 }),
    makeTrade('defensive', 'B', 5, { risk_reward: 0.8 }),
    makeTrade('defensive', 'C', 7, { risk_reward: 1.5 }),
  ]
  const data = computeModeData(trades, 0, { minRR: '1.0' })
  assert.equal(data.defensive.stats.tradeCount, 2) // A + C
})

test('maxRR filters out high risk_reward trades', () => {
  const trades = [
    makeTrade('defensive', 'A', 10, { risk_reward: 3.0 }),
    makeTrade('defensive', 'B', 5, { risk_reward: 1.2 }),
  ]
  const data = computeModeData(trades, 0, { maxRR: '2.0' })
  assert.equal(data.defensive.stats.tradeCount, 1) // B
  approx(data.defensive.stats.totalReturn, 5, 0.01)
})

test('minRR + maxRR range filter', () => {
  const trades = [
    makeTrade('defensive', 'A', 10, { risk_reward: 3.0 }),
    makeTrade('defensive', 'B', 5, { risk_reward: 1.5 }),
    makeTrade('defensive', 'C', -2, { risk_reward: 0.5 }),
  ]
  const data = computeModeData(trades, 0, { minRR: '1.0', maxRR: '2.0' })
  assert.equal(data.defensive.stats.tradeCount, 1) // B
})

// ────────────────────────────────────────
console.log('\n── 3. AvgReturn filter ──')
// ────────────────────────────────────────

test('minAvgReturn filters out low avg_return trades', () => {
  const trades = [
    makeTrade('defensive', 'A', 10, { avg_return: 8 }),
    makeTrade('defensive', 'B', 5, { avg_return: 2 }),
  ]
  const data = computeModeData(trades, 0, { minAvgReturn: '5' })
  assert.equal(data.defensive.stats.tradeCount, 1) // A
})

test('maxAvgReturn filters out high avg_return trades', () => {
  const trades = [
    makeTrade('defensive', 'A', 10, { avg_return: 12 }),
    makeTrade('defensive', 'B', 5, { avg_return: 3 }),
  ]
  const data = computeModeData(trades, 0, { maxAvgReturn: '10' })
  assert.equal(data.defensive.stats.tradeCount, 1) // B
})

test('minAvgReturn + maxAvgReturn range filter', () => {
  const trades = [
    makeTrade('defensive', 'A', 10, { avg_return: 15 }),
    makeTrade('defensive', 'B', 5, { avg_return: 7 }),
    makeTrade('defensive', 'C', -3, { avg_return: 1 }),
  ]
  const data = computeModeData(trades, 0, { minAvgReturn: '5', maxAvgReturn: '10' })
  assert.equal(data.defensive.stats.tradeCount, 1) // B
})

// ────────────────────────────────────────
console.log('\n── 4. MarketCap filter ──')
// ────────────────────────────────────────

test('minMarketCap filters out small cap stocks', () => {
  const trades = [
    makeTrade('defensive', 'AAPL', 10, { market_cap: 3000e9 }),  // 3T
    makeTrade('defensive', 'SMALL', 5, { market_cap: 2e9 }),     // 2B
    makeTrade('defensive', 'MID', 7, { market_cap: 50e9 }),      // 50B
  ]
  const data = computeModeData(trades, 0, { minMarketCap: '10' }) // 10 Mrd = 10B
  assert.equal(data.defensive.stats.tradeCount, 2) // AAPL + MID
  approx(data.defensive.stats.totalReturn, 17, 0.01)
})

test('minMarketCap=100 filters to mega caps only', () => {
  const trades = [
    makeTrade('defensive', 'AAPL', 10, { market_cap: 3000e9 }),
    makeTrade('defensive', 'NVDA', 15, { market_cap: 2000e9 }),
    makeTrade('defensive', 'MID', 7, { market_cap: 50e9 }),
    makeTrade('defensive', 'SMALL', 3, { market_cap: 5e9 }),
  ]
  const data = computeModeData(trades, 0, { minMarketCap: '100' }) // 100B
  assert.equal(data.defensive.stats.tradeCount, 2) // AAPL + NVDA
  approx(data.defensive.stats.totalReturn, 25, 0.01)
})

test('market_cap=0 (missing data) is filtered by any minMarketCap', () => {
  const trades = [
    makeTrade('defensive', 'A', 10, { market_cap: 100e9 }),
    makeTrade('defensive', 'B', 5, { market_cap: 0 }),
  ]
  const data = computeModeData(trades, 0, { minMarketCap: '1' }) // 1B
  assert.equal(data.defensive.stats.tradeCount, 1) // only A
  approx(data.defensive.stats.totalReturn, 10, 0.01)
})

test('minMarketCap with no filter string passes all trades', () => {
  const trades = [
    makeTrade('defensive', 'A', 10, { market_cap: 100e9 }),
    makeTrade('defensive', 'B', 5, { market_cap: 0 }),
  ]
  const data = computeModeData(trades, 0, { minMarketCap: '' })
  assert.equal(data.defensive.stats.tradeCount, 2)
})

test('minMarketCap boundary: exactly at threshold passes', () => {
  const trades = [
    makeTrade('defensive', 'A', 10, { market_cap: 10e9 }),  // exactly 10B
    makeTrade('defensive', 'B', 5, { market_cap: 9.99e9 }), // just below
  ]
  const data = computeModeData(trades, 0, { minMarketCap: '10' })
  assert.equal(data.defensive.stats.tradeCount, 1) // only A (10e9 >= 10e9)
})

// ────────────────────────────────────────
console.log('\n── 5. Combined filters ──')
// ────────────────────────────────────────

test('multiple filters applied simultaneously', () => {
  const trades = [
    makeTrade('defensive', 'A', 10, { win_rate: 70, risk_reward: 2.0, avg_return: 8, market_cap: 200e9 }),
    makeTrade('defensive', 'B', 5, { win_rate: 40, risk_reward: 1.5, avg_return: 5, market_cap: 100e9 }),
    makeTrade('defensive', 'C', 7, { win_rate: 60, risk_reward: 0.8, avg_return: 6, market_cap: 50e9 }),
    makeTrade('defensive', 'D', -3, { win_rate: 55, risk_reward: 1.2, avg_return: 4, market_cap: 10e9 }),
  ]
  // minWinrate=50 → excludes B (40)
  // minRR=1.0 → excludes C (0.8)
  // minMarketCap=20 → excludes D (10B)
  // Only A survives all filters
  const data = computeModeData(trades, 0, { minWinrate: '50', minRR: '1.0', minMarketCap: '20' })
  assert.equal(data.defensive.stats.tradeCount, 1)
  approx(data.defensive.stats.totalReturn, 10, 0.01)
})

test('combined winrate + marketcap filter', () => {
  const trades = [
    makeTrade('defensive', 'A', 10, { win_rate: 70, market_cap: 500e9 }),
    makeTrade('defensive', 'B', 5, { win_rate: 30, market_cap: 500e9 }),
    makeTrade('defensive', 'C', 7, { win_rate: 70, market_cap: 5e9 }),
  ]
  const data = computeModeData(trades, 0, { minWinrate: '50', minMarketCap: '10' })
  assert.equal(data.defensive.stats.tradeCount, 1) // only A
})

// ────────────────────────────────────────
console.log('\n── 6. Empty/edge cases ──')
// ────────────────────────────────────────

test('empty filter strings filter nothing', () => {
  const trades = [
    makeTrade('defensive', 'A', 10),
    makeTrade('defensive', 'B', 5),
  ]
  const data = computeModeData(trades, 0, {
    minWinrate: '', maxWinrate: '', minRR: '', maxRR: '',
    minAvgReturn: '', maxAvgReturn: '', minMarketCap: '',
  })
  assert.equal(data.defensive.stats.tradeCount, 2)
})

test('all trades filtered → stats are 0', () => {
  const trades = [
    makeTrade('defensive', 'A', 10, { win_rate: 30 }),
    makeTrade('defensive', 'B', 5, { win_rate: 20 }),
  ]
  const data = computeModeData(trades, 0, { minWinrate: '90' })
  assert.equal(data.defensive.stats.tradeCount, 0)
  approx(data.defensive.stats.winRate, 0, 0.01)
  approx(data.defensive.stats.totalReturn, 0, 0.01)
  approx(data.defensive.stats.avgReturn, 0, 0.01)
  approx(data.defensive.stats.riskReward, 0, 0.01)
})

test('no trades at all → all modes empty', () => {
  const data = computeModeData([], 0, { minWinrate: '50' })
  for (const m of MODES) {
    assert.equal(data[m.key].stats.tradeCount, 0)
  }
})

test('filter applies per mode independently', () => {
  const trades = [
    makeTrade('defensive', 'A', 10, { win_rate: 70 }),
    makeTrade('aggressive', 'B', 15, { win_rate: 40 }),
  ]
  const data = computeModeData(trades, 0, { minWinrate: '50' })
  assert.equal(data.defensive.stats.tradeCount, 1)
  assert.equal(data.aggressive.stats.tradeCount, 0)
})

// ────────────────────────────────────────
// Summary
// ────────────────────────────────────────

console.log('\n═══════════════════════════════════════')
console.log(`Results: ${passed} passed, ${failed} failed out of ${passed + failed} tests`)
if (errors.length > 0) {
  console.log('\nFailed tests:')
  for (const e of errors) console.log(`  ✗ ${e.name}: ${e.error}`)
}
console.log('═══════════════════════════════════════\n')

process.exit(failed > 0 ? 1 : 0)
