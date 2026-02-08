// Debug: Simulate BXtrender for BBAI to find simultaneous BUY+SELL

function calculateEMA(data, period) {
  const result = new Array(data.length).fill(0)
  const multiplier = 2 / (period + 1)
  let sum = 0
  for (let i = 0; i < period && i < data.length; i++) sum += data[i]
  result[period - 1] = sum / period
  for (let i = period; i < data.length; i++) {
    result[i] = (data[i] - result[i - 1]) * multiplier + result[i - 1]
  }
  return result
}

function calculateRSI(data, period) {
  const result = new Array(data.length).fill(50)
  if (data.length < period + 1) return result
  let gainSum = 0, lossSum = 0
  for (let i = 1; i <= period; i++) {
    const change = data[i] - data[i - 1]
    if (change > 0) gainSum += change
    else lossSum -= change
  }
  let avgGain = gainSum / period
  let avgLoss = lossSum / period
  result[period] = avgLoss === 0 ? 100 : 100 - 100 / (1 + avgGain / avgLoss)
  for (let i = period + 1; i < data.length; i++) {
    const change = data[i] - data[i - 1]
    avgGain = (avgGain * (period - 1) + (change > 0 ? change : 0)) / period
    avgLoss = (avgLoss * (period - 1) + (change < 0 ? -change : 0)) / period
    result[i] = avgLoss === 0 ? 100 : 100 - 100 / (1 + avgGain / avgLoss)
  }
  return result
}

async function fetchMonthlyData(symbol) {
  const url = `https://query1.finance.yahoo.com/v8/finance/chart/${symbol}?range=max&interval=1mo`
  const resp = await fetch(url, { headers: { 'User-Agent': 'Mozilla/5.0' } })
  const json = await resp.json()
  const result = json.chart.result[0]
  const timestamps = result.timestamp
  const quotes = result.indicators.quote[0]
  const data = []
  for (let i = 0; i < timestamps.length; i++) {
    if (quotes.close[i] && quotes.open[i]) {
      data.push({ time: timestamps[i], open: quotes.open[i], close: quotes.close[i] })
    }
  }
  return data
}

function fmt(ts) {
  return new Date(ts * 1000).toLocaleDateString('de-DE', { month: 'short', year: 'numeric' })
}

async function main() {
  console.log('=== BBAI BXtrender Debug ===\n')
  const ohlcv = await fetchMonthlyData('BBAI')
  const closes = ohlcv.map(b => b.close)
  const opens = ohlcv.map(b => b.open)
  const times = ohlcv.map(b => b.time)

  const shortL1 = 5, shortL2 = 20, shortL3 = 15, longL1 = 20, longL2 = 15
  const ema1 = calculateEMA(closes, shortL1)
  const ema2 = calculateEMA(closes, shortL2)
  const diff = closes.map((_, i) => ema1[i] - ema2[i])
  const short = calculateRSI(diff, shortL3).map(v => v - 50)
  const emaLong = calculateEMA(closes, longL1)
  const long = calculateRSI(emaLong, longL2).map(v => v - 50)

  const startIdx = Math.max(shortL2, longL1) + shortL3
  const tslPercent = 20.0

  // Show last 12 months of indicators
  console.log('Letzte 12 Monate:')
  console.log('Monat          | Short   | Long    | Beide>0 | Beide<0')
  console.log('---------------|---------|---------|---------|--------')
  for (let i = Math.max(0, ohlcv.length - 12); i < ohlcv.length; i++) {
    const bp = short[i] > 0 && long[i] > 0 ? '  JA  ' : ' NEIN '
    const bn = short[i] < 0 && long[i] < 0 ? '  JA  ' : ' NEIN '
    console.log(`${fmt(times[i]).padEnd(14)} | ${short[i].toFixed(2).padStart(7)} | ${long[i].toFixed(2).padStart(7)} | ${bp}  | ${bn}`)
  }

  // Simulate all 3 modes with detailed logging
  for (const mode of ['quant', 'ditz', 'trader']) {
    console.log(`\n=== Modus: ${mode.toUpperCase()} ===`)
    let inPosition = false
    let entryPrice = 0, highestPrice = 0

    for (let i = Math.max(startIdx, 1); i < ohlcv.length; i++) {
      const shortCurr = short[i], shortPrev = short[i - 1]
      const longCurr = long[i], longPrev = long[i - 1]
      const price = closes[i]

      if (inPosition && price > highestPrice) highestPrice = price

      let tslTriggered = false
      if (inPosition && highestPrice > 0 && price <= highestPrice * (1 - tslPercent / 100)) {
        tslTriggered = true
      }

      const bothPos = shortCurr > 0 && longCurr > 0
      const bothPosPrev = shortPrev > 0 && longPrev > 0
      const bothNeg = shortCurr < 0 && longCurr < 0

      let entrySignal, exitSignal
      if (mode === 'quant') {
        entrySignal = !inPosition && bothPos && (shortPrev <= 0 || longPrev <= 0)
        exitSignal = inPosition && ((shortCurr < 0 || longCurr < 0) || tslTriggered)
      } else {
        entrySignal = !inPosition && bothPos && (!bothPosPrev || !inPosition)
        exitSignal = inPosition && (bothNeg || tslTriggered)
      }

      const month = fmt(times[i])
      const execMonth = i + 1 < ohlcv.length ? fmt(times[i + 1]) : 'N/A'

      // Check for simultaneous BUY+SELL on same execution bar
      if (entrySignal && exitSignal) {
        console.log(`  *** SIMULTANEOUS BUY+SELL @ ${month} → exec ${execMonth} ***`)
        console.log(`      Short=${shortCurr.toFixed(2)} Long=${longCurr.toFixed(2)} TSL=${tslTriggered} inPos=${inPosition}`)
      }

      if (entrySignal && !inPosition && i + 1 < ohlcv.length) {
        console.log(`  BUY  Signal @ ${month} → Exec @ ${execMonth} ($${opens[i+1].toFixed(2)}) [Short=${shortCurr.toFixed(2)} Long=${longCurr.toFixed(2)}]`)
        inPosition = true
        entryPrice = opens[i + 1]
        highestPrice = entryPrice
      }

      // Check: after BUY, does SELL also want to fire on same bar?
      // Re-evaluate exit with new state (like the current code does since it's separate if blocks)
      if (mode === 'quant') {
        exitSignal = inPosition && ((shortCurr < 0 || longCurr < 0) || tslTriggered)
      } else {
        exitSignal = inPosition && (bothNeg || tslTriggered)
      }

      if (exitSignal && inPosition && entryPrice > 0 && i + 1 < ohlcv.length) {
        const exitPrice = opens[i + 1]
        const ret = ((exitPrice - entryPrice) / entryPrice) * 100
        const reason = tslTriggered ? 'TSL' : 'Signal'
        console.log(`  SELL Signal @ ${month} → Exec @ ${execMonth} ($${exitPrice.toFixed(2)}) [${reason}] Return: ${ret >= 0 ? '+' : ''}${ret.toFixed(1)}%`)

        // Check if this SELL is on the same execution bar as the BUY
        if (entrySignal) {
          console.log(`  *** BUY UND SELL AUF GLEICHEM BAR! Entry=$${entryPrice.toFixed(2)} Exit=$${exitPrice.toFixed(2)} ***`)
        }

        inPosition = false
        entryPrice = 0
        highestPrice = 0
      }
    }
    console.log(`  Final: ${inPosition ? 'IN POSITION' : 'FLAT'}`)
  }
}

main().catch(console.error)
