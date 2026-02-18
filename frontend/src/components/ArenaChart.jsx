import { Component, useEffect, useRef } from 'react'
import { createChart } from 'lightweight-charts'

class ChartErrorBoundary extends Component {
  state = { hasError: false, error: null }
  static getDerivedStateFromError(error) { return { hasError: true, error } }
  componentDidCatch(err) { console.error('[ChartErrorBoundary]', err) }
  render() {
    if (this.state.hasError) {
      return (
        <div className="w-full flex items-center justify-center bg-dark-800 rounded-lg border border-red-500/30 text-red-400 text-sm p-4" style={{ minHeight: 200 }}>
          Chart-Fehler: {this.state.error?.message || 'Unbekannter Fehler'} — Bitte Aktie oder Strategie wechseln.
        </div>
      )
    }
    return this.props.children
  }
}

const INTERVAL_PERIOD_MAP = {
  '1m': '1d',
  '5m': '5d',
  '15m': '60d',
  '60m': '60d',
  '2h': '60d',
  '4h': '60d',
  '1d': '2y',
  '1wk': '10y',
  '1mo': 'max',
}

function ArenaChart({ symbol, interval = '60m', token, markers, overlays, customData, loading }) {
  const chartContainerRef = useRef(null)
  const chartRef = useRef(null)

  useEffect(() => {
    if (!symbol || !chartContainerRef.current) return
    // Don't fetch from API while backtest is loading — show skeleton instead
    if (loading && !customData?.length) return
    let cancelled = false

    const initChart = async () => {
      let data

      if (customData?.length) {
        // Use provided OHLCV data directly
        data = customData
      } else {
        // Fetch from API
        const period = INTERVAL_PERIOD_MAP[interval] || '60d'
        try {
          const res = await fetch(`/api/history/${encodeURIComponent(symbol)}?period=${period}&interval=${interval}`, {
            headers: token ? { 'Authorization': `Bearer ${token}` } : {},
          })
          if (!res.ok || cancelled) return
          const json = await res.json()
          data = json.data || []
        } catch (err) {
          console.error('[ArenaChart] Fetch error:', err)
          return
        }
      }

      if (!data || data.length === 0 || cancelled) return

      // Cleanup previous chart
      if (chartRef.current) {
        chartRef.current.remove()
        chartRef.current = null
      }

      try {

      const isIntraday = ['1m', '5m', '15m', '60m', '2h', '4h'].includes(interval)

      const chart = createChart(chartContainerRef.current, {
        layout: {
          background: { color: '#12121a' },
          textColor: '#9ca3af',
        },
        grid: {
          vertLines: { visible: false },
          horzLines: { color: '#1e1e2e', style: 1 },
        },
        width: chartContainerRef.current.clientWidth,
        height: 450,
        timeScale: {
          borderColor: '#2a2a34',
          timeVisible: isIntraday,
          secondsVisible: false,
        },
        rightPriceScale: {
          borderColor: '#2a2a34',
          scaleMargins: { top: 0.05, bottom: 0.25 },
        },
        crosshair: {
          mode: 0,
          vertLine: { color: '#6366f1', width: 1, style: 2 },
          horzLine: { color: '#6366f1', width: 1, style: 2 },
        },
      })
      chartRef.current = chart

      // Candlestick series
      const candleSeries = chart.addCandlestickSeries({
        upColor: '#22c55e',
        downColor: '#ef4444',
        borderDownColor: '#ef4444',
        borderUpColor: '#22c55e',
        wickDownColor: '#ef4444',
        wickUpColor: '#22c55e',
      })

      const candleData = data.map(d => ({
        time: d.time,
        open: d.open,
        high: d.high,
        low: d.low,
        close: d.close,
      }))
      candleSeries.setData(candleData)

      // Apply markers if provided
      if (markers?.length) {
        candleSeries.setMarkers(
          markers
            .filter(m => m.time)
            .sort((a, b) => a.time - b.time)
        )
      }

      // Overlay lines and band fills
      if (overlays?.length) {
        overlays.forEach(overlay => {
          try {
            if (!overlay.data?.length) return
            const pts = overlay.data
              .filter(d => d.time && Number.isFinite(d.value))
              .map(d => ({ time: d.time, value: d.value }))
            if (!pts.length) return

            if (overlay.fill_color) {
              const areaSeries = chart.addAreaSeries({
                lineColor: overlay.color || '#888',
                lineWidth: overlay.style === 2 ? 1 : 2,
                lineStyle: overlay.style || 0,
                topColor: overlay.invert_fill ? 'rgba(0,0,0,0)' : overlay.fill_color,
                bottomColor: overlay.invert_fill ? overlay.fill_color : 'rgba(0,0,0,0)',
                priceScaleId: 'right',
                lastValueVisible: false,
                priceLineVisible: false,
                crosshairMarkerVisible: false,
                invertFilledArea: !!overlay.invert_fill,
              })
              areaSeries.setData(pts)
            } else {
              const lineSeries = chart.addLineSeries({
                color: overlay.color || '#888',
                lineWidth: overlay.style === 2 ? 1 : 2,
                lineStyle: overlay.style || 0,
                priceScaleId: 'right',
                lastValueVisible: false,
                priceLineVisible: false,
                crosshairMarkerVisible: false,
              })
              lineSeries.setData(pts)
            }
          } catch (err) {
            console.warn('[ArenaChart] Overlay render error:', overlay.name, err)
          }
        })
      }

      // Volume histogram on separate price scale
      const volumeSeries = chart.addHistogramSeries({
        priceFormat: { type: 'volume' },
        priceScaleId: 'volume',
      })
      chart.priceScale('volume').applyOptions({
        scaleMargins: { top: 0.8, bottom: 0 },
      })

      const volumeData = data.map(d => ({
        time: d.time,
        value: d.volume,
        color: d.close >= d.open ? 'rgba(34, 197, 94, 0.3)' : 'rgba(239, 68, 68, 0.3)',
      }))
      volumeSeries.setData(volumeData)

      chart.timeScale().fitContent()

      // Resize handler
      const handleResize = () => {
        if (chartContainerRef.current && chartRef.current) {
          chartRef.current.applyOptions({ width: chartContainerRef.current.clientWidth })
        }
      }
      window.addEventListener('resize', handleResize)

      // Store cleanup for resize
      chartRef.current._resizeCleanup = () => window.removeEventListener('resize', handleResize)

      } catch (err) {
        console.error('[ArenaChart] Chart render error:', err)
      }
    }

    initChart()

    return () => {
      cancelled = true
      if (chartRef.current) {
        if (chartRef.current._resizeCleanup) chartRef.current._resizeCleanup()
        chartRef.current.remove()
        chartRef.current = null
      }
    }
  }, [symbol, interval, token, markers, overlays, customData, loading])

  if (loading && !customData?.length) {
    return (
      <div className="w-full rounded-lg overflow-hidden flex items-center justify-center bg-dark-900" style={{ minHeight: 450 }}>
        <div className="flex flex-col items-center gap-3">
          <svg className="animate-spin h-8 w-8 text-indigo-500" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24">
            <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" />
            <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z" />
          </svg>
          <span className="text-gray-400 text-sm">Backtest läuft...</span>
        </div>
      </div>
    )
  }

  return (
    <div ref={chartContainerRef} className="w-full rounded-lg overflow-hidden" style={{ minHeight: 450 }} />
  )
}

function ArenaChartWithBoundary(props) {
  return <ChartErrorBoundary><ArenaChart {...props} /></ChartErrorBoundary>
}

export default ArenaChartWithBoundary
