import { useEffect, useRef, useState } from 'react'
import { createChart } from 'lightweight-charts'

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

function calculateBXtrender(ohlcv) {
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
  const opens = validData.map(d => d.open != null ? d.open : d.close) // Fallback to close if no open
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
  let entryIdx = 0

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

    // Short-term histogram colors
    let shortColor
    if (shortVal > 0) {
      shortColor = shortVal > shortPrev ? '#00FF00' : '#228B22' // lime : dark green
    } else {
      shortColor = shortVal > shortPrev ? '#FF0000' : '#8B0000' // red : dark red
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

    // Detect BUY signal: Red -> Green (shortVal crosses above 0)
    if (isBullish && !wasBullish && !inPosition && opens[i] > 0) {
      inPosition = true
      entryPrice = opens[i]
      entryDate = times[i]
      entryIdx = shortData.length

      markers.push({
        time: times[i],
        position: 'belowBar',
        color: '#00FF00',
        shape: 'arrowUp',
        text: `BUY $${opens[i].toFixed(2)}`
      })
    }

    // Detect SELL signal: Green -> Red (shortVal crosses below 0)
    if (!isBullish && wasBullish && inPosition && opens[i] > 0 && entryPrice > 0) {
      const exitPrice = opens[i]
      const returnPct = ((exitPrice - entryPrice) / entryPrice) * 100

      // Store complete trade (BUY + SELL together)
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
function calculateMetrics(trades) {
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

function BXtrenderChart({ symbol, timeframe = 'M', onTradesUpdate }) {
  const chartContainerRef = useRef(null)
  const chartRef = useRef(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState(null)

  useEffect(() => {
    if (!symbol || !chartContainerRef.current) return

    let cancelled = false

    // Map timeframe to API parameters - load maximum historical data
    const getApiParams = (tf) => {
      switch (tf) {
        case 'M': // Monthly
          return { period: 'max', interval: '1mo' }
        case 'W': // Weekly
          return { period: 'max', interval: '1wk' }
        case 'D': // Daily
        default:
          return { period: 'max', interval: '1d' }
      }
    }

    const { period, interval } = getApiParams(timeframe)

    const fetchAndRender = async () => {
      setLoading(true)
      setError(null)

      try {
        const res = await fetch(`/api/history/${symbol}?period=${period}&interval=${interval}`)

        if (!res.ok) {
          throw new Error('Failed to fetch data')
        }

        const json = await res.json()

        if (cancelled) return

        if (!json.data || json.data.length === 0) {
          setError('No historical data available')
          setLoading(false)
          return
        }

        const { short, long, signal, trades, markers } = calculateBXtrender(json.data)

        if (short.length === 0) {
          setError('Not enough data for indicator')
          setLoading(false)
          return
        }

        // Calculate and report metrics
        const metrics = calculateMetrics(trades)
        if (onTradesUpdate) {
          onTradesUpdate({ trades, metrics })
        }

        // Clear previous chart
        if (chartRef.current) {
          chartRef.current.remove()
          chartRef.current = null
        }

        // Ensure container has dimensions
        if (!chartContainerRef.current || chartContainerRef.current.clientWidth === 0) {
          setTimeout(() => !cancelled && fetchAndRender(), 100)
          return
        }

        // Create chart
        const chart = createChart(chartContainerRef.current, {
          layout: {
            background: { color: '#12121a' },
            textColor: '#9ca3af',
          },
          grid: {
            vertLines: { color: '#1f1f2e' },
            horzLines: { color: '#1f1f2e' },
          },
          width: chartContainerRef.current.clientWidth,
          height: 200,
          timeScale: {
            borderColor: '#2a2a3c',
            timeVisible: true,
          },
          rightPriceScale: {
            borderColor: '#2a2a3c',
            scaleMargins: {
              top: 0.05,
              bottom: 0.05,
            },
          },
          crosshair: {
            mode: 1,
          },
        })

        chartRef.current = chart

        // Add short-term histogram (main oscillator)
        const histogramSeries = chart.addHistogramSeries({
          priceFormat: { type: 'price', precision: 2 },
          priceScaleId: 'right',
        })
        histogramSeries.setData(short)

        // Add markers for BUY/SELL signals
        if (markers.length > 0) {
          histogramSeries.setMarkers(markers)
        }

        // Add signal line (T3)
        const signalSeries = chart.addLineSeries({
          color: '#ffffff',
          lineWidth: 2,
          priceScaleId: 'right',
          lastValueVisible: false,
          priceLineVisible: false,
        })
        signalSeries.setData(signal.map(d => ({ time: d.time, value: d.value })))

        // Add zero line
        const zeroLine = chart.addLineSeries({
          color: '#4b5563',
          lineWidth: 1,
          lineStyle: 2,
          priceScaleId: 'right',
          crosshairMarkerVisible: false,
          lastValueVisible: false,
          priceLineVisible: false,
        })
        zeroLine.setData(short.map(d => ({ time: d.time, value: 0 })))

        chart.timeScale().fitContent()
        setLoading(false)

      } catch (err) {
        console.error('B-Xtrender error:', err)
        if (!cancelled) {
          setError('Failed to load indicator data')
          setLoading(false)
        }
      }
    }

    fetchAndRender()

    // Handle resize
    const handleResize = () => {
      if (chartContainerRef.current && chartRef.current) {
        chartRef.current.applyOptions({ width: chartContainerRef.current.clientWidth })
      }
    }
    window.addEventListener('resize', handleResize)

    return () => {
      cancelled = true
      window.removeEventListener('resize', handleResize)
      if (chartRef.current) {
        chartRef.current.remove()
        chartRef.current = null
      }
    }
  }, [symbol, timeframe, onTradesUpdate])

  return (
    <div className="bg-dark-800 rounded-xl border border-dark-600 overflow-hidden">
      <div className="px-4 py-2 border-b border-dark-600 flex items-center justify-between">
        <div className="flex items-center gap-2">
          <span className="text-sm font-medium text-white">B-Xtrender</span>
          <span className="text-xs text-gray-500">@Puppytherapy</span>
        </div>
        <div className="flex items-center gap-3 text-xs">
          <div className="flex items-center gap-1">
            <div className="w-2 h-2 bg-lime-400 rounded-sm"></div>
            <span className="text-gray-400">Bull Rising</span>
          </div>
          <div className="flex items-center gap-1">
            <div className="w-2 h-2 bg-green-700 rounded-sm"></div>
            <span className="text-gray-400">Bull Falling</span>
          </div>
          <div className="flex items-center gap-1">
            <div className="w-2 h-2 bg-red-500 rounded-sm"></div>
            <span className="text-gray-400">Bear Rising</span>
          </div>
          <div className="flex items-center gap-1">
            <div className="w-2 h-2 bg-red-900 rounded-sm"></div>
            <span className="text-gray-400">Bear Falling</span>
          </div>
        </div>
      </div>
      <div ref={chartContainerRef} className="h-[200px] relative">
        {loading && (
          <div className="absolute inset-0 flex items-center justify-center bg-dark-800">
            <div className="text-center">
              <div className="w-6 h-6 border-2 border-accent-500 border-t-transparent rounded-full animate-spin mx-auto mb-2"></div>
              <p className="text-gray-500 text-sm">Loading B-Xtrender...</p>
            </div>
          </div>
        )}
        {error && (
          <div className="absolute inset-0 flex items-center justify-center bg-dark-800">
            <p className="text-gray-500 text-sm">{error}</p>
          </div>
        )}
      </div>
    </div>
  )
}

export default BXtrenderChart
