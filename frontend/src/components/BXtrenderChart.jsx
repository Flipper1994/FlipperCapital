import { useEffect, useRef, useState } from 'react'
import { createChart } from 'lightweight-charts'
import { useTradingMode } from '../context/TradingModeContext'
import { calculateBXtrender, calculateMetrics, savePerformanceToBackend } from '../utils/bxtrender'

const timeframeLabels = { 'M': 'Monthly', 'W': 'Weekly', 'D': 'Daily' }

function BXtrenderChart({ symbol, stockName = '', timeframe = 'M', onTradesUpdate }) {
  const chartContainerRef = useRef(null)
  const chartRef = useRef(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState(null)
  const { isAggressive } = useTradingMode()

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

        // Calculate B-Xtrender with current mode
        const { short, long, signal, trades, markers } = calculateBXtrender(json.data, isAggressive)

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

        // Get current price and save to backend (only for monthly timeframe)
        // Save both defensive and aggressive data when in monthly view
        if (timeframe === 'M' && json.data.length > 0) {
          const currentPrice = json.data[json.data.length - 1].close

          // Save current mode's data
          savePerformanceToBackend(symbol, stockName || symbol, metrics, trades, short, currentPrice, isAggressive)

          // Also save the other mode's data for completeness
          const otherModeResult = calculateBXtrender(json.data, !isAggressive)
          const otherMetrics = calculateMetrics(otherModeResult.trades)
          savePerformanceToBackend(symbol, stockName || symbol, otherMetrics, otherModeResult.trades, otherModeResult.short, currentPrice, !isAggressive)
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
  }, [symbol, timeframe, onTradesUpdate, isAggressive])

  return (
    <div className={`bg-dark-800 rounded-xl border overflow-hidden ${isAggressive ? 'border-orange-500/50' : 'border-dark-600'}`}>
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
          {isAggressive ? (
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
        </div>
        <div className="hidden md:flex items-center gap-3 text-xs">
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
