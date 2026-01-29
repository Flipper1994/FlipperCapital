import { useEffect, useRef, useState } from 'react'
import { createChart } from 'lightweight-charts'

function PortfolioChart({ userId, token, height = 300 }) {
  const chartContainerRef = useRef(null)
  const chartRef = useRef(null)
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
        const endpoint = userId
          ? `/api/portfolios/history/${userId}?period=${period}`
          : `/api/portfolio/history?period=${period}`

        const res = await fetch(endpoint, {
          headers: { 'Authorization': `Bearer ${token}` }
        })

        if (!res.ok) throw new Error('Failed to fetch data')

        const data = await res.json()

        if (!data || data.length === 0) {
          setError('Keine Daten verfügbar')
          setLoading(false)
          return
        }

        // Clear existing series
        chartRef.current.timeScale().fitContent()

        // Remove old series if any
        const series = chartRef.current.addAreaSeries({
          lineColor: '#6366f1',
          topColor: 'rgba(99, 102, 241, 0.4)',
          bottomColor: 'rgba(99, 102, 241, 0.0)',
          lineWidth: 2,
          priceFormat: {
            type: 'percent',
          },
        })

        // Format data for chart - use percentage change
        const chartData = data.map(point => ({
          time: point.time,
          value: point.pct
        }))

        series.setData(chartData)

        // Add baseline at 0%
        series.createPriceLine({
          price: 0,
          color: '#4b5563',
          lineWidth: 1,
          lineStyle: 2,
          axisLabelVisible: true,
          title: '0%',
        })

        chartRef.current.timeScale().fitContent()
      } catch (err) {
        console.error('Error fetching portfolio history:', err)
        setError('Fehler beim Laden der Daten')
      } finally {
        setLoading(false)
      }
    }

    fetchData()
  }, [period, userId, token])

  return (
    <div className="bg-dark-800 rounded-xl border border-dark-600 overflow-hidden">
      {/* Header with period selector */}
      <div className="flex flex-col sm:flex-row sm:items-center justify-between p-4 border-b border-dark-600 gap-3">
        <h3 className="text-white font-semibold">Portfolio Performance</h3>
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
        Performance in % seit Kauf (gleichgewichtete Positionen)
      </div>
    </div>
  )
}

export default PortfolioChart
