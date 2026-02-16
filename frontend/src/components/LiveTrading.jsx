import { useState, useEffect, useCallback, useRef } from 'react'
import { useCurrency } from '../context/CurrencyContext'

const STRATEGY_LABELS = {
  regression_scalping: 'Regression Scalping',
  hybrid_ai_trend: 'NW Bollinger Bands',
  diamond_signals: 'Diamond Signals',
}

function TestOrderPanel({ headers, onOrderPlaced, tradeAmount, currency }) {
  const [symbol, setSymbol] = useState('AAPL')
  const [side, setSide] = useState('buy')
  const [sl, setSl] = useState('')
  const [tp, setTp] = useState('')
  const [loading, setLoading] = useState(false)
  const [result, setResult] = useState(null)

  const placeOrder = async () => {
    setLoading(true)
    setResult(null)
    try {
      const body = { symbol: symbol.toUpperCase(), side }
      if (sl) body.stop_loss = parseFloat(sl)
      if (tp) body.take_profit = parseFloat(tp)
      const res = await fetch('/api/trading/live/alpaca/test-order', {
        method: 'POST',
        headers: { ...headers, 'Content-Type': 'application/json' },
        body: JSON.stringify(body),
      })
      const data = await res.json()
      if (res.ok) {
        setResult({ ok: true, ...data })
        if (onOrderPlaced) setTimeout(onOrderPlaced, 2000)
      } else {
        setResult({ ok: false, error: data.error })
      }
    } catch {
      setResult({ ok: false, error: 'Verbindungsfehler' })
    }
    setLoading(false)
  }

  return (
    <div className="border-t border-dark-600 pt-3 mt-1">
      <div className="flex items-center gap-2 mb-2">
        <span className="text-xs text-gray-400 font-medium">Test-Order (Paper)</span>
        <span className="text-[10px] text-gray-600">Einsatz: {tradeAmount} {currency} pro Trade</span>
      </div>
      <div className="flex flex-wrap items-end gap-2">
        <div>
          <label className="text-[10px] text-gray-500 block mb-0.5">Symbol</label>
          <input type="text" value={symbol} onChange={e => setSymbol(e.target.value)} className="w-24 bg-dark-700 border border-dark-500 rounded px-2 py-1.5 text-xs text-white uppercase focus:border-accent-500 focus:outline-none" />
        </div>
        <div>
          <label className="text-[10px] text-gray-500 block mb-0.5">Seite</label>
          <select value={side} onChange={e => setSide(e.target.value)} className="bg-dark-700 border border-dark-500 rounded px-2 py-1.5 text-xs text-white focus:border-accent-500 focus:outline-none">
            <option value="buy">Buy</option>
            <option value="sell">Sell</option>
          </select>
        </div>
        {side === 'buy' && (
          <>
            <div>
              <label className="text-[10px] text-red-400/70 block mb-0.5">Stop Loss $</label>
              <input type="number" value={sl} onChange={e => setSl(e.target.value)} placeholder="z.B. 180" step="0.01" className="w-20 bg-dark-700 border border-dark-500 rounded px-2 py-1.5 text-xs text-white focus:border-red-500 focus:outline-none" />
            </div>
            <div>
              <label className="text-[10px] text-green-400/70 block mb-0.5">Take Profit $</label>
              <input type="number" value={tp} onChange={e => setTp(e.target.value)} placeholder="z.B. 250" step="0.01" className="w-20 bg-dark-700 border border-dark-500 rounded px-2 py-1.5 text-xs text-white focus:border-green-500 focus:outline-none" />
            </div>
          </>
        )}
        <button
          onClick={placeOrder}
          disabled={loading || !symbol}
          className="px-3 py-1.5 text-xs bg-amber-600 hover:bg-amber-500 disabled:bg-dark-600 disabled:text-gray-600 text-white rounded transition-colors font-medium"
        >
          {loading ? 'Sende...' : 'Test-Order senden'}
        </button>
      </div>
      {result && (
        <div className={`mt-2 text-xs p-2 rounded ${result.ok ? 'bg-green-500/10 text-green-400 border border-green-500/20' : 'bg-red-500/10 text-red-400 border border-red-500/20'}`}>
          {result.ok ? (
            <div>
              <div>Order platziert — {result.side.toUpperCase()} {result.qty}x {result.symbol} ({result.trade_amount} {result.currency})</div>
              <div>Status: {result.status} | Typ: {result.order_class || 'simple'} | ID: {result.order_id?.slice(0, 8)}...</div>
              {result.stop_loss > 0 && <div>SL: ${result.stop_loss.toFixed(2)} | TP: ${result.take_profit?.toFixed(2)}</div>}
              {result.legs?.length > 0 && <div>Legs: {result.legs.map(l => `${l.type} (${l.status})`).join(', ')}</div>}
            </div>
          ) : `Fehler: ${result.error}`}
        </div>
      )}
    </div>
  )
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
  const [hiddenLogLevels, setHiddenLogLevels] = useState(new Set())
  const [debugLogs, setDebugLogs] = useState([])
  const [lastLogId, setLastLogId] = useState(0)
  const [notificationsEnabled, setNotificationsEnabled] = useState(false)
  const [showAlpaca, setShowAlpaca] = useState(false)
  const [alpacaKey, setAlpacaKey] = useState('')
  const [alpacaSecret, setAlpacaSecret] = useState('')
  const [alpacaEnabled, setAlpacaEnabled] = useState(false)
  const [alpacaPaper, setAlpacaPaper] = useState(true)
  const [alpacaValidation, setAlpacaValidation] = useState(null)
  const [alpacaValidating, setAlpacaValidating] = useState(false)
  const [tradeAmount, setTradeAmount] = useState(500)
  const [alpacaPortfolio, setAlpacaPortfolio] = useState(null)
  const [alpacaPortfolioLoading, setAlpacaPortfolioLoading] = useState(false)
  const [showAlpacaOrders, setShowAlpacaOrders] = useState(false)
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
        if (data.alpaca_api_key) setAlpacaKey(data.alpaca_api_key)
        if (data.alpaca_secret_key) setAlpacaSecret(data.alpaca_secret_key)
        if (data.alpaca_enabled != null) setAlpacaEnabled(data.alpaca_enabled)
        if (data.alpaca_paper != null) setAlpacaPaper(data.alpaca_paper)
        if (data.trade_amount) setTradeAmount(data.trade_amount)
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

  const fetchAlpacaPortfolio = useCallback(async () => {
    try {
      const res = await fetch('/api/trading/live/alpaca/portfolio', { headers })
      if (res.ok) {
        const data = await res.json()
        setAlpacaPortfolio(data)
      }
    } catch { /* ignore */ }
  }, [token])

  // Load on mount
  useEffect(() => {
    fetchConfig()
    fetchStatus()
    fetchSessions()
  }, [])

  // Fetch Alpaca portfolio when enabled
  const alpacaPollRef = useRef(null)
  useEffect(() => {
    if (!alpacaEnabled) {
      if (alpacaPollRef.current) clearInterval(alpacaPollRef.current)
      setAlpacaPortfolio(null)
      return
    }
    fetchAlpacaPortfolio()
    alpacaPollRef.current = setInterval(fetchAlpacaPortfolio, 30000)
    return () => clearInterval(alpacaPollRef.current)
  }, [alpacaEnabled])

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

  const deleteSession = async (id) => {
    if (!confirm(`Session #${id} wirklich löschen? Alle Positionen und Logs werden entfernt.`)) return
    try {
      const res = await fetch(`/api/trading/live/session/${id}`, { method: 'DELETE', headers })
      if (res.ok) {
        fetchSessions()
        if (selectedSessionId === id) { setSelectedSessionId(null); setSelectedSessionPositions([]) }
      } else {
        const data = await res.json()
        alert(data.error || 'Fehler beim Löschen')
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
  const totalInvested = positions.reduce((s, p) => s + (p.invested_amount || 0), 0)
  const totalRenditePct = totalInvested > 0 ? (totalPnlEur / totalInvested) * 100 : 0
  // Win Rate: alle Positionen (offen: + = win, - = loss)
  const allPositionsForWinRate = positions.filter(p => p.is_closed || p.profit_loss_pct != null)
  const totalWins = allPositionsForWinRate.filter(p => (p.profit_loss_pct || 0) > 0).length
  const totalLosses = allPositionsForWinRate.filter(p => (p.profit_loss_pct || 0) <= 0).length
  const totalAll = allPositionsForWinRate.length
  const winRate = totalAll > 0 ? (totalWins / totalAll) * 100 : 0
  const totalClosed = closedPositions.length
  // Avg returns
  const avgReturnPerTrade = totalAll > 0 ? positions.reduce((s, p) => s + (p.profit_loss_pct || 0), 0) / totalAll : 0
  const winPositions = allPositionsForWinRate.filter(p => (p.profit_loss_pct || 0) > 0)
  const losePositions = allPositionsForWinRate.filter(p => (p.profit_loss_pct || 0) <= 0)
  const avgWin = winPositions.length > 0 ? winPositions.reduce((s, p) => s + p.profit_loss_pct, 0) / winPositions.length : 0
  const avgLoss = losePositions.length > 0 ? losePositions.reduce((s, p) => s + p.profit_loss_pct, 0) / losePositions.length : 0
  const riskReward = avgLoss !== 0 ? Math.abs(avgWin / avgLoss) : 0

  // Bot Actions timeline — one OPEN row per position + one CLOSE row per closed position
  const botActions = positions.flatMap(p => {
    const actions = [{
      time: p.entry_time,
      type: 'OPEN',
      symbol: p.symbol,
      direction: p.direction,
      price: p.entry_price,
      qty: p.quantity,
      invested: p.invested_amount,
      sl: p.stop_loss,
      tp: p.take_profit,
      pnlPct: null,
      pnlAmt: null,
      alpaca: !!p.alpaca_order_id,
    }]
    if (p.is_closed) {
      actions.push({
        time: p.close_time,
        type: p.close_reason || 'CLOSE',
        symbol: p.symbol,
        direction: p.direction,
        price: p.close_price,
        qty: p.quantity,
        invested: p.invested_amount,
        sl: null,
        tp: null,
        pnlPct: p.profit_loss_pct,
        pnlAmt: p.profit_loss_amt,
        alpaca: !!p.alpaca_order_id,
      })
    }
    return actions
  }).sort((a, b) => new Date(b.time) - new Date(a.time))

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

      {/* Alpaca Broker Section — Admin only */}
      {isAdmin && (
        <div className="bg-dark-800 rounded-lg border border-dark-600 mb-4">
          <button
            onClick={() => setShowAlpaca(!showAlpaca)}
            className="w-full flex items-center justify-between p-4 text-sm hover:bg-dark-700 transition-colors rounded-lg"
          >
            <div className="flex items-center gap-2">
              <span className="text-gray-300 font-medium">Broker-Anbindung</span>
              {alpacaEnabled && (
                <span className="px-2 py-0.5 rounded text-xs bg-green-500/20 text-green-400 border border-green-500/30">
                  {alpacaPaper ? 'Paper' : 'Live'}
                </span>
              )}
              {!alpacaEnabled && <span className="text-gray-600 text-xs">Optional</span>}
            </div>
            <svg className={`w-4 h-4 text-gray-500 transition-transform ${showAlpaca ? 'rotate-180' : ''}`} fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 9l-7 7-7-7" />
            </svg>
          </button>
          {showAlpaca && (
            <div className="px-4 pb-4 space-y-3">
              <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
                <div>
                  <label className="text-xs text-gray-500 block mb-1">Alpaca API Key</label>
                  <input
                    type="text"
                    value={alpacaKey}
                    onChange={e => setAlpacaKey(e.target.value)}
                    placeholder="PK..."
                    className="w-full bg-dark-700 border border-dark-500 rounded px-3 py-2 text-sm text-white placeholder-gray-600 focus:border-accent-500 focus:outline-none"
                  />
                </div>
                <div>
                  <label className="text-xs text-gray-500 block mb-1">Secret Key</label>
                  <input
                    type="password"
                    value={alpacaSecret}
                    onChange={e => setAlpacaSecret(e.target.value)}
                    placeholder="••••••••"
                    className="w-full bg-dark-700 border border-dark-500 rounded px-3 py-2 text-sm text-white placeholder-gray-600 focus:border-accent-500 focus:outline-none"
                  />
                </div>
              </div>
              <div>
                <label className="text-xs text-gray-500 block mb-1">Betrag pro Trade ({currency || 'EUR'})</label>
                <input
                  type="number"
                  value={tradeAmount}
                  onChange={e => setTradeAmount(Number(e.target.value) || 0)}
                  min="1"
                  step="50"
                  className="w-full md:w-48 bg-dark-700 border border-dark-500 rounded px-3 py-2 text-sm text-white placeholder-gray-600 focus:border-accent-500 focus:outline-none"
                />
              </div>
              <div className="flex items-center gap-4">
                <button
                  onClick={async () => {
                    setAlpacaValidating(true)
                    setAlpacaValidation(null)
                    try {
                      const res = await fetch('/api/trading/live/alpaca/validate', {
                        method: 'POST',
                        headers: { ...headers, 'Content-Type': 'application/json' },
                        body: JSON.stringify({ api_key: alpacaKey, secret_key: alpacaSecret, paper: alpacaPaper })
                      })
                      const data = await res.json()
                      if (res.ok) {
                        setAlpacaValidation({ ok: true, ...data })
                      } else {
                        setAlpacaValidation({ ok: false, error: data.error })
                      }
                    } catch {
                      setAlpacaValidation({ ok: false, error: 'Verbindungsfehler' })
                    }
                    setAlpacaValidating(false)
                  }}
                  disabled={!alpacaKey || !alpacaSecret || alpacaValidating}
                  className="px-3 py-1.5 text-xs bg-accent-600 hover:bg-accent-500 disabled:bg-dark-600 disabled:text-gray-600 text-white rounded transition-colors"
                >
                  {alpacaValidating ? 'Prüfe...' : 'Verbindung testen'}
                </button>
                <label className="flex items-center gap-2 text-sm cursor-pointer">
                  <input
                    type="checkbox"
                    checked={alpacaEnabled}
                    onChange={e => setAlpacaEnabled(e.target.checked)}
                    className="rounded bg-dark-700 border-dark-500 text-accent-500 focus:ring-accent-500"
                  />
                  <span className="text-gray-300">Orders an Alpaca senden</span>
                </label>
              </div>
              {alpacaValidation && (
                <div className={`text-xs p-2 rounded ${alpacaValidation.ok ? 'bg-green-500/10 text-green-400 border border-green-500/20' : 'bg-red-500/10 text-red-400 border border-red-500/20'}`}>
                  {alpacaValidation.ok
                    ? `Verbunden — Status: ${alpacaValidation.status} | Kaufkraft: $${Number(alpacaValidation.buying_power).toLocaleString('de-DE')} | ${alpacaValidation.paper ? 'Paper Trading' : 'LIVE'}`
                    : `Fehler: ${alpacaValidation.error}`
                  }
                </div>
              )}
              {alpacaEnabled && (
                <div className="text-xs text-yellow-400/70 bg-yellow-500/5 border border-yellow-500/10 rounded p-2">
                  {alpacaPaper
                    ? 'Paper-Trading aktiv — keine echten Trades. Orders werden an die Alpaca Paper-API gesendet.'
                    : 'LIVE-Trading aktiv — echte Orders werden ausgeführt!'
                  }
                </div>
              )}
              <div className="flex justify-end">
                <button
                  onClick={async () => {
                    try {
                      await fetch('/api/trading/live/config', {
                        method: 'POST',
                        headers: { ...headers, 'Content-Type': 'application/json' },
                        body: JSON.stringify({
                          ...config,
                          alpaca_api_key: alpacaKey,
                          alpaca_secret_key: alpacaSecret,
                          alpaca_enabled: alpacaEnabled,
                          alpaca_paper: alpacaPaper,
                          trade_amount: tradeAmount,
                        })
                      })
                      setShowAlpaca(false)
                    } catch { alert('Speichern fehlgeschlagen') }
                  }}
                  className="px-4 py-1.5 text-xs bg-dark-600 hover:bg-dark-500 text-white rounded transition-colors"
                >
                  Speichern
                </button>
              </div>
              {alpacaEnabled && alpacaPaper && (
                <TestOrderPanel headers={headers} tradeAmount={tradeAmount} currency={config?.currency || 'EUR'} onOrderPlaced={() => {
                  fetchAlpacaPortfolio()
                  if (status?.session_id) fetchPositions(status.session_id)
                }} />
              )}
            </div>
          )}
        </div>
      )}

      {/* Alpaca Portfolio */}
      {alpacaEnabled && alpacaPortfolio && (
        <div className="bg-dark-800 rounded-lg border border-dark-600 p-4 mb-4">
          <div className="flex items-center justify-between mb-4">
            <div className="flex items-center gap-2">
              <h3 className="text-sm font-semibold text-white">Alpaca Portfolio</h3>
              <span className={`text-[10px] px-1.5 py-0.5 rounded ${alpacaPortfolio.account.paper ? 'bg-amber-500/20 text-amber-400 border border-amber-500/30' : 'bg-green-500/20 text-green-400 border border-green-500/30'}`}>
                {alpacaPortfolio.account.paper ? 'PAPER' : 'LIVE'}
              </span>
              <span className={`text-[10px] px-1.5 py-0.5 rounded ${alpacaPortfolio.account.status === 'ACTIVE' ? 'bg-green-500/20 text-green-400' : 'bg-red-500/20 text-red-400'}`}>
                {alpacaPortfolio.account.status}
              </span>
            </div>
            <button
              onClick={() => { setAlpacaPortfolioLoading(true); fetchAlpacaPortfolio().finally(() => setAlpacaPortfolioLoading(false)) }}
              className="text-xs text-gray-500 hover:text-gray-300 transition-colors"
              title="Aktualisieren"
            >
              {alpacaPortfolioLoading ? '...' : '↻'}
            </button>
          </div>

          {/* Account Overview */}
          <div className="grid grid-cols-2 md:grid-cols-5 gap-2 mb-4">
            {[
              { label: 'Gesamtwert', value: `$${alpacaPortfolio.account.equity.toLocaleString('de-DE', { minimumFractionDigits: 2 })}`, color: 'text-white' },
              { label: 'Kaufkraft', value: `$${alpacaPortfolio.account.buying_power.toLocaleString('de-DE', { minimumFractionDigits: 2 })}`, color: 'text-accent-400' },
              { label: 'Bargeld', value: `$${alpacaPortfolio.account.cash.toLocaleString('de-DE', { minimumFractionDigits: 2 })}`, color: 'text-gray-300' },
              { label: 'Tagesänderung', value: `${alpacaPortfolio.account.day_change >= 0 ? '+' : ''}$${alpacaPortfolio.account.day_change.toLocaleString('de-DE', { minimumFractionDigits: 2 })}`, color: alpacaPortfolio.account.day_change >= 0 ? 'text-green-400' : 'text-red-400' },
              { label: 'Tages %', value: `${alpacaPortfolio.account.day_change_pct >= 0 ? '+' : ''}${alpacaPortfolio.account.day_change_pct.toFixed(2)}%`, color: alpacaPortfolio.account.day_change_pct >= 0 ? 'text-green-400' : 'text-red-400' },
            ].map((item, i) => (
              <div key={i} className="bg-dark-700 rounded-lg p-3">
                <div className="text-[10px] text-gray-500 uppercase tracking-wider">{item.label}</div>
                <div className={`text-sm font-bold mt-1 ${item.color}`}>{item.value}</div>
              </div>
            ))}
          </div>

          {/* Open Positions */}
          {alpacaPortfolio.positions.length > 0 && (
            <div className="mb-4">
              <h4 className="text-xs font-medium text-gray-400 mb-2 uppercase tracking-wider">Offene Positionen ({alpacaPortfolio.positions.length})</h4>

              {/* Mobile Cards */}
              <div className="md:hidden space-y-2">
                {alpacaPortfolio.positions.map((p, i) => (
                  <div key={i} className="bg-dark-700 rounded-lg p-3">
                    <div className="flex items-center justify-between mb-2">
                      <div className="flex items-center gap-2">
                        <span className="text-sm font-bold text-accent-400">{p.symbol}</span>
                        <span className={`text-[10px] px-1.5 py-0.5 rounded ${p.side === 'long' ? 'bg-green-500/20 text-green-400' : 'bg-red-500/20 text-red-400'}`}>{p.side.toUpperCase()}</span>
                        <span className="text-[10px] text-gray-500">{p.qty}x</span>
                      </div>
                      <span className={`text-sm font-bold ${p.unrealized_pl >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                        {p.unrealized_pl >= 0 ? '+' : ''}{p.unrealized_pl_pct.toFixed(2)}%
                      </span>
                    </div>
                    <div className="grid grid-cols-3 gap-2 text-[10px]">
                      <div><span className="text-gray-500">Einstieg:</span> <span className="text-gray-300">${p.avg_entry_price.toFixed(2)}</span></div>
                      <div><span className="text-gray-500">Aktuell:</span> <span className="text-gray-300">${p.current_price.toFixed(2)}</span></div>
                      <div><span className="text-gray-500">G/V:</span> <span className={p.unrealized_pl >= 0 ? 'text-green-400' : 'text-red-400'}>{p.unrealized_pl >= 0 ? '+' : ''}${p.unrealized_pl.toFixed(2)}</span></div>
                    </div>
                  </div>
                ))}
              </div>

              {/* Desktop Table */}
              <div className="hidden md:block overflow-x-auto">
                <table className="w-full text-xs">
                  <thead>
                    <tr className="text-gray-500 text-left">
                      <th className="pb-2 pr-3">Symbol</th>
                      <th className="pb-2 pr-3">Seite</th>
                      <th className="pb-2 pr-3 text-right">Stück</th>
                      <th className="pb-2 pr-3 text-right">Einstieg</th>
                      <th className="pb-2 pr-3 text-right">Aktuell</th>
                      <th className="pb-2 pr-3 text-right">Marktwert</th>
                      <th className="pb-2 pr-3 text-right">G/V $</th>
                      <th className="pb-2 text-right">G/V %</th>
                    </tr>
                  </thead>
                  <tbody>
                    {alpacaPortfolio.positions.map((p, i) => (
                      <tr key={i} className="border-t border-dark-600/50">
                        <td className="py-2 pr-3 font-medium text-accent-400">{p.symbol}</td>
                        <td className={`py-2 pr-3 font-medium ${p.side === 'long' ? 'text-green-400' : 'text-red-400'}`}>{p.side.toUpperCase()}</td>
                        <td className="py-2 pr-3 text-right text-gray-300">{p.qty}</td>
                        <td className="py-2 pr-3 text-right text-gray-400">${p.avg_entry_price.toFixed(2)}</td>
                        <td className="py-2 pr-3 text-right text-gray-300">${p.current_price.toFixed(2)}</td>
                        <td className="py-2 pr-3 text-right text-gray-300">${p.market_value.toLocaleString('de-DE', { minimumFractionDigits: 2 })}</td>
                        <td className={`py-2 pr-3 text-right font-medium ${p.unrealized_pl >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                          {p.unrealized_pl >= 0 ? '+' : ''}${p.unrealized_pl.toFixed(2)}
                        </td>
                        <td className={`py-2 text-right font-medium ${p.unrealized_pl_pct >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                          {p.unrealized_pl_pct >= 0 ? '+' : ''}{p.unrealized_pl_pct.toFixed(2)}%
                        </td>
                      </tr>
                    ))}
                  </tbody>
                  <tfoot>
                    <tr className="border-t border-dark-500">
                      <td colSpan={5} className="py-2 pr-3 text-gray-400 font-medium">Gesamt</td>
                      <td className="py-2 pr-3 text-right text-white font-medium">
                        ${alpacaPortfolio.positions.reduce((s, p) => s + p.market_value, 0).toLocaleString('de-DE', { minimumFractionDigits: 2 })}
                      </td>
                      <td className={`py-2 pr-3 text-right font-bold ${alpacaPortfolio.positions.reduce((s, p) => s + p.unrealized_pl, 0) >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                        {alpacaPortfolio.positions.reduce((s, p) => s + p.unrealized_pl, 0) >= 0 ? '+' : ''}${alpacaPortfolio.positions.reduce((s, p) => s + p.unrealized_pl, 0).toFixed(2)}
                      </td>
                      <td></td>
                    </tr>
                  </tfoot>
                </table>
              </div>
            </div>
          )}

          {alpacaPortfolio.positions.length === 0 && (
            <div className="text-center py-4 text-gray-600 text-sm">Keine offenen Positionen</div>
          )}

          {/* Recent Orders */}
          {alpacaPortfolio.orders.length > 0 && (
            <div>
              <button
                onClick={() => setShowAlpacaOrders(!showAlpacaOrders)}
                className="flex items-center gap-1 text-xs text-gray-500 hover:text-gray-300 transition-colors mb-2"
              >
                <svg className={`w-3 h-3 transition-transform ${showAlpacaOrders ? 'rotate-180' : ''}`} fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 9l-7 7-7-7" />
                </svg>
                Letzte Orders ({alpacaPortfolio.orders.length})
              </button>
              {showAlpacaOrders && (
                <div className="overflow-x-auto">
                  <table className="w-full text-[10px]">
                    <thead>
                      <tr className="text-gray-600">
                        <th className="text-left pb-1">Symbol</th>
                        <th className="text-left pb-1">Seite</th>
                        <th className="text-left pb-1">Typ</th>
                        <th className="text-right pb-1">Stück</th>
                        <th className="text-right pb-1">Preis</th>
                        <th className="text-right pb-1">SL</th>
                        <th className="text-right pb-1">TP</th>
                        <th className="text-left pb-1">Status</th>
                        <th className="text-left pb-1">Datum</th>
                      </tr>
                    </thead>
                    <tbody>
                      {alpacaPortfolio.orders.map((o, i) => (
                        <tr key={i} className="border-t border-dark-700/50">
                          <td className="py-1 text-gray-300">{o.symbol}</td>
                          <td className={o.side === 'buy' ? 'text-green-400' : 'text-red-400'}>{o.side?.toUpperCase()}</td>
                          <td className="py-1">
                            {o.order_class === 'bracket' ? (
                              <span className="px-1 py-0.5 rounded bg-purple-500/20 text-purple-400">BRACKET</span>
                            ) : o.order_class === 'oto' ? (
                              <span className="px-1 py-0.5 rounded bg-blue-500/20 text-blue-400">OTO</span>
                            ) : (
                              <span className="text-gray-600">{o.order_type || 'market'}</span>
                            )}
                          </td>
                          <td className="py-1 text-right text-gray-400">{o.filled_qty || o.qty}</td>
                          <td className="py-1 text-right text-gray-400">{o.filled_avg_price > 0 ? `$${o.filled_avg_price.toFixed(2)}` : '-'}</td>
                          <td className="py-1 text-right text-red-400/60">{o.stop_price > 0 ? `$${o.stop_price.toFixed(2)}` : o.legs?.find(l => l.type === 'stop')?.stop_price > 0 ? `$${o.legs.find(l => l.type === 'stop').stop_price.toFixed(2)}` : '-'}</td>
                          <td className="py-1 text-right text-green-400/60">{o.limit_price > 0 ? `$${o.limit_price.toFixed(2)}` : o.legs?.find(l => l.type === 'limit')?.limit_price > 0 ? `$${o.legs.find(l => l.type === 'limit').limit_price.toFixed(2)}` : '-'}</td>
                          <td className={`py-1 ${o.status === 'filled' ? 'text-green-400' : o.status === 'canceled' || o.status === 'cancelled' ? 'text-gray-600' : 'text-amber-400'}`}>{o.status}</td>
                          <td className="py-1 text-gray-600">{o.filled_at ? new Date(o.filled_at).toLocaleString('de-DE', { day: '2-digit', month: '2-digit', hour: '2-digit', minute: '2-digit' }) : o.created_at ? new Date(o.created_at).toLocaleString('de-DE', { day: '2-digit', month: '2-digit', hour: '2-digit', minute: '2-digit' }) : '-'}</td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              )}
            </div>
          )}
        </div>
      )}

      {/* Resume Banner */}
      {isAdmin && !status?.is_running && status?.last_session && (
        <div className={`rounded-lg p-4 mb-4 flex items-center justify-between ${
          status.last_session.can_resume
            ? 'bg-amber-500/10 border border-amber-500/30'
            : 'bg-dark-700 border border-dark-500'
        }`}>
          <div>
            <div className={`text-sm font-medium ${status.last_session.can_resume ? 'text-amber-400' : 'text-gray-400'}`}>
              Session #{status.last_session.id} wurde {status.last_session.can_resume ? 'unterbrochen' : 'beendet'}
            </div>
            <div className={`text-xs mt-0.5 ${status.last_session.can_resume ? 'text-amber-400/70' : 'text-gray-500'}`}>
              {STRATEGY_LABELS[status.last_session.strategy] || status.last_session.strategy} | {status.last_session.interval} | {status.last_session.symbols_count} Aktien | {status.last_session.total_polls} Polls
              {status.last_session.open_positions > 0 && (
                <span className={`ml-2 ${status.last_session.can_resume ? 'text-amber-300' : 'text-yellow-500'}`}>{status.last_session.open_positions} offene Position(en)</span>
              )}
              {!status.last_session.can_resume && (
                <span className="text-gray-600 ml-2">Config geändert — nicht fortsetzbar</span>
              )}
            </div>
          </div>
          {status.last_session.can_resume && (
            <button
              onClick={() => resumeSession(status.last_session.id)}
              className="px-4 py-2 bg-amber-600 hover:bg-amber-500 text-white text-sm font-medium rounded-lg transition-colors"
            >
              Fortsetzen
            </button>
          )}
        </div>
      )}

      {/* Performance */}
      {(status?.is_running || positions.length > 0) && (
        <div className="bg-dark-800 rounded-lg border border-dark-600 p-4 mb-4">
          <div className="flex items-center justify-between mb-3">
            <h3 className="text-sm font-medium text-white">Performance</h3>
            <div className="flex items-center gap-3 text-[10px] text-gray-500">
              <span>{openPositions.length} offen</span>
              <span>{totalClosed} geschlossen</span>
              <span>{symbols.length} Aktien</span>
              <span>{STRATEGY_LABELS[config?.strategy] || '-'}</span>
            </div>
          </div>
          <div className="grid grid-cols-2 md:grid-cols-4 lg:grid-cols-7 gap-2">
            {[
              { label: 'Rendite', value: `${totalRenditePct >= 0 ? '+' : ''}${totalRenditePct.toFixed(2)}%`, sub: `(${totalPnlEur >= 0 ? '+' : ''}${totalPnlEur.toFixed(2)}€)`, color: totalPnlEur >= 0 ? 'text-green-400' : 'text-red-400' },
              { label: 'Win Rate', value: `${winRate.toFixed(0)}%`, sub: `${totalWins}W / ${totalLosses}L`, color: winRate >= 50 ? 'text-green-400' : 'text-red-400' },
              { label: 'R/R', value: riskReward > 0 ? riskReward.toFixed(2) : '-', sub: 'Risk/Reward', color: riskReward >= 1 ? 'text-green-400' : riskReward > 0 ? 'text-red-400' : 'text-gray-400' },
              { label: 'Ø / Trade', value: `${avgReturnPerTrade >= 0 ? '+' : ''}${avgReturnPerTrade.toFixed(2)}%`, color: avgReturnPerTrade >= 0 ? 'text-green-400' : 'text-red-400' },
              { label: 'Ø Win', value: winPositions.length > 0 ? `+${avgWin.toFixed(2)}%` : '-', color: 'text-green-400' },
              { label: 'Ø Loss', value: losePositions.length > 0 ? `${avgLoss.toFixed(2)}%` : '-', color: 'text-red-400' },
              { label: 'Investiert', value: `${totalInvested.toFixed(0)}€`, color: 'text-white' },
            ].map((m, i) => (
              <div key={i} className="bg-dark-700 rounded-lg p-2.5">
                <div className="text-[10px] text-gray-500">{m.label}</div>
                <div className={`text-sm font-bold ${m.color || 'text-white'}`}>
                  {m.value}
                  {m.sub && <span className="text-[10px] text-gray-500 font-normal ml-1">{m.sub}</span>}
                </div>
              </div>
            ))}
          </div>
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
                    {p.alpaca_order_id && <span className="text-[10px] px-1 py-0.5 rounded bg-purple-500/20 text-purple-400 border border-purple-500/30" title={`Order: ${p.alpaca_order_id}`}>ALPACA</span>}
                  </div>
                  <span className={`text-sm font-bold ${p.profit_loss_pct >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                    {p.profit_loss_pct >= 0 ? '+' : ''}{p.profit_loss_pct?.toFixed(2)}%
                  </span>
                </div>
                <div className="grid grid-cols-2 gap-x-4 gap-y-1 text-[11px]">
                  <div><span className="text-gray-500">Entry:</span> <span className="text-gray-300">{formatPrice(p.entry_price, p.symbol)}</span></div>
                  <div><span className="text-gray-500">Aktuell:</span> <span className="text-white font-medium">{formatPrice(p.current_price, p.symbol)}</span></div>
                  <div><span className="text-gray-500">Stk:</span> <span className="text-gray-300">{p.quantity || '-'}x</span></div>
                  <div><span className="text-gray-500">Buy-In:</span> <span className="text-gray-300">{p.invested_amount?.toFixed(2)} €</span></div>
                  <div><span className="text-gray-500">Rendite:</span> <span className={p.profit_loss_pct >= 0 ? 'text-green-400' : 'text-red-400'}>{p.profit_loss_pct >= 0 ? '+' : ''}{p.profit_loss_pct?.toFixed(2)}% ({p.profit_loss_amt >= 0 ? '+' : ''}{p.profit_loss_amt?.toFixed(2)}€)</span></div>
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
                  <th className="pb-2 pr-3 text-right">Stk</th>
                  <th className="pb-2 pr-3 text-right">Buy-In</th>
                  <th className="pb-2 pr-3 text-right">Rendite</th>
                  <th className="pb-2 text-right">SL / TP</th>
                </tr>
              </thead>
              <tbody>
                {openPositions.map(p => (
                  <tr key={p.id} className="border-b border-dark-700/50">
                    <td className="py-2 pr-3 font-medium text-accent-400">
                      {p.symbol}
                      {p.alpaca_order_id && <span className="ml-1 text-[10px] px-1 py-0.5 rounded bg-purple-500/20 text-purple-400" title={`Order: ${p.alpaca_order_id}`}>A</span>}
                    </td>
                    <td className={`py-2 pr-3 font-medium ${p.direction === 'LONG' ? 'text-green-400' : 'text-red-400'}`}>{p.direction}</td>
                    <td className="py-2 pr-3 text-gray-400">
                      <div>{formatPrice(p.entry_price, p.symbol)}</div>
                      <div className="text-gray-600 text-[10px]">{formatTime(p.entry_time)}</div>
                    </td>
                    <td className="py-2 pr-3 text-white font-medium">
                      {formatPrice(p.current_price, p.symbol)}
                    </td>
                    <td className="py-2 pr-3 text-right text-gray-300">{p.quantity ? `${p.quantity}x` : '-'}</td>
                    <td className="py-2 pr-3 text-right text-gray-400">{p.invested_amount?.toFixed(2)} €</td>
                    <td className={`py-2 pr-3 text-right font-medium ${p.profit_loss_pct >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                      {p.profit_loss_pct >= 0 ? '+' : ''}{p.profit_loss_pct?.toFixed(2)}%
                      <span className="text-gray-500 font-normal ml-1">({p.profit_loss_amt >= 0 ? '+' : ''}{p.profit_loss_amt?.toFixed(2)}€)</span>
                    </td>
                    <td className="py-2 text-right text-gray-400 text-[10px]">
                      {p.stop_loss > 0 || p.take_profit > 0 ? (
                        <>{p.stop_loss > 0 ? formatPrice(p.stop_loss, p.symbol) : '-'} / {p.take_profit > 0 ? formatPrice(p.take_profit, p.symbol) : '-'}</>
                      ) : '-'}
                    </td>
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
          <div className="flex items-center justify-between mb-3">
            <h3 className="text-sm font-medium text-white">Trade History ({closedPositions.length})</h3>
            <div className="flex items-center gap-3 text-[10px] text-gray-500">
              <span><span className="inline-block w-1.5 h-1.5 rounded-full bg-green-400 mr-1" />TP = Take Profit</span>
              <span><span className="inline-block w-1.5 h-1.5 rounded-full bg-red-400 mr-1" />SL = Stop Loss</span>
              <span><span className="inline-block w-1.5 h-1.5 rounded-full bg-yellow-400 mr-1" />SIGNAL = Strategie-Signal</span>
              <span><span className="inline-block w-1.5 h-1.5 rounded-full bg-gray-400 mr-1" />MANUAL = Manuell geschlossen</span>
            </div>
          </div>
          <div className="overflow-auto max-h-96">
            <table className="w-full text-xs">
              <thead className="sticky top-0 bg-dark-800">
                <tr className="text-left text-gray-500 border-b border-dark-600">
                  <th className="pb-2 pr-2">Symbol</th>
                  <th className="pb-2 pr-2">Dir</th>
                  <th className="pb-2 pr-2">Entry</th>
                  <th className="pb-2 pr-2">Exit</th>
                  <th className="pb-2 pr-2 text-right">Stk</th>
                  <th className="pb-2 pr-2 text-right">Rendite</th>
                  <th className="pb-2 text-right">Reason</th>
                </tr>
              </thead>
              <tbody>
                {[...closedPositions].sort((a, b) => new Date(b.close_time) - new Date(a.close_time)).map(p => (
                  <tr key={p.id} className="border-b border-dark-700/50">
                    <td className="py-1.5 pr-2 font-medium text-accent-400">{p.symbol}</td>
                    <td className={`py-1.5 pr-2 ${p.direction === 'LONG' ? 'text-green-400' : 'text-red-400'}`}>{p.direction}</td>
                    <td className="py-1.5 pr-2 text-gray-400">
                      <div>{formatPrice(p.entry_price, p.symbol)}</div>
                      <div className="text-gray-600 text-[10px]">{formatTime(p.entry_time)}</div>
                    </td>
                    <td className="py-1.5 pr-2 text-gray-400">
                      <div>{formatPrice(p.close_price, p.symbol)}</div>
                      <div className="text-gray-600 text-[10px]">{formatTime(p.close_time)}</div>
                    </td>
                    <td className="py-1.5 pr-2 text-right text-gray-400">{p.quantity ? `${p.quantity}x` : '-'}</td>
                    <td className={`py-1.5 pr-2 text-right font-medium ${p.profit_loss_pct >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                      {p.profit_loss_pct >= 0 ? '+' : ''}{p.profit_loss_pct?.toFixed(2)}%
                      <span className="text-gray-500 font-normal ml-1">({p.profit_loss_amt >= 0 ? '+' : ''}{p.profit_loss_amt?.toFixed(2)}€)</span>
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

      {/* Letzte Bot Aktionen */}
      {botActions.length > 0 && (
        <div className="bg-dark-800 rounded-lg border border-dark-600 p-4 mb-4">
          <h3 className="text-sm font-medium text-white mb-3">Letzte Bot Aktionen ({botActions.length})</h3>

          {/* Mobile: Cards */}
          <div className="md:hidden space-y-2">
            {botActions.map((a, i) => (
              <div key={i} className="bg-dark-700 rounded-lg p-3 border border-dark-600">
                <div className="flex items-center justify-between mb-2">
                  <div className="flex items-center gap-2">
                    <span className={`text-[10px] font-bold px-1.5 py-0.5 rounded ${
                      a.type === 'OPEN' ? 'bg-blue-500/20 text-blue-400' :
                      a.type === 'TP' ? 'bg-green-500/20 text-green-400' :
                      a.type === 'SL' ? 'bg-red-500/20 text-red-400' :
                      'bg-yellow-500/20 text-yellow-400'
                    }`}>{a.type}</span>
                    <span className="text-sm font-bold text-accent-400">{a.symbol}</span>
                    <span className={`text-[10px] ${a.direction === 'LONG' ? 'text-green-400' : 'text-red-400'}`}>{a.direction}</span>
                    {a.alpaca && <span className="text-[10px] px-1 py-0.5 rounded bg-purple-500/20 text-purple-400">A</span>}
                  </div>
                  {a.pnlPct != null && (
                    <span className={`text-sm font-bold ${a.pnlPct >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                      {a.pnlPct >= 0 ? '+' : ''}{a.pnlPct.toFixed(2)}%
                    </span>
                  )}
                </div>
                <div className="grid grid-cols-2 gap-x-4 gap-y-1 text-[11px]">
                  <div><span className="text-gray-500">Kurs:</span> <span className="text-gray-300">{formatPrice(a.price, a.symbol)}</span></div>
                  <div><span className="text-gray-500">Stk:</span> <span className="text-gray-300">{a.qty || '-'}</span></div>
                  {a.type === 'OPEN' && <>
                    <div><span className="text-gray-500">Buy-In:</span> <span className="text-gray-300">{a.invested?.toFixed(0)} €</span></div>
                    <div><span className="text-gray-500">SL:</span> <span className="text-gray-400">{a.sl > 0 ? formatPrice(a.sl, a.symbol) : '-'}</span></div>
                  </>}
                  {a.pnlAmt != null && (
                    <div><span className="text-gray-500">G/V:</span> <span className={a.pnlAmt >= 0 ? 'text-green-400' : 'text-red-400'}>{a.pnlAmt >= 0 ? '+' : ''}{a.pnlAmt.toFixed(2)} €</span></div>
                  )}
                  <div className="text-gray-600">{formatTime(a.time)}</div>
                </div>
              </div>
            ))}
          </div>

          {/* Desktop: Table */}
          <div className="hidden md:block overflow-auto max-h-96">
            <table className="w-full text-xs">
              <thead className="sticky top-0 bg-dark-800">
                <tr className="text-left text-gray-500 border-b border-dark-600">
                  <th className="pb-2 pr-2">Zeit</th>
                  <th className="pb-2 pr-2">Aktion</th>
                  <th className="pb-2 pr-2">Symbol</th>
                  <th className="pb-2 pr-2">Dir</th>
                  <th className="pb-2 pr-2 text-right">Kurs</th>
                  <th className="pb-2 pr-2 text-right">Stk</th>
                  <th className="pb-2 pr-2 text-right">Invest</th>
                  <th className="pb-2 pr-2 text-right">SL</th>
                  <th className="pb-2 pr-2 text-right">TP</th>
                  <th className="pb-2 pr-2 text-right">G/V %</th>
                  <th className="pb-2 text-right">G/V €</th>
                </tr>
              </thead>
              <tbody>
                {botActions.map((a, i) => (
                  <tr key={i} className={`border-b border-dark-700/50 ${a.type === 'OPEN' ? 'bg-dark-800' : 'bg-dark-750'}`}>
                    <td className="py-1.5 pr-2 text-gray-500 whitespace-nowrap">{formatTime(a.time)}</td>
                    <td className="py-1.5 pr-2">
                      <span className={`px-1.5 py-0.5 rounded text-[10px] font-bold ${
                        a.type === 'OPEN' ? 'bg-blue-500/20 text-blue-400' :
                        a.type === 'TP' ? 'bg-green-500/20 text-green-400' :
                        a.type === 'SL' ? 'bg-red-500/20 text-red-400' :
                        a.type === 'SIGNAL' ? 'bg-yellow-500/20 text-yellow-400' :
                        'bg-gray-500/20 text-gray-400'
                      }`}>{a.type}</span>
                      {a.alpaca && <span className="ml-1 text-[10px] px-1 py-0.5 rounded bg-purple-500/20 text-purple-400">A</span>}
                    </td>
                    <td className="py-1.5 pr-2 font-medium text-accent-400">{a.symbol}</td>
                    <td className={`py-1.5 pr-2 font-medium ${a.direction === 'LONG' ? 'text-green-400' : 'text-red-400'}`}>{a.direction}</td>
                    <td className="py-1.5 pr-2 text-right text-gray-300">{formatPrice(a.price, a.symbol)}</td>
                    <td className="py-1.5 pr-2 text-right text-gray-400">{a.qty || '-'}</td>
                    <td className="py-1.5 pr-2 text-right text-gray-400">{a.type === 'OPEN' ? `${a.invested?.toFixed(0)}` : '-'}</td>
                    <td className="py-1.5 pr-2 text-right text-red-400/60">{a.sl > 0 ? formatPrice(a.sl, a.symbol) : '-'}</td>
                    <td className="py-1.5 pr-2 text-right text-green-400/60">{a.tp > 0 ? formatPrice(a.tp, a.symbol) : '-'}</td>
                    <td className={`py-1.5 pr-2 text-right font-medium ${a.pnlPct != null ? (a.pnlPct >= 0 ? 'text-green-400' : 'text-red-400') : 'text-gray-600'}`}>
                      {a.pnlPct != null ? `${a.pnlPct >= 0 ? '+' : ''}${a.pnlPct.toFixed(2)}%` : '-'}
                    </td>
                    <td className={`py-1.5 text-right font-medium ${a.pnlAmt != null ? (a.pnlAmt >= 0 ? 'text-green-400' : 'text-red-400') : 'text-gray-600'}`}>
                      {a.pnlAmt != null ? `${a.pnlAmt >= 0 ? '+' : ''}${a.pnlAmt.toFixed(2)}` : '-'}
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
          {showDebug && (() => {
            const levelColors = {
              SCAN: 'text-blue-400',
              SIGNAL: 'text-yellow-400',
              OPEN: 'text-green-400',
              CLOSE: 'text-red-400',
              SL: 'text-red-400',
              TP: 'text-green-400',
              SKIP: 'text-gray-500',
              INFO: 'text-gray-400',
              ERROR: 'text-orange-400',
              TRADE: 'text-purple-400',
              ALPACA: 'text-purple-400',
            }
            const levelBg = {
              SCAN: 'bg-blue-500/20 border-blue-500/30',
              SIGNAL: 'bg-yellow-500/20 border-yellow-500/30',
              OPEN: 'bg-green-500/20 border-green-500/30',
              CLOSE: 'bg-red-500/20 border-red-500/30',
              SL: 'bg-red-500/20 border-red-500/30',
              TP: 'bg-green-500/20 border-green-500/30',
              SKIP: 'bg-gray-500/20 border-gray-500/30',
              INFO: 'bg-gray-500/20 border-gray-500/30',
              ERROR: 'bg-orange-500/20 border-orange-500/30',
              TRADE: 'bg-purple-500/20 border-purple-500/30',
              ALPACA: 'bg-purple-500/20 border-purple-500/30',
            }
            const allLevels = [...new Set(debugLogs.map(l => l.level))].sort()
            const filteredLogs = debugLogs.filter(l => !hiddenLogLevels.has(l.level))
            const toggleLevel = (level) => {
              setHiddenLogLevels(prev => {
                const next = new Set(prev)
                if (next.has(level)) next.delete(level)
                else next.add(level)
                return next
              })
            }
            return (
              <div className="px-4 pb-4 border-t border-dark-600">
                {allLevels.length > 0 && (
                  <div className="flex flex-wrap items-center gap-1.5 mt-3 mb-2">
                    <span className="text-[10px] text-gray-600 mr-1">Filter:</span>
                    {allLevels.map(level => (
                      <button
                        key={level}
                        onClick={() => toggleLevel(level)}
                        className={`px-2 py-0.5 rounded border text-[10px] font-medium transition-all ${
                          hiddenLogLevels.has(level)
                            ? 'bg-dark-700 border-dark-500 text-gray-600 line-through opacity-50'
                            : `${levelBg[level] || 'bg-gray-500/20 border-gray-500/30'} ${levelColors[level] || 'text-gray-400'}`
                        }`}
                      >
                        {level}
                        <span className="ml-1 text-gray-500 font-normal">{debugLogs.filter(l => l.level === level).length}</span>
                      </button>
                    ))}
                    {hiddenLogLevels.size > 0 && (
                      <button onClick={() => setHiddenLogLevels(new Set())} className="text-[10px] text-gray-500 hover:text-gray-300 ml-1 transition-colors">Alle zeigen</button>
                    )}
                  </div>
                )}
                <div className="max-h-80 overflow-auto font-mono text-[11px] space-y-0.5">
                  {filteredLogs.length === 0 && debugLogs.length > 0 && (
                    <div className="text-gray-600 py-2 text-center">Alle Einträge gefiltert</div>
                  )}
                  {debugLogs.length === 0 && (
                    <div className="text-gray-600 py-2 text-center">Noch keine Log-Einträge</div>
                  )}
                  {filteredLogs.map(log => {
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
            )
          })()}
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
                          {!s.is_active && !s.can_resume && <span className="text-[10px] text-gray-500 font-medium">BEENDET</span>}
                        </div>
                        <div className="flex items-center gap-2">
                          {s.stopped_at && !s.is_active && (
                            <span className="text-[10px] text-gray-600">Gestoppt {formatTime(s.stopped_at)}</span>
                          )}
                          {!s.stopped_at && !s.is_active && (
                            <span className="text-[10px] text-gray-600">Gestartet {formatTime(s.started_at)}</span>
                          )}
                          {s.is_active && (
                            <span className="text-[10px] text-gray-500">Seit {formatTime(s.started_at)}</span>
                          )}
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
                          {isAdmin && !s.is_active && (
                            <button
                              onClick={(e) => { e.stopPropagation(); deleteSession(s.id) }}
                              className="px-2.5 py-1 bg-dark-600 hover:bg-red-600 text-gray-400 hover:text-white text-[10px] font-medium rounded transition-colors"
                              title="Session löschen"
                            >
                              Löschen
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
