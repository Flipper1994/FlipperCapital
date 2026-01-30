import { useEffect, useRef, useState, useCallback } from 'react'
import { Link } from 'react-router-dom'
import { formatPrice, formatChange } from '../utils/currency'
import BXtrenderChart from './BXtrenderChart'

function ChartArea({ stock, stocks, onBacktestUpdate, onSelectStock, backtestData }) {
  const chartContainerRef = useRef(null)
  const [timeframe, setTimeframe] = useState('M')
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

  // Feature news cards for empty state
  const featureNews = [
    {
      id: 'charts',
      icon: 'M13 7h8m0 0v8m0-8l-8 8-4-4-6 6',
      title: 'Echtzeit-Charts',
      description: 'TradingView Charts mit Live-Kursen und technischen Indikatoren',
      color: 'from-blue-500/20 to-blue-600/10',
      borderColor: 'border-blue-500/30'
    },
    {
      id: 'signals',
      icon: 'M9 19v-6a2 2 0 00-2-2H5a2 2 0 00-2 2v6a2 2 0 002 2h2a2 2 0 002-2zm0 0V9a2 2 0 012-2h2a2 2 0 012 2v10m-6 0a2 2 0 002 2h2a2 2 0 002-2m0 0V5a2 2 0 012-2h2a2 2 0 012 2v14a2 2 0 01-2 2h-2a2 2 0 01-2-2z',
      title: 'Trading-Signale',
      description: 'Automatische BUY/SELL Signale mit dem B-Xtrender Indikator',
      color: 'from-green-500/20 to-green-600/10',
      borderColor: 'border-green-500/30'
    },
    {
      id: 'portfolio',
      icon: 'M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2',
      title: 'Portfolio Tracking',
      description: 'Verwalte deine Investments und tracke deine Performance',
      color: 'from-purple-500/20 to-purple-600/10',
      borderColor: 'border-purple-500/30'
    },
    {
      id: 'compare',
      icon: 'M16 8v8m-4-5v5m-4-2v2m-2 4h12a2 2 0 002-2V6a2 2 0 00-2-2H6a2 2 0 00-2 2v12a2 2 0 002 2z',
      title: 'Portfolio Vergleich',
      description: 'Vergleiche deine Performance mit anderen Nutzern',
      color: 'from-amber-500/20 to-amber-600/10',
      borderColor: 'border-amber-500/30'
    },
    {
      id: 'tracker',
      icon: 'M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z',
      title: 'Aktien Tracker',
      description: 'Übersicht aller Signale mit Win Rate und Performance',
      color: 'from-cyan-500/20 to-cyan-600/10',
      borderColor: 'border-cyan-500/30'
    },
    {
      id: 'currency',
      icon: 'M12 8c-1.657 0-3 .895-3 2s1.343 2 3 2 3 .895 3 2-1.343 2-3 2m0-8c1.11 0 2.08.402 2.599 1M12 8V7m0 1v8m0 0v1m0-1c-1.11 0-2.08-.402-2.599-1M21 12a9 9 0 11-18 0 9 9 0 0118 0z',
      title: 'Multi-Währung',
      description: 'Preise in USD, EUR, GBP oder CHF anzeigen',
      color: 'from-pink-500/20 to-pink-600/10',
      borderColor: 'border-pink-500/30'
    },
    {
      id: 'flipperbot',
      icon: 'M19.428 15.428a2 2 0 00-1.022-.547l-2.387-.477a6 6 0 00-3.86.517l-.318.158a6 6 0 01-3.86.517L6.05 15.21a2 2 0 00-1.806.547M8 4h8l-1 1v5.172a2 2 0 00.586 1.414l5 5c1.26 1.26.367 3.414-1.415 3.414H4.828c-1.782 0-2.674-2.154-1.414-3.414l5-5A2 2 0 009 10.172V5L8 4z',
      title: 'FlipperBot Lab',
      description: 'Automatisiertes Trading basierend auf B-Xtrender Signalen',
      color: 'from-purple-500/20 to-pink-500/10',
      borderColor: 'border-purple-500/30',
      badge: 'BETA'
    }
  ]

  if (!stock) {
    return (
      <div className="h-full flex flex-col">
        {/* Header */}
        <div className="mb-4">
          <h2 className="text-xl font-semibold text-white">Willkommen bei FlipperCapital</h2>
          <p className="text-gray-500 text-sm">Wähle eine Aktie aus der Watchlist oder entdecke unsere Features</p>
        </div>

        {/* Main Content Area */}
        <div className="flex-1 overflow-auto">
          {/* Quick Stock Selection */}
          {stocks && stocks.length > 0 && (
            <div className="mb-6">
              <h3 className="text-sm font-medium text-gray-400 mb-3">Schnellauswahl</h3>
              <div className="grid grid-cols-2 md:grid-cols-4 gap-3">
                {stocks.slice(0, 4).map((s) => {
                  const change = formatChange(s.change, s.change_percent)
                  return (
                    <div
                      key={s.id}
                      onClick={() => onSelectStock && onSelectStock(s)}
                      className="bg-dark-800 rounded-xl p-4 cursor-pointer hover:bg-dark-700 transition-all border border-dark-600 hover:border-accent-500 group"
                    >
                      <div className="font-semibold text-white group-hover:text-accent-400 transition-colors">{s.symbol}</div>
                      <div className="text-lg font-bold text-white mt-1">{formatPrice(s.price)}</div>
                      {change && (
                        <div className={`text-xs mt-1 ${change.isPositive ? 'text-green-400' : 'text-red-400'}`}>
                          {change.text}
                        </div>
                      )}
                    </div>
                  )
                })}
              </div>
            </div>
          )}

          {/* Feature News Grid */}
          <div className="mb-6">
            <div className="flex items-center justify-between mb-3">
              <h3 className="text-sm font-medium text-gray-400">Verfügbare Features</h3>
              <Link
                to="/help"
                className="text-xs text-accent-400 hover:text-accent-300 flex items-center gap-1"
              >
                Alle Features
                <svg className="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 5l7 7-7 7" />
                </svg>
              </Link>
            </div>

            {/* Desktop Grid */}
            <div className="hidden md:grid grid-cols-3 gap-4">
              {featureNews.map((feature) => (
                <Link
                  key={feature.id}
                  to="/help"
                  className={`bg-gradient-to-br ${feature.color} rounded-xl border ${feature.borderColor} p-4 hover:scale-[1.02] transition-transform cursor-pointer group`}
                >
                  <div className="flex items-start gap-3">
                    <div className="w-10 h-10 bg-dark-800/50 rounded-lg flex items-center justify-center shrink-0">
                      <svg className="w-5 h-5 text-white" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d={feature.icon} />
                      </svg>
                    </div>
                    <div className="flex-1 min-w-0">
                      <div className="flex items-center gap-2">
                        <h4 className="font-medium text-white group-hover:text-accent-400 transition-colors">{feature.title}</h4>
                        {feature.badge && (
                          <span className="px-1.5 py-0.5 bg-gradient-to-r from-purple-500 to-pink-500 text-white text-[10px] font-bold rounded">
                            {feature.badge}
                          </span>
                        )}
                      </div>
                      <p className="text-xs text-gray-400 mt-1 line-clamp-2">{feature.description}</p>
                    </div>
                  </div>
                </Link>
              ))}
            </div>

            {/* Mobile List */}
            <div className="md:hidden space-y-3">
              {featureNews.slice(0, 4).map((feature) => (
                <Link
                  key={feature.id}
                  to="/help"
                  className={`block bg-gradient-to-br ${feature.color} rounded-xl border ${feature.borderColor} p-4`}
                >
                  <div className="flex items-center gap-3">
                    <div className="w-10 h-10 bg-dark-800/50 rounded-lg flex items-center justify-center shrink-0">
                      <svg className="w-5 h-5 text-white" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d={feature.icon} />
                      </svg>
                    </div>
                    <div className="flex-1 min-w-0">
                      <h4 className="font-medium text-white">{feature.title}</h4>
                      <p className="text-xs text-gray-400 truncate">{feature.description}</p>
                    </div>
                    <svg className="w-5 h-5 text-gray-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 5l7 7-7 7" />
                    </svg>
                  </div>
                </Link>
              ))}

              {/* Show more link on mobile */}
              <Link
                to="/help"
                className="block text-center py-3 text-accent-400 hover:text-accent-300 text-sm font-medium"
              >
                Alle Features & Hilfe anzeigen
              </Link>
            </div>
          </div>

          {/* Help CTA */}
          <div className="bg-dark-800 rounded-xl border border-dark-600 p-6 text-center">
            <div className="w-16 h-16 mx-auto mb-4 bg-accent-500/20 rounded-full flex items-center justify-center">
              <svg className="w-8 h-8 text-accent-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M8.228 9c.549-1.165 2.03-2 3.772-2 2.21 0 4 1.343 4 3 0 1.4-1.278 2.575-3.006 2.907-.542.104-.994.54-.994 1.093m0 3h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
              </svg>
            </div>
            <h3 className="text-lg font-medium text-white mb-2">Neu hier?</h3>
            <p className="text-gray-500 text-sm mb-4 max-w-md mx-auto">
              Schau dir unsere Hilfe-Seite an, um alle Funktionen von FlipperCapital kennenzulernen.
            </p>
            <Link
              to="/help"
              className="inline-flex items-center gap-2 px-5 py-2.5 bg-accent-500 text-white rounded-lg hover:bg-accent-400 transition-colors font-medium text-sm"
            >
              <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
              </svg>
              Zur Hilfe
            </Link>
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

      {/* Main TradingView Chart - fills available space with max height to keep BXtrender visible */}
      <div className="flex-1 min-h-[250px] max-h-[350px] md:min-h-[300px] md:max-h-[calc(100vh-480px)] bg-dark-800 rounded-xl border border-dark-600 overflow-hidden mb-4" ref={chartContainerRef}>
        <div className="h-full flex items-center justify-center">
          <div className="w-8 h-8 border-2 border-accent-500 border-t-transparent rounded-full animate-spin"></div>
        </div>
      </div>

      {/* B-Xtrender Indicator */}
      <BXtrenderChart
        symbol={stock.symbol}
        stockName={stock.name}
        timeframe={timeframe}
        onTradesUpdate={handleTradesUpdate}
      />
    </div>
  )
}

export default ChartArea
