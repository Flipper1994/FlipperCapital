import { useState, useEffect, useMemo } from 'react'
import { Link } from 'react-router-dom'
import { useCurrency } from '../context/CurrencyContext'
import PortfolioChart from './PortfolioChart'
import StockDetailOverlay from './StockDetailOverlay'

function DitzLab({ isAdmin = false, isLoggedIn = false, token = '' }) {
  const [portfolio, setPortfolio] = useState(null)
  const [actions, setActions] = useState([])
  const [performance, setPerformance] = useState(null)
  const [loading, setLoading] = useState(true)
  const [hasUserPortfolio, setHasUserPortfolio] = useState(false)
  const [checkingAccess, setCheckingAccess] = useState(true)
  const [showTradeHistory, setShowTradeHistory] = useState(false)
  const [completedTrades, setCompletedTrades] = useState([])
  const [loadingCompletedTrades, setLoadingCompletedTrades] = useState(false)
  const [isLive, setIsLive] = useState(true)
  const [hasLivePositions, setHasLivePositions] = useState(false)
  const [selectedPosition, setSelectedPosition] = useState(null)
  const { formatPrice } = useCurrency()

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
      fetchCompletedTrades()
    } else {
      setLoading(false)
    }
  }, [isLoggedIn, hasUserPortfolio])

  const fetchData = async () => {
    setLoading(true)
    try {
      const [portfolioRes, actionsRes, perfRes] = await Promise.all([
        fetch('/api/ditz/portfolio', {
          headers: { 'Authorization': `Bearer ${token}` }
        }),
        fetch('/api/ditz/actions', {
          headers: { 'Authorization': `Bearer ${token}` }
        }),
        fetch('/api/ditz/performance', {
          headers: { 'Authorization': `Bearer ${token}` }
        })
      ])

      const portfolioData = await portfolioRes.json()
      const actionsData = await actionsRes.json()
      const perfData = await perfRes.json()

      setPortfolio(portfolioData)
      setActions(actionsData)
      setPerformance(perfData)

      const liveCount = portfolioData?.positions?.filter(p => p.is_live)?.length || 0
      setHasLivePositions(liveCount > 0)
      if (liveCount === 0) setIsLive(false)
    } catch (err) {
      console.error('Failed to fetch Ditz data:', err)
    } finally {
      setLoading(false)
    }
  }

  const fetchCompletedTrades = async () => {
    setLoadingCompletedTrades(true)
    try {
      const res = await fetch('/api/ditz/completed-trades', {
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

  const calcLivePerformance = (positions, trades) => {
    const invested = positions.reduce((sum, p) => sum + (p.avg_price || 0) * (p.quantity || 1), 0)
    const currentVal = positions.reduce((sum, p) => sum + (p.current_price || 0) * (p.quantity || 1), 0)
    const unrealized = currentVal - invested
    const unrealizedPct = invested > 0 ? (unrealized / invested) * 100 : 0
    const realized = trades.reduce((sum, t) => sum + (t.profit_loss || 0), 0)
    const allItems = [
      ...trades.map(t => ({ pct: t.profit_loss_pct || 0 })),
      ...positions.map(p => ({ pct: p.total_return_pct || 0 }))
    ]
    const allWins = allItems.filter(i => i.pct > 0)
    const allLosses = allItems.filter(i => i.pct < 0)
    const wins = allWins.length
    const losses = allLosses.length
    const winRate = allItems.length > 0 ? (wins / allItems.length) * 100 : 0
    const avgReturn = allItems.length > 0 ? allItems.reduce((sum, i) => sum + i.pct, 0) / allItems.length : 0
    const avgWinPct = allWins.length > 0 ? allWins.reduce((s, i) => s + i.pct, 0) / allWins.length : 0
    const avgLossPct = allLosses.length > 0 ? Math.abs(allLosses.reduce((s, i) => s + i.pct, 0) / allLosses.length) : 0
    const riskReward = avgLossPct > 0 ? avgWinPct / avgLossPct : avgWinPct > 0 ? Infinity : 0
    const totalGain = unrealized + realized
    const tradeCosts = trades.reduce((sum, t) => sum + (t.buy_price || 0) * (t.quantity || 1), 0)
    const totalInvested = invested + tradeCosts
    const overallPct = totalInvested > 0 ? (totalGain / totalInvested) * 100 : 0
    return {
      open_positions: positions.length, invested_in_positions: invested, current_value: currentVal,
      unrealized_gain: unrealized, total_return_pct: unrealizedPct, realized_profit: realized,
      total_trades: allItems.length, wins, losses, win_rate: winRate, avg_return_per_trade: avgReturn,
      avg_win_pct: avgWinPct, avg_loss_pct: avgLossPct, risk_reward: riskReward,
      total_gain: totalGain, overall_return_pct: overallPct, total_buys: positions.length + trades.length
    }
  }

  const displayedPositions = useMemo(() => {
    if (!portfolio?.positions) return []
    return isLive ? portfolio.positions.filter(p => p.is_live) : portfolio.positions
  }, [portfolio, isLive])

  const displayedActions = useMemo(() => {
    if (!actions?.length) return []
    return isLive ? actions.filter(a => a.is_live) : actions
  }, [actions, isLive])

  const displayedCompletedTrades = useMemo(() => {
    if (!completedTrades?.length) return []
    return isLive ? completedTrades.filter(t => t.is_live) : completedTrades
  }, [completedTrades, isLive])

  const displayedPerformance = useMemo(() => {
    if (!portfolio?.positions) return null
    const positions = isLive ? portfolio.positions.filter(p => p.is_live) : portfolio.positions
    const trades = isLive ? (completedTrades?.filter(t => t.is_live) || []) : (completedTrades || [])
    return calcLivePerformance(positions, trades)
  }, [portfolio, completedTrades, isLive])

  if (checkingAccess || loading) {
    return (
      <div className="flex-1 p-4 md:p-6 flex items-center justify-center">
        <div className="text-center">
          <div className="w-12 h-12 border-2 border-cyan-500 border-t-transparent rounded-full animate-spin mx-auto mb-4"></div>
          <p className="text-gray-400">Lade Ditz Daten...</p>
        </div>
      </div>
    )
  }

  if (!isLoggedIn || !hasUserPortfolio) {
    return (
      <div className="flex-1 p-4 md:p-6 flex items-center justify-center">
        <div className="max-w-md text-center">
          <div className="w-24 h-24 mx-auto mb-6 bg-gradient-to-br from-cyan-500/20 to-teal-500/20 rounded-2xl flex items-center justify-center border border-cyan-500/30">
            <svg className="w-12 h-12 text-cyan-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z" />
            </svg>
          </div>
          <h1 className="text-2xl font-bold text-white mb-2 flex items-center justify-center gap-2">
            Ditz Lab
            <span className="px-2 py-0.5 bg-gradient-to-r from-cyan-500 to-teal-500 text-white text-xs font-bold rounded">BETA</span>
          </h1>
          {!isLoggedIn ? (
            <>
              <p className="text-gray-400 mb-6">Melde dich an, um den Ditz Lab zu nutzen und die automatisierte Trading-Strategie zu verfolgen.</p>
              <div className="flex flex-col sm:flex-row gap-3 justify-center">
                <Link to="/login" className="inline-flex items-center justify-center gap-2 px-6 py-3 bg-accent-500 text-white rounded-lg hover:bg-accent-400 transition-colors font-medium">
                  <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M11 16l-4-4m0 0l4-4m-4 4h14m-5 4v1a3 3 0 01-3 3H6a3 3 0 01-3-3V7a3 3 0 013-3h7a3 3 0 013 3v1" /></svg>
                  Anmelden
                </Link>
                <Link to="/register" className="inline-flex items-center justify-center gap-2 px-6 py-3 bg-dark-700 text-white rounded-lg hover:bg-dark-600 transition-colors font-medium border border-dark-600">
                  <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M18 9v3m0 0v3m0-3h3m-3 0h-3m-2-5a4 4 0 11-8 0 4 4 0 018 0zM3 20a6 6 0 0112 0v1H3v-1z" /></svg>
                  Registrieren
                </Link>
              </div>
            </>
          ) : (
            <>
              <p className="text-gray-400 mb-6">Um den Ditz Lab zu nutzen, musst du mindestens eine Aktie in deinem Portfolio haben.</p>
              <Link to="/portfolio" className="inline-flex items-center justify-center gap-2 px-6 py-3 bg-accent-500 text-white rounded-lg hover:bg-accent-400 transition-colors font-medium">
                <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 6v6m0 0v6m0-6h6m-6 0H6" /></svg>
                Portfolio aufbauen
              </Link>
            </>
          )}
          <div className="mt-10 p-4 bg-dark-800 rounded-xl border border-dark-600 text-left">
            <h3 className="text-sm font-medium text-gray-300 mb-3">Was dich erwartet:</h3>
            <ul className="space-y-2 text-sm text-gray-500">
              <li className="flex items-center gap-2">
                <svg className="w-4 h-4 text-green-400" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 13l4 4L19 7" /></svg>
                Automatisiertes Trading nach B-Xtrender Signalen
              </li>
              <li className="flex items-center gap-2">
                <svg className="w-4 h-4 text-green-400" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 13l4 4L19 7" /></svg>
                Performance-Tracking mit Rendite-Anzeige
              </li>
              <li className="flex items-center gap-2">
                <svg className="w-4 h-4 text-green-400" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 13l4 4L19 7" /></svg>
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
              <div className="w-12 h-12 bg-gradient-to-br from-cyan-500 to-teal-600 rounded-xl flex items-center justify-center">
                <svg className="w-7 h-7 text-white" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M9 19v-6a2 2 0 00-2-2H5a2 2 0 00-2 2v6a2 2 0 002 2h2a2 2 0 002-2zm0 0V9a2 2 0 012-2h2a2 2 0 012 2v10m-6 0a2 2 0 002 2h2a2 2 0 002-2m0 0V5a2 2 0 012-2h2a2 2 0 012 2v14a2 2 0 01-2 2h-2a2 2 0 01-2-2z" />
                </svg>
              </div>
              <div>
                <h1 className="text-2xl font-bold text-white flex items-center gap-2">
                  Ditz Lab
                  <span className="px-2 py-0.5 bg-gradient-to-r from-cyan-500 to-teal-500 text-white text-xs font-bold rounded">BETA</span>
                </h1>
                <p className="text-gray-500 text-sm">Signal-Line Trading seit 01.01.2026</p>
              </div>
            </div>
          </div>
        </div>

        {/* Live/Simulation Toggle */}
        <div className="mb-6">
          <div className="inline-flex items-center bg-dark-800 rounded-xl border border-dark-600 p-1">
            <button
              onClick={() => hasLivePositions && setIsLive(true)}
              disabled={!hasLivePositions}
              className={`flex items-center gap-2 px-4 py-2 rounded-lg text-sm font-medium transition-all ${
                !hasLivePositions
                  ? 'opacity-50 cursor-not-allowed text-gray-600'
                  : isLive
                    ? 'bg-green-500/20 text-green-400 border border-green-500/30'
                    : 'text-gray-400 hover:text-white'
              }`}
              title={!hasLivePositions ? 'Keine Live-Positionen vorhanden' : ''}
            >
              <span className={`w-2 h-2 rounded-full ${isLive && hasLivePositions ? 'bg-green-400 animate-pulse' : 'bg-gray-600'}`}></span>
              Live
            </button>
            <button
              onClick={() => setIsLive(false)}
              className={`flex items-center gap-2 px-4 py-2 rounded-lg text-sm font-medium transition-all ${
                !isLive
                  ? 'bg-cyan-500/20 text-cyan-400 border border-cyan-500/30'
                  : 'text-gray-400 hover:text-white'
              }`}
            >
              <svg className={`w-4 h-4 ${!isLive ? 'text-cyan-400' : 'text-gray-600'}`} fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9.75 17L9 20l-1 1h8l-1-1-.75-3M3 13h18M5 17h14a2 2 0 002-2V5a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z" />
              </svg>
              Simulation
            </button>
          </div>
          <p className="text-xs text-gray-500 mt-2">
            {!hasLivePositions
              ? 'Keine Live-Positionen vorhanden'
              : isLive
                ? 'Echte Trades mit realem Kapital'
                : 'Simulierte Trades auf Basis historischer Signale'}
          </p>
        </div>

        {/* Performance Chart */}
        <div className="mb-6">
          <PortfolioChart
            token={token}
            botType="ditz"
            height={250}
            title="Ditz Performance"
          />
        </div>

        {/* Performance Uebersicht */}
        {displayedPerformance && (
          <div className="bg-dark-800 rounded-xl border border-dark-600 p-4 md:p-6 mb-6">
            <h2 className="text-lg font-semibold text-white mb-4">Performance Uebersicht</h2>
            <div className="grid grid-cols-2 md:grid-cols-4 gap-3 md:gap-4 mb-4">
              <div className="bg-gradient-to-r from-cyan-500/20 to-teal-500/20 rounded-lg p-3 md:p-4 border border-cyan-500/30">
                <div className="text-xs text-gray-400 mb-1">Gesamt Rendite</div>
                <div className={`text-xl md:text-2xl font-bold ${displayedPerformance.overall_return_pct >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                  {formatPercent(displayedPerformance.overall_return_pct)}
                </div>
                <div className={`text-xs mt-1 ${displayedPerformance.total_gain >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                  {formatPrice(displayedPerformance.total_gain)} Gewinn
                </div>
              </div>
              <div className="bg-dark-700 rounded-lg p-3 md:p-4">
                <div className="text-xs text-gray-500 mb-1">Investiert</div>
                <div className="text-lg md:text-xl font-bold text-white">{formatPrice(displayedPerformance.invested_in_positions || 0)}</div>
                <div className="text-xs text-gray-500 mt-1">Aktuell: {formatPrice(displayedPerformance.current_value || 0)}</div>
              </div>
              <div className="bg-dark-700 rounded-lg p-3 md:p-4">
                <div className="text-xs text-gray-500 mb-1">Unrealisiert</div>
                <div className={`text-lg md:text-xl font-bold ${displayedPerformance.unrealized_gain >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                  {formatPrice(displayedPerformance.unrealized_gain)}
                </div>
                <div className={`text-xs mt-1 ${displayedPerformance.total_return_pct >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                  {formatPercent(displayedPerformance.total_return_pct)}
                </div>
              </div>
              <div className="bg-dark-700 rounded-lg p-3 md:p-4">
                <div className="text-xs text-gray-500 mb-1">Realisiert</div>
                <div className={`text-lg md:text-xl font-bold ${displayedPerformance.realized_profit >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                  {formatPrice(displayedPerformance.realized_profit)}
                </div>
                <div className="text-xs text-gray-500 mt-1">{displayedPerformance.total_trades || 0} Trades</div>
              </div>
            </div>
            <div className="grid grid-cols-2 md:grid-cols-5 gap-3 md:gap-4">
              <div className="bg-dark-700/50 rounded-lg p-3">
                <div className="text-xs text-gray-500 mb-1">Win Rate</div>
                <div className={`text-base font-bold ${(displayedPerformance.win_rate || 0) >= 50 ? 'text-green-400' : 'text-red-400'}`}>{displayedPerformance.win_rate?.toFixed(1) || 0}%</div>
                <div className="text-xs text-gray-500 mt-1">{displayedPerformance.wins || 0}W / {displayedPerformance.losses || 0}L</div>
              </div>
              <div className="bg-dark-700/50 rounded-lg p-3">
                <div className="text-xs text-gray-500 mb-1">Risiko-Rendite</div>
                <div className={`text-base font-bold ${(displayedPerformance.risk_reward || 0) >= 1 ? 'text-green-400' : 'text-red-400'}`}>
                  {displayedPerformance.risk_reward === Infinity ? '∞' : (displayedPerformance.risk_reward || 0).toFixed(2)}
                </div>
                <div className="text-xs text-gray-500 mt-1">Ø Gewinn / Ø Verlust</div>
              </div>
              <div className="bg-dark-700/50 rounded-lg p-3">
                <div className="text-xs text-gray-500 mb-1">Ø Gewinn-Trade</div>
                <div className="text-base font-bold text-green-400">+{(displayedPerformance.avg_win_pct || 0).toFixed(2)}%</div>
                <div className="text-xs text-gray-500 mt-1">{displayedPerformance.wins || 0} Trades</div>
              </div>
              <div className="bg-dark-700/50 rounded-lg p-3">
                <div className="text-xs text-gray-500 mb-1">Ø Verlust-Trade</div>
                <div className="text-base font-bold text-red-400">-{(displayedPerformance.avg_loss_pct || 0).toFixed(2)}%</div>
                <div className="text-xs text-gray-500 mt-1">{displayedPerformance.losses || 0} Trades</div>
              </div>
              <div className="bg-dark-700/50 rounded-lg p-3">
                <div className="text-xs text-gray-500 mb-1">Offene Positionen</div>
                <div className="text-base font-bold text-white">{displayedPerformance.open_positions || 0}</div>
                <div className="text-xs text-gray-500 mt-1">von {displayedPerformance.total_buys || 0} Kaeufen</div>
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
                {!isLive && displayedPositions.filter(p => p.is_live).length > 0 && (
                  <span className="px-2 py-1 bg-green-500/20 text-green-400 text-sm font-medium rounded flex items-center gap-1">
                    <span className="w-1.5 h-1.5 bg-green-400 rounded-full animate-pulse"></span>
                    {displayedPositions.filter(p => p.is_live).length} Live
                  </span>
                )}
                <span className="px-2 py-1 bg-cyan-500/20 text-cyan-400 text-sm font-medium rounded">
                  {displayedPositions.length} offen
                </span>
              </div>
            </div>
          </div>

          {displayedPositions.length === 0 ? (
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
                {displayedPositions.slice().sort((a, b) => (b.is_live ? 1 : 0) - (a.is_live ? 1 : 0) || (b.total_return_pct || 0) - (a.total_return_pct || 0)).map((pos) => {
                  const totalValue = (pos.current_price || 0) * (pos.quantity || 1)
                  const totalCost = (pos.avg_price || 0) * (pos.quantity || 1)
                  const gain = totalValue - totalCost
                  return (
                    <div key={pos.id} className={`bg-dark-700 rounded-lg p-4 ${pos.is_live ? 'border-l-4 border-green-500' : ''}`}>
                      <div className="flex justify-between items-start mb-3">
                        <div className="cursor-pointer" onClick={() => setSelectedPosition({ symbol: pos.symbol, name: pos.name, mode: 'ditz' })}>
                          <div className="font-semibold text-white flex items-center gap-2 hover:text-cyan-400">
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
                        {pos.stop_loss_price > 0 && (
                          <div>
                            <div className="text-xs text-gray-500">SL</div>
                            <div className={`font-medium text-sm ${
                              pos.current_price > 0 && ((pos.current_price - pos.stop_loss_price) / pos.current_price * 100) > 10
                                ? 'text-green-400' : ((pos.current_price - pos.stop_loss_price) / pos.current_price * 100) > 5
                                  ? 'text-orange-400' : 'text-red-400'
                            }`}>{formatPrice(pos.stop_loss_price)}</div>
                          </div>
                        )}
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
                      <th className="pt-4 pb-3 px-4">SL</th>
                      <th className="pt-4 pb-3 px-4">Kaufdatum</th>
                    </tr>
                  </thead>
                  <tbody>
                    {displayedPositions.slice().sort((a, b) => (b.is_live ? 1 : 0) - (a.is_live ? 1 : 0) || (b.total_return_pct || 0) - (a.total_return_pct || 0)).map((pos) => {
                      const totalValue = (pos.current_price || 0) * (pos.quantity || 1)
                      const totalCost = (pos.avg_price || 0) * (pos.quantity || 1)
                      const gain = totalValue - totalCost
                      return (
                        <tr key={pos.id} className={`border-b border-dark-700/50 last:border-0 ${pos.is_live ? 'bg-green-500/5' : ''}`}>
                          <td className="py-3 px-4 cursor-pointer" onClick={() => setSelectedPosition({ symbol: pos.symbol, name: pos.name, mode: 'ditz' })}>
                            <div className="flex items-center gap-2">
                              <div>
                                <div className="font-medium text-white flex items-center gap-2 hover:text-cyan-400">
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
                          <td className="py-3 px-4"><div className="text-white">{formatPrice(pos.avg_price)}</div></td>
                          <td className="py-3 px-4"><div className="text-gray-400">{(pos.quantity || 1).toFixed(4)}</div></td>
                          <td className="py-3 px-4">
                            <div className="text-white">{formatPrice(totalValue)}</div>
                            <div className="text-xs text-gray-500">@ {formatPrice(pos.current_price)}</div>
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
                            {pos.stop_loss_price > 0 ? (
                              <div>
                                <span className={`font-medium ${
                                  pos.current_price > 0 && ((pos.current_price - pos.stop_loss_price) / pos.current_price * 100) > 10
                                    ? 'text-green-400' : ((pos.current_price - pos.stop_loss_price) / pos.current_price * 100) > 5
                                      ? 'text-orange-400' : 'text-red-400'
                                }`}>{formatPrice(pos.stop_loss_price)}</span>
                                <div className="text-[10px] text-gray-500">{pos.stop_loss_percent ? `${pos.stop_loss_percent}%` : 'default'} {pos.stop_loss_type || 'trailing'}</div>
                              </div>
                            ) : <span className="text-gray-600">-</span>}
                          </td>
                          <td className="py-3 px-4"><div className="text-gray-400 text-sm">{formatDate(pos.buy_date)}</div></td>
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
            <h2 className="text-lg font-semibold text-white">Trade History ({displayedCompletedTrades.length})</h2>
            <svg className={`w-5 h-5 text-gray-400 transition-transform ${showTradeHistory ? 'rotate-180' : ''}`} fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 9l-7 7-7-7" />
            </svg>
          </button>
          {showTradeHistory && (
            <div className="border-t border-dark-600">
              {loadingCompletedTrades ? (
                <div className="p-8 text-center">
                  <div className="w-8 h-8 border-2 border-cyan-500 border-t-transparent rounded-full animate-spin mx-auto"></div>
                </div>
              ) : displayedCompletedTrades.length === 0 ? (
                <div className="p-8 text-center text-gray-500">Noch keine abgeschlossenen Trades</div>
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
                      {displayedCompletedTrades.map((trade) => (
                        <tr key={trade.id} className={`border-b border-dark-700/50 last:border-0 ${trade.is_live ? 'bg-green-500/5' : ''}`}>
                          <td className="py-3 px-4">
                            <div className="flex items-center gap-2">
                              <div className="font-medium text-white">{trade.symbol}</div>
                              {trade.is_live && <span className="px-1.5 py-0.5 bg-green-500 text-white text-[10px] font-bold rounded">LIVE</span>}
                              {trade.is_stop_loss && <span className="px-1.5 py-0.5 bg-red-500/20 text-red-400 text-[10px] rounded font-medium">SL</span>}
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
                            <div className="text-xs">{trade.profit_loss >= 0 ? '+' : ''}{formatPrice(trade.profit_loss)}</div>
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
        {displayedActions.length > 0 && (
          <div className="bg-dark-800 rounded-xl border border-dark-600 overflow-hidden mb-6">
            <div className="p-4 border-b border-dark-600">
              <h2 className="text-lg font-semibold text-white">Letzte Aktionen ({displayedActions.length})</h2>
            </div>
            <div className="divide-y divide-dark-700 max-h-[300px] overflow-auto">
              {displayedActions.slice().sort((a, b) => new Date(b.signal_date) - new Date(a.signal_date)).map((action) => (
                <div key={action.id} className={`p-4 hover:bg-dark-700/50 transition-colors ${action.is_live ? 'border-l-4 border-green-500 bg-green-500/5' : ''}`}>
                  <div className="flex items-center justify-between mb-2">
                    <div className="flex items-center gap-2">
                      <span className={`px-2 py-1 rounded text-xs font-bold ${action.action === 'BUY' ? 'bg-green-500/20 text-green-400' : 'bg-red-500/20 text-red-400'}`}>
                        {action.action}
                      </span>
                      {action.is_stop_loss && <span className="px-1.5 py-0.5 bg-red-500/20 text-red-400 text-[10px] rounded font-medium">SL</span>}
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
                    <div className="text-gray-400">{action.quantity?.toFixed(4)}x @ {formatPrice(action.price)}</div>
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
        <div className="bg-cyan-500/10 border border-cyan-500/30 rounded-xl p-4 mb-6">
          <div className="flex items-start gap-3">
            <svg className="w-5 h-5 text-cyan-400 mt-0.5 shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 19v-6a2 2 0 00-2-2H5a2 2 0 00-2 2v6a2 2 0 002 2h2a2 2 0 002-2zm0 0V9a2 2 0 012-2h2a2 2 0 012 2v10m-6 0a2 2 0 002 2h2a2 2 0 002-2m0 0V5a2 2 0 012-2h2a2 2 0 012 2v14a2 2 0 01-2 2h-2a2 2 0 01-2-2z" />
            </svg>
            <div>
              <h3 className="font-medium text-cyan-400">Strategie: Signal-Line Crossover mit Trendfilter</h3>
              <p className="text-sm text-gray-400 mt-2">
                Handelt auf Basis der BX Trender Signal-Linie (T3 Moving Average). Entry bei bullishem Farbwechsel (Rot auf Gruen), Exit bei baerischem Farbwechsel (Gruen auf Rot). MA-200-Trendfilter als Long-Bias-Konfirmation und dynamischer Trailing Stop Loss (TSL). Die T3-Glaettung filtert kurzfristiges Marktrauschen effektiver als der reine Histogramm-Ansatz.
              </p>
            </div>
          </div>
        </div>
      </div>
      {selectedPosition && (
        <StockDetailOverlay
          symbol={selectedPosition.symbol}
          name={selectedPosition.name}
          mode={selectedPosition.mode}
          onClose={() => setSelectedPosition(null)}
        />
      )}
    </div>
  )
}

export default DitzLab
