import { useEffect, useRef, useState } from 'react'
import { createChart } from 'lightweight-charts'

// Color palette for portfolio lines
const PORTFOLIO_COLORS = [
  '#6366f1', // indigo
  '#22c55e', // green
  '#f59e0b', // amber
  '#ef4444', // red
  '#8b5cf6', // violet
  '#06b6d4', // cyan
  '#ec4899', // pink
  '#f97316', // orange
  '#14b8a6', // teal
  '#a855f7', // purple
]

export function getPortfolioColor(index) {
  return PORTFOLIO_COLORS[index % PORTFOLIO_COLORS.length]
}

function MultiPortfolioChart({ token, height = 300, portfolios = [], onColorMap }) {
  const chartContainerRef = useRef(null)
  const chartRef = useRef(null)
  const seriesRef = useRef([])
  const [period, setPeriod] = useState('1m')
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState(null)

  const periodLabels = {
    '1w': 'Woche',
    '1m': 'Monat',
    '3m': '3M',
    '6m': '6M',
    'ytd': 'YTD',
    '1y': '1J',
    '5y': '5J'
  }

  useEffect(() => {
    if (!chartContainerRef.current) return

    // Create chart
    const chart = createChart(chartContainerRef.current, {
      layout: {
        background: { type: 'solid', color: '#12121a' },
        textColor: '#9ca3af',
      },
      grid: {
        vertLines: { color: '#2a2a34', style: 1 },
        horzLines: { color: '#2a2a34', style: 1 },
      },
      width: chartContainerRef.current.clientWidth,
      height: height,
      rightPriceScale: {
        borderColor: '#2a2a34',
        scaleMargins: { top: 0.1, bottom: 0.1 },
      },
      timeScale: {
        borderColor: '#2a2a34',
        timeVisible: true,
        secondsVisible: false,
      },
      crosshair: {
        mode: 1,
        vertLine: { color: '#6366f1', width: 1, style: 2 },
        horzLine: { color: '#6366f1', width: 1, style: 2 },
      },
    })

    chartRef.current = chart

    // Handle resize
    const handleResize = () => {
      if (chartContainerRef.current && chart) {
        chart.applyOptions({ width: chartContainerRef.current.clientWidth })
      }
    }
    window.addEventListener('resize', handleResize)

    return () => {
      window.removeEventListener('resize', handleResize)
      chart.remove()
    }
  }, [height])

  useEffect(() => {
    if (!chartRef.current || !token) return

    const fetchData = async () => {
      setLoading(true)
      setError(null)

      try {
        const res = await fetch(`/api/portfolios/history/all?period=${period}`, {
          headers: { 'Authorization': `Bearer ${token}` }
        })

        if (!res.ok) throw new Error('Failed to fetch data')

        const data = await res.json()

        if (!data || data.length === 0) {
          setError('Keine Daten verfugbar')
          setLoading(false)
          return
        }

        // Remove old series
        seriesRef.current.forEach(series => {
          try {
            chartRef.current.removeSeries(series)
          } catch (e) {
            // Series might already be removed
          }
        })
        seriesRef.current = []

        // Build color map for parent component
        const colorMap = {}

        // Add a line series for each portfolio
        data.forEach((portfolio, index) => {
          const color = getPortfolioColor(index)
          colorMap[portfolio.user_id] = color

          const series = chartRef.current.addLineSeries({
            color: color,
            lineWidth: 2,
            priceFormat: {
              type: 'percent',
            },
            title: portfolio.username,
          })

          // Format data for chart
          const chartData = portfolio.history.map(point => ({
            time: point.time,
            value: point.pct
          }))

          series.setData(chartData)
          seriesRef.current.push(series)
        })

        // Notify parent about color mapping
        if (onColorMap) {
          onColorMap(colorMap)
        }

        // Add baseline at 0%
        if (seriesRef.current.length > 0) {
          seriesRef.current[0].createPriceLine({
            price: 0,
            color: '#4b5563',
            lineWidth: 1,
            lineStyle: 2,
            axisLabelVisible: true,
            title: '0%',
          })
        }

        chartRef.current.timeScale().fitContent()
      } catch (err) {
        console.error('Error fetching portfolio histories:', err)
        setError('Fehler beim Laden der Daten')
      } finally {
        setLoading(false)
      }
    }

    fetchData()
  }, [period, token, onColorMap])

  return (
    <div className="bg-dark-800 rounded-xl border border-dark-600 overflow-hidden">
      {/* Header with period selector */}
      <div className="flex flex-col sm:flex-row sm:items-center justify-between p-4 border-b border-dark-600 gap-3">
        <h3 className="text-white font-semibold">Portfolio Vergleich</h3>
        <div className="flex flex-wrap gap-1">
          {Object.entries(periodLabels).map(([key, label]) => (
            <button
              key={key}
              onClick={() => setPeriod(key)}
              className={`px-2 md:px-3 py-1 text-xs md:text-sm font-medium rounded-md transition-colors ${
                period === key
                  ? 'bg-accent-500 text-white'
                  : 'bg-dark-700 text-gray-400 hover:text-white'
              }`}
            >
              {label}
            </button>
          ))}
        </div>
      </div>

      {/* Chart container */}
      <div className="relative" style={{ height: `${height}px` }}>
        {loading && (
          <div className="absolute inset-0 flex items-center justify-center bg-dark-800/80 z-10">
            <div className="w-8 h-8 border-2 border-accent-500 border-t-transparent rounded-full animate-spin"></div>
          </div>
        )}
        {error && !loading && (
          <div className="absolute inset-0 flex items-center justify-center bg-dark-800">
            <p className="text-gray-500">{error}</p>
          </div>
        )}
        <div ref={chartContainerRef} className="w-full h-full" />
      </div>

      {/* Legend */}
      <div className="p-3 border-t border-dark-600 text-xs text-gray-500 text-center">
        Performance in % seit Kauf
      </div>
    </div>
  )
}

export default MultiPortfolioChart
