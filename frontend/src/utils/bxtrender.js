// B-Xtrender Implementation based on original Pine Script by @Puppytherapy
// shortTermXtrender = rsi(ema(close, 5) - ema(close, 20), 15) - 50
// longTermXtrender = rsi(ema(close, 20), 15) - 50

// Pine Script compatible EMA - initializes with SMA of first 'period' values
function calculateEMA(data, period) {
  const ema = new Array(data.length).fill(0)

  if (data.length < period) return ema

  // Initialize with SMA of first 'period' values
  let sum = 0
  for (let i = 0; i < period; i++) {
    sum += data[i]
  }
  ema[period - 1] = sum / period

  // Calculate EMA for remaining values
  const multiplier = 2 / (period + 1)
  for (let i = period; i < data.length; i++) {
    ema[i] = (data[i] - ema[i - 1]) * multiplier + ema[i - 1]
  }

  // Fill initial values with first valid EMA
  for (let i = 0; i < period - 1; i++) {
    ema[i] = ema[period - 1]
  }

  return ema
}

// Pine Script compatible RMA (Wilder's smoothing) for RSI
function calculateRMA(data, period) {
  const rma = new Array(data.length).fill(0)

  if (data.length < period) return rma

  // Initialize with SMA
  let sum = 0
  for (let i = 0; i < period; i++) {
    sum += data[i]
  }
  rma[period - 1] = sum / period

  // Calculate RMA (Wilder's smoothing: alpha = 1/period)
  const alpha = 1 / period
  for (let i = period; i < data.length; i++) {
    rma[i] = alpha * data[i] + (1 - alpha) * rma[i - 1]
  }

  return rma
}

// Pine Script compatible RSI using RMA
function calcRSI(data, period) {
  const result = new Array(data.length).fill(50)

  if (data.length < period + 1) return result

  // Calculate gains and losses
  const gains = new Array(data.length).fill(0)
  const losses = new Array(data.length).fill(0)

  for (let i = 1; i < data.length; i++) {
    const change = data[i] - data[i - 1]
    gains[i] = change > 0 ? change : 0
    losses[i] = change < 0 ? Math.abs(change) : 0
  }

  // Calculate RMA of gains and losses
  const avgGain = calculateRMA(gains.slice(1), period)
  const avgLoss = calculateRMA(losses.slice(1), period)

  // Calculate RSI
  for (let i = period; i < data.length; i++) {
    const ag = avgGain[i - 1]
    const al = avgLoss[i - 1]
    if (al === 0) {
      result[i] = ag === 0 ? 50 : 100
    } else {
      const rs = ag / al
      result[i] = 100 - 100 / (1 + rs)
    }
  }

  return result
}

// T3 Moving Average (Tillson T3)
function calculateT3(data, period) {
  const b = 0.7
  const c1 = -b * b * b
  const c2 = 3 * b * b + 3 * b * b * b
  const c3 = -6 * b * b - 3 * b - 3 * b * b * b
  const c4 = 1 + 3 * b + b * b * b + 3 * b * b

  const e1 = calculateEMA(data, period)
  const e2 = calculateEMA(e1, period)
  const e3 = calculateEMA(e2, period)
  const e4 = calculateEMA(e3, period)
  const e5 = calculateEMA(e4, period)
  const e6 = calculateEMA(e5, period)

  return data.map((_, i) => c1 * e6[i] + c2 * e5[i] + c3 * e4[i] + c4 * e3[i])
}

export function calculateBXtrender(ohlcv, isAggressive = false) {
  const shortL1 = 5
  const shortL2 = 20
  const shortL3 = 15
  const longL1 = 20
  const longL2 = 15

  if (!ohlcv || ohlcv.length < Math.max(shortL2, longL1) + shortL3 + 10) {
    return { short: [], long: [], signal: [], trades: [], markers: [] }
  }

  // Filter out invalid data points
  const validData = ohlcv.filter(d => d && d.close != null && !isNaN(d.close))
  if (validData.length < Math.max(shortL2, longL1) + shortL3 + 10) {
    return { short: [], long: [], signal: [], trades: [], markers: [] }
  }

  const closes = validData.map(d => d.close)
  const opens = validData.map(d => d.open != null ? d.open : d.close)
  const times = validData.map(d => d.time)

  // Short-term: RSI(EMA(close, 5) - EMA(close, 20), 15) - 50
  const emaShort1 = calculateEMA(closes, shortL1)
  const emaShort2 = calculateEMA(closes, shortL2)
  const emaDiff = emaShort1.map((v, i) => v - emaShort2[i])
  const rsiShort = calcRSI(emaDiff, shortL3)
  const shortTermXtrender = rsiShort.map(v => v - 50)

  // Long-term: RSI(EMA(close, 20), 15) - 50
  const emaLong = calculateEMA(closes, longL1)
  const rsiLong = calcRSI(emaLong, longL2)
  const longTermXtrender = rsiLong.map(v => v - 50)

  // T3 Signal line of short-term
  const signalLine = calculateT3(shortTermXtrender, 5)

  // Build result with colors
  const shortData = []
  const longData = []
  const signalData = []
  const trades = []
  const markers = []

  const startIdx = Math.max(shortL2, longL1) + shortL3

  // Track trading state
  let inPosition = false
  let entryPrice = 0
  let entryDate = null

  for (let i = startIdx; i < closes.length; i++) {
    const shortVal = shortTermXtrender[i]
    const shortPrev = shortTermXtrender[i - 1]
    const longVal = longTermXtrender[i]
    const longPrev = longTermXtrender[i - 1]
    const sigVal = signalLine[i]
    const sigPrev = signalLine[i - 1]

    // Determine if bullish (green) or bearish (red)
    const isBullish = shortVal > 0
    const wasBullish = shortPrev > 0

    // Aggressive mode specific states
    const isLightRed = shortVal < 0 && shortVal > shortPrev
    const isDarkRed = shortVal < 0 && shortVal <= shortPrev
    const wasLightRed = shortPrev < 0 && i > startIdx && shortPrev > shortTermXtrender[i - 2]
    const wasDarkRed = shortPrev < 0 && i > startIdx && shortPrev <= shortTermXtrender[i - 2]

    // Short-term histogram colors
    let shortColor
    if (shortVal > 0) {
      shortColor = shortVal > shortPrev ? '#00FF00' : '#228B22'
    } else {
      shortColor = shortVal > shortPrev ? '#FF0000' : '#8B0000'
    }

    // Long-term histogram colors
    let longColor
    if (longVal > 0) {
      longColor = longVal > longPrev ? '#00FF00' : '#228B22'
    } else {
      longColor = longVal > longPrev ? '#FF0000' : '#8B0000'
    }

    // Signal line color
    const signalColor = sigVal > sigPrev ? '#00FF00' : '#FF0000'

    let buySignal = false
    let sellSignal = false

    // Count consecutive light red bars for aggressive mode
    let consecutiveLightRed = 0
    if (isAggressive && isLightRed) {
      consecutiveLightRed = 1
      for (let j = i - 1; j >= startIdx; j--) {
        const v = shortTermXtrender[j]
        const vPrev = shortTermXtrender[j - 1]
        if (v < 0 && v > vPrev) {
          consecutiveLightRed++
        } else {
          break
        }
      }
    }

    if (isAggressive) {
      // Aggressive BUY: First or second light red bar, OR red->green transition
      const justTurnedGreen = isBullish && !wasBullish
      if (!inPosition && opens[i] > 0) {
        if ((isLightRed && consecutiveLightRed <= 2) || justTurnedGreen) {
          buySignal = true
        }
      }
      // Aggressive SELL: First dark red bar immediately
      if (isDarkRed && inPosition && opens[i] > 0 && entryPrice > 0) {
        sellSignal = true
      }
    } else {
      // Defensive BUY: Enter when crossing above 0 (conservative)
      if (isBullish && !wasBullish && !inPosition && opens[i] > 0) {
        buySignal = true
      }
      // Defensive SELL: Exit when crossing below 0 (conservative)
      if (!isBullish && wasBullish && inPosition && opens[i] > 0 && entryPrice > 0) {
        sellSignal = true
      }
    }

    if (buySignal) {
      inPosition = true
      entryPrice = opens[i]
      entryDate = times[i]

      markers.push({
        time: times[i],
        position: 'belowBar',
        color: '#00FF00',
        shape: 'arrowUp',
        text: `BUY $${opens[i].toFixed(2)}`
      })
    }

    if (sellSignal) {
      const exitPrice = opens[i]
      const returnPct = ((exitPrice - entryPrice) / entryPrice) * 100

      trades.push({
        entryDate: entryDate,
        entryPrice: entryPrice,
        exitDate: times[i],
        exitPrice: exitPrice,
        returnPct: returnPct,
        isOpen: false
      })

      const returnText = returnPct >= 0 ? `+${returnPct.toFixed(1)}%` : `${returnPct.toFixed(1)}%`
      markers.push({
        time: times[i],
        position: 'aboveBar',
        color: '#FF0000',
        shape: 'arrowDown',
        text: `SELL $${exitPrice.toFixed(2)} ${returnText}`
      })

      inPosition = false
      entryPrice = 0
      entryDate = null
    }

    shortData.push({
      time: times[i],
      value: shortVal,
      color: shortColor
    })

    longData.push({
      time: times[i],
      value: longVal,
      color: longColor
    })

    signalData.push({
      time: times[i],
      value: sigVal,
      color: signalColor
    })
  }

  // If still in position, add open trade info
  if (inPosition && entryPrice > 0) {
    const lastIdx = closes.length - 1
    const currentPrice = closes[lastIdx]
    const unrealizedReturn = ((currentPrice - entryPrice) / entryPrice) * 100

    trades.push({
      entryDate: entryDate,
      entryPrice: entryPrice,
      exitDate: null,
      exitPrice: null,
      currentPrice: currentPrice,
      returnPct: unrealizedReturn,
      isOpen: true
    })
  }

  return { short: shortData, long: longData, signal: signalData, trades, markers }
}

// Calculate performance metrics from trades
export function calculateMetrics(trades) {
  const completedTrades = trades.filter(t => !t.isOpen)

  if (completedTrades.length === 0) {
    return {
      winRate: 0,
      riskReward: 0,
      totalReturn: 0,
      totalTrades: 0,
      wins: 0,
      losses: 0
    }
  }

  const wins = completedTrades.filter(t => t.returnPct > 0)
  const losses = completedTrades.filter(t => t.returnPct <= 0)

  const winRate = (wins.length / completedTrades.length) * 100

  const avgWin = wins.length > 0
    ? wins.reduce((sum, t) => sum + t.returnPct, 0) / wins.length
    : 0

  const avgLoss = losses.length > 0
    ? Math.abs(losses.reduce((sum, t) => sum + t.returnPct, 0) / losses.length)
    : 1

  const riskReward = avgLoss > 0 ? avgWin / avgLoss : avgWin

  // Compounded total return
  const totalReturn = completedTrades.reduce((acc, t) => acc * (1 + t.returnPct / 100), 1) - 1

  return {
    winRate,
    riskReward,
    totalReturn: totalReturn * 100,
    totalTrades: completedTrades.length,
    wins: wins.length,
    losses: losses.length
  }
}

// Calculate signal based on BX Trender bars (for defensive mode)
// or based on trade history (for aggressive mode)
export function calculateSignal(shortData, isAggressive = false, trades = []) {
  if (shortData.length < 3) return { signal: 'WAIT', bars: 0 }

  const lastIdx = shortData.length - 1
  const lastValue = shortData[lastIdx].value
  const prevValue = shortData[lastIdx - 1].value
  const isPositive = lastValue > 0

  // Count consecutive bars of the same sign from the end
  let consecutiveBars = 1
  for (let i = shortData.length - 2; i >= 0; i--) {
    const val = shortData[i].value
    if ((val > 0) === isPositive) {
      consecutiveBars++
    } else {
      break
    }
  }

  if (isAggressive) {
    // AGGRESSIVE MODE SIGNALS (based on trade history):
    // BUY: BUY triggered this/last month AND open position exists
    // SELL: SELL triggered this/last month AND no open position
    // HOLD: No recent BUY but open position exists
    // WAIT: No open position and no recent SELL

    const now = new Date()
    const thisMonth = now.getMonth()
    const thisYear = now.getFullYear()
    const lastMonth = thisMonth === 0 ? 11 : thisMonth - 1
    const lastMonthYear = thisMonth === 0 ? thisYear - 1 : thisYear

    // Check for open position
    const hasOpenPosition = trades.some(t => t.isOpen)

    // Find the last trade (most recent)
    const sortedTrades = [...trades].sort((a, b) => {
      const dateA = a.exitDate || a.entryDate
      const dateB = b.exitDate || b.entryDate
      return dateB - dateA
    })

    // Check for recent BUY (this month or last month with open position)
    let recentBuy = false
    let recentSell = false
    let buyBars = 0
    let sellBars = 0

    for (const trade of trades) {
      if (trade.entryDate) {
        const entryDate = new Date(trade.entryDate * 1000)
        const entryMonth = entryDate.getMonth()
        const entryYear = entryDate.getFullYear()

        const isThisMonth = entryMonth === thisMonth && entryYear === thisYear
        const isLastMonth = entryMonth === lastMonth && entryYear === lastMonthYear

        if ((isThisMonth || isLastMonth) && trade.isOpen) {
          recentBuy = true
          buyBars = isThisMonth ? 1 : 2
        }
      }

      if (trade.exitDate && !trade.isOpen) {
        const exitDate = new Date(trade.exitDate * 1000)
        const exitMonth = exitDate.getMonth()
        const exitYear = exitDate.getFullYear()

        const isThisMonth = exitMonth === thisMonth && exitYear === thisYear
        const isLastMonth = exitMonth === lastMonth && exitYear === lastMonthYear

        if (isThisMonth || isLastMonth) {
          recentSell = true
          sellBars = isThisMonth ? 1 : 2
        }
      }
    }

    // BUY: Recent buy with open position
    if (recentBuy && hasOpenPosition) {
      return { signal: 'BUY', bars: buyBars }
    }

    // SELL: Recent sell and no open position
    if (recentSell && !hasOpenPosition) {
      return { signal: 'SELL', bars: sellBars }
    }

    // HOLD: Open position but no recent buy
    if (hasOpenPosition) {
      return { signal: 'HOLD', bars: consecutiveBars }
    }

    // WAIT: No open position and no recent sell
    return { signal: 'WAIT', bars: consecutiveBars }
  } else {
    // DEFENSIVE MODE SIGNALS (original logic):
    // BUY: First 1-2 green bars
    // HOLD: Green continuation (3+ bars)
    // SELL: First 1-2 red bars
    // WAIT: Red continuation (3+ bars)

    if (isPositive) {
      if (consecutiveBars <= 2) {
        return { signal: 'BUY', bars: consecutiveBars }
      } else {
        return { signal: 'HOLD', bars: consecutiveBars }
      }
    } else {
      if (consecutiveBars <= 2) {
        return { signal: 'SELL', bars: consecutiveBars }
      } else {
        return { signal: 'WAIT', bars: consecutiveBars }
      }
    }
  }
}

// Save performance data to backend
export async function savePerformanceToBackend(symbol, name, metrics, trades, shortData, currentPrice, isAggressive = false) {
  try {
    // Pass trades to calculateSignal for aggressive mode (signal based on trade history)
    const { signal, bars } = calculateSignal(shortData, isAggressive, trades)
    const endpoint = isAggressive ? '/api/performance/aggressive' : '/api/performance'

    const res = await fetch(endpoint, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        symbol,
        name,
        win_rate: metrics.winRate,
        risk_reward: metrics.riskReward,
        total_return: metrics.totalReturn,
        total_trades: metrics.totalTrades,
        wins: metrics.wins,
        losses: metrics.losses,
        signal,
        signal_bars: bars,
        trades,
        current_price: currentPrice
      })
    })

    if (!res.ok) {
      console.warn(`Failed to save ${isAggressive ? 'aggressive' : 'defensive'} performance for ${symbol}: ${res.status}`)
    }
  } catch (err) {
    console.warn('Failed to save performance data:', err)
  }
}

// Fetch historical data from backend
export async function fetchHistoricalData(symbol) {
  const url = `/api/history/${symbol}?period=max&interval=1mo`

  const res = await fetch(url)
  if (!res.ok) {
    const errorText = await res.text().catch(() => 'Unknown error')
    throw new Error(`Failed to fetch data for ${symbol}: ${res.status} - ${errorText}`)
  }

  const json = await res.json()
  if (!json.data) {
    throw new Error(`No data field in response for ${symbol}`)
  }
  return json.data
}

// Process a single stock and save both modes' performance
export async function processStock(symbol, name) {
  try {
    const data = await fetchHistoricalData(symbol)

    if (!data || data.length === 0) {
      return { success: false, error: 'No data' }
    }

    const currentPrice = data[data.length - 1].close

    // Calculate and save defensive mode
    const defensiveResult = calculateBXtrender(data, false)
    const defensiveMetrics = calculateMetrics(defensiveResult.trades)
    await savePerformanceToBackend(symbol, name, defensiveMetrics, defensiveResult.trades, defensiveResult.short, currentPrice, false)

    // Calculate and save aggressive mode
    const aggressiveResult = calculateBXtrender(data, true)
    const aggressiveMetrics = calculateMetrics(aggressiveResult.trades)
    await savePerformanceToBackend(symbol, name, aggressiveMetrics, aggressiveResult.trades, aggressiveResult.short, currentPrice, true)

    return { success: true }
  } catch (err) {
    return { success: false, error: err.message }
  }
}
