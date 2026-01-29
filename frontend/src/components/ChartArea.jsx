import { useEffect, useRef, useState, useCallback } from 'react'
import { formatPrice, formatChange } from '../utils/currency'
import BXtrenderChart from './BXtrenderChart'
import BacktestPanel from './BacktestPanel'

function ChartArea({ stock, stocks, onBacktestUpdate, onSelectStock, backtestData }) {
  const chartContainerRef = useRef(null)
  const [timeframe, setTimeframe] = useState('M')
  const [performanceExpanded, setPerformanceExpanded] = useState(false)
  const [, forceUpdate] = useState(0)

  useEffect(() => {
    const handleCurrencyChange = () => forceUpdate(n => n + 1)
    window.addEventListener('currencyChanged', handleCurrencyChange)
    return () => window.removeEventListener('currencyChanged', handleCurrencyChange)
  }, [])

  const handleTradesUpdate = useCallback((data) => {
    if (onBacktestUpdate) {
      onBacktestUpdate(data)
    }
  }, [onBacktestUpdate])

  // Only re-render chart when symbol or timeframe changes, not on price updates
  const stockSymbol = stock?.symbol

  useEffect(() => {
    if (!stockSymbol || !chartContainerRef.current) return

    chartContainerRef.current.innerHTML = ''

    const script = document.createElement('script')
    script.src = 'https://s3.tradingview.com/external-embedding/embed-widget-advanced-chart.js'
    script.type = 'text/javascript'
    script.async = true
    script.innerHTML = JSON.stringify({
      autosize: true,
      symbol: stockSymbol,
      interval: timeframe,
      timezone: 'Europe/Berlin',
      theme: 'dark',
      style: '1',
      locale: 'en',
      backgroundColor: 'rgba(18, 18, 26, 1)',
      gridColor: 'rgba(42, 42, 52, 0.5)',
      hide_top_toolbar: false,
      hide_legend: false,
      allow_symbol_change: true,
      save_image: false,
      calendar: false,
      support_host: 'https://www.tradingview.com'
    })

    const container = document.createElement('div')
    container.className = 'tradingview-widget-container'
    container.style.height = '100%'
    container.style.width = '100%'

    const widgetDiv = document.createElement('div')
    widgetDiv.className = 'tradingview-widget-container__widget'
    widgetDiv.style.height = '100%'
    widgetDiv.style.width = '100%'

    container.appendChild(widgetDiv)
    container.appendChild(script)
    chartContainerRef.current.appendChild(container)

    return () => {
      if (chartContainerRef.current) {
        chartContainerRef.current.innerHTML = ''
      }
    }
  }, [stockSymbol, timeframe])

  if (!stock) {
    return (
      <div className="h-full flex flex-col">
        <div className="mb-4">
          <h2 className="text-xl font-semibold text-white">Market Overview</h2>
          <p className="text-gray-500 text-sm">Select a stock from your watchlist</p>
        </div>

        <div className="flex-1 bg-dark-800 rounded-xl border border-dark-600 flex items-center justify-center">
          <div className="text-center p-8">
            <div className="w-24 h-24 mx-auto mb-6 bg-dark-700 rounded-full flex items-center justify-center">
              <svg className="w-12 h-12 text-gray-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M13 7h8m0 0v8m0-8l-8 8-4-4-6 6" />
              </svg>
            </div>
            <h3 className="text-lg font-medium text-gray-400 mb-2">Select a Stock</h3>
            <p className="text-gray-600 text-sm max-w-md">
              Click on a stock in your watchlist to view its chart with real-time price data.
            </p>

            {stocks && stocks.length > 0 && (
              <div className="mt-6 grid grid-cols-2 md:grid-cols-4 gap-3 max-w-2xl mx-auto">
                {stocks.slice(0, 4).map((s) => {
                  const change = formatChange(s.change, s.change_percent)
                  return (
                    <div
                      key={s.id}
                      onClick={() => onSelectStock && onSelectStock(s)}
                      className="bg-dark-700 rounded-lg p-3 text-left cursor-pointer hover:bg-dark-600 transition-colors border border-transparent hover:border-accent-500"
                    >
                      <div className="font-semibold text-white text-sm">{s.symbol}</div>
                      <div className="text-lg font-bold text-white">{formatPrice(s.price)}</div>
                      {change && (
                        <div className={`text-xs ${change.isPositive ? 'text-green-400' : 'text-red-400'}`}>
                          {change.text}
                        </div>
                      )}
                    </div>
                  )
                })}
              </div>
            )}
          </div>
        </div>
      </div>
    )
  }

  const changeData = formatChange(stock.change, stock.change_percent)

  return (
    <div className="h-full flex flex-col">
      <div className="mb-4 flex items-start justify-between">
        <div>
          <div className="flex items-center gap-3">
            <h2 className="text-2xl font-bold text-white">{stock.symbol}</h2>
            {changeData && (
              <span className={`px-2 py-1 rounded text-xs font-medium ${changeData.isPositive ? 'bg-green-500/20 text-green-400' : 'bg-red-500/20 text-red-400'}`}>
                {changeData.isPositive ? 'UP' : 'DOWN'}
              </span>
            )}
          </div>
          <p className="text-gray-500 text-sm">{stock.name}</p>
        </div>
        <div className="flex items-center gap-4">
          {/* Timeframe Selection */}
          <div className="flex bg-dark-700 rounded-lg p-1">
            {[
              { key: 'M', label: 'M', title: 'Monthly' },
              { key: 'W', label: 'W', title: 'Weekly' },
              { key: 'D', label: 'D', title: 'Daily' }
            ].map(({ key, label, title }) => (
              <button
                key={key}
                onClick={() => setTimeframe(key)}
                title={title}
                className={`px-3 py-1.5 text-sm font-medium rounded-md transition-colors ${
                  timeframe === key
                    ? 'bg-accent-500 text-white'
                    : 'text-gray-400 hover:text-white hover:bg-dark-600'
                }`}
              >
                {label}
              </button>
            ))}
          </div>
          <div className="text-right">
            <div className="text-3xl font-bold text-white">{formatPrice(stock.price)}</div>
            {changeData && (
              <div className={`text-sm ${changeData.isPositive ? 'text-green-400' : 'text-red-400'}`}>
                {changeData.text}
              </div>
            )}
          </div>
        </div>
      </div>

      {/* Main TradingView Chart - constrained height to keep BXtrender visible */}
      <div className="min-h-[250px] max-h-[350px] md:min-h-[350px] md:max-h-[55vh] bg-dark-800 rounded-xl border border-dark-600 overflow-hidden mb-4" ref={chartContainerRef}>
        <div className="h-full flex items-center justify-center">
          <div className="w-8 h-8 border-2 border-accent-500 border-t-transparent rounded-full animate-spin"></div>
        </div>
      </div>

      {/* System Performance - Mobile only, collapsible */}
      <div className="md:hidden mb-4 bg-dark-800 rounded-xl border border-dark-600 overflow-hidden">
        <button
          onClick={() => setPerformanceExpanded(!performanceExpanded)}
          className="flex items-center justify-between w-full px-4 py-3 bg-dark-700"
        >
          <div className="flex items-center gap-2">
            <svg className="w-5 h-5 text-accent-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M9 19v-6a2 2 0 00-2-2H5a2 2 0 00-2 2v6a2 2 0 002 2h2a2 2 0 002-2zm0 0V9a2 2 0 012-2h2a2 2 0 012 2v10m-6 0a2 2 0 002 2h2a2 2 0 002-2m0 0V5a2 2 0 012-2h2a2 2 0 012 2v14a2 2 0 01-2 2h-2a2 2 0 01-2-2z" />
            </svg>
            <span className="text-white font-medium">System Performance</span>
          </div>
          <svg
            className={`w-5 h-5 text-gray-400 transition-transform duration-200 ${performanceExpanded ? 'rotate-180' : ''}`}
            fill="none"
            stroke="currentColor"
            viewBox="0 0 24 24"
          >
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 9l-7 7-7-7" />
          </svg>
        </button>
        <div className={`
          overflow-hidden transition-all duration-300 ease-in-out
          ${performanceExpanded ? 'max-h-[400px] overflow-y-auto' : 'max-h-0'}
        `}>
          <BacktestPanel
            trades={backtestData?.trades || []}
            metrics={backtestData?.metrics || null}
          />
        </div>
      </div>

      {/* B-Xtrender Indicator */}
      <BXtrenderChart
        symbol={stock.symbol}
        timeframe={timeframe}
        onTradesUpdate={handleTradesUpdate}
      />
    </div>
  )
}

export default ChartArea
