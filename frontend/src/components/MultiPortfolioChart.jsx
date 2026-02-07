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

function MultiPortfolioChart({ token, height = 300, portfolios = [], onColorMap, period: externalPeriod, onPeriodChange, onDataLoaded }) {
  const chartContainerRef = useRef(null)
  const chartRef = useRef(null)
  const seriesRef = useRef([])
  const [internalPeriod, setInternalPeriod] = useState('1m')
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState(null)
  const [legendItems, setLegendItems] = useState([])

  const period = externalPeriod !== undefined ? externalPeriod : internalPeriod
  const handlePeriodChange = (newPeriod) => {
    if (onPeriodChange) onPeriodChange(newPeriod)
    else setInternalPeriod(newPeriod)
  }

  const periodLabels = {
    '1d': '1D',
    '1w': '1W',
    '1m': '1M',
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
        vertLines: { visible: false },
        horzLines: { visible: false },
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
        const legend = []
        data.forEach((portfolio, index) => {
          const color = getPortfolioColor(index)
          colorMap[portfolio.user_id] = color

          const series = chartRef.current.addLineSeries({
            color: color,
            lineWidth: 2,
            priceLineVisible: false,
            lastValueVisible: false,
            priceFormat: {
              type: 'custom',
              formatter: (price) => price.toFixed(2) + '%',
              minMove: 0.01,
            },
          })

          // Format data for chart
          const chartData = portfolio.history.map(point => ({
            time: point.time,
            value: point.pct
          }))

          const lastPct = chartData.length > 0 ? chartData[chartData.length - 1].value : 0
          legend.push({ name: portfolio.username, color, pct: lastPct })

          series.setData(chartData)
          seriesRef.current.push(series)
        })
        setLegendItems(legend)

        // Notify parent about color mapping
        if (onColorMap) {
          onColorMap(colorMap)
        }

        // Notify parent about loaded data
        if (onDataLoaded) {
          onDataLoaded(data)
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
  }, [period, token, onColorMap, onDataLoaded])

  return (
    <div className="bg-dark-800 rounded-xl border border-dark-600 overflow-hidden">
      {/* Header with period selector */}
      <div className="flex flex-col sm:flex-row sm:items-center justify-between p-4 border-b border-dark-600 gap-3">
        <h3 className="text-white font-semibold">Portfolio Vergleich</h3>
        <div className="flex flex-wrap gap-1">
          {Object.entries(periodLabels).map(([key, label]) => (
            <button
              key={key}
              onClick={() => handlePeriodChange(key)}
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
      {legendItems.length > 0 && (
        <div className="px-4 py-2 border-t border-dark-600 flex flex-wrap gap-x-4 gap-y-1">
          {legendItems.map((item, i) => (
            <div key={i} className="flex items-center gap-1.5 text-xs">
              <div className="w-2.5 h-2.5 rounded-full shrink-0" style={{ backgroundColor: item.color }} />
              <span className="text-gray-400">{item.name}</span>
              <span className={`font-medium ${item.pct >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                {item.pct >= 0 ? '+' : ''}{item.pct.toFixed(2)}%
              </span>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}

export default MultiPortfolioChart
