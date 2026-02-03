import { useState, useEffect, useRef } from 'react'
import { Link } from 'react-router-dom'
import { useCurrency } from '../context/CurrencyContext'
import PortfolioChart from './PortfolioChart'

function FlipperBotLab({ isAdmin = false, isLoggedIn = false, token = '' }) {
  const [portfolio, setPortfolio] = useState(null)
  const [actions, setActions] = useState([])
  const [performance, setPerformance] = useState(null)
  const [loading, setLoading] = useState(true)
  const [logs, setLogs] = useState([])
  const [hasUserPortfolio, setHasUserPortfolio] = useState(false)
  const [checkingAccess, setCheckingAccess] = useState(true)
  const [showTradeHistory, setShowTradeHistory] = useState(false)
  const [completedTrades, setCompletedTrades] = useState([])
  const [loadingCompletedTrades, setLoadingCompletedTrades] = useState(false)
  const logEndRef = useRef(null)
  const { formatPrice } = useCurrency()

  // Check if user has portfolio positions
  useEffect(() => {
    const checkUserPortfolio = async () => {
      if (!isLoggedIn || !token) {
        setCheckingAccess(false)
        return
      }

      try {
        const res = await fetch('/api/portfolio', {
          headers: { 'Authorization': `Bearer ${token}` }
        })
        const data = await res.json()
        setHasUserPortfolio(data && data.length > 0)
      } catch (err) {
        console.error('Failed to check portfolio:', err)
        setHasUserPortfolio(false)
      } finally {
        setCheckingAccess(false)
      }
    }

    checkUserPortfolio()
  }, [isLoggedIn, token])

  useEffect(() => {
    if (isLoggedIn && hasUserPortfolio) {
      fetchData()
      fetchLogs()
      fetchCompletedTrades()
    } else {
      setLoading(false)
    }
  }, [isLoggedIn, hasUserPortfolio])

  useEffect(() => {
    // Auto-scroll to bottom of log
    if (logEndRef.current) {
      logEndRef.current.scrollIntoView({ behavior: 'smooth' })
    }
  }, [logs])

  const fetchData = async () => {
    setLoading(true)
    try {
      const [portfolioRes, actionsRes, perfRes] = await Promise.all([
        fetch('/api/flipperbot/portfolio', {
          headers: { 'Authorization': `Bearer ${token}` }
        }),
        fetch('/api/flipperbot/actions', {
          headers: { 'Authorization': `Bearer ${token}` }
        }),
        fetch('/api/flipperbot/performance', {
          headers: { 'Authorization': `Bearer ${token}` }
        })
      ])

      const portfolioData = await portfolioRes.json()
      const actionsData = await actionsRes.json()
      const perfData = await perfRes.json()

      setPortfolio(portfolioData)
      setActions(actionsData)
      setPerformance(perfData)
    } catch (err) {
      console.error('Failed to fetch FlipperBot data:', err)
    } finally {
      setLoading(false)
    }
  }

  const fetchLogs = async () => {
    if (!isAdmin) return
    try {
      const res = await fetch('/api/flipperbot/logs', {
        headers: { 'Authorization': `Bearer ${token}` }
      })
      const data = await res.json()
      // Transform to match existing format
      const formattedLogs = (data || []).map(log => ({
        level: log.level,
        message: log.message,
        time: new Date(log.created_at).toLocaleTimeString('de-DE')
      })).reverse() // oldest first
      setLogs(formattedLogs)
    } catch (err) {
      console.error('Failed to fetch logs:', err)
    }
  }

  const fetchCompletedTrades = async () => {
    setLoadingCompletedTrades(true)
    try {
      const res = await fetch('/api/flipperbot/completed-trades', {
        headers: { 'Authorization': `Bearer ${token}` }
      })
      const data = await res.json()
      setCompletedTrades(data || [])
    } catch (err) {
      console.error('Failed to fetch completed trades:', err)
    } finally {
      setLoadingCompletedTrades(false)
    }
  }

  const formatPercent = (value) => {
    if (value === undefined || value === null || isNaN(value)) return '--'
    const sign = value >= 0 ? '+' : ''
    return `${sign}${value.toFixed(2)}%`
  }

  const formatDate = (dateStr) => {
    if (!dateStr) return '-'
    const date = new Date(dateStr)
    return date.toLocaleDateString('de-DE', { day: '2-digit', month: '2-digit', year: 'numeric' })
  }

  const getLogColor = (level) => {
    switch (level) {
      case 'ERROR': return 'text-red-400'
      case 'WARN': return 'text-yellow-400'
      case 'ACTION': return 'text-green-400'
      case 'SKIP': return 'text-gray-500'
      case 'DEBUG': return 'text-blue-400'
      default: return 'text-gray-300'
    }
  }

  const getLogBadge = (level) => {
    switch (level) {
      case 'ERROR': return 'bg-red-500/20 text-red-400'
      case 'WARN': return 'bg-yellow-500/20 text-yellow-400'
      case 'ACTION': return 'bg-green-500/20 text-green-400'
      case 'SKIP': return 'bg-gray-500/20 text-gray-500'
      case 'DEBUG': return 'bg-blue-500/20 text-blue-400'
      default: return 'bg-gray-500/20 text-gray-400'
    }
  }

  // Show loading while checking access
  if (checkingAccess || loading) {
    return (
      <div className="flex-1 p-4 md:p-6 flex items-center justify-center">
        <div className="text-center">
          <div className="w-12 h-12 border-2 border-accent-500 border-t-transparent rounded-full animate-spin mx-auto mb-4"></div>
          <p className="text-gray-400">Lade FlipperBot Daten...</p>
        </div>
      </div>
    )
  }

  // Access denied screen for non-logged-in users or users without portfolio
  if (!isLoggedIn || !hasUserPortfolio) {
    return (
      <div className="flex-1 p-4 md:p-6 flex items-center justify-center">
        <div className="max-w-md text-center">
          {/* Icon */}
          <div className="w-24 h-24 mx-auto mb-6 bg-gradient-to-br from-purple-500/20 to-pink-500/20 rounded-2xl flex items-center justify-center border border-purple-500/30">
            <svg className="w-12 h-12 text-purple-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z" />
            </svg>
          </div>

          {/* Title */}
          <h1 className="text-2xl font-bold text-white mb-2 flex items-center justify-center gap-2">
            FlipperBot Lab
            <span className="px-2 py-0.5 bg-gradient-to-r from-purple-500 to-pink-500 text-white text-xs font-bold rounded">
              BETA
            </span>
          </h1>

          {/* Message based on state */}
          {!isLoggedIn ? (
            <>
              <p className="text-gray-400 mb-6">
                Melde dich an, um den FlipperBot Lab zu nutzen und die automatisierte Trading-Strategie zu verfolgen.
              </p>
              <div className="flex flex-col sm:flex-row gap-3 justify-center">
                <Link
                  to="/login"
                  className="inline-flex items-center justify-center gap-2 px-6 py-3 bg-accent-500 text-white rounded-lg hover:bg-accent-400 transition-colors font-medium"
                >
                  <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M11 16l-4-4m0 0l4-4m-4 4h14m-5 4v1a3 3 0 01-3 3H6a3 3 0 01-3-3V7a3 3 0 013-3h7a3 3 0 013 3v1" />
                  </svg>
                  Anmelden
                </Link>
                <Link
                  to="/register"
                  className="inline-flex items-center justify-center gap-2 px-6 py-3 bg-dark-700 text-white rounded-lg hover:bg-dark-600 transition-colors font-medium border border-dark-600"
                >
                  <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M18 9v3m0 0v3m0-3h3m-3 0h-3m-2-5a4 4 0 11-8 0 4 4 0 018 0zM3 20a6 6 0 0112 0v1H3v-1z" />
                  </svg>
                  Registrieren
                </Link>
              </div>
            </>
          ) : (
            <>
              <p className="text-gray-400 mb-6">
                Um den FlipperBot Lab zu nutzen, musst du mindestens eine Aktie in deinem Portfolio haben.
                Füge zuerst eine Position hinzu, um die automatisierte Trading-Strategie zu verfolgen.
              </p>
              <Link
                to="/portfolio"
                className="inline-flex items-center justify-center gap-2 px-6 py-3 bg-accent-500 text-white rounded-lg hover:bg-accent-400 transition-colors font-medium"
              >
                <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 6v6m0 0v6m0-6h6m-6 0H6" />
                </svg>
                Portfolio aufbauen
              </Link>
            </>
          )}

          {/* Feature preview */}
          <div className="mt-10 p-4 bg-dark-800 rounded-xl border border-dark-600 text-left">
            <h3 className="text-sm font-medium text-gray-300 mb-3">Was dich erwartet:</h3>
            <ul className="space-y-2 text-sm text-gray-500">
              <li className="flex items-center gap-2">
                <svg className="w-4 h-4 text-green-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 13l4 4L19 7" />
                </svg>
                Automatisiertes Trading nach B-Xtrender Signalen
              </li>
              <li className="flex items-center gap-2">
                <svg className="w-4 h-4 text-green-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 13l4 4L19 7" />
                </svg>
                Performance-Tracking mit Rendite-Anzeige
              </li>
              <li className="flex items-center gap-2">
                <svg className="w-4 h-4 text-green-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 13l4 4L19 7" />
                </svg>
                Vergleich mit anderen Nutzern
              </li>
            </ul>
          </div>
        </div>
      </div>
    )
  }

  return (
    <div className="flex-1 p-4 md:p-6 overflow-auto">
      <div className="max-w-6xl mx-auto">
        {/* Header */}
        <div className="flex flex-col md:flex-row md:items-center justify-between gap-4 mb-6">
          <div>
            <div className="flex items-center gap-3">
              <div className="w-12 h-12 bg-gradient-to-br from-purple-500 to-pink-600 rounded-xl flex items-center justify-center">
                <svg className="w-7 h-7 text-white" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M19.428 15.428a2 2 0 00-1.022-.547l-2.387-.477a6 6 0 00-3.86.517l-.318.158a6 6 0 01-3.86.517L6.05 15.21a2 2 0 00-1.806.547M8 4h8l-1 1v5.172a2 2 0 00.586 1.414l5 5c1.26 1.26.367 3.414-1.415 3.414H4.828c-1.782 0-2.674-2.154-1.414-3.414l5-5A2 2 0 009 10.172V5L8 4z" />
                </svg>
              </div>
              <div>
                <h1 className="text-2xl font-bold text-white flex items-center gap-2">
                  FlipperBot Lab
                  <span className="px-2 py-0.5 bg-gradient-to-r from-purple-500 to-pink-500 text-white text-xs font-bold rounded">
                    BETA
                  </span>
                </h1>
                <p className="text-gray-500 text-sm">Automatisiertes Trading seit 01.01.2026</p>
              </div>
            </div>
          </div>
        </div>

        {/* Performance Chart */}
        {portfolio?.positions?.length > 0 && (
          <div className="mb-6">
            <PortfolioChart
              token={token}
              botType="flipperbot"
              height={250}
              title="FlipperBot Performance"
            />
          </div>
        )}

        {/* Performance Übersicht */}
        {performance && (
          <div className="bg-dark-800 rounded-xl border border-dark-600 p-4 md:p-6 mb-6">
            <h2 className="text-lg font-semibold text-white mb-4">Performance Übersicht</h2>

            {/* Main Stats */}
            <div className="grid grid-cols-2 md:grid-cols-4 gap-3 md:gap-4 mb-4">
              <div className="bg-gradient-to-r from-purple-500/20 to-pink-500/20 rounded-lg p-3 md:p-4 border border-purple-500/30">
                <div className="text-xs text-gray-400 mb-1">Gesamt Rendite</div>
                <div className={`text-xl md:text-2xl font-bold ${
                  performance.overall_return_pct >= 0 ? 'text-green-400' : 'text-red-400'
                }`}>
                  {formatPercent(performance.overall_return_pct)}
                </div>
                <div className={`text-xs mt-1 ${performance.total_gain >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                  {formatPrice(performance.total_gain)} Gewinn
                </div>
              </div>
              <div className="bg-dark-700 rounded-lg p-3 md:p-4">
                <div className="text-xs text-gray-500 mb-1">Investiert</div>
                <div className="text-lg md:text-xl font-bold text-white">
                  {formatPrice(performance.invested_in_positions || 0)}
                </div>
                <div className="text-xs text-gray-500 mt-1">
                  Aktuell: {formatPrice(performance.current_value || 0)}
                </div>
              </div>
              <div className="bg-dark-700 rounded-lg p-3 md:p-4">
                <div className="text-xs text-gray-500 mb-1">Unrealisiert</div>
                <div className={`text-lg md:text-xl font-bold ${
                  performance.unrealized_gain >= 0 ? 'text-green-400' : 'text-red-400'
                }`}>
                  {formatPrice(performance.unrealized_gain)}
                </div>
                <div className={`text-xs mt-1 ${
                  performance.total_return_pct >= 0 ? 'text-green-400' : 'text-red-400'
                }`}>
                  {formatPercent(performance.total_return_pct)}
                </div>
              </div>
              <div className="bg-dark-700 rounded-lg p-3 md:p-4">
                <div className="text-xs text-gray-500 mb-1">Realisiert</div>
                <div className={`text-lg md:text-xl font-bold ${
                  performance.realized_profit >= 0 ? 'text-green-400' : 'text-red-400'
                }`}>
                  {formatPrice(performance.realized_profit)}
                </div>
                <div className="text-xs text-gray-500 mt-1">
                  {performance.total_trades || 0} Trades
                </div>
              </div>
            </div>

            {/* Secondary Stats */}
            <div className="grid grid-cols-2 md:grid-cols-4 gap-3 md:gap-4">
              <div className="bg-dark-700/50 rounded-lg p-3">
                <div className="text-xs text-gray-500 mb-1">Win Rate</div>
                <div className="text-base font-bold text-white">
                  {performance.win_rate?.toFixed(1) || 0}%
                </div>
                <div className="text-xs text-gray-500 mt-1">
                  {performance.wins || 0}W / {performance.losses || 0}L
                </div>
              </div>
              <div className="bg-dark-700/50 rounded-lg p-3">
                <div className="text-xs text-gray-500 mb-1">Ø Rendite/Trade</div>
                <div className={`text-base font-bold ${
                  (performance.avg_return_per_trade || 0) >= 0 ? 'text-green-400' : 'text-red-400'
                }`}>
                  {(performance.avg_return_per_trade || 0) >= 0 ? '+' : ''}{(performance.avg_return_per_trade || 0).toFixed(2)}%
                </div>
                <div className="text-xs text-gray-500 mt-1">
                  gleichgewichtet
                </div>
              </div>
              <div className="bg-dark-700/50 rounded-lg p-3">
                <div className="text-xs text-gray-500 mb-1">Offene Positionen</div>
                <div className="text-base font-bold text-white">
                  {performance.open_positions || 0}
                </div>
                <div className="text-xs text-gray-500 mt-1">
                  von {performance.total_buys || 0} Käufen
                </div>
              </div>
              <div className="bg-dark-700/50 rounded-lg p-3">
                <div className="text-xs text-gray-500 mb-1">Live Positionen</div>
                <div className="text-base font-bold text-green-400">
                  {portfolio?.positions?.filter(p => p.is_live)?.length || 0}
                </div>
                <div className="text-xs text-gray-500 mt-1">
                  mit echtem Geld
                </div>
              </div>
            </div>
          </div>
        )}

        {/* Aktuelle Positionen */}
        <div className="bg-dark-800 rounded-xl border border-dark-600 overflow-hidden mb-6">
          <div className="p-4 border-b border-dark-600">
            <div className="flex items-center justify-between">
              <h2 className="text-lg font-semibold text-white">Aktuelle Positionen</h2>
              <div className="flex items-center gap-2">
                {portfolio?.positions?.filter(p => p.is_live)?.length > 0 && (
                  <span className="px-2 py-1 bg-green-500/20 text-green-400 text-sm font-medium rounded flex items-center gap-1">
                    <span className="w-1.5 h-1.5 bg-green-400 rounded-full animate-pulse"></span>
                    {portfolio.positions.filter(p => p.is_live).length} Live
                  </span>
                )}
                {portfolio?.positions && (
                  <span className="px-2 py-1 bg-accent-500/20 text-accent-400 text-sm font-medium rounded">
                    {portfolio.positions.length} offen
                  </span>
                )}
              </div>
            </div>
          </div>

          {portfolio?.positions?.length === 0 ? (
            <div className="p-8 text-center">
              <div className="w-16 h-16 mx-auto mb-4 bg-dark-700 rounded-full flex items-center justify-center">
                <svg className="w-8 h-8 text-gray-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M20 13V6a2 2 0 00-2-2H6a2 2 0 00-2 2v7m16 0v5a2 2 0 01-2 2H6a2 2 0 01-2-2v-5m16 0h-2.586a1 1 0 00-.707.293l-2.414 2.414a1 1 0 01-.707.293h-3.172a1 1 0 01-.707-.293l-2.414-2.414A1 1 0 006.586 13H4" />
                </svg>
              </div>
              <p className="text-gray-500">Keine offenen Positionen</p>
              <p className="text-gray-600 text-sm mt-1">Warte auf neue Signale</p>
            </div>
          ) : (
            <>
              {/* Mobile Card View */}
              <div className="md:hidden space-y-3 p-4">
                {portfolio?.positions?.slice().sort((a, b) => (b.is_live ? 1 : 0) - (a.is_live ? 1 : 0)).map((pos) => {
                  const totalValue = (pos.current_price || 0) * (pos.quantity || 1)
                  const totalCost = (pos.avg_price || 0) * (pos.quantity || 1)
                  const gain = totalValue - totalCost
                  return (
                    <div key={pos.id} className={`bg-dark-700 rounded-lg p-4 ${pos.is_live ? 'border-l-4 border-green-500' : ''}`}>
                      <div className="flex justify-between items-start mb-3">
                        <div>
                          <div className="font-semibold text-white flex items-center gap-2">
                            {pos.symbol}
                            {pos.is_live && (
                              <span className="px-2 py-0.5 bg-green-500 text-white text-[10px] font-bold rounded flex items-center gap-1">
                                <span className="w-1.5 h-1.5 bg-white rounded-full animate-pulse"></span>
                                LIVE
                              </span>
                            )}
                          </div>
                          <div className="text-xs text-gray-500 truncate max-w-[180px]">{pos.name}</div>
                        </div>
                      </div>
                      <div className="grid grid-cols-2 gap-3 text-sm">
                        <div>
                          <div className="text-xs text-gray-500">Kaufkurs</div>
                          <div className="text-white">{formatPrice(pos.avg_price)}</div>
                        </div>
                        <div>
                          <div className="text-xs text-gray-500">Wert</div>
                          <div className="text-white">{formatPrice((pos.current_price || 0) * (pos.quantity || 1))}</div>
                        </div>
                        <div>
                          <div className="text-xs text-gray-500">Rendite</div>
                          <div className={`font-medium ${pos.total_return_pct >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                            {formatPercent(pos.total_return_pct)}
                          </div>
                        </div>
                        <div>
                          <div className="text-xs text-gray-500">Gewinn</div>
                          <div className={`font-medium ${gain >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                            {gain >= 0 ? '+' : ''}{formatPrice(gain)}
                          </div>
                        </div>
                      </div>
                      {pos.buy_date && (
                        <div className="mt-2 pt-2 border-t border-dark-600 text-xs text-gray-500">
                          Gekauft: {formatDate(pos.buy_date)}
                        </div>
                      )}
                    </div>
                  )
                })}
              </div>

              {/* Desktop Table View */}
              <div className="hidden md:block overflow-x-auto">
                <table className="w-full">
                  <thead>
                    <tr className="text-left text-xs text-gray-500 border-b border-dark-600">
                      <th className="pt-4 pb-3 px-4">Symbol</th>
                      <th className="pt-4 pb-3 px-4">Kaufkurs</th>
                      <th className="pt-4 pb-3 px-4">Anzahl</th>
                      <th className="pt-4 pb-3 px-4">Wert</th>
                      <th className="pt-4 pb-3 px-4">Rendite</th>
                      <th className="pt-4 pb-3 px-4">Kaufdatum</th>
                    </tr>
                  </thead>
                  <tbody>
                    {portfolio?.positions?.slice().sort((a, b) => (b.is_live ? 1 : 0) - (a.is_live ? 1 : 0)).map((pos) => {
                      const totalValue = (pos.current_price || 0) * (pos.quantity || 1)
                      const totalCost = (pos.avg_price || 0) * (pos.quantity || 1)
                      const gain = totalValue - totalCost
                      return (
                        <tr key={pos.id} className={`border-b border-dark-700/50 last:border-0 ${pos.is_live ? 'bg-green-500/5' : ''}`}>
                          <td className="py-3 px-4">
                            <div className="flex items-center gap-2">
                              <div>
                                <div className="font-medium text-white flex items-center gap-2">
                                  {pos.symbol}
                                  {pos.is_live && (
                                    <span className="px-2 py-0.5 bg-green-500 text-white text-[10px] font-bold rounded flex items-center gap-1">
                                      <span className="w-1.5 h-1.5 bg-white rounded-full animate-pulse"></span>
                                      LIVE
                                    </span>
                                  )}
                                </div>
                                <div className="text-xs text-gray-500 truncate max-w-[150px]">{pos.name}</div>
                              </div>
                            </div>
                          </td>
                          <td className="py-3 px-4">
                            <div className="text-white">{formatPrice(pos.avg_price)}</div>
                          </td>
                          <td className="py-3 px-4">
                            <div className="text-gray-400">{(pos.quantity || 1).toFixed(4)}</div>
                          </td>
                          <td className="py-3 px-4">
                            <div className="text-white">{formatPrice(totalValue)}</div>
                            <div className="text-xs text-gray-500">
                              @ {formatPrice(pos.current_price)}
                            </div>
                          </td>
                          <td className="py-3 px-4">
                            <div className={`font-medium ${pos.total_return_pct >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                              {formatPercent(pos.total_return_pct)}
                            </div>
                            <div className={`text-xs ${gain >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                              {gain >= 0 ? '+' : ''}{formatPrice(gain)}
                            </div>
                          </td>
                          <td className="py-3 px-4">
                            <div className="text-gray-400 text-sm">{formatDate(pos.buy_date)}</div>
                          </td>
                        </tr>
                      )
                    })}
                  </tbody>
                </table>
              </div>
            </>
          )}
        </div>

        {/* Trade History (Expandable) */}
        <div className="bg-dark-800 rounded-xl border border-dark-600 overflow-hidden mb-6">
          <button
            onClick={() => setShowTradeHistory(!showTradeHistory)}
            className="w-full p-4 flex items-center justify-between hover:bg-dark-700/50 transition-colors"
          >
            <h2 className="text-lg font-semibold text-white">Trade History ({completedTrades.length})</h2>
            <svg className={`w-5 h-5 text-gray-400 transition-transform ${showTradeHistory ? 'rotate-180' : ''}`} fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 9l-7 7-7-7" />
            </svg>
          </button>

          {showTradeHistory && (
            <div className="border-t border-dark-600">
              {loadingCompletedTrades ? (
                <div className="p-8 text-center">
                  <div className="w-8 h-8 border-2 border-accent-500 border-t-transparent rounded-full animate-spin mx-auto"></div>
                </div>
              ) : completedTrades.length === 0 ? (
                <div className="p-8 text-center text-gray-500">
                  Noch keine abgeschlossenen Trades
                </div>
              ) : (
                <div className="overflow-x-auto">
                  <table className="w-full text-sm">
                    <thead>
                      <tr className="text-left text-xs text-gray-500 border-b border-dark-600">
                        <th className="pt-4 pb-3 px-4">Symbol</th>
                        <th className="pt-4 pb-3 px-4">Kauf</th>
                        <th className="pt-4 pb-3 px-4">Verkauf</th>
                        <th className="pt-4 pb-3 px-4 text-right">Rendite</th>
                      </tr>
                    </thead>
                    <tbody>
                      {completedTrades.map((trade) => (
                        <tr key={trade.id} className={`border-b border-dark-700/50 last:border-0 ${trade.is_live ? 'bg-green-500/5' : ''}`}>
                          <td className="py-3 px-4">
                            <div className="flex items-center gap-2">
                              <div className="font-medium text-white">{trade.symbol}</div>
                              {trade.is_live && (
                                <span className="px-1.5 py-0.5 bg-green-500 text-white text-[10px] font-bold rounded">LIVE</span>
                              )}
                            </div>
                          </td>
                          <td className="py-3 px-4">
                            <div className="text-gray-300">{formatPrice(trade.buy_price)}</div>
                            <div className="text-xs text-gray-500">{formatDate(trade.buy_date)}</div>
                          </td>
                          <td className="py-3 px-4">
                            <div className="text-gray-300">{formatPrice(trade.sell_price)}</div>
                            <div className="text-xs text-gray-500">{formatDate(trade.sell_date)}</div>
                          </td>
                          <td className={`py-3 px-4 text-right font-medium ${trade.profit_loss_pct >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                            <div>{formatPercent(trade.profit_loss_pct)}</div>
                            <div className="text-xs">
                              {trade.profit_loss >= 0 ? '+' : ''}{formatPrice(trade.profit_loss)}
                            </div>
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              )}
            </div>
          )}
        </div>

        {/* Letzte Aktionen */}
        {actions.length > 0 && (
          <div className="bg-dark-800 rounded-xl border border-dark-600 overflow-hidden mb-6">
            <div className="p-4 border-b border-dark-600">
              <h2 className="text-lg font-semibold text-white">Letzte Aktionen ({actions.length})</h2>
            </div>
            <div className="divide-y divide-dark-700 max-h-[300px] overflow-auto">
              {actions.slice().sort((a, b) => new Date(b.signal_date) - new Date(a.signal_date)).map((action) => (
                <div key={action.id} className={`p-4 hover:bg-dark-700/50 transition-colors ${action.is_live ? 'border-l-4 border-green-500 bg-green-500/5' : ''}`}>
                  <div className="flex items-center justify-between mb-2">
                    <div className="flex items-center gap-2">
                      <span className={`px-2 py-1 rounded text-xs font-bold ${
                        action.action === 'BUY'
                          ? 'bg-green-500/20 text-green-400'
                          : 'bg-red-500/20 text-red-400'
                      }`}>
                        {action.action}
                      </span>
                      <span className="font-semibold text-white">{action.symbol}</span>
                      {action.is_live && (
                        <span className="px-2 py-0.5 bg-green-500 text-white text-[10px] font-bold rounded flex items-center gap-1">
                          <span className="w-1.5 h-1.5 bg-white rounded-full animate-pulse"></span>
                          LIVE
                        </span>
                      )}
                    </div>
                    <span className="text-gray-400 text-sm">{formatDate(action.signal_date)}</span>
                  </div>
                  <div className="flex items-center justify-between text-sm">
                    <div className="text-gray-400">
                      {action.quantity?.toFixed(4)}x @ {formatPrice(action.price)}
                    </div>
                    {action.action === 'SELL' && action.profit_loss_pct !== null && (
                      <span className={`font-medium ${action.profit_loss_pct >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                        {formatPercent(action.profit_loss_pct)} ({formatPrice(action.profit_loss)})
                      </span>
                    )}
                  </div>
                </div>
              ))}
            </div>
          </div>
        )}

        {/* Info Box */}
        <div className="bg-purple-500/10 border border-purple-500/30 rounded-xl p-4 mb-6">
          <div className="flex items-start gap-3">
            <svg className="w-5 h-5 text-purple-400 mt-0.5 shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
            </svg>
            <div>
              <h3 className="font-medium text-purple-400">So funktioniert der FlipperBot</h3>
              <ul className="text-sm text-gray-400 mt-2 space-y-1">
                <li>Folgt den BUY/SELL Signalen aus dem Aktien Tracker</li>
                <li>Kauft 1 Aktie bei BUY-Signal, verkauft bei SELL-Signal</li>
                <li>Startdatum: 01.01.2026 (keine Trades davor)</li>
                <li>Sichtbar im Portfolio-Vergleich als "FlipperBot"</li>
                <li>Signale werden automatisch verarbeitet</li>
              </ul>
            </div>
          </div>
        </div>

        {/* Admin-Only Section: Debug Log */}
        {isAdmin && (
          <div className="bg-dark-800 rounded-xl border border-dark-600 overflow-hidden">
            <div className="p-4 border-b border-dark-600 flex items-center gap-2">
              <svg className="w-4 h-4 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
              </svg>
              <h3 className="text-sm font-medium text-white">Debug Log</h3>
              {logs.length > 0 && (
                <span className="text-xs text-gray-500">({logs.length})</span>
              )}
            </div>
            <div className="bg-dark-900 p-4 max-h-[300px] overflow-auto font-mono text-xs">
              {logs.length === 0 ? (
                <div className="text-gray-500 text-center py-4">
                  Keine Logs vorhanden
                </div>
              ) : (
                <div className="space-y-1">
                  {logs.map((log, idx) => (
                    <div key={idx} className="flex items-start gap-2">
                      <span className="text-gray-600 shrink-0">[{log.time}]</span>
                      <span className={`px-1.5 py-0.5 rounded text-[10px] font-bold shrink-0 ${getLogBadge(log.level)}`}>
                        {log.level}
                      </span>
                      <span className={getLogColor(log.level)}>{log.message}</span>
                    </div>
                  ))}
                  <div ref={logEndRef} />
                </div>
              )}
            </div>
            {logs.length > 0 && (
              <div className="p-2 border-t border-dark-700 flex justify-end">
                <button
                  onClick={() => setLogs([])}
                  className="text-xs text-gray-500 hover:text-gray-400 px-2 py-1"
                >
                  Log leeren
                </button>
              </div>
            )}
          </div>
        )}
      </div>
    </div>
  )
}

export default FlipperBotLab
