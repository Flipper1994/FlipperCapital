import { useState, useEffect, useRef } from 'react'
import { Link } from 'react-router-dom'
import { useCurrency } from '../context/CurrencyContext'

function LutzLab({ isAdmin = false, isLoggedIn = false, token = '' }) {
  const [portfolio, setPortfolio] = useState(null)
  const [actions, setActions] = useState([])
  const [performance, setPerformance] = useState(null)
  const [loading, setLoading] = useState(true)
  const [updating, setUpdating] = useState(false)
  const [resetting, setResetting] = useState(false)
  const [lastUpdate, setLastUpdate] = useState(null)
  const [updateResult, setUpdateResult] = useState(null)
  const [logs, setLogs] = useState([])
  const [showLogs, setShowLogs] = useState(true)
  const [hasUserPortfolio, setHasUserPortfolio] = useState(false)
  const [checkingAccess, setCheckingAccess] = useState(true)
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
        fetch('/api/lutz/portfolio', {
          headers: { 'Authorization': `Bearer ${token}` }
        }),
        fetch('/api/lutz/actions', {
          headers: { 'Authorization': `Bearer ${token}` }
        }),
        fetch('/api/lutz/performance', {
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
      console.error('Failed to fetch Lutz data:', err)
    } finally {
      setLoading(false)
    }
  }

  const handleUpdate = async () => {
    setUpdating(true)
    setUpdateResult(null)
    setLogs([{ level: 'INFO', message: 'Update gestartet...', time: new Date().toLocaleTimeString('de-DE') }])

    try {
      const res = await fetch('/api/lutz/update', {
        headers: { 'Authorization': `Bearer ${token}` }
      })
      const data = await res.json()
      setUpdateResult(data)
      setLastUpdate(new Date())

      // Add logs from response
      if (data.logs) {
        setLogs(data.logs)
      }

      await fetchData()
    } catch (err) {
      console.error('Failed to update Lutz:', err)
      setUpdateResult({ error: 'Update failed' })
      setLogs(prev => [...prev, { level: 'ERROR', message: 'Update fehlgeschlagen: ' + err.message, time: new Date().toLocaleTimeString('de-DE') }])
    } finally {
      setUpdating(false)
    }
  }

  const handleReset = async () => {
    if (!confirm('Lutz komplett zurücksetzen? Alle Trades und Positionen werden gelöscht!')) {
      return
    }
    setResetting(true)
    setLogs([{ level: 'WARN', message: 'Lutz wird zurückgesetzt...', time: new Date().toLocaleTimeString('de-DE') }])

    try {
      await fetch('/api/lutz/reset', {
        method: 'POST',
        headers: { 'Authorization': `Bearer ${token}` }
      })
      setLogs(prev => [...prev, { level: 'INFO', message: 'Reset abgeschlossen', time: new Date().toLocaleTimeString('de-DE') }])
      setUpdateResult(null)
      await fetchData()
    } catch (err) {
      console.error('Failed to reset Lutz:', err)
      setLogs(prev => [...prev, { level: 'ERROR', message: 'Reset fehlgeschlagen', time: new Date().toLocaleTimeString('de-DE') }])
    } finally {
      setResetting(false)
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

  const formatDateTime = (dateStr) => {
    if (!dateStr) return '-'
    const date = new Date(dateStr)
    return date.toLocaleDateString('de-DE', {
      day: '2-digit',
      month: '2-digit',
      year: 'numeric',
      hour: '2-digit',
      minute: '2-digit'
    })
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
          <p className="text-gray-400">Lade Lutz Daten...</p>
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
          <div className="w-24 h-24 mx-auto mb-6 bg-gradient-to-br from-orange-500/20 to-red-500/20 rounded-2xl flex items-center justify-center border border-orange-500/30">
            <svg className="w-12 h-12 text-orange-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z" />
            </svg>
          </div>

          {/* Title */}
          <h1 className="text-2xl font-bold text-white mb-2 flex items-center justify-center gap-2">
            Lutz Lab
            <span className="px-2 py-0.5 bg-gradient-to-r from-orange-500 to-red-500 text-white text-xs font-bold rounded">
              BETA
            </span>
          </h1>

          {/* Message based on state */}
          {!isLoggedIn ? (
            <>
              <p className="text-gray-400 mb-6">
                Melde dich an, um den Lutz Lab zu nutzen und die automatisierte Trading-Strategie zu verfolgen.
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
                Um den Lutz Lab zu nutzen, musst du mindestens eine Aktie in deinem Portfolio haben.
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
              <div className="w-12 h-12 bg-gradient-to-br from-orange-500 to-red-600 rounded-xl flex items-center justify-center">
                <svg className="w-7 h-7 text-white" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M19.428 15.428a2 2 0 00-1.022-.547l-2.387-.477a6 6 0 00-3.86.517l-.318.158a6 6 0 01-3.86.517L6.05 15.21a2 2 0 00-1.806.547M8 4h8l-1 1v5.172a2 2 0 00.586 1.414l5 5c1.26 1.26.367 3.414-1.415 3.414H4.828c-1.782 0-2.674-2.154-1.414-3.414l5-5A2 2 0 009 10.172V5L8 4z" />
                </svg>
              </div>
              <div>
                <h1 className="text-2xl font-bold text-white flex items-center gap-2">
                  Lutz Lab
                  <span className="px-2 py-0.5 bg-gradient-to-r from-orange-500 to-red-500 text-white text-xs font-bold rounded">
                    BETA
                  </span>
                </h1>
                <p className="text-gray-500 text-sm">Aggressives Trading seit 01.01.2026</p>
              </div>
            </div>
          </div>
          <div className="flex items-center gap-3">
            {isAdmin ? (
              <>
                <button
                  onClick={handleUpdate}
                  disabled={updating}
                  className="flex items-center gap-2 px-4 py-2.5 bg-accent-500 text-white rounded-lg hover:bg-accent-400 transition-colors disabled:opacity-50 font-medium"
                >
                  {updating ? (
                    <>
                      <div className="w-4 h-4 border-2 border-white border-t-transparent rounded-full animate-spin"></div>
                      Aktualisiere...
                    </>
                  ) : (
                    <>
                      <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
                      </svg>
                      Update & Trade
                    </>
                  )}
                </button>
                <button
                  onClick={handleReset}
                  disabled={resetting}
                  className="flex items-center gap-2 px-4 py-2.5 bg-red-500/20 text-red-400 rounded-lg hover:bg-red-500/30 transition-colors disabled:opacity-50 font-medium"
                >
                  <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
                  </svg>
                  Reset
                </button>
              </>
            ) : (
              <div className="flex items-center gap-3">
                <div
                  className="flex items-center gap-2 px-4 py-2.5 bg-gray-600/30 text-gray-500 rounded-lg cursor-not-allowed font-medium"
                  title="Nur für Administratoren"
                >
                  <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z" />
                  </svg>
                  Update & Trade
                </div>
                <div
                  className="flex items-center gap-2 px-4 py-2.5 bg-gray-600/30 text-gray-500 rounded-lg cursor-not-allowed font-medium"
                  title="Nur für Administratoren"
                >
                  <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z" />
                  </svg>
                  Reset
                </div>
              </div>
            )}
          </div>
        </div>

        {/* Non-Admin Info Banner */}
        {!isAdmin && (
          <div className="mb-6 p-4 rounded-xl border bg-gray-500/10 border-gray-500/30 text-gray-400">
            <div className="flex items-center gap-3">
              <svg className="w-5 h-5 shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
              </svg>
              <div>
                <span className="font-medium text-gray-300">Nur-Lesen Modus</span>
                <span className="mx-2">–</span>
                <span>Du kannst die Performance des Lutz einsehen. Trading-Aktionen und Debug-Logs sind Administratoren vorbehalten.</span>
              </div>
            </div>
          </div>
        )}

        {/* Update Result Banner */}
        {updateResult && !updateResult.error && updateResult.action_count > 0 && (
          <div className="mb-6 p-4 rounded-xl border bg-green-500/10 border-green-500/30 text-green-400">
            <div className="flex items-center gap-3">
              <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
              </svg>
              <div>
                <span className="font-medium">
                  {updateResult.action_count} neue Aktion{updateResult.action_count !== 1 ? 'en' : ''} ausgeführt
                </span>
              </div>
            </div>
            {updateResult.actions && updateResult.actions.length > 0 && (
              <div className="mt-3 space-y-1">
                {updateResult.actions.map((action, idx) => (
                  <div key={idx} className="text-sm flex items-center gap-2">
                    <span className={`px-2 py-0.5 rounded text-xs font-medium ${
                      action.action === 'BUY' ? 'bg-green-500/20 text-green-400' : 'bg-red-500/20 text-red-400'
                    }`}>
                      {action.action}
                    </span>
                    <span className="font-medium">{action.symbol}</span>
                    <span className="text-gray-400">@ {formatPrice(action.price)}</span>
                    <span className="text-gray-500 text-xs">({action.date})</span>
                    {action.profit_loss_pct !== undefined && (
                      <span className={action.profit_loss_pct >= 0 ? 'text-green-400' : 'text-red-400'}>
                        {formatPercent(action.profit_loss_pct)}
                      </span>
                    )}
                  </div>
                ))}
              </div>
            )}
          </div>
        )}

        {/* Performance Stats - Prominent Rendite Display */}
        {performance && (
          <>
            {/* Main Rendite Card */}
            <div className="mb-4 bg-gradient-to-r from-orange-500/20 to-red-500/20 rounded-xl border border-orange-500/30 p-5">
              <div className="flex flex-col md:flex-row md:items-center justify-between gap-4">
                <div>
                  <div className="text-sm text-gray-400 mb-1">Portfolio Rendite (Simulation)</div>
                  <div className={`text-4xl md:text-5xl font-bold ${
                    performance.overall_return_pct >= 0 ? 'text-green-400' : 'text-red-400'
                  }`}>
                    {formatPercent(performance.overall_return_pct)}
                  </div>
                  <div className="text-sm text-gray-400 mt-2">
                    Gesamt: {formatPrice(performance.total_gain)} Gewinn
                  </div>
                </div>
                <div className="flex gap-6">
                  <div className="text-center">
                    <div className="text-xs text-gray-500 mb-1">Investiert</div>
                    <div className="text-xl font-bold text-gray-300">
                      {formatPrice(performance.invested_in_positions || 0)}
                    </div>
                  </div>
                  <div className="text-center">
                    <div className="text-xs text-gray-500 mb-1">Aktueller Wert</div>
                    <div className="text-xl font-bold text-white">
                      {formatPrice(performance.current_value || 0)}
                    </div>
                  </div>
                </div>
              </div>
            </div>

            {/* Live Rendite Card */}
            {(() => {
              const livePositions = portfolio?.positions?.filter(p => p.is_live) || []
              if (livePositions.length === 0) return null
              const liveInvested = livePositions.reduce((sum, p) => sum + (p.avg_price * p.quantity), 0)
              const liveValue = livePositions.reduce((sum, p) => sum + (p.current_price * p.quantity), 0)
              const liveGain = liveValue - liveInvested
              const liveReturnPct = liveInvested > 0 ? (liveGain / liveInvested) * 100 : 0
              return (
                <div className="mb-4 bg-gradient-to-r from-green-500/20 to-emerald-500/20 rounded-xl border border-green-500/30 p-5">
                  <div className="flex flex-col md:flex-row md:items-center justify-between gap-4">
                    <div>
                      <div className="text-sm text-green-300 mb-1 flex items-center gap-2">
                        <span className="w-2 h-2 bg-green-500 rounded-full animate-pulse"></span>
                        Live Rendite ({livePositions.length} Position{livePositions.length !== 1 ? 'en' : ''})
                      </div>
                      <div className={`text-3xl md:text-4xl font-bold ${
                        liveReturnPct >= 0 ? 'text-green-400' : 'text-red-400'
                      }`}>
                        {formatPercent(liveReturnPct)}
                      </div>
                      <div className="text-sm text-green-300/70 mt-2">
                        {liveGain >= 0 ? '+' : ''}{formatPrice(liveGain)} Gewinn
                      </div>
                    </div>
                    <div className="flex gap-6">
                      <div className="text-center">
                        <div className="text-xs text-green-400/70 mb-1">Investiert</div>
                        <div className="text-lg font-bold text-green-300">
                          {formatPrice(liveInvested)}
                        </div>
                      </div>
                      <div className="text-center">
                        <div className="text-xs text-green-400/70 mb-1">Aktueller Wert</div>
                        <div className="text-lg font-bold text-white">
                          {formatPrice(liveValue)}
                        </div>
                      </div>
                    </div>
                  </div>
                </div>
              )
            })()}

            {/* Detailed Stats */}
            <div className="grid grid-cols-2 md:grid-cols-4 gap-3 md:gap-4 mb-6">
              <div className="bg-dark-800 rounded-xl border border-dark-600 p-4">
                <div className="text-xs text-gray-500 mb-1">Unrealisiert</div>
                <div className={`text-xl md:text-2xl font-bold ${
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
              <div className="bg-dark-800 rounded-xl border border-dark-600 p-4">
                <div className="text-xs text-gray-500 mb-1">Realisiert</div>
                <div className={`text-xl md:text-2xl font-bold ${
                  performance.realized_profit >= 0 ? 'text-green-400' : 'text-red-400'
                }`}>
                  {formatPrice(performance.realized_profit)}
                </div>
                <div className="text-xs text-gray-500 mt-1">
                  {performance.total_trades || 0} abgeschl. Trades
                </div>
              </div>
              <div className="bg-dark-800 rounded-xl border border-dark-600 p-4">
                <div className="text-xs text-gray-500 mb-1">Win Rate</div>
                <div className="text-xl md:text-2xl font-bold text-white">
                  {performance.win_rate?.toFixed(1) || 0}%
                </div>
                <div className="text-xs text-gray-500 mt-1">
                  {performance.wins || 0}W / {performance.losses || 0}L
                </div>
              </div>
              <div className="bg-dark-800 rounded-xl border border-dark-600 p-4">
                <div className="text-xs text-gray-500 mb-1">Offene Positionen</div>
                <div className="text-xl md:text-2xl font-bold text-white">
                  {performance.open_positions || 0}
                </div>
                <div className="text-xs text-gray-500 mt-1">
                  von {performance.total_buys || 0} Käufen
                </div>
              </div>
            </div>
          </>
        )}

        <div className="grid md:grid-cols-2 gap-6">
          {/* Portfolio */}
          <div className="bg-dark-800 rounded-xl border border-dark-600 overflow-hidden">
            <div className="p-4 border-b border-dark-600">
              <h2 className="text-lg font-semibold text-white">Aktuelle Positionen</h2>
              {portfolio && (
                <div className="flex items-center gap-4 mt-2 text-sm">
                  <span className="text-gray-400">
                    Wert: <span className="text-white font-medium">{formatPrice(portfolio.total_value)}</span>
                  </span>
                  <span className={portfolio.total_return_pct >= 0 ? 'text-green-400' : 'text-red-400'}>
                    {formatPercent(portfolio.total_return_pct)}
                  </span>
                </div>
              )}
            </div>

            {portfolio?.positions?.length === 0 ? (
              <div className="p-8 text-center">
                <div className="w-16 h-16 mx-auto mb-4 bg-dark-700 rounded-full flex items-center justify-center">
                  <svg className="w-8 h-8 text-gray-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M20 13V6a2 2 0 00-2-2H6a2 2 0 00-2 2v7m16 0v5a2 2 0 01-2 2H6a2 2 0 01-2-2v-5m16 0h-2.586a1 1 0 00-.707.293l-2.414 2.414a1 1 0 01-.707.293h-3.172a1 1 0 01-.707-.293l-2.414-2.414A1 1 0 006.586 13H4" />
                  </svg>
                </div>
                <p className="text-gray-500">Keine offenen Positionen</p>
                <p className="text-gray-600 text-sm mt-1">Klicke "Update & Trade" um Signale zu verarbeiten</p>
              </div>
            ) : (
              <div className="divide-y divide-dark-700 max-h-[400px] overflow-auto">
                {portfolio?.positions?.slice().sort((a, b) => (b.is_live ? 1 : 0) - (a.is_live ? 1 : 0)).map((pos) => {
                  const totalCost = (pos.avg_price || 0) * (pos.quantity || 1)
                  const totalValue = (pos.current_price || 0) * (pos.quantity || 1)
                  const gain = totalValue - totalCost
                  return (
                    <div key={pos.id} className={`p-4 hover:bg-dark-700/50 transition-colors ${pos.is_live ? 'border-l-4 border-green-500 bg-green-500/5' : ''}`}>
                      <div className="flex justify-between items-start mb-2">
                        <div>
                          <div className="font-semibold text-white flex items-center gap-2">
                            {pos.symbol}
                            {pos.is_live && (
                              <span className="px-2 py-0.5 bg-green-500 text-white text-[10px] font-bold rounded animate-pulse flex items-center gap-1">
                                <span className="w-1.5 h-1.5 bg-white rounded-full"></span>
                                LIVE
                              </span>
                            )}
                          </div>
                          <div className="text-xs text-gray-500 truncate max-w-[150px]">{pos.name}</div>
                        </div>
                        <div className="text-right">
                          <div className={`text-lg font-bold ${pos.total_return_pct >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                            {formatPercent(pos.total_return_pct)}
                          </div>
                          <div className={`text-xs ${gain >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                            {gain >= 0 ? '+' : ''}{formatPrice(gain)}
                          </div>
                        </div>
                      </div>
                      <div className="grid grid-cols-2 md:grid-cols-4 gap-2 text-xs mt-3 p-2 bg-dark-900/50 rounded-lg">
                        <div>
                          <span className="text-gray-500 block">Anteile</span>
                          <span className="text-white font-medium">{(pos.quantity || 1).toFixed(4)}</span>
                        </div>
                        <div>
                          <span className="text-gray-500 block">Kaufpreis/Stk</span>
                          <span className="text-gray-300">{formatPrice(pos.avg_price)}</span>
                        </div>
                        <div>
                          <span className="text-gray-500 block">Gesamt Kauf</span>
                          <span className="text-gray-300">{formatPrice(totalCost)}</span>
                        </div>
                        <div>
                          <span className="text-gray-500 block">Aktueller Wert</span>
                          <span className="text-white font-medium">{formatPrice(totalValue)}</span>
                        </div>
                      </div>
                      <div className="flex justify-between items-center mt-2 text-xs text-gray-500">
                        <span>Kauf: {formatDate(pos.buy_date)}</span>
                        <span>Kurs: {formatPrice(pos.current_price)} ({formatPercent(pos.change_percent)} heute)</span>
                      </div>
                    </div>
                  )
                })}
              </div>
            )}
          </div>

          {/* Recent Actions */}
          <div className="bg-dark-800 rounded-xl border border-dark-600 overflow-hidden">
            <div className="p-4 border-b border-dark-600">
              <h2 className="text-lg font-semibold text-white">Letzte Aktionen</h2>
              <p className="text-xs text-gray-500 mt-1">Trade-Historie des Lutz</p>
            </div>

            {actions.length === 0 ? (
              <div className="p-8 text-center">
                <div className="w-16 h-16 mx-auto mb-4 bg-dark-700 rounded-full flex items-center justify-center">
                  <svg className="w-8 h-8 text-gray-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2" />
                  </svg>
                </div>
                <p className="text-gray-500">Noch keine Trades ausgeführt</p>
              </div>
            ) : (
              <div className="divide-y divide-dark-700 max-h-[400px] overflow-auto">
                {actions.slice().sort((a, b) => (b.is_live ? 1 : 0) - (a.is_live ? 1 : 0)).map((action) => (
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
                        {action.quantity}x @ {formatPrice(action.price)}
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
            )}
          </div>
        </div>

        {/* Info Box */}
        <div className="mt-6 bg-orange-500/10 border border-orange-500/30 rounded-xl p-4">
          <div className="flex items-start gap-3">
            <svg className="w-5 h-5 text-orange-400 mt-0.5 shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M17.657 18.657A8 8 0 016.343 7.343S7 9 9 10c0-2 .5-5 2.986-7C14 5 16.09 5.777 17.656 7.343A7.975 7.975 0 0120 13a7.975 7.975 0 01-2.343 5.657z" />
            </svg>
            <div>
              <h3 className="font-medium text-orange-400">So funktioniert der Lutz (Aggressiv)</h3>
              <ul className="text-sm text-gray-400 mt-2 space-y-1">
                <li><span className="text-orange-400">Kauft bei erstem hellroten Balken</span> – früherer Einstieg als FlipperBot</li>
                <li><span className="text-orange-400">Verkauft nur bei dunkelrotem Balken</span> – längere Haltezeit für mehr Gewinn</li>
                <li>Aggressivere Strategie mit höherem Risiko und Potenzial</li>
                <li>Startdatum: 01.01.2026 (keine Trades davor)</li>
                <li>Sichtbar im Portfolio-Vergleich als "Lutz"</li>
              </ul>
              <div className="mt-3 p-2 bg-dark-800/50 rounded text-xs text-gray-500">
                <strong className="text-gray-400">Unterschied zu FlipperBot:</strong> FlipperBot wartet auf grüne Balken (konservativ),
                Lutz steigt schon bei hellrot ein (aggressiv) und hält länger.
              </div>
            </div>
          </div>
        </div>

        {/* Log Box */}
        <div className="mt-6 bg-dark-800 rounded-xl border border-dark-600 overflow-hidden">
          <button
            onClick={() => setShowLogs(!showLogs)}
            className="w-full flex items-center justify-between p-4 hover:bg-dark-700/50 transition-colors"
          >
            <div className="flex items-center gap-3">
              <svg className="w-5 h-5 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
              </svg>
              <span className="font-medium text-white">Debug Log</span>
              {logs.length > 0 && (
                <span className="text-xs text-gray-500">({logs.length} Einträge)</span>
              )}
              {!isAdmin && (
                <span className="text-xs text-gray-500 flex items-center gap-1">
                  <svg className="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z" />
                  </svg>
                  Nur für Admins
                </span>
              )}
            </div>
            <svg
              className={`w-5 h-5 text-gray-400 transition-transform ${showLogs ? 'rotate-180' : ''}`}
              fill="none"
              stroke="currentColor"
              viewBox="0 0 24 24"
            >
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 9l-7 7-7-7" />
            </svg>
          </button>

          {showLogs && (
            <div className="border-t border-dark-600">
              <div className="bg-dark-900 p-4 max-h-[300px] overflow-auto font-mono text-xs">
                {!isAdmin ? (
                  <div className="text-gray-500 text-center py-8">
                    <svg className="w-10 h-10 mx-auto mb-3 text-gray-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z" />
                    </svg>
                    <p className="font-medium text-gray-400">Debug Log nur für Administratoren</p>
                    <p className="text-gray-600 mt-1">Diese Funktion ist Admins vorbehalten</p>
                  </div>
                ) : logs.length === 0 ? (
                  <div className="text-gray-500 text-center py-4">
                    Klicke "Update & Trade" um Logs zu sehen
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
              {isAdmin && logs.length > 0 && (
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
    </div>
  )
}

export default LutzLab
