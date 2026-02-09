import { useEffect, useRef, useState } from 'react'
import { createChart } from 'lightweight-charts'
import { useTradingMode } from '../context/TradingModeContext'
import {
  calculateBXtrender,
  calculateBXtrenderQuant,
  calculateMetrics,
  calculateSignal,
  savePerformanceToBackend,
  saveQuantPerformanceToBackend,
  saveDitzPerformanceToBackend,
  saveTraderPerformanceToBackend,
  calculateDitzSignal,
  calculateTraderSignal,
  fetchBXtrenderConfig,
  fetchBXtrenderQuantConfig,
  fetchBXtrenderDitzConfig,
  fetchBXtrenderTraderConfig
} from '../utils/bxtrender'

const timeframeLabels = { 'M': 'Monthly', 'W': 'Weekly', 'D': 'Daily' }

const intervalLabels = {
  '1mo': 'Monthly',
  '1wk': 'Weekly',
  '1d': 'Daily'
}

function BXtrenderChart({ symbol, stockName = '', timeframe = 'M', onTradesUpdate }) {
  const chartContainerRef = useRef(null)
  const chartRef = useRef(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState(null)
  const [intervalWarning, setIntervalWarning] = useState(null) // Warning if Yahoo returns different interval
  const [dataSource, setDataSource] = useState(null)
  const [apiWarnings, setApiWarnings] = useState([])
  const { isAggressive, isQuant, isDitz, isTrader, mode } = useTradingMode()

  useEffect(() => {
    if (!symbol || !chartContainerRef.current) return

    let cancelled = false

    // Map timeframe to API parameters
    // IMPORTANT: Yahoo Finance only supports certain range/interval combinations:
    // - Monthly (1mo): range=max works
    // - Weekly (1wk): max range is 10y (range=max returns monthly data!)
    // - Daily (1d): max range is 2y (longer ranges may return weekly/monthly)
    const getApiParams = (tf) => {
      switch (tf) {
        case 'M': // Monthly
          return { period: 'max', interval: '1mo' }
        case 'W': // Weekly - use 10y to get actual weekly data
          return { period: '10y', interval: '1wk' }
        case 'D': // Daily - use 2y to get actual daily data
        default:
          return { period: '2y', interval: '1d' }
      }
    }

    const { period, interval } = getApiParams(timeframe)

    const fetchAndRender = async () => {
      setLoading(true)
      setError(null)
      setIntervalWarning(null)
      setDataSource(null)

      try {
        // Fetch appropriate config based on mode
        const configPromise = isTrader ? fetchBXtrenderTraderConfig() : isDitz ? fetchBXtrenderDitzConfig() : isQuant ? fetchBXtrenderQuantConfig() : fetchBXtrenderConfig()

        const [configData, res] = await Promise.all([
          configPromise,
          fetch(`/api/history/${symbol}?period=${period}&interval=${interval}`)
        ])

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

        // Check if Yahoo Finance returned a different interval than requested
        if (json.actualInterval && json.requestedInterval && json.actualInterval !== json.requestedInterval) {
          const requestedLabel = intervalLabels[json.requestedInterval] || json.requestedInterval
          const actualLabel = intervalLabels[json.actualInterval] || json.actualInterval
          setIntervalWarning(`${requestedLabel} nicht verfügbar, zeige ${actualLabel}`)
        }

        // Set data source
        if (json.source) {
          setDataSource(json.source)
        }

        // API warnings (e.g. Twelve Data rate limit)
        if (json.warnings && json.warnings.length > 0) {
          setApiWarnings(json.warnings)
        } else {
          setApiWarnings([])
        }

        // Nur abgeschlossene Monatskerzen für Signalberechnung (aktuellen Monat entfernen)
        // Yahoo liefert manchmal mehrere Datenpunkte für den aktuellen Monat
        let calcData = json.data
        let nextOpen = null
        if (timeframe === 'M' && calcData.length > 0) {
          const now = new Date()
          const strippedCandles = calcData.filter(d => {
            const t = new Date(d.time * 1000)
            return t.getUTCFullYear() === now.getUTCFullYear() && t.getUTCMonth() === now.getUTCMonth()
          })
          calcData = calcData.filter(d => {
            const t = new Date(d.time * 1000)
            return !(t.getUTCFullYear() === now.getUTCFullYear() && t.getUTCMonth() === now.getUTCMonth())
          })
          nextOpen = strippedCandles.length > 0
            ? { time: strippedCandles[0].time, open: strippedCandles[0].open, close: strippedCandles[strippedCandles.length - 1].close }
            : null
        }

        // Get config and calculate B-Xtrender based on mode
        let short, long, signal, trades, markers

        if (isQuant || isDitz || isTrader) {
          // Quant/Ditz/Trader mode - use QuantTherapy algorithm
          const result = calculateBXtrenderQuant(calcData, configData, isTrader ? 'trader' : isDitz ? 'ditz' : 'quant', nextOpen)
          short = result.short
          long = result.long
          signal = result.signal
          trades = result.trades
          markers = result.markers
        } else {
          // Defensive or Aggressive mode
          const config = isAggressive ? configData.aggressive : configData.defensive
          const result = calculateBXtrender(calcData, isAggressive, config, nextOpen)
          short = result.short
          long = result.long
          signal = result.signal
          trades = result.trades
          markers = result.markers
        }

        if (short.length === 0) {
          setError('Not enough data for indicator')
          setLoading(false)
          return
        }

        // Calculate current signal and report metrics
        const metrics = calculateMetrics(trades)
        let currentSignal = 'WAIT'
        if (isTrader) {
          const traderSig = calculateTraderSignal(signal, trades)
          currentSignal = traderSig.signal
        } else if (isDitz) {
          // Ditz signal: based on trade history
          const ditzSig = calculateDitzSignal(signal, trades)
          currentSignal = ditzSig.signal
        } else if (isQuant) {
          // Quant signal: check both indicators at second-to-last bar
          if (short.length >= 2 && long.length >= 2) {
            const idx = short.length - 2
            const sv = short[idx].value
            const lv = long[idx].value
            const hasOpen = trades && trades.some(t => t.isOpen)
            if (sv > 0 && lv > 0) currentSignal = hasOpen ? 'HOLD' : 'BUY'
            else if (hasOpen) currentSignal = 'SELL'
            else currentSignal = sv < 0 && lv < 0 ? 'SELL' : 'WAIT'
          }
        } else {
          const sig = calculateSignal(short, isAggressive, trades)
          currentSignal = sig.signal
        }
        if (onTradesUpdate) {
          onTradesUpdate({ trades, metrics, signal: currentSignal })
        }

        // Get current price and save to backend (only for monthly timeframe)
        if (timeframe === 'M' && json.data.length > 0) {
          const currentPrice = json.data[json.data.length - 1].close

          if (isTrader) {
            saveTraderPerformanceToBackend(symbol, stockName || symbol, metrics, trades, signal, currentPrice)
          } else if (isDitz) {
            // Save Ditz mode data
            saveDitzPerformanceToBackend(symbol, stockName || symbol, metrics, trades, signal, currentPrice)
          } else if (isQuant) {
            // Save Quant mode data
            saveQuantPerformanceToBackend(symbol, stockName || symbol, metrics, trades, short, long, currentPrice)
          } else {
            // Save current mode's data (Defensive or Aggressive)
            savePerformanceToBackend(symbol, stockName || symbol, metrics, trades, short, currentPrice, isAggressive)

            // Also save the other mode's data for completeness
            const otherConfig = isAggressive ? configData.defensive : configData.aggressive
            const otherModeResult = calculateBXtrender(calcData, !isAggressive, otherConfig, nextOpen)
            const otherMetrics = calculateMetrics(otherModeResult.trades)
            savePerformanceToBackend(symbol, stockName || symbol, otherMetrics, otherModeResult.trades, otherModeResult.short, currentPrice, !isAggressive)
          }
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
            vertLines: { visible: false },
            horzLines: { visible: false },
          },
          width: chartContainerRef.current.clientWidth,
          height: 200,
          timeScale: {
            borderColor: '#2a2a34',
            timeVisible: true,
          },
          rightPriceScale: {
            borderColor: '#2a2a34',
            scaleMargins: {
              top: 0.1,
              bottom: 0.1,
            },
          },
          crosshair: {
            mode: 1,
            vertLine: { color: '#6366f1', width: 1, style: 2 },
            horzLine: { color: '#6366f1', width: 1, style: 2 },
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

        // Add long-term Xtrender as colored line (like TradingView)
        const longSeries = chart.addLineSeries({
          lineWidth: 2,
          priceScaleId: 'right',
          lastValueVisible: false,
          priceLineVisible: false,
          crosshairMarkerVisible: false,
        })
        longSeries.setData(long.map(d => ({ time: d.time, value: d.value, color: d.color })))

        // Add signal line (T3) with per-point coloring
        const signalSeries = chart.addLineSeries({
          lineWidth: 2,
          priceScaleId: 'right',
          lastValueVisible: false,
          priceLineVisible: false,
        })
        signalSeries.setData(signal.map(d => ({ time: d.time, value: d.value, color: d.color })))

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
  }, [symbol, timeframe, onTradesUpdate, isAggressive, isQuant, isDitz, isTrader, mode])

  return (
    <div className={`bg-dark-800 rounded-xl border overflow-hidden ${isTrader ? 'border-emerald-500/50' : isDitz ? 'border-cyan-500/50' : isQuant ? 'border-violet-500/50' : isAggressive ? 'border-orange-500/50' : 'border-dark-600'}`}>
      <div className="px-4 py-2 border-b border-dark-600 flex items-center justify-between">
        <div className="flex items-center gap-2">
          <span className="text-sm font-medium text-white">B-Xtrender</span>
          <span className="text-xs text-gray-500">@Puppytherapy</span>
          <span className={`px-1.5 py-0.5 text-[10px] font-bold rounded ${
            timeframe === 'M' ? 'bg-purple-500/20 text-purple-400' :
            timeframe === 'W' ? 'bg-cyan-500/20 text-cyan-400' :
            'bg-gray-500/20 text-gray-400'
          }`}>
            {timeframeLabels[timeframe] || timeframe}
          </span>
          {isTrader ? (
            <span className="px-1.5 py-0.5 bg-emerald-500/20 text-emerald-400 text-[10px] font-bold rounded flex items-center gap-1">
              <svg className="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M13 7h8m0 0v8m0-8l-8 8-4-4-6 6" />
              </svg>
              TRADER
            </span>
          ) : isDitz ? (
            <span className="px-1.5 py-0.5 bg-cyan-500/20 text-cyan-400 text-[10px] font-bold rounded flex items-center gap-1">
              <svg className="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 7h6m0 10v-3m-3 3h.01M9 17h.01M9 14h.01M12 14h.01M15 11h.01M12 11h.01M9 11h.01M7 21h10a2 2 0 002-2V5a2 2 0 00-2-2H7a2 2 0 00-2 2v14a2 2 0 002 2z" />
              </svg>
              DITZ
            </span>
          ) : isQuant ? (
            <span className="px-1.5 py-0.5 bg-violet-500/20 text-violet-400 text-[10px] font-bold rounded flex items-center gap-1">
              <svg className="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 19v-6a2 2 0 00-2-2H5a2 2 0 00-2 2v6a2 2 0 002 2h2a2 2 0 002-2zm0 0V9a2 2 0 012-2h2a2 2 0 012 2v10m-6 0a2 2 0 002 2h2a2 2 0 002-2m0 0V5a2 2 0 012-2h2a2 2 0 012 2v14a2 2 0 01-2 2h-2a2 2 0 01-2-2z" />
              </svg>
              QUANT
            </span>
          ) : isAggressive ? (
            <span className="px-1.5 py-0.5 bg-orange-500/20 text-orange-400 text-[10px] font-bold rounded flex items-center gap-1">
              <svg className="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M17.657 18.657A8 8 0 016.343 7.343S7 9 9 10c0-2 .5-5 2.986-7C14 5 16.09 5.777 17.656 7.343A7.975 7.975 0 0120 13a7.975 7.975 0 01-2.343 5.657z" />
              </svg>
              AGGRESSIV
            </span>
          ) : (
            <span className="px-1.5 py-0.5 bg-blue-500/20 text-blue-400 text-[10px] font-bold rounded flex items-center gap-1">
              <svg className="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 12l2 2 4-4m5.618-4.016A11.955 11.955 0 0112 2.944a11.955 11.955 0 01-8.618 3.04A12.02 12.02 0 003 9c0 5.591 3.824 10.29 9 11.622 5.176-1.332 9-6.03 9-11.622 0-1.042-.133-2.052-.382-3.016z" />
              </svg>
              DEFENSIV
            </span>
          )}
          {intervalWarning && (
            <span className="px-1.5 py-0.5 bg-yellow-500/20 text-yellow-400 text-[10px] font-medium rounded flex items-center gap-1">
              <svg className="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
              </svg>
              {intervalWarning}
            </span>
          )}
          {dataSource && dataSource !== 'yahoo' && (
            <span className={`px-1.5 py-0.5 text-[10px] font-medium rounded flex items-center gap-1 ${
              dataSource === 'twelvedata'
                ? 'bg-cyan-500/20 text-cyan-400'
                : 'bg-gray-500/20 text-gray-400'
            }`}>
              <svg className="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 7v10c0 2.21 3.582 4 8 4s8-1.79 8-4V7M4 7c0 2.21 3.582 4 8 4s8-1.79 8-4M4 7c0-2.21 3.582-4 8-4s8 1.79 8 4" />
              </svg>
              {dataSource === 'twelvedata' ? 'Twelve Data' : dataSource}
            </span>
          )}
        </div>
        <div className="hidden md:flex items-center gap-3 text-xs">
          {isTrader ? (
            <>
              <div className="flex items-center gap-1">
                <div className="w-2 h-2 bg-lime-400 rounded-sm"></div>
                <span className="text-gray-400">Signal steigend</span>
              </div>
              <div className="flex items-center gap-1">
                <div className="w-2 h-2 bg-red-500 rounded-sm"></div>
                <span className="text-gray-400">Signal fallend</span>
              </div>
            </>
          ) : (isQuant || isDitz) ? (
            <>
              <div className="flex items-center gap-1">
                <div className="w-2 h-2 bg-lime-400 rounded-sm"></div>
                <span className="text-gray-400">Both Aligned +</span>
              </div>
              <div className="flex items-center gap-1">
                <div className="w-2 h-2 bg-red-500 rounded-sm"></div>
                <span className="text-gray-400">Both Aligned -</span>
              </div>
              <div className="flex items-center gap-1">
                <div className="w-2 h-2 bg-gray-500 rounded-sm"></div>
                <span className="text-gray-400">Mixed</span>
              </div>
            </>
          ) : (
            <>
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
                <span className="text-gray-400">{isAggressive ? 'BUY Zone' : 'Bear Rising'}</span>
              </div>
              <div className="flex items-center gap-1">
                <div className="w-2 h-2 bg-red-900 rounded-sm"></div>
                <span className="text-gray-400">{isAggressive ? 'SELL Zone' : 'Bear Falling'}</span>
              </div>
            </>
          )}
        </div>
      </div>
      {apiWarnings.length > 0 && (
        <div className="mx-3 mb-1 px-3 py-2 bg-amber-500/10 border border-amber-500/30 rounded-lg">
          {apiWarnings.map((w, i) => (
            <p key={i} className="text-amber-400 text-xs flex items-center gap-2">
              <svg className="w-3.5 h-3.5 flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
              </svg>
              {w}
            </p>
          ))}
        </div>
      )}
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
