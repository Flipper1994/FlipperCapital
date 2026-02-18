import { Component, useEffect, useRef } from 'react'
import { createChart } from 'lightweight-charts'

class IndicatorErrorBoundary extends Component {
  state = { hasError: false, error: null }
  static getDerivedStateFromError(error) { return { hasError: true, error } }
  componentDidCatch(err) { console.error('[IndicatorErrorBoundary]', err) }
  render() {
    if (this.state.hasError) {
      return (
        <div className="w-full flex items-center justify-center bg-dark-800 rounded-lg border border-red-500/30 text-red-400 text-sm p-4" style={{ minHeight: 100 }}>
          Indikator-Fehler: {this.state.error?.message || 'Unbekannter Fehler'}
        </div>
      )
    }
    return this.props.children
  }
}

function ArenaIndicatorChart({ indicators, markers, strategyName }) {
  const chartContainerRef = useRef(null)
  const chartRef = useRef(null)

  useEffect(() => {
    if (!indicators?.length || !chartContainerRef.current) return

    // Cleanup previous chart
    if (chartRef.current) {
      if (chartRef.current._resizeCleanup) chartRef.current._resizeCleanup()
      chartRef.current.remove()
      chartRef.current = null
    }

    try {

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
        secondsVisible: false,
      },
      rightPriceScale: {
        borderColor: '#2a2a34',
        scaleMargins: { top: 0.1, bottom: 0.1 },
      },
      crosshair: {
        mode: 1,
        vertLine: { color: '#6366f1', width: 1, style: 2 },
        horzLine: { color: '#6366f1', width: 1, style: 2 },
      },
    })
    chartRef.current = chart

    let markerSeries = null

    indicators.forEach(series => {
      try {
        if (!series.data?.length) return
        const validPt = d => d.time && Number.isFinite(d.value)
        if (series.type === 'histogram') {
          const histSeries = chart.addHistogramSeries({
            priceFormat: { type: 'price', precision: 2 },
            priceScaleId: 'right',
          })
          histSeries.setData(
            series.data.filter(validPt).map(d => ({
              time: d.time,
              value: d.value,
              color: d.color || series.color || '#6366f1',
            }))
          )
          if (!markerSeries) markerSeries = histSeries
        } else if (series.type === 'reference_line') {
          const refData = indicators[0]?.data
          if (refData?.length) {
            const refLine = chart.addLineSeries({
              color: series.color || '#4b5563',
              lineWidth: 1,
              lineStyle: 2,
              priceScaleId: 'right',
              crosshairMarkerVisible: false,
              lastValueVisible: false,
              priceLineVisible: false,
            })
            const val = series.data?.[0]?.value ?? 0
            refLine.setData(refData.filter(d => d.time).map(d => ({ time: d.time, value: val })))
          }
        } else if (series.type === 'line') {
          const lineSeries = chart.addLineSeries({
            color: series.color || '#f59e0b',
            lineWidth: 2,
            priceScaleId: 'right',
            lastValueVisible: false,
            priceLineVisible: false,
          })
          lineSeries.setData(
            series.data.filter(validPt).map(d => ({
              time: d.time,
              value: d.value,
            }))
          )
          if (!markerSeries) markerSeries = lineSeries
        }
      } catch (err) {
        console.warn('[ArenaIndicatorChart] Series render error:', series.name, err)
      }
    })

    // Zero line
    const firstData = indicators[0]?.data
    if (firstData?.length) {
      const zeroLine = chart.addLineSeries({
        color: '#4b5563',
        lineWidth: 1,
        lineStyle: 2,
        priceScaleId: 'right',
        crosshairMarkerVisible: false,
        lastValueVisible: false,
        priceLineVisible: false,
      })
      zeroLine.setData(firstData.filter(d => d.time).map(d => ({ time: d.time, value: 0 })))
    }

    // Set markers on first available series (histogram or line)
    if (markerSeries && markers?.length) {
      markerSeries.setMarkers(
        markers.filter(m => m.time).sort((a, b) => a.time - b.time)
      )
    }

    chart.timeScale().fitContent()

    // Resize handler (must be inside try — chart is block-scoped)
    const handleResize = () => {
      if (chartContainerRef.current && chartRef.current) {
        chartRef.current.applyOptions({ width: chartContainerRef.current.clientWidth })
      }
    }
    window.addEventListener('resize', handleResize)
    chartRef.current._resizeCleanup = () => window.removeEventListener('resize', handleResize)

    } catch (err) {
      console.error('[ArenaIndicatorChart] Chart render error:', err)
      return
    }

    return () => {
      if (chartRef.current) {
        if (chartRef.current._resizeCleanup) chartRef.current._resizeCleanup()
        chartRef.current.remove()
        chartRef.current = null
      }
    }
  }, [indicators, markers])

  if (!indicators?.length) return null

  return (
    <div className="bg-dark-800 rounded-lg border border-dark-600 overflow-hidden mb-4">
      <div className="px-4 py-2 border-b border-dark-600 flex items-center justify-between">
        <div className="flex items-center gap-2">
          <span className="text-sm font-medium text-white">{strategyName || 'Indikator'}</span>
          {indicators.map((s, i) => (
            <div key={i} className="flex items-center gap-1">
              <div className="w-2 h-2 rounded-sm" style={{ backgroundColor: s.color || '#6366f1' }} />
              <span className="text-xs text-gray-400">{s.name}</span>
            </div>
          ))}
        </div>
      </div>
      <div ref={chartContainerRef} className="w-full" style={{ height: 200 }} />
    </div>
  )
}

function ArenaIndicatorChartWithBoundary(props) {
  return <IndicatorErrorBoundary><ArenaIndicatorChart {...props} /></IndicatorErrorBoundary>
}

export default ArenaIndicatorChartWithBoundary
