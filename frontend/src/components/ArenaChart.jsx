import { useEffect, useRef } from 'react'
import { createChart } from 'lightweight-charts'

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

function ArenaChart({ symbol, interval = '60m', token, markers, overlays, customData }) {
  const chartContainerRef = useRef(null)
  const chartRef = useRef(null)

  useEffect(() => {
    if (!symbol || !chartContainerRef.current) return
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
          const pts = overlay.data
            .filter(d => d.time)
            .map(d => ({ time: d.time, value: d.value }))

          if (overlay.fill_color) {
            // Area series for band fill effect
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
            // Standard line series
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
  }, [symbol, interval, token, markers, overlays, customData])

  return (
    <div ref={chartContainerRef} className="w-full rounded-lg overflow-hidden" style={{ minHeight: 450 }} />
  )
}

export default ArenaChart
