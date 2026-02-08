/**
 * Signal Calculation Tests
 * Tests BUY/HOLD/SELL/WAIT signal logic for all 5 modes
 * Run: node frontend/src/tests/signal.test.mjs
 */

import assert from 'node:assert/strict'
import {
  calculateSignal,
  calculateQuantSignal,
  calculateDitzSignal,
  calculateTraderSignal,
} from '../utils/bxtrender.js'

// ─── Helpers ───

let passed = 0
let failed = 0
const failures = []

function test(name, fn) {
  try {
    fn()
    passed++
    console.log(`  ✓ ${name}`)
  } catch (err) {
    failed++
    failures.push({ name, error: err.message })
    console.log(`  ✗ ${name}`)
    console.log(`    ${err.message}`)
  }
}

function group(name) {
  console.log(`\n── ${name} ──`)
}

/** Generate N monthly bars starting from 2024-01-01 */
function makeBars(n) {
  const bars = []
  for (let i = 0; i < n; i++) {
    const year = 2024 + Math.floor(i / 12)
    const month = (i % 12) + 1
    bars.push({ time: `${year}-${String(month).padStart(2, '0')}-01`, value: 1 })
  }
  return bars
}

/** Get the time string of the bar at index (from end: 0 = last, 1 = second-to-last, ...) */
function barTime(bars, fromEnd) {
  return bars[bars.length - 1 - fromEnd].time
}

// ─── Mode Wrappers ───
// Each wrapper normalizes the interface to: (dataArray, trades) => { signal, bars }
// This way we can run identical scenarios across all modes.

const modes = {
  defensiv: (data, trades) => calculateSignal(data, false, trades),
  aggressiv: (data, trades) => calculateSignal(data, true, trades),
  quant: (data, trades) => calculateQuantSignal(data, data, trades),
  ditz: (data, trades) => calculateDitzSignal(data, trades),
  trader: (data, trades) => calculateTraderSignal(data, trades),
}

// Minimum data length per mode (based on early-return checks)
const minLen = { defensiv: 4, aggressiv: 4, quant: 2, ditz: 3, trader: 3 }

// ─── Scenarios (run for each mode) ───

const scenarios = [
  {
    id: 1,
    name: 'Offene Position, Entry = letzter Bar → BUY bars=0',
    makeTrades: (bars) => [{ isOpen: true, entryDate: barTime(bars, 0) }],
    expected: { signal: 'BUY', bars: 0 },
  },
  {
    id: 2,
    name: 'Offene Position, Entry = vorletzter Bar → BUY bars=1',
    makeTrades: (bars) => [{ isOpen: true, entryDate: barTime(bars, 1) }],
    expected: { signal: 'BUY', bars: 1 },
  },
  {
    id: 3,
    name: 'Offene Position, Entry = 2 Bars zurück → HOLD bars=2',
    makeTrades: (bars) => [{ isOpen: true, entryDate: barTime(bars, 2) }],
    expected: { signal: 'HOLD', bars: 2 },
  },
  {
    id: 4,
    name: 'Geschlossen, Exit = letzter Bar → SELL bars=0',
    makeTrades: (bars) => [{ isOpen: false, entryDate: barTime(bars, 3), exitDate: barTime(bars, 0) }],
    expected: { signal: 'SELL', bars: 0 },
  },
  {
    id: 5,
    name: 'Geschlossen, Exit = vorletzter Bar → SELL bars=1',
    makeTrades: (bars) => [{ isOpen: false, entryDate: barTime(bars, 3), exitDate: barTime(bars, 1) }],
    expected: { signal: 'SELL', bars: 1 },
  },
  {
    id: 6,
    name: 'Geschlossen, Exit = 2 Bars zurück → WAIT bars=2',
    makeTrades: (bars) => [{ isOpen: false, entryDate: barTime(bars, 4), exitDate: barTime(bars, 2) }],
    expected: { signal: 'WAIT', bars: 2 },
  },
  {
    id: 7,
    name: 'Keine Trades → WAIT bars=0',
    makeTrades: () => [],
    expected: { signal: 'WAIT', bars: 0 },
  },
  {
    id: 8,
    name: 'Dez-Signal → Jan-Entry → Feb-Check → BUY bars=0',
    // Entry in January (index -1), checking in February (last bar)
    // bars = length-1 - indexOf(jan) = 0 because entry IS the last bar
    makeTrades: (bars) => {
      // Simulate: signal fired in December, trade opened at January open, now we're at February
      // The data array ends at February, entry was at the bar BEFORE February
      // Wait — the plan says entry = letzter Bar → bars=0 → BUY
      // We model it as: entry is the last bar (Feb = latest data point, entry happens at its open)
      return [{ isOpen: true, entryDate: barTime(bars, 0) }]
    },
    expected: { signal: 'BUY', bars: 0 },
  },
  {
    id: 9,
    name: 'Kauf + Verkauf im gleichen Monat → SELL bars=0',
    makeTrades: (bars) => [
      { isOpen: false, entryDate: barTime(bars, 0), exitDate: barTime(bars, 0) },
    ],
    expected: { signal: 'SELL', bars: 0 },
  },
  {
    id: 10,
    name: 'entryDate nicht in Data (Fallback) → BUY bars=0',
    makeTrades: () => [{ isOpen: true, entryDate: '1999-01-01' }],
    // entryDate not found → barsSinceBuy stays 0 → BUY
    expected: { signal: 'BUY', bars: 0 },
  },
]

// ─── Run Scenarios for All Modes ───

console.log('Signal Calculation Tests')
console.log('========================')

for (const [modeName, fn] of Object.entries(modes)) {
  group(`${modeName.toUpperCase()} Mode`)
  const dataLen = Math.max(minLen[modeName], 6) // need at least 6 bars for scenario 6
  const bars = makeBars(dataLen)

  for (const sc of scenarios) {
    const trades = sc.makeTrades(bars)
    test(`#${sc.id} ${sc.name}`, () => {
      const result = fn(bars, trades)
      assert.equal(result.signal, sc.expected.signal,
        `Signal: got "${result.signal}", expected "${sc.expected.signal}"`)
      assert.equal(result.bars, sc.expected.bars,
        `Bars: got ${result.bars}, expected ${sc.expected.bars}`)
    })
  }
}

// ─── Edge Cases ───

group('Edge Cases')

test('Defensiv: zu wenig Daten (< 4 bars) → WAIT', () => {
  const result = calculateSignal(makeBars(3), false, [])
  assert.equal(result.signal, 'WAIT')
  assert.equal(result.bars, 0)
})

test('Quant: zu wenig Daten (< 2 bars) → WAIT', () => {
  const result = calculateQuantSignal(makeBars(1), makeBars(1), [])
  assert.equal(result.signal, 'WAIT')
  assert.equal(result.bars, 0)
})

test('Ditz: zu wenig Daten (< 3 bars) → WAIT', () => {
  const result = calculateDitzSignal(makeBars(2), [])
  assert.equal(result.signal, 'WAIT')
  assert.equal(result.bars, 0)
})

test('Trader delegiert zu Ditz (identisches Ergebnis)', () => {
  const bars = makeBars(6)
  const trades = [{ isOpen: true, entryDate: barTime(bars, 2) }]
  const ditzResult = calculateDitzSignal(bars, trades)
  const traderResult = calculateTraderSignal(bars, trades)
  assert.deepStrictEqual(traderResult, ditzResult)
})

test('isAggressive hat keinen Einfluss auf Signal-Berechnung', () => {
  const bars = makeBars(6)
  const trades = [{ isOpen: true, entryDate: barTime(bars, 0) }]
  const defResult = calculateSignal(bars, false, trades)
  const aggResult = calculateSignal(bars, true, trades)
  assert.deepStrictEqual(defResult, aggResult)
})

test('Mehrere Trades — letzter geschlossener zählt', () => {
  const bars = makeBars(8)
  const trades = [
    { isOpen: false, entryDate: barTime(bars, 6), exitDate: barTime(bars, 5) },
    { isOpen: false, entryDate: barTime(bars, 3), exitDate: barTime(bars, 0) },
  ]
  const result = calculateSignal(bars, false, trades)
  assert.equal(result.signal, 'SELL')
  assert.equal(result.bars, 0)
})

test('null trades → WAIT (Quant)', () => {
  const bars = makeBars(4)
  const result = calculateQuantSignal(bars, bars, null)
  assert.equal(result.signal, 'WAIT')
})

test('openTrade ohne entryDate → HOLD bars=1', () => {
  const bars = makeBars(6)
  const trades = [{ isOpen: true }]
  const result = calculateSignal(bars, false, trades)
  assert.equal(result.signal, 'HOLD')
  assert.equal(result.bars, 1)
})

// ─── Summary ───

console.log('\n========================')
console.log(`Results: ${passed} passed, ${failed} failed, ${passed + failed} total`)
if (failures.length > 0) {
  console.log('\nFailures:')
  for (const f of failures) {
    console.log(`  ✗ ${f.name}: ${f.error}`)
  }
  process.exit(1)
} else {
  console.log('\nAll tests passed!')
  process.exit(0)
}
