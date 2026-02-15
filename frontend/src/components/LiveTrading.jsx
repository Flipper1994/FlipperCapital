import { useState, useEffect, useCallback, useRef } from 'react'
import { useCurrency } from '../context/CurrencyContext'

const STRATEGY_LABELS = {
  regression_scalping: 'Regression Scalping',
  hybrid_ai_trend: 'NW Bollinger Bands',
  diamond_signals: 'Diamond Signals',
}

function LiveTrading({ isAdmin, token }) {
  const [config, setConfig] = useState(null)
  const [status, setStatus] = useState(null)
  const [positions, setPositions] = useState([])
  const [sessions, setSessions] = useState([])
  const [showHistory, setShowHistory] = useState(false)
  const [selectedSessionId, setSelectedSessionId] = useState(null)
  const [selectedSessionPositions, setSelectedSessionPositions] = useState([])
  const [symbolPrices, setSymbolPrices] = useState({})
  const [countdown, setCountdown] = useState(null)
  const [showDebug, setShowDebug] = useState(false)
  const [debugLogs, setDebugLogs] = useState([])
  const [lastLogId, setLastLogId] = useState(0)
  const [notificationsEnabled, setNotificationsEnabled] = useState(false)
  const { formatPrice, currency } = useCurrency()
  const pollRef = useRef(null)
  const posPollRef = useRef(null)
  const debugPollRef = useRef(null)
  const notifyPollRef = useRef(null)
  const lastNotifyLogId = useRef(0)

  const headers = token ? { 'Authorization': `Bearer ${token}` } : {}

  const fetchConfig = useCallback(async () => {
    try {
      const res = await fetch('/api/trading/live/config', { headers })
      if (res.ok) {
        const data = await res.json()
        if (data.symbols) setConfig(data)
      }
    } catch { /* ignore */ }
  }, [token])

  const fetchStatus = useCallback(async () => {
    try {
      const res = await fetch('/api/trading/live/status', { headers })
      if (res.ok) {
        const data = await res.json()
        setStatus(data)
        if (data.symbol_prices) setSymbolPrices(data.symbol_prices)
      }
    } catch { /* ignore */ }
  }, [token])

  const fetchPositions = useCallback(async (sessionId) => {
    if (!sessionId) return
    try {
      const res = await fetch(`/api/trading/live/session/${sessionId}`, { headers })
      if (res.ok) {
        const data = await res.json()
        setPositions(data.positions || [])
        if (data.symbol_prices) setSymbolPrices(data.symbol_prices)
      }
    } catch { /* ignore */ }
  }, [token])

  const fetchSessions = useCallback(async () => {
    try {
      const res = await fetch('/api/trading/live/sessions', { headers })
      if (res.ok) {
        const data = await res.json()
        setSessions(data.sessions || [])
      }
    } catch { /* ignore */ }
  }, [token])

  // Load on mount
  useEffect(() => {
    fetchConfig()
    fetchStatus()
    fetchSessions()
  }, [])

  // Poll status when session is active
  useEffect(() => {
    if (!status?.is_running) {
      if (pollRef.current) clearInterval(pollRef.current)
      return
    }
    pollRef.current = setInterval(fetchStatus, 5000)
    return () => clearInterval(pollRef.current)
  }, [status?.is_running])

  // Poll positions when session is active
  useEffect(() => {
    if (!status?.is_running || !status?.session_id) {
      if (posPollRef.current) clearInterval(posPollRef.current)
      return
    }
    fetchPositions(status.session_id)
    posPollRef.current = setInterval(() => fetchPositions(status.session_id), 10000)
    return () => clearInterval(posPollRef.current)
  }, [status?.is_running, status?.session_id])

  // Countdown timer
  useEffect(() => {
    if (!status?.is_running || !status?.next_poll_at) {
      setCountdown(null)
      return
    }
    const calc = () => {
      const diff = Math.floor((new Date(status.next_poll_at).getTime() - Date.now()) / 1000)
      setCountdown(diff > 0 ? diff : 0)
    }
    calc()
    const iv = setInterval(calc, 1000)
    return () => clearInterval(iv)
  }, [status?.is_running, status?.next_poll_at])

  // Debug log polling
  useEffect(() => {
    if (debugPollRef.current) clearInterval(debugPollRef.current)
    if (!showDebug || !status?.session_id) return
    const fetchLogs = async () => {
      try {
        const url = `/api/trading/live/logs/${status.session_id}${lastLogId ? `?after_id=${lastLogId}` : ''}`
        const res = await fetch(url, { headers })
        if (res.ok) {
          const data = await res.json()
          const newLogs = data.logs || []
          if (newLogs.length > 0) {
            setDebugLogs(prev => {
              const existingIds = new Set(prev.map(l => l.id))
              const unique = newLogs.filter(l => !existingIds.has(l.id))
              return [...unique, ...prev].sort((a, b) => b.id - a.id)
            })
            const maxId = Math.max(...newLogs.map(l => l.id))
            setLastLogId(prev => Math.max(prev, maxId))
          }
        }
      } catch { /* ignore */ }
    }
    fetchLogs()
    debugPollRef.current = setInterval(fetchLogs, 5000)
    return () => clearInterval(debugPollRef.current)
  }, [showDebug, status?.session_id])

  // Trade event notification polling (independent of debug panel)
  const playNotificationSound = useCallback((isWin) => {
    try {
      const ctx = new (window.AudioContext || window.webkitAudioContext)()
      const osc = ctx.createOscillator()
      const gain = ctx.createGain()
      osc.connect(gain)
      gain.connect(ctx.destination)
      gain.gain.value = 0.3
      if (isWin) {
        // Rising tone for wins/opens
        osc.frequency.value = 600
        osc.type = 'sine'
        osc.start()
        osc.frequency.linearRampToValueAtTime(900, ctx.currentTime + 0.15)
      } else {
        // Falling tone for losses
        osc.frequency.value = 500
        osc.type = 'sine'
        osc.start()
        osc.frequency.linearRampToValueAtTime(300, ctx.currentTime + 0.15)
      }
      gain.gain.linearRampToValueAtTime(0, ctx.currentTime + 0.3)
      osc.stop(ctx.currentTime + 0.3)
    } catch { /* Audio not available */ }
  }, [])

  const enableNotifications = useCallback(() => {
    if ('Notification' in window && Notification.permission === 'default') {
      Notification.requestPermission().then(p => {
        if (p === 'granted') setNotificationsEnabled(true)
      })
    } else if ('Notification' in window && Notification.permission === 'granted') {
      setNotificationsEnabled(true)
    }
    setNotificationsEnabled(true) // enable sound even without browser notification permission
  }, [])

  useEffect(() => {
    if (notifyPollRef.current) clearInterval(notifyPollRef.current)
    if (!notificationsEnabled || !status?.is_running || !status?.session_id) return
    const TRADE_LEVELS = new Set(['OPEN', 'CLOSE', 'SL', 'TP'])
    const checkTradeEvents = async () => {
      try {
        const afterId = lastNotifyLogId.current
        const url = `/api/trading/live/logs/${status.session_id}${afterId ? `?after_id=${afterId}` : ''}`
        const res = await fetch(url, { headers })
        if (!res.ok) return
        const data = await res.json()
        const logs = data.logs || []
        if (logs.length === 0) return
        const maxId = Math.max(...logs.map(l => l.id))
        if (afterId === 0) {
          // First fetch — just set the baseline, don't notify for old events
          lastNotifyLogId.current = maxId
          return
        }
        lastNotifyLogId.current = maxId
        const tradeEvents = logs.filter(l => TRADE_LEVELS.has(l.level))
        for (const evt of tradeEvents) {
          const isOpen = evt.level === 'OPEN'
          const isWin = evt.level === 'TP' || (evt.level === 'CLOSE' && evt.message.includes('+'))
          playNotificationSound(isOpen || isWin)
          if ('Notification' in window && Notification.permission === 'granted') {
            const icon = isOpen ? 'OPEN' : evt.level === 'TP' ? 'TP' : evt.level === 'SL' ? 'SL' : 'CLOSE'
            new Notification(`${icon} ${evt.symbol}`, {
              body: evt.message,
              icon: isOpen || isWin ? undefined : undefined,
              tag: `trade-${evt.id}`,
            })
          }
        }
      } catch { /* ignore */ }
    }
    checkTradeEvents()
    notifyPollRef.current = setInterval(checkTradeEvents, 5000)
    return () => clearInterval(notifyPollRef.current)
  }, [notificationsEnabled, status?.is_running, status?.session_id])

  const goLive = async () => {
    try {
      const res = await fetch('/api/trading/live/start', { method: 'POST', headers })
      if (res.ok) {
        fetchStatus()
        fetchSessions()
      } else {
        const data = await res.json()
        alert(data.error || 'Fehler beim Starten')
      }
    } catch { alert('Verbindungsfehler') }
  }

  const stopLive = async () => {
    try {
      const res = await fetch('/api/trading/live/stop', { method: 'POST', headers })
      if (res.ok) {
        fetchStatus()
        fetchSessions()
        setPositions([])
      } else {
        const data = await res.json()
        alert(data.error || 'Fehler beim Stoppen')
      }
    } catch { alert('Verbindungsfehler') }
  }

  const resumeSession = async (id) => {
    try {
      const res = await fetch(`/api/trading/live/session/${id}/resume`, { method: 'POST', headers })
      if (res.ok) {
        fetchStatus()
        fetchSessions()
      } else {
        const data = await res.json()
        alert(data.error || 'Fehler beim Fortsetzen')
      }
    } catch { alert('Verbindungsfehler') }
  }

  const loadSession = async (id) => {
    setSelectedSessionId(id === selectedSessionId ? null : id)
    if (id === selectedSessionId) { setSelectedSessionPositions([]); return }
    try {
      const res = await fetch(`/api/trading/live/session/${id}`, { headers })
      if (res.ok) {
        const data = await res.json()
        setSelectedSessionPositions(data.positions || [])
      }
    } catch { /* ignore */ }
  }

  const formatTime = (ts) => {
    if (!ts) return '-'
    return new Date(ts).toLocaleString('de-DE', { day: '2-digit', month: '2-digit', year: '2-digit', hour: '2-digit', minute: '2-digit', second: '2-digit' })
  }

  const symbols = config?.symbols || []
  const openPositions = positions.filter(p => !p.is_closed)
  const closedPositions = positions.filter(p => p.is_closed)

  // Per-symbol aggregation
  const symbolStats = {}
  symbols.forEach(sym => { symbolStats[sym] = { totalReturn: 0, trades: 0, openPos: null } })
  positions.forEach(p => {
    if (!symbolStats[p.symbol]) symbolStats[p.symbol] = { totalReturn: 0, trades: 0, openPos: null }
    if (p.is_closed) {
      symbolStats[p.symbol].totalReturn += p.profit_loss_pct
      symbolStats[p.symbol].trades++
    } else {
      symbolStats[p.symbol].openPos = p
    }
  })

  const totalPnlEur = positions.reduce((s, p) => s + (p.profit_loss_amt || 0), 0)
  const totalWins = positions.filter(p => p.is_closed && p.profit_loss_pct > 0).length
  const totalClosed = closedPositions.length
  const winRate = totalClosed > 0 ? (totalWins / totalClosed) * 100 : 0

  return (
    <div className="min-h-screen bg-dark-900 p-4 md:p-6 max-w-7xl mx-auto">
      {/* Header */}
      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="text-2xl font-bold text-white">Live Trading</h1>
          {config && (
            <div>
              <div className="flex items-center gap-3 mt-1 text-sm text-gray-400">
                <span>{STRATEGY_LABELS[config.strategy] || config.strategy}</span>
                <span className="text-gray-600">|</span>
                <span>{config.interval}</span>
                <span className="text-gray-600">|</span>
                <span>{symbols.length} Aktien</span>
                {config.long_only && <span className="text-accent-400 text-xs">Long Only</span>}
                <span className="text-gray-600">|</span>
                <span>{config.trade_amount} {config.currency || 'EUR'}/Trade</span>
              </div>
              {config.params && Object.keys(config.params).length > 0 && (
                <div className="flex items-center gap-2 mt-1 flex-wrap">
                  {Object.entries(config.params).map(([key, val]) => (
                    <span key={key} className="text-[10px] bg-dark-700 text-gray-400 px-1.5 py-0.5 rounded">
                      {key.replace(/_/g, ' ')}: <span className="text-white">{typeof val === 'boolean' ? (val ? 'an' : 'aus') : val}</span>
                    </span>
                  ))}
                </div>
              )}
            </div>
          )}
        </div>
        {!config && (
          <div className="text-gray-500 text-sm">Keine Konfiguration. Bitte in der Trading Arena "Start Live Trading" drücken.</div>
        )}
      </div>

      {/* Status Bar + Go Live Button */}
      <div className="bg-dark-800 rounded-lg border border-dark-600 p-4 mb-4">
        <div className="flex items-center justify-between mb-3">
          <div className="flex items-center gap-3">
            <span className={`w-3 h-3 rounded-full ${status?.is_running ? 'bg-green-400 animate-pulse' : 'bg-gray-600'}`} />
            <span className={`text-sm font-medium ${status?.is_running ? 'text-green-400' : 'text-gray-400'}`}>
              {status?.is_running ? 'Live Trading Aktiv' : 'Inaktiv'}
            </span>
            {status?.is_polling && (
              <span className="text-xs text-accent-400 animate-pulse">Aktualisiere...</span>
            )}
            {status?.is_running && (
              <button
                onClick={() => notificationsEnabled ? setNotificationsEnabled(false) : enableNotifications()}
                className={`ml-2 px-2 py-1 text-xs rounded transition-colors ${
                  notificationsEnabled
                    ? 'bg-yellow-500/20 text-yellow-400 border border-yellow-500/30'
                    : 'bg-dark-600 text-gray-500 hover:text-gray-300 border border-dark-500'
                }`}
                title={notificationsEnabled ? 'Benachrichtigungen aktiv' : 'Benachrichtigungen aktivieren'}
              >
                {notificationsEnabled ? 'Alerts AN' : 'Alerts AUS'}
              </button>
            )}
          </div>
          {isAdmin ? (
            <button
              onClick={status?.is_running ? stopLive : goLive}
              disabled={!config}
              className={`px-5 py-2 rounded-lg text-sm font-medium transition-all ${
                status?.is_running
                  ? 'bg-red-600 hover:bg-red-500 text-white'
                  : config
                    ? 'bg-gradient-to-r from-green-600 to-emerald-600 hover:from-green-500 hover:to-emerald-500 text-white'
                    : 'bg-dark-700 text-gray-500 cursor-not-allowed'
              }`}
            >
              {status?.is_running ? 'Stop Live' : 'Go Live'}
            </button>
          ) : (
            <span className="flex items-center gap-2 px-4 py-2 bg-dark-700 border border-dark-600 rounded-lg text-sm text-gray-500 cursor-not-allowed">
              <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z" />
              </svg>
              Pro Abo
            </span>
          )}
        </div>

        {status?.is_running && (
          <>
            <div className="grid grid-cols-2 md:grid-cols-5 gap-2 text-xs">
              {[
                { label: 'Interval', value: status.interval },
                { label: 'Letzter Poll', value: formatTime(status.last_poll_at) },
                { label: 'Nächster Poll', value: countdown != null ? (countdown === 0 ? 'Jetzt...' : `${Math.floor(countdown / 60)}:${String(countdown % 60).padStart(2, '0')}`) : formatTime(status.next_poll_at) },
                { label: 'Session Start', value: formatTime(status.started_at) },
                { label: 'Polls', value: status.total_polls },
              ].map((item, i) => (
                <div key={i} className="bg-dark-700 rounded p-2">
                  <div className="text-gray-500">{item.label}</div>
                  <div className="text-white font-medium">{item.value}</div>
                </div>
              ))}
            </div>
            {status.is_polling && (
              <div className="mt-2">
                <div className="flex items-center justify-between text-xs mb-1">
                  <span className="text-accent-400 animate-pulse">
                    Prüfe: {status.current_symbol || '...'} ({status.scan_progress_current || 0}/{status.scan_progress_total || 0})
                  </span>
                </div>
                <div className="h-1.5 bg-dark-600 rounded-full overflow-hidden">
                  <div
                    className="h-full bg-accent-500 rounded-full transition-all duration-500"
                    style={{ width: status.scan_progress_total > 0 ? `${(status.scan_progress_current / status.scan_progress_total) * 100}%` : '0%' }}
                  />
                </div>
              </div>
            )}
          </>
        )}
      </div>

      {/* Resume Banner */}
      {isAdmin && !status?.is_running && status?.last_session?.can_resume && (
        <div className="bg-amber-500/10 border border-amber-500/30 rounded-lg p-4 mb-4 flex items-center justify-between">
          <div>
            <div className="text-sm font-medium text-amber-400">
              Session #{status.last_session.id} wurde unterbrochen
            </div>
            <div className="text-xs text-amber-400/70 mt-0.5">
              {STRATEGY_LABELS[status.last_session.strategy] || status.last_session.strategy} | {status.last_session.interval} | {status.last_session.symbols_count} Aktien | {status.last_session.total_polls} Polls
              {status.last_session.open_positions > 0 && (
                <span className="text-amber-300 ml-2">{status.last_session.open_positions} offene Position(en)</span>
              )}
            </div>
          </div>
          <button
            onClick={() => resumeSession(status.last_session.id)}
            className="px-4 py-2 bg-amber-600 hover:bg-amber-500 text-white text-sm font-medium rounded-lg transition-colors"
          >
            Fortsetzen
          </button>
        </div>
      )}

      {/* Session Metrics */}
      {(status?.is_running || positions.length > 0) && (
        <div className="grid grid-cols-2 md:grid-cols-6 gap-2 mb-4">
          {[
            { label: 'Offene Pos.', value: openPositions.length, color: 'text-white' },
            { label: 'Geschlossen', value: totalClosed, color: 'text-white' },
            { label: 'Win Rate', value: `${winRate.toFixed(0)}%`, color: winRate >= 50 ? 'text-green-400' : 'text-red-400' },
            { label: 'G/V', value: `${totalPnlEur >= 0 ? '+' : ''}${totalPnlEur.toFixed(2)} ${config?.currency || 'EUR'}`, color: totalPnlEur >= 0 ? 'text-green-400' : 'text-red-400' },
            { label: 'Aktien', value: `${symbols.length}` },
            { label: 'Strategie', value: STRATEGY_LABELS[config?.strategy] || '-' },
          ].map((m, i) => (
            <div key={i} className="bg-dark-800 rounded-lg border border-dark-600 p-3">
              <div className="text-[10px] text-gray-500">{m.label}</div>
              <div className={`text-sm font-bold ${m.color || 'text-white'}`}>{m.value}</div>
            </div>
          ))}
        </div>
      )}

      {/* Symbols Grid */}
      {symbols.length > 0 && (
        <div className="bg-dark-800 rounded-lg border border-dark-600 p-4 mb-4">
          <h3 className="text-sm font-medium text-white mb-3">Aktien ({symbols.length})</h3>
          <div className="grid grid-cols-2 md:grid-cols-4 lg:grid-cols-6 xl:grid-cols-8 gap-2">
            {symbols.map(sym => {
              const stat = symbolStats[sym] || { totalReturn: 0, trades: 0, openPos: null }
              return (
                <div key={sym} className="bg-dark-700 rounded p-2 border border-transparent hover:border-accent-500/30 transition-colors">
                  <div className="flex items-center justify-between">
                    <span className="text-xs font-medium text-white truncate">{sym}</span>
                    {stat.openPos && <span className="w-1.5 h-1.5 rounded-full bg-green-400 animate-pulse" />}
                  </div>
                  {symbolPrices[sym] != null && (
                    <div className="text-[10px] text-gray-400 mt-0.5">{formatPrice(symbolPrices[sym], sym)}</div>
                  )}
                  <div className="flex justify-between mt-1 text-[10px]">
                    {stat.trades > 0 ? (
                      <span className={stat.totalReturn >= 0 ? 'text-green-400' : 'text-red-400'}>
                        {stat.totalReturn >= 0 ? '+' : ''}{stat.totalReturn.toFixed(1)}%
                      </span>
                    ) : (
                      <span className="text-gray-600">-</span>
                    )}
                    <span className="text-gray-500">{stat.trades}T</span>
                    {stat.openPos && (
                      <span className={stat.openPos.profit_loss_pct >= 0 ? 'text-green-400' : 'text-red-400'}>
                        {stat.openPos.profit_loss_pct >= 0 ? '+' : ''}{stat.openPos.profit_loss_pct.toFixed(1)}%
                      </span>
                    )}
                  </div>
                </div>
              )
            })}
          </div>
        </div>
      )}

      {/* Open Positions */}
      {openPositions.length > 0 && (
        <div className="bg-dark-800 rounded-lg border border-dark-600 p-4 mb-4">
          <h3 className="text-sm font-medium text-white mb-3">Offene Positionen ({openPositions.length})</h3>
          {/* Mobile: Cards */}
          <div className="md:hidden grid grid-cols-1 gap-2">
            {openPositions.map(p => (
              <div key={p.id} className="bg-dark-700 rounded-lg p-3 border border-dark-600">
                <div className="flex items-center justify-between mb-2">
                  <div className="flex items-center gap-2">
                    <span className="text-sm font-bold text-accent-400">{p.symbol}</span>
                    <span className={`text-xs font-medium px-1.5 py-0.5 rounded ${p.direction === 'LONG' ? 'bg-green-500/20 text-green-400' : 'bg-red-500/20 text-red-400'}`}>{p.direction}</span>
                  </div>
                  <span className={`text-sm font-bold ${p.profit_loss_pct >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                    {p.profit_loss_pct >= 0 ? '+' : ''}{p.profit_loss_pct?.toFixed(2)}%
                  </span>
                </div>
                <div className="grid grid-cols-2 gap-x-4 gap-y-1 text-[11px]">
                  <div><span className="text-gray-500">Entry:</span> <span className="text-gray-300">{p.entry_price?.toFixed(2)}</span></div>
                  <div><span className="text-gray-500">Aktuell:</span> <span className="text-white font-medium">{p.current_price?.toFixed(2)}</span></div>
                  <div><span className="text-gray-500">SL:</span> <span className="text-red-400/60">{p.stop_loss?.toFixed(2)}</span></div>
                  <div><span className="text-gray-500">TP:</span> <span className="text-green-400/60">{p.take_profit?.toFixed(2)}</span></div>
                  <div><span className="text-gray-500">G/V:</span> <span className={p.profit_loss_amt >= 0 ? 'text-green-400' : 'text-red-400'}>{p.profit_loss_amt >= 0 ? '+' : ''}{p.profit_loss_amt?.toFixed(2)} {config?.currency || 'EUR'}</span></div>
                  <div className="text-gray-600">{formatTime(p.entry_time)}</div>
                </div>
              </div>
            ))}
          </div>
          {/* Desktop: Table */}
          <div className="hidden md:block overflow-auto">
            <table className="w-full text-xs">
              <thead>
                <tr className="text-left text-gray-500 border-b border-dark-600">
                  <th className="pb-2 pr-3">Symbol</th>
                  <th className="pb-2 pr-3">Dir</th>
                  <th className="pb-2 pr-3">Entry</th>
                  <th className="pb-2 pr-3">Aktuell</th>
                  <th className="pb-2 pr-3 text-right">P&L %</th>
                  <th className="pb-2 pr-3 text-right">P&L {config?.currency || 'EUR'}</th>
                  <th className="pb-2 pr-3 text-right">SL</th>
                  <th className="pb-2 text-right">TP</th>
                </tr>
              </thead>
              <tbody>
                {openPositions.map(p => (
                  <tr key={p.id} className="border-b border-dark-700/50">
                    <td className="py-2 pr-3 font-medium text-accent-400">{p.symbol}</td>
                    <td className={`py-2 pr-3 font-medium ${p.direction === 'LONG' ? 'text-green-400' : 'text-red-400'}`}>{p.direction}</td>
                    <td className="py-2 pr-3 text-gray-400">
                      <div>{formatPrice(p.entry_price_usd, p.symbol)}</div>
                      <div className="text-gray-600 text-[10px]">{formatTime(p.entry_time)}</div>
                    </td>
                    <td className="py-2 pr-3 text-white font-medium">
                      {p.current_price?.toFixed(2)} {p.native_currency}
                    </td>
                    <td className={`py-2 pr-3 text-right font-medium ${p.profit_loss_pct >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                      {p.profit_loss_pct >= 0 ? '+' : ''}{p.profit_loss_pct?.toFixed(2)}%
                    </td>
                    <td className={`py-2 pr-3 text-right font-medium ${p.profit_loss_amt >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                      {p.profit_loss_amt >= 0 ? '+' : ''}{p.profit_loss_amt?.toFixed(2)}
                    </td>
                    <td className="py-2 pr-3 text-right text-red-400/60">{p.stop_loss?.toFixed(2)}</td>
                    <td className="py-2 text-right text-green-400/60">{p.take_profit?.toFixed(2)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}

      {/* Closed Trades */}
      {closedPositions.length > 0 && (
        <div className="bg-dark-800 rounded-lg border border-dark-600 p-4 mb-4">
          <h3 className="text-sm font-medium text-white mb-3">Trade History ({closedPositions.length})</h3>
          <div className="overflow-auto max-h-96">
            <table className="w-full text-xs">
              <thead className="sticky top-0 bg-dark-800">
                <tr className="text-left text-gray-500 border-b border-dark-600">
                  <th className="pb-2 pr-2">Symbol</th>
                  <th className="pb-2 pr-2">Dir</th>
                  <th className="pb-2 pr-2">Entry</th>
                  <th className="pb-2 pr-2">Exit</th>
                  <th className="pb-2 pr-2 text-right">Return</th>
                  <th className="pb-2 pr-2 text-right">G/V {config?.currency || 'EUR'}</th>
                  <th className="pb-2 text-right">Reason</th>
                </tr>
              </thead>
              <tbody>
                {[...closedPositions].sort((a, b) => new Date(b.close_time) - new Date(a.close_time)).map(p => (
                  <tr key={p.id} className="border-b border-dark-700/50">
                    <td className="py-1.5 pr-2 font-medium text-accent-400">{p.symbol}</td>
                    <td className={`py-1.5 pr-2 ${p.direction === 'LONG' ? 'text-green-400' : 'text-red-400'}`}>{p.direction}</td>
                    <td className="py-1.5 pr-2 text-gray-400">
                      <div>{formatPrice(p.entry_price_usd, p.symbol)}</div>
                      <div className="text-gray-600 text-[10px]">{formatTime(p.entry_time)}</div>
                    </td>
                    <td className="py-1.5 pr-2 text-gray-400">
                      <div>{formatPrice(p.close_price_usd, p.symbol)}</div>
                      <div className="text-gray-600 text-[10px]">{formatTime(p.close_time)}</div>
                    </td>
                    <td className={`py-1.5 pr-2 text-right font-medium ${p.profit_loss_pct >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                      {p.profit_loss_pct >= 0 ? '+' : ''}{p.profit_loss_pct?.toFixed(2)}%
                    </td>
                    <td className={`py-1.5 pr-2 text-right font-medium ${p.profit_loss_amt >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                      {p.profit_loss_amt >= 0 ? '+' : ''}{p.profit_loss_amt?.toFixed(2)}
                    </td>
                    <td className="py-1.5 text-right">
                      <span className={`px-1.5 py-0.5 rounded text-[10px] font-medium ${
                        p.close_reason === 'TP' ? 'bg-green-500/20 text-green-400' :
                        p.close_reason === 'SL' ? 'bg-red-500/20 text-red-400' :
                        p.close_reason === 'SIGNAL' ? 'bg-yellow-500/20 text-yellow-400' :
                        'bg-gray-500/20 text-gray-400'
                      }`}>{p.close_reason}</span>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}

      {/* Debug-Log */}
      {status?.session_id && (
        <div className="bg-dark-800 rounded-lg border border-dark-600 mb-4">
          <button
            onClick={() => setShowDebug(!showDebug)}
            className="w-full px-4 py-3 flex items-center justify-between text-left hover:bg-dark-700/50 transition-colors"
          >
            <span className="text-sm font-medium text-white">
              Debug-Log {debugLogs.length > 0 && `(${debugLogs.length})`}
            </span>
            <svg className={`w-4 h-4 text-gray-400 transition-transform ${showDebug ? 'rotate-180' : ''}`} fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 9l-7 7-7-7" />
            </svg>
          </button>
          {showDebug && (
            <div className="px-4 pb-4 border-t border-dark-600">
              <div className="mt-2 max-h-80 overflow-auto font-mono text-[11px] space-y-0.5">
                {debugLogs.length === 0 && (
                  <div className="text-gray-600 py-2 text-center">Noch keine Log-Einträge</div>
                )}
                {debugLogs.map(log => {
                  const levelColors = {
                    SCAN: 'text-blue-400',
                    SIGNAL: 'text-yellow-400',
                    OPEN: 'text-green-400',
                    CLOSE: 'text-red-400',
                    SL: 'text-red-400',
                    TP: 'text-red-400',
                    SKIP: 'text-gray-500',
                    INFO: 'text-gray-400',
                  }
                  const time = new Date(log.created_at).toLocaleTimeString('de-DE', { hour: '2-digit', minute: '2-digit', second: '2-digit' })
                  return (
                    <div key={log.id} className="flex gap-2 py-0.5 border-b border-dark-700/30">
                      <span className="text-gray-600 shrink-0">{time}</span>
                      <span className={`shrink-0 w-14 text-right ${levelColors[log.level] || 'text-gray-400'}`}>{log.level}</span>
                      <span className="text-gray-500 shrink-0 w-16 text-right">{log.symbol !== '-' ? log.symbol : ''}</span>
                      <span className="text-gray-300">{log.message}</span>
                    </div>
                  )
                })}
              </div>
            </div>
          )}
        </div>
      )}

      {/* Session-Übersicht */}
      {sessions.length > 0 && (
        <div className="bg-dark-800 rounded-lg border border-dark-600">
          <button
            onClick={() => setShowHistory(!showHistory)}
            className="w-full px-4 py-3 flex items-center justify-between text-left hover:bg-dark-700/50 transition-colors"
          >
            <span className="text-sm font-medium text-white">Session-Übersicht ({sessions.length})</span>
            <svg className={`w-4 h-4 text-gray-400 transition-transform ${showHistory ? 'rotate-180' : ''}`} fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 9l-7 7-7-7" />
            </svg>
          </button>
          {showHistory && (
            <div className="px-4 pb-4 border-t border-dark-600">
              <div className="space-y-2 mt-3">
                {sessions.map(s => (
                  <div key={s.id}>
                    <div
                      onClick={() => loadSession(s.id)}
                      className={`p-3 rounded-lg cursor-pointer transition-colors ${
                        s.is_active
                          ? 'bg-green-500/10 border border-green-500/20'
                          : s.can_resume
                            ? 'bg-amber-500/5 border border-amber-500/20 hover:bg-amber-500/10'
                            : 'bg-dark-700 hover:bg-dark-600'
                      }`}
                    >
                      <div className="flex items-center justify-between mb-1">
                        <div className="flex items-center gap-2">
                          <span className={`w-2 h-2 rounded-full ${
                            s.is_active ? 'bg-green-400 animate-pulse' :
                            s.can_resume ? 'bg-amber-400' : 'bg-gray-600'
                          }`} />
                          <span className="text-xs font-medium text-white">
                            #{s.id} {STRATEGY_LABELS[s.strategy] || s.strategy} ({s.interval})
                          </span>
                          <span className="text-[10px] text-gray-500">{s.symbols_count} Aktien</span>
                          {s.is_active && <span className="text-[10px] text-green-400 font-medium">AKTIV</span>}
                          {!s.is_active && s.can_resume && <span className="text-[10px] text-amber-400 font-medium">FORTSETZBAR</span>}
                        </div>
                        <div className="flex items-center gap-2">
                          <span className="text-[10px] text-gray-500">{formatTime(s.started_at)}</span>
                          {isAdmin && !s.is_active && s.can_resume && !status?.is_running && (
                            <button
                              onClick={(e) => { e.stopPropagation(); resumeSession(s.id) }}
                              className="px-2.5 py-1 bg-amber-600 hover:bg-amber-500 text-white text-[10px] font-medium rounded transition-colors"
                            >
                              Fortsetzen
                            </button>
                          )}
                          {isAdmin && s.is_active && (
                            <button
                              onClick={(e) => { e.stopPropagation(); stopLive() }}
                              className="px-2.5 py-1 bg-red-600 hover:bg-red-500 text-white text-[10px] font-medium rounded transition-colors"
                            >
                              Stoppen
                            </button>
                          )}
                        </div>
                      </div>
                      <div className="flex items-center gap-4 text-[10px]">
                        <span className="text-gray-400">{s.total_trades} Trades</span>
                        <span className={s.total_pnl >= 0 ? 'text-green-400' : 'text-red-400'}>
                          {s.total_pnl >= 0 ? '+' : ''}{s.total_pnl?.toFixed(2)} {s.currency || 'EUR'}
                        </span>
                        <span className={s.win_rate >= 50 ? 'text-green-400' : 'text-red-400'}>
                          {s.win_rate?.toFixed(0)}% Win
                        </span>
                        <span className="text-gray-500">{s.total_polls} Polls</span>
                        {s.stopped_at && <span className="text-gray-600">bis {formatTime(s.stopped_at)}</span>}
                      </div>
                    </div>
                    {/* Expanded session trades */}
                    {selectedSessionId === s.id && selectedSessionPositions.length > 0 && (
                      <div className="mt-1 ml-4 p-2 bg-dark-700/50 rounded">
                        <table className="w-full text-[10px]">
                          <thead>
                            <tr className="text-gray-500">
                              <th className="text-left pb-1">Symbol</th>
                              <th className="text-left pb-1">Dir</th>
                              <th className="text-right pb-1">Return</th>
                              <th className="text-right pb-1">G/V</th>
                              <th className="text-right pb-1">Reason</th>
                            </tr>
                          </thead>
                          <tbody>
                            {selectedSessionPositions.map(p => (
                              <tr key={p.id} className="border-t border-dark-600/50">
                                <td className="py-1 text-gray-300">{p.symbol}</td>
                                <td className={p.direction === 'LONG' ? 'text-green-400' : 'text-red-400'}>{p.direction}</td>
                                <td className={`text-right ${p.profit_loss_pct >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                                  {p.profit_loss_pct >= 0 ? '+' : ''}{p.profit_loss_pct?.toFixed(2)}%
                                </td>
                                <td className={`text-right ${p.profit_loss_amt >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                                  {p.profit_loss_amt?.toFixed(2)}
                                </td>
                                <td className="text-right text-gray-400">{p.close_reason || 'OPEN'}</td>
                              </tr>
                            ))}
                          </tbody>
                        </table>
                      </div>
                    )}
                  </div>
                ))}
              </div>
            </div>
          )}
        </div>
      )}

      {/* Empty state */}
      {!config && !status?.is_running && sessions.length === 0 && (
        <div className="bg-dark-800 rounded-lg border border-dark-600 p-8 text-center">
          <div className="text-gray-500 mb-2">Noch keine Live-Trading Konfiguration</div>
          <div className="text-gray-600 text-sm">
            Gehe zur Trading Arena, konfiguriere deine Strategie und Filter,
            und drücke "Start Live Trading" um zu beginnen.
          </div>
        </div>
      )}
    </div>
  )
}

export default LiveTrading
