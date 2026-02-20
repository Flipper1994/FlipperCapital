import { useState, useEffect, useCallback, useRef, useMemo } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { useReactTable, getCoreRowModel, getSortedRowModel, getFilteredRowModel, flexRender, createColumnHelper } from '@tanstack/react-table'
import { useCurrency } from '../context/CurrencyContext'
import ArenaChart from './ArenaChart'
import ArenaIndicatorChart from './ArenaIndicatorChart'
import ArenaBacktestPanel from './ArenaBacktestPanel'
import { STRATEGIES } from '../utils/arenaConfig'

const STRATEGY_LABELS = Object.fromEntries(STRATEGIES.map(s => [s.value, s.label]))

const BETA_STRATEGIES = new Set(['regression_scalping'])

// Cached Intl formatters — creating these is expensive (~1-2ms each), reusing is ~0.001ms
const dateFmt = new Intl.DateTimeFormat('de-DE', {
  day: '2-digit', month: '2-digit', year: '2-digit',
  hour: '2-digit', minute: '2-digit', second: '2-digit'
})
const dateShortFmt = new Intl.DateTimeFormat('de-DE', {
  day: '2-digit', month: '2-digit', hour: '2-digit', minute: '2-digit'
})
const timeFmt = new Intl.DateTimeFormat('de-DE', {
  hour: '2-digit', minute: '2-digit', second: '2-digit'
})
const etFmt = new Intl.DateTimeFormat('en-US', {
  timeZone: 'America/New_York',
  weekday: 'short', hour: 'numeric', minute: 'numeric', second: 'numeric', hour12: false,
})
const numFmtDE = new Intl.NumberFormat('de-DE', { minimumFractionDigits: 2, maximumFractionDigits: 2 })
const numFmtDE0 = new Intl.NumberFormat('de-DE')

const StatCard = ({ label, value, sub, color }) => (
  <div className="bg-dark-700/50 rounded-lg p-2">
    <div className="text-[9px] text-gray-500 uppercase tracking-wider">{label}</div>
    <div className={`text-xs font-bold mt-0.5 ${color || 'text-white'}`}>{value}</div>
    {sub && <div className={`text-[9px] mt-0.5 ${color || 'text-gray-400'} opacity-70`}>{sub}</div>}
  </div>
)

const symColHelper = createColumnHelper()

const symbolsColumns = [
  symColHelper.accessor('symbol', {
    header: 'Symbol',
    cell: info => <span className="font-medium text-accent-400">{info.getValue()}</span>,
    size: 80,
  }),
  symColHelper.accessor('price', {
    header: 'Preis',
    cell: info => {
      const v = info.getValue()
      return v != null ? <span className="text-gray-400">{v}</span> : <span className="text-gray-600">-</span>
    },
    size: 80,
    enableSorting: false,
  }),
  symColHelper.accessor('openPos', {
    header: 'Offen',
    cell: info => {
      const pos = info.getValue()
      if (!pos) return <span className="text-gray-600">-</span>
      return (
        <span className="flex items-center gap-1">
          <span className="w-1.5 h-1.5 rounded-full bg-green-400 animate-pulse" />
          <span className={pos.profit_loss_pct >= 0 ? 'text-green-400' : 'text-red-400'}>
            {pos.profit_loss_pct >= 0 ? '+' : ''}{pos.profit_loss_pct.toFixed(1)}%
          </span>
        </span>
      )
    },
    size: 80,
    sortingFn: (a, b) => {
      const pa = a.original.openPos
      const pb = b.original.openPos
      if (!pa && !pb) return 0
      if (!pa) return -1
      if (!pb) return 1
      return pa.profit_loss_pct - pb.profit_loss_pct
    },
  }),
  symColHelper.accessor('totalReturn', {
    header: 'Total',
    cell: info => {
      const v = info.getValue()
      const trades = info.row.original.trades
      if (trades === 0) return <span className="text-gray-600">-</span>
      return <span className={v >= 0 ? 'text-green-400' : 'text-red-400'}>{v >= 0 ? '+' : ''}{v.toFixed(1)}%</span>
    },
    size: 70,
  }),
  symColHelper.accessor('trades', {
    header: 'Trades',
    cell: info => <span className="text-gray-400">{info.getValue()}</span>,
    size: 60,
  }),
]

function TestOrderPanel({ headers, onOrderPlaced, tradeAmount }) {
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
        <span className="text-[10px] text-gray-600">Einsatz: ${tradeAmount} pro Trade</span>
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
              <div>Order platziert — {result.side.toUpperCase()} {result.qty}x {result.symbol} (${result.trade_amount})</div>
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

function getNextMarketOpen() {
  const now = new Date()
  const parts = Object.fromEntries(etFmt.formatToParts(now).map(p => [p.type, p.value]))
  const dayOfWeek = parts.weekday // Mon, Tue, ...
  const hour = Number(parts.hour)
  const minute = Number(parts.minute)
  const second = Number(parts.second)
  const etMinutes = hour * 60 + minute
  const marketOpen = 9 * 60 + 30 // 9:30 ET

  // Find next market open (skip weekends)
  let daysAhead = 0
  const dayMap = { Mon: 1, Tue: 2, Wed: 3, Thu: 4, Fri: 5, Sat: 6, Sun: 0 }
  const dayNum = dayMap[dayOfWeek] ?? 0

  if (dayNum >= 1 && dayNum <= 5 && etMinutes < marketOpen) {
    daysAhead = 0 // Today before open
  } else if (dayNum === 5) {
    daysAhead = 3 // Friday after close → Monday
  } else if (dayNum === 6) {
    daysAhead = 2 // Saturday → Monday
  } else if (dayNum === 0) {
    daysAhead = 1 // Sunday → Monday
  } else {
    daysAhead = 1 // Weekday after close → next day
  }

  // Calculate seconds until next open
  const secondsToday = hour * 3600 + minute * 60 + second
  const openSeconds = 9 * 3600 + 30 * 60
  if (daysAhead === 0) {
    return openSeconds - secondsToday
  }
  return (daysAhead - 1) * 86400 + (86400 - secondsToday) + openSeconds
}

function MarketClosedBanner() {
  const [secondsLeft, setSecondsLeft] = useState(() => getNextMarketOpen())

  useEffect(() => {
    const timer = setInterval(() => {
      setSecondsLeft(getNextMarketOpen())
    }, 1000)
    return () => clearInterval(timer)
  }, [])

  const h = Math.floor(secondsLeft / 3600)
  const m = Math.floor((secondsLeft % 3600) / 60)
  const s = secondsLeft % 60

  return (
    <div className="bg-yellow-500/10 border border-yellow-500/30 rounded-lg px-4 py-3 mb-3 flex items-center gap-3">
      <svg className="w-4 h-4 text-yellow-400 shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor">
        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
      </svg>
      <span className="text-yellow-400 font-medium text-sm">Börse geschlossen</span>
      <span className="text-yellow-400/70 text-xs">Nächste Öffnung in {String(h).padStart(2, '0')}:{String(m).padStart(2, '0')}:{String(s).padStart(2, '0')}</span>
    </div>
  )
}

function CountdownDisplay({ nextPollAt, fallback }) {
  const [, tick] = useState(0)
  useEffect(() => {
    if (!nextPollAt) return
    const iv = setInterval(() => tick(c => c + 1), 1000)
    return () => clearInterval(iv)
  }, [nextPollAt])
  if (!nextPollAt) return <>{fallback || '-'}</>
  const diff = Math.max(0, Math.floor((new Date(nextPollAt).getTime() - Date.now()) / 1000))
  if (diff === 0) return <>Jetzt...</>
  return <>{Math.floor(diff / 60)}:{String(diff % 60).padStart(2, '0')}</>
}

const _renderCount = { current: 0 }

function LiveTrading({ isAdmin, token }) {
  const { sessionId: urlSessionId } = useParams()
  const navigate = useNavigate()
  const [config, setConfig] = useState(null)
  const [status, setStatus] = useState(null)
  const [positions, setPositions] = useState([])

  // DEBUG: Render counter
  _renderCount.current++
  const _rc = _renderCount.current
  const _t0 = performance.now()
  console.log(`[LT] RENDER #${_rc} start, urlSessionId=${urlSessionId}`)
  const [sessions, setSessions] = useState([])
  const [showHistory, setShowHistory] = useState(false)
  const [selectedSessionId, setSelectedSessionId] = useState(null)
  const [selectedSessionPositions, setSelectedSessionPositions] = useState([])
  const [symbolPrices, setSymbolPrices] = useState({})
  const [showDebug, setShowDebug] = useState(false)
  const [hiddenLogLevels, setHiddenLogLevels] = useState(new Set(['DEBUG']))
  const [debugSearch, setDebugSearch] = useState('')
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
  const [ordersPage, setOrdersPage] = useState(1)
  const [ordersSearch, setOrdersSearch] = useState('')
  const [alpacaPosSort, setAlpacaPosSort] = useState({ field: null, dir: 'desc' })
  const [appPosSort, setAppPosSort] = useState({ field: null, dir: 'desc' })
  const [showBrokerInfo, setShowBrokerInfo] = useState(false)
  const [showSessionStats, setShowSessionStats] = useState(false)
  const [analysisSymbol, setAnalysisSymbol] = useState(null)
  const [analysisData, setAnalysisData] = useState(null)
  const [analysisLoading, setAnalysisLoading] = useState(false)
  const [analysisStrategyId, setAnalysisStrategyId] = useState(null)
  const [symbolSearch, setSymbolSearch] = useState('')
  const [tradeHistorySearch, setTradeHistorySearch] = useState('')
  const [botActionSearch, setBotActionSearch] = useState('')
  const [symbolsSorting, setSymbolsSorting] = useState([{ id: 'trades', desc: true }])
  const [strategies, setStrategies] = useState([])
  const [showStrategies, setShowStrategies] = useState(false)
  const [strategyFilter, setStrategyFilter] = useState('')
  const { formatPrice, currency } = useCurrency()
  const pollRef = useRef(null)
  const posPollRef = useRef(null)
  const debugPollRef = useRef(null)
  const notifyPollRef = useRef(null)
  const lastNotifyLogId = useRef(0)

  const headers = token ? { 'Authorization': `Bearer ${token}` } : {}

  const openAnalysis = useCallback(async (symbol, strategyId) => {
    if (!urlSessionId) return
    setAnalysisSymbol(symbol)
    setAnalysisLoading(true)
    setAnalysisData(null)
    if (strategyId != null) setAnalysisStrategyId(strategyId)
    try {
      const body = { session_id: Number(urlSessionId), symbol }
      if (strategyId) body.strategy_id = strategyId
      const res = await fetch('/api/trading/live/analyze', {
        method: 'POST',
        headers: { ...headers, 'Content-Type': 'application/json' },
        body: JSON.stringify(body),
      })
      if (res.ok) {
        const data = await res.json()
        setAnalysisData(data)
        if (data.active_strategy?.id) setAnalysisStrategyId(data.active_strategy.id)
      }
    } catch (err) {
      console.error('[LiveTrading] Analysis error:', err)
    }
    setAnalysisLoading(false)
  }, [token, urlSessionId])

  const closeAnalysis = useCallback(() => {
    setAnalysisSymbol(null)
    setAnalysisData(null)
  }, [])

  // ESC to close overlay
  useEffect(() => {
    if (!analysisSymbol) return
    const onKey = (e) => { if (e.key === 'Escape') closeAnalysis() }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [analysisSymbol, closeAnalysis])

  const fetchConfig = useCallback(async (sid) => {
    if (!sid) return
    console.log(`[LT] fetchConfig start sid=${sid}`)
    try {
      const res = await fetch(`/api/trading/live/config?session_id=${sid}`, { headers })
      if (res.ok) {
        const data = await res.json()
        console.log(`[LT] fetchConfig OK: symbols=${data.symbols?.length}, alpaca=${data.alpaca_enabled}, amount=${data.trade_amount}`)
        if (data.symbols) setConfig(data)
        if (data.alpaca_api_key) setAlpacaKey(data.alpaca_api_key)
        if (data.alpaca_secret_key) setAlpacaSecret(data.alpaca_secret_key)
        if (data.alpaca_enabled != null) setAlpacaEnabled(data.alpaca_enabled)
        if (data.alpaca_paper != null) setAlpacaPaper(data.alpaca_paper)
        if (data.trade_amount) setTradeAmount(data.trade_amount)
      }
    } catch (err) { console.log(`[LT] fetchConfig error:`, err) }
  }, [token])

  const fetchStatus = useCallback(async () => {
    console.log(`[LT] fetchStatus start`)
    try {
      const res = await fetch('/api/trading/live/status', { headers })
      if (res.ok) {
        const data = await res.json()
        console.log(`[LT] fetchStatus OK: is_running=${data.is_running}, active_sessions=${data.active_sessions?.length}, prices=${data.symbol_prices ? Object.keys(data.symbol_prices).length : 0}`)
        setStatus(data)
        if (data.symbol_prices) setSymbolPrices(data.symbol_prices)
      }
    } catch (err) { console.log(`[LT] fetchStatus error:`, err) }
  }, [token])

  const fetchPositions = useCallback(async (sessionId) => {
    if (!sessionId) return
    console.log(`[LT] fetchPositions start sid=${sessionId}`)
    try {
      const res = await fetch(`/api/trading/live/session/${sessionId}`, { headers })
      if (res.ok) {
        const data = await res.json()
        console.log(`[LT] fetchPositions OK: positions=${(data.positions||[]).length}, prices=${data.symbol_prices ? Object.keys(data.symbol_prices).length : 0}, strategies=${(data.strategies||[]).length}`)
        setPositions(data.positions || [])
        if (data.symbol_prices) setSymbolPrices(data.symbol_prices)
        if (data.strategies) setStrategies(data.strategies)
      }
    } catch (err) { console.log(`[LT] fetchPositions error:`, err) }
  }, [token])

  const fetchSessions = useCallback(async () => {
    try {
      const res = await fetch('/api/trading/live/sessions', { headers })
      if (res.ok) {
        const data = await res.json()
        const list = data.sessions || []
        setSessions(list)
        return list
      }
    } catch { /* ignore */ }
    return []
  }, [token])

  const fetchAlpacaPortfolio = useCallback(async (sid) => {
    if (!sid) return
    console.log(`[LT] fetchAlpacaPortfolio start sid=${sid}`)
    try {
      const res = await fetch(`/api/trading/live/alpaca/portfolio?session_id=${sid}`, { headers })
      if (res.ok) {
        const data = await res.json()
        console.log(`[LT] fetchAlpacaPortfolio OK: positions=${data.positions?.length}, orders=${data.orders?.length}`)
        setAlpacaPortfolio(data)
      } else {
        console.log(`[LT] fetchAlpacaPortfolio failed: ${res.status}`)
      }
    } catch (err) { console.log(`[LT] fetchAlpacaPortfolio error:`, err) }
  }, [token])

  // Load on mount / when URL session changes
  useEffect(() => {
    console.log(`[LT] EFFECT:mount urlSessionId=${urlSessionId}`)
    fetchStatus()
    fetchSessions().then(list => {
      console.log(`[LT] sessions loaded: ${list?.length}, urlSessionId=${urlSessionId}`)
      if (!urlSessionId && list && list.length > 0) {
        console.log(`[LT] navigating to session ${list[0].id}`)
        navigate(`/live-trading/${list[0].id}`, { replace: true })
      }
    })
    if (urlSessionId) {
      fetchConfig(urlSessionId)
      fetchPositions(urlSessionId)
    }
  }, [urlSessionId])

  // Fetch Alpaca portfolio when enabled + session known + keys saved
  const alpacaPollRef = useRef(null)
  useEffect(() => {
    console.log(`[LT] EFFECT:alpaca enabled=${alpacaEnabled} sid=${urlSessionId} key=${!!alpacaKey}`)
    if (!alpacaEnabled || !urlSessionId || !alpacaKey) {
      if (alpacaPollRef.current) clearInterval(alpacaPollRef.current)
      if (!alpacaEnabled) setAlpacaPortfolio(null)
      return
    }
    fetchAlpacaPortfolio(urlSessionId)
    alpacaPollRef.current = setInterval(() => fetchAlpacaPortfolio(urlSessionId), 30000)
    return () => clearInterval(alpacaPollRef.current)
  }, [alpacaEnabled, urlSessionId, alpacaKey])

  // Poll status + sessions when session is active
  useEffect(() => {
    console.log(`[LT] EFFECT:statusPoll is_running=${status?.is_running}`)
    if (!status?.is_running) {
      if (pollRef.current) clearInterval(pollRef.current)
      return
    }
    pollRef.current = setInterval(() => {
      fetchStatus()
      fetchSessions()
    }, 5000)
    return () => clearInterval(pollRef.current)
  }, [status?.is_running])

  // Poll positions when session is active
  useEffect(() => {
    console.log(`[LT] EFFECT:posPoll is_running=${status?.is_running} sid=${urlSessionId}`)
    if (!status?.is_running || !urlSessionId) {
      if (posPollRef.current) clearInterval(posPollRef.current)
      return
    }
    fetchPositions(urlSessionId)
    posPollRef.current = setInterval(() => fetchPositions(urlSessionId), 10000)
    return () => clearInterval(posPollRef.current)
  }, [status?.is_running, urlSessionId])

  // Debug log polling
  useEffect(() => {
    if (debugPollRef.current) clearInterval(debugPollRef.current)
    if (!showDebug || !urlSessionId) return
    const fetchLogs = async () => {
      try {
        const url = `/api/trading/live/logs/${urlSessionId}${lastLogId ? `?after_id=${lastLogId}` : ''}`
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
  }, [showDebug, urlSessionId])

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
    if (!notificationsEnabled || !status?.is_running || !urlSessionId) return
    const TRADE_LEVELS = new Set(['OPEN', 'CLOSE', 'SL', 'TP'])
    const checkTradeEvents = async () => {
      try {
        const afterId = lastNotifyLogId.current
        const url = `/api/trading/live/logs/${urlSessionId}${afterId ? `?after_id=${afterId}` : ''}`
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
  }, [notificationsEnabled, status?.is_running, urlSessionId])

  const goLive = async () => {
    if (!urlSessionId) return
    try {
      const res = await fetch(`/api/trading/live/session/${urlSessionId}/resume`, { method: 'POST', headers })
      if (res.ok) {
        setPositions([])
        fetchStatus()
        fetchSessions()
      } else {
        const data = await res.json()
        alert(data.error || 'Fehler beim Starten')
      }
    } catch { alert('Verbindungsfehler') }
  }

  const stopLive = async (sessionId) => {
    const sid = sessionId || urlSessionId
    if (!sid) return
    try {
      const res = await fetch(`/api/trading/live/stop?session_id=${sid}`, { method: 'POST', headers })
      if (res.ok) {
        fetchStatus()
        fetchSessions()
        if (String(sid) === String(urlSessionId)) setPositions([])
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
        setPositions([])
        fetchConfig(id)
        fetchPositions(id)
        await fetchSessions()
        await fetchStatus()
        navigate(`/live-trading/${id}`)
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

  const toggleStrategy = async (stratId) => {
    if (!urlSessionId) return
    try {
      const res = await fetch(`/api/trading/live/session/${urlSessionId}/strategy/${stratId}`, {
        method: 'PUT', headers,
      })
      if (res.ok) {
        fetchPositions(urlSessionId)
      } else {
        const data = await res.json()
        alert(data.error || 'Fehler')
      }
    } catch { alert('Verbindungsfehler') }
  }

  const formatTime = (ts) => {
    if (!ts) return '-'
    const d = new Date(ts)
    if (d.getFullYear() < 2000) return '-' // Guard against Go zero time
    return dateFmt.format(d)
  }

  const symbols = config?.symbols || []
  const openPositions = useMemo(() => positions.filter(p => !p.is_closed), [positions])
  const closedPositions = useMemo(() => positions.filter(p => p.is_closed), [positions])

  const toggleSort = (setter) => (field) => {
    setter(prev => {
      if (prev.field === field) return prev.dir === 'desc' ? { field, dir: 'asc' } : { field: null, dir: 'desc' }
      return { field, dir: 'desc' }
    })
  }
  const toggleAlpacaSort = toggleSort(setAlpacaPosSort)
  const toggleAppSort = toggleSort(setAppPosSort)

  const sortedAlpacaPositions = useMemo(() => {
    const pos = alpacaPortfolio?.positions || []
    if (!alpacaPosSort.field) return pos
    const { field, dir } = alpacaPosSort
    return [...pos].sort((a, b) => {
      const va = field === 'market_value' ? a.market_value : a.unrealized_pl_pct
      const vb = field === 'market_value' ? b.market_value : b.unrealized_pl_pct
      return dir === 'desc' ? vb - va : va - vb
    })
  }, [alpacaPortfolio?.positions, alpacaPosSort])

  const sortedAppPositions = useMemo(() => {
    if (!appPosSort.field) return openPositions
    const { field, dir } = appPosSort
    return [...openPositions].sort((a, b) => {
      const va = field === 'marktwert' ? ((a.invested_amount || 0) + (a.profit_loss_amt || 0)) : (a.profit_loss_pct || 0)
      const vb = field === 'marktwert' ? ((b.invested_amount || 0) + (b.profit_loss_amt || 0)) : (b.profit_loss_pct || 0)
      return dir === 'desc' ? vb - va : va - vb
    })
  }, [openPositions, appPosSort])

  // Memoize sorted closed positions for trade history table
  const sortedClosedPositions = useMemo(() =>
    [...closedPositions].sort((a, b) => new Date(b.close_time) - new Date(a.close_time)),
  [closedPositions])

  // Per-symbol aggregation
  const symbolStats = useMemo(() => {
    const stats = {}
    symbols.forEach(sym => { stats[sym] = { totalReturn: 0, trades: 0, openPos: null } })
    positions.forEach(p => {
      if (!stats[p.symbol]) stats[p.symbol] = { totalReturn: 0, trades: 0, openPos: null }
      if (p.is_closed) {
        stats[p.symbol].totalReturn += p.profit_loss_pct
        stats[p.symbol].trades++
      } else {
        stats[p.symbol].openPos = p
      }
    })
    return stats
  }, [symbols, positions])

  const symbolsTableData = useMemo(() =>
    symbols.map(sym => {
      const stat = symbolStats[sym] || { totalReturn: 0, trades: 0, openPos: null }
      return {
        symbol: sym,
        price: symbolPrices[sym] != null ? formatPrice(symbolPrices[sym], sym) : null,
        priceRaw: symbolPrices[sym] ?? 0,
        openPos: stat.openPos,
        totalReturn: stat.totalReturn,
        trades: stat.trades,
      }
    }),
  [symbols, symbolStats, symbolPrices, currency])

  const globalFilterFn = useCallback((row, _columnId, filterValue) =>
    row.original.symbol.toLowerCase().includes(filterValue.toLowerCase()), [])

  const symbolsTable = useReactTable({
    data: symbolsTableData,
    columns: symbolsColumns,
    state: { sorting: symbolsSorting, globalFilter: symbolSearch },
    onSortingChange: setSymbolsSorting,
    onGlobalFilterChange: setSymbolSearch,
    getCoreRowModel: getCoreRowModel(),
    getSortedRowModel: getSortedRowModel(),
    getFilteredRowModel: getFilteredRowModel(),
    globalFilterFn,
  })

  const perfMetrics = useMemo(() => {
    const totalPnlEur = positions.reduce((s, p) => s + (p.profit_loss_amt || 0), 0)
    const totalInvested = positions.reduce((s, p) => s + (p.invested_amount || 0), 0)
    const totalRenditePct = totalInvested > 0 ? (totalPnlEur / totalInvested) * 100 : 0
    const allForWR = positions.filter(p => p.is_closed || p.profit_loss_pct != null)
    const totalWins = allForWR.filter(p => (p.profit_loss_pct || 0) > 0).length
    const totalLosses = allForWR.filter(p => (p.profit_loss_pct || 0) <= 0).length
    const totalAll = allForWR.length
    const winRate = totalAll > 0 ? (totalWins / totalAll) * 100 : 0
    const totalClosed = closedPositions.length
    const avgReturnPerTrade = totalAll > 0 ? positions.reduce((s, p) => s + (p.profit_loss_pct || 0), 0) / totalAll : 0
    const winPositions = allForWR.filter(p => (p.profit_loss_pct || 0) > 0)
    const losePositions = allForWR.filter(p => (p.profit_loss_pct || 0) <= 0)
    const avgWin = winPositions.length > 0 ? winPositions.reduce((s, p) => s + p.profit_loss_pct, 0) / winPositions.length : 0
    const avgLoss = losePositions.length > 0 ? losePositions.reduce((s, p) => s + p.profit_loss_pct, 0) / losePositions.length : 0
    const riskReward = avgLoss !== 0 ? Math.abs(avgWin / avgLoss) : 0
    return { totalPnlEur, totalInvested, totalRenditePct, totalWins, totalLosses, winRate, totalClosed, avgReturnPerTrade, winPositions, losePositions, avgWin, avgLoss, riskReward }
  }, [positions, closedPositions])

  // Bot Actions timeline — one OPEN row per position + one CLOSE row per closed position
  const botActions = useMemo(() => positions.flatMap(p => {
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
  }).sort((a, b) => new Date(b.time) - new Date(a.time)), [positions])

  const currentSession = useMemo(() => sessions.find(s => String(s.id) === String(urlSessionId)), [sessions, urlSessionId])
  const isSessionActive = useMemo(() => currentSession?.is_active || status?.active_sessions?.some(s => String(s.session_id) === String(urlSessionId)), [currentSession, status?.active_sessions, urlSessionId])

  // DEBUG: render timing
  console.log(`[LT] RENDER #${_rc} computed in ${(performance.now() - _t0).toFixed(1)}ms — positions=${positions.length}, symbols=${symbols.length}, config=${!!config}, status=${!!status}, isActive=${isSessionActive}`)

  // No session selected and no sessions exist — show empty state
  if (!urlSessionId && sessions.length === 0) {
    return (
      <div className="flex-1 min-h-0 bg-dark-900 p-4 md:p-6 max-w-7xl mx-auto flex items-center justify-center overflow-y-scroll">
        <div className="text-center">
          <h2 className="text-lg text-gray-400 mb-2">Keine Sessions vorhanden</h2>
          <p className="text-sm text-gray-500 mb-4">Erstelle eine neue Session in der Trading Arena.</p>
          <a href="/trading-arena" className="px-4 py-2 bg-accent-600 hover:bg-accent-500 text-white rounded text-sm font-medium transition-colors inline-block">
            Zur Trading Arena
          </a>
        </div>
      </div>
    )
  }

  return (
    <div className="flex-1 min-h-0 bg-dark-900 p-4 md:p-6 max-w-7xl mx-auto overflow-y-scroll">
      {/* Header */}
      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="text-2xl font-bold text-white">Live Trading</h1>
          {config && (
            <div>
              <div className="flex items-center gap-3 mt-1 text-sm text-gray-400">
                <span>{config.interval}</span>
                <span className="text-gray-600">|</span>
                <span>{symbols.length} Aktien</span>
                <span className="text-gray-600">|</span>
                <span>${config.trade_amount}/Trade</span>
                <span className="text-gray-600">|</span>
                <span>{strategies.filter(s => s.is_enabled).length}/{strategies.length} Strategien aktiv</span>
              </div>
              {strategies.length > 0 && (
                <div className="mt-2 space-y-1.5">
                  {strategies.map(s => {
                    let params = {}
                    try { params = JSON.parse(s.params_json || '{}') } catch {}
                    let stratSymbols = []
                    try { stratSymbols = JSON.parse(s.symbols || '[]') } catch {}
                    const paramEntries = Object.entries(params).filter(([, v]) => v !== null && v !== undefined)
                    return (
                      <div key={s.id} className="flex items-start gap-2">
                        <span className={`shrink-0 mt-0.5 text-[10px] px-1.5 py-0.5 rounded font-bold ${
                          s.is_enabled ? 'bg-green-500/20 text-green-400' : 'bg-gray-700 text-gray-500'
                        }`}>
                          {s.is_enabled ? 'ON' : 'OFF'}
                        </span>
                        <div className="min-w-0">
                          <div className="flex items-center gap-2 text-xs">
                            <span className="text-white font-medium">{STRATEGY_LABELS[s.name] || s.name}</span>
                            <span className="text-gray-600">|</span>
                            <span className="text-gray-500">{stratSymbols.length} Symbole</span>
                            <span className="text-gray-600">|</span>
                            <span className="text-gray-500">{s.long_only ? 'Long Only' : 'Long+Short'}</span>
                          </div>
                          {paramEntries.length > 0 && (
                            <div className="flex items-center gap-1.5 mt-0.5 flex-wrap">
                              {paramEntries.map(([key, val]) => (
                                <span key={key} className="text-[10px] bg-dark-700 text-gray-500 px-1.5 py-0.5 rounded">
                                  {key.replace(/_/g, ' ')}: <span className="text-gray-300">{val === 1 ? 'an' : val === 0 ? 'aus' : val}</span>
                                </span>
                              ))}
                            </div>
                          )}
                        </div>
                      </div>
                    )
                  })}
                </div>
              )}
              {strategies.length === 0 && config.params && Object.keys(config.params).length > 0 && (
                <div className="flex items-center gap-2 mt-1 flex-wrap">
                  <span className="text-xs text-gray-400">{STRATEGY_LABELS[config.strategy] || config.strategy}</span>
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
          <div className="text-gray-500 text-sm">Keine Konfiguration. Bitte in der Trading Arena "Neue Session starten" drücken.</div>
        )}
      </div>

      {/* Live Trader Explainer */}
      <div className="bg-dark-800 rounded-lg border border-dark-600 mb-4 overflow-hidden">
        <button
          onClick={() => setShowBrokerInfo(!showBrokerInfo)}
          className="w-full flex items-center justify-between px-4 py-2.5 hover:bg-dark-700/50 transition-colors"
        >
          <div className="flex items-center gap-2">
            <span className="text-[10px] px-1.5 py-0.5 rounded bg-accent-500/20 text-accent-400 border border-accent-500/30 font-medium">INFO</span>
            <span className="text-xs text-gray-400">Wie funktioniert der Live Trader?</span>
          </div>
          <svg className={`w-4 h-4 text-gray-500 transition-transform ${showBrokerInfo ? 'rotate-180' : ''}`} fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 9l-7 7-7-7" />
          </svg>
        </button>
        {showBrokerInfo && (
          <div className="px-4 pb-4 border-t border-dark-600 space-y-3 mt-3">
            <div className="bg-dark-700/50 rounded-lg p-3">
              <div className="text-[10px] text-accent-400 uppercase tracking-wider font-medium mb-1.5">Algorithmische Signalgenerierung</div>
              <p className="text-[11px] text-gray-300 leading-relaxed">
                Der Live Trader scannt in definierten Intervallen das konfigurierte Aktienuniversum via OHLCV-Datenfeed.
                Pro Candle-Close wird die gewählte Strategie (z.B. Regression Scalping, NW Bollinger Bands) auf die Price-Action angewendet.
                Entry- und Exit-Signale werden regelbasiert generiert — inklusive dynamischem Trailing Stop Loss (TSL) und optionalem Take-Profit-Level.
                Die Positionsgröße wird automatisch via Fixed-Fractional-Sizing berechnet (EUR/USD-Konvertierung in Echtzeit).
              </p>
            </div>
            {alpacaEnabled && (
              <div className="bg-dark-700/50 rounded-lg p-3">
                <div className="text-[10px] text-purple-400 uppercase tracking-wider font-medium mb-1.5">Broker-Execution via Alpaca Securities</div>
                <p className="text-[11px] text-gray-300 leading-relaxed">
                  Order-Routing über die Alpaca Securities LLC API (FINRA/SIPC-reguliert) — Direct Market Access an NYSE, NASDAQ und AMEX.
                  <span className="text-green-400 font-medium"> Zero-Commission</span> auf alle US-Equity- und ETF-Trades.
                  Regulatorische Pass-Through-Gebühren (nur bei Sell-Orders): FINRA TAF $0.000166/Share (Cap $8.30), SEC Fee ~$0.00/Mio, CAT Fee $0.0000265/Share.
                  Bei typischen Positionsgrößen von 100–500€ liegen die effektiven Gesamtkosten pro Roundturn unter $0.02.
                </p>
              </div>
            )}
            <div className="bg-dark-700/50 rounded-lg p-3">
              <div className="text-[10px] text-green-400 uppercase tracking-wider font-medium mb-1.5">Steuerlicher Vorteil: Steuerstundungseffekt</div>
              <p className="text-[11px] text-gray-300 leading-relaxed">
                Durch die Ausführung über einen US-Broker findet <span className="text-white font-medium">kein automatischer Kapitalertragssteuer-Abzug</span> auf realisierte Gewinne statt (keine Abgeltungssteuer an der Quelle).
                Gewinne werden erst mit der Einkommensteuererklärung im Folgejahr fällig — das freigesetzte Kapital steht damit ganzjährig als Compound-Basis zur Verfügung.
                Dieser Steuerstundungseffekt potenziert den Zinseszins: Reinvestierte Gewinne generieren über das gesamte Steuerjahr hinweg zusätzliche Rendite,
                bevor die Steuerlast realisiert wird. In Kombination mit kommissionsfreier Execution ist dies die optimale Kostenstruktur für hochfrequentes Intraday-Trading.
              </p>
            </div>
          </div>
        )}
      </div>

      {/* Session Header + Switcher */}
          <div className="bg-dark-800 rounded-lg border border-dark-600 p-4 mb-4">
            {/* Session Name + Switcher */}
            <div className="flex items-center justify-between mb-3 pb-3 border-b border-dark-700">
              <div className="flex items-center gap-3">
                <span className="text-xs text-gray-500">Session:</span>
                <span className="text-sm font-medium text-white">{currentSession?.name || `#${urlSessionId}`}</span>
                {isSessionActive && <span className="text-[10px] px-1.5 py-0.5 bg-green-500/20 text-green-400 border border-green-500/30 rounded">AKTIV</span>}
                {currentSession && !isSessionActive && <span className="text-[10px] px-1.5 py-0.5 bg-gray-500/20 text-gray-400 border border-gray-500/30 rounded">INAKTIV</span>}
              </div>
              {sessions.length > 1 && (
                <select
                  value={urlSessionId || ''}
                  onChange={(e) => navigate(`/live-trading/${e.target.value}`)}
                  className="bg-dark-700 border border-dark-500 rounded px-2 py-1 text-xs text-white focus:border-accent-500 focus:outline-none"
                >
                  {sessions.map(s => (
                    <option key={s.id} value={s.id}>{s.name} {s.is_active ? '(Aktiv)' : ''}</option>
                  ))}
                </select>
              )}
            </div>

            {/* Status + Go Live / Stop */}
            <div className="flex items-center justify-between mb-3">
              <div className="flex items-center gap-3">
                <span className={`w-3 h-3 rounded-full ${isSessionActive ? 'bg-green-400 animate-pulse' : 'bg-gray-600'}`} />
                <span className={`text-sm font-medium ${isSessionActive ? 'text-green-400' : 'text-gray-400'}`}>
                  {isSessionActive ? 'Live Trading Aktiv' : 'Inaktiv'}
                </span>
                {status?.is_polling && isSessionActive && (
                  <span className="text-xs text-accent-400 animate-pulse">Aktualisiere...</span>
                )}
                {isSessionActive && (
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
                  onClick={() => isSessionActive ? stopLive(urlSessionId) : goLive()}
                  className={`px-5 py-2 rounded-lg text-sm font-medium transition-all ${
                    isSessionActive
                      ? 'bg-red-600 hover:bg-red-500 text-white'
                      : 'bg-gradient-to-r from-green-600 to-emerald-600 hover:from-green-500 hover:to-emerald-500 text-white'
                  }`}
                >
                  {isSessionActive ? 'Stop Live' : 'Go Live'}
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

        {status && status.market_open === false && (
          <MarketClosedBanner />
        )}

        {isSessionActive && status && (() => {
          const sess = status.active_sessions?.find(s => String(s.session_id) === String(urlSessionId)) || status
          const isWS = sess.mode === 'websocket'
          return <>
            {/* WebSocket / Polling mode badge */}
            {isWS && (
              <div className="flex items-center gap-2 mb-2">
                <span className={`inline-flex items-center gap-1.5 px-2.5 py-1 rounded text-xs font-medium ${
                  sess.ws_connected
                    ? 'bg-green-500/15 text-green-400 border border-green-500/30'
                    : sess.last_bar_received
                      ? 'bg-yellow-500/15 text-yellow-400 border border-yellow-500/30'
                      : 'bg-blue-500/15 text-blue-400 border border-blue-500/30'
                }`}>
                  <span className={`w-2 h-2 rounded-full ${sess.ws_connected ? 'bg-green-400' : sess.last_bar_received ? 'bg-yellow-400 animate-pulse' : 'bg-blue-400 animate-pulse'}`} />
                  {sess.ws_connected ? 'WebSocket (IEX) aktiv' : sess.last_bar_received ? 'Reconnecting...' : 'Verbinde...'}
                </span>
                {sess.last_bar_received && (
                  <span className="text-xs text-gray-500">
                    Letzte Bar: {formatTime(sess.last_bar_received)}
                  </span>
                )}
              </div>
            )}
            <div className="grid grid-cols-2 md:grid-cols-5 gap-2 text-xs">
              {[
                { label: 'Interval', value: sess.interval },
                { label: isWS ? 'Modus' : 'Letzter Poll', value: isWS ? 'WebSocket' : formatTime(sess.last_poll_at) },
                { label: isWS ? 'Letzte Bar' : 'Nächster Poll', value: isWS
                  ? (sess.last_bar_received ? formatTime(sess.last_bar_received) : '-')
                  : <CountdownDisplay nextPollAt={sess.next_poll_at} fallback={formatTime(sess.next_poll_at)} />
                },
                { label: 'Session Start', value: formatTime(sess.started_at) },
                { label: isWS ? 'Verbindung' : 'Polls', value: isWS ? (sess.ws_connected ? 'Verbunden' : sess.last_bar_received ? 'Getrennt' : 'Verbinde...') : sess.total_polls },
              ].map((item, i) => (
                <div key={i} className="bg-dark-700 rounded p-2">
                  <div className="text-gray-500">{item.label}</div>
                  <div className="text-white font-medium">{item.value}</div>
                </div>
              ))}
            </div>
            {!isWS && sess.is_polling && (
              <div className="mt-2">
                <div className="flex items-center justify-between text-xs mb-1">
                  <span className="text-accent-400 animate-pulse">
                    Prüfe: {sess.current_symbol || '...'} ({sess.scan_progress_current || 0}/{sess.scan_progress_total || 0})
                  </span>
                </div>
                <div className="h-1.5 bg-dark-600 rounded-full overflow-hidden">
                  <div
                    className="h-full bg-accent-500 rounded-full transition-all duration-500"
                    style={{ width: sess.scan_progress_total > 0 ? `${(sess.scan_progress_current / sess.scan_progress_total) * 100}%` : '0%' }}
                  />
                </div>
              </div>
            )}
          </>
        })()}
          </div>

      {/* Strategies Panel */}
      {strategies.length > 0 && (
        <div className="bg-dark-800 rounded-lg border border-dark-600 mb-4">
          <button
            onClick={() => setShowStrategies(!showStrategies)}
            className="w-full flex items-center justify-between p-4 text-sm hover:bg-dark-700 transition-colors rounded-lg"
          >
            <div className="flex items-center gap-2">
              <span className="text-gray-300 font-medium">Strategien</span>
              <span className="px-2 py-0.5 rounded text-xs bg-accent-500/20 text-accent-400">{strategies.length}</span>
              <span className="text-xs text-gray-500">{strategies.filter(s => s.is_enabled).length} aktiv</span>
            </div>
            <svg className={`w-4 h-4 text-gray-500 transition-transform ${showStrategies ? 'rotate-180' : ''}`} fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 9l-7 7-7-7" />
            </svg>
          </button>
          {showStrategies && (
            <div className="px-4 pb-4 space-y-2">
              {strategies.map(s => {
                let stratSymbols = []
                try { stratSymbols = JSON.parse(s.symbols || '[]') } catch {}
                return (
                  <div key={s.id} className="flex items-center justify-between bg-dark-700 rounded-lg p-3">
                    <div className="flex items-center gap-3">
                      <span className={`px-2 py-0.5 rounded text-[10px] font-bold ${
                        s.is_enabled ? 'bg-green-500/20 text-green-400' : 'bg-gray-700 text-gray-500'
                      }`}>
                        {s.is_enabled ? 'AKTIV' : 'INAKTIV'}
                      </span>
                      <div>
                        <span className="text-sm text-white font-medium">{STRATEGY_LABELS[s.name] || s.name}</span>
                        <div className="text-[10px] text-gray-500">
                          {stratSymbols.length} Symbole | {s.long_only ? 'Long Only' : 'Long+Short'} | {new Date(s.created_at).toLocaleDateString('de-DE')}
                        </div>
                      </div>
                    </div>
                    {!isSessionActive && (isAdmin ? (
                      <button
                        onClick={() => toggleStrategy(s.id)}
                        className={`px-3 py-1 rounded text-xs font-medium transition-colors ${
                          s.is_enabled
                            ? 'bg-red-500/20 text-red-400 hover:bg-red-500/30'
                            : 'bg-green-500/20 text-green-400 hover:bg-green-500/30'
                        }`}
                      >
                        {s.is_enabled ? 'Deaktivieren' : 'Aktivieren'}
                      </button>
                    ) : (
                      <span className="flex items-center gap-1 px-2 py-1 text-[10px] text-gray-500 cursor-not-allowed">
                        <svg className="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z" /></svg>
                        <span className="px-1 py-0.5 bg-amber-500/20 text-amber-400 font-bold rounded">PRO</span>
                      </span>
                    ))}
                  </div>
                )
              })}
              {isSessionActive && isAdmin && (
                <p className="text-[10px] text-gray-600 mt-1">Strategien nur bei gestoppter Session aktivieren/deaktivieren.</p>
              )}
            </div>
          )}
        </div>
      )}

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
                <label className="text-xs text-gray-500 block mb-1">Betrag pro Trade (USD)</label>
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
                    ? `Verbunden — Status: ${alpacaValidation.status} | Kaufkraft: $${numFmtDE0.format(Number(alpacaValidation.buying_power))} | ${alpacaValidation.paper ? 'Paper Trading' : 'LIVE'}`
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
                      const configId = config?.id
                      const saveUrl = configId ? `/api/trading/live/config?config_id=${configId}` : '/api/trading/live/config'
                      await fetch(saveUrl, {
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
                      // Re-fetch config + portfolio after save (short delay for DB consistency)
                      if (urlSessionId) {
                        fetchConfig(urlSessionId)
                        setTimeout(() => fetchAlpacaPortfolio(urlSessionId), 500)
                      }
                    } catch { alert('Speichern fehlgeschlagen') }
                  }}
                  className="px-4 py-1.5 text-xs bg-dark-600 hover:bg-dark-500 text-white rounded transition-colors"
                >
                  Speichern
                </button>
              </div>
              {alpacaEnabled && alpacaPaper && (
                <TestOrderPanel headers={headers} tradeAmount={tradeAmount} onOrderPlaced={() => {
                  if (urlSessionId) fetchAlpacaPortfolio(urlSessionId)
                  if (urlSessionId) fetchPositions(urlSessionId)
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
              {(() => {
                const sess = status?.active_sessions?.find(s => s.alpaca_active != null)
                const alpacaOk = sess?.alpaca_active
                const lastChecked = sess?.alpaca_last_checked
                const acctStatus = alpacaPortfolio.account.status
                const isActive = acctStatus === 'ACTIVE'
                // If we have session-level info, use it; otherwise fall back to account status
                if (sess) {
                  const neverChecked = !lastChecked
                  return <>
                    <span className={`text-[10px] px-1.5 py-0.5 rounded ${
                      alpacaOk ? 'bg-green-500/20 text-green-400'
                        : neverChecked ? 'bg-blue-500/20 text-blue-400'
                        : 'bg-red-500/20 text-red-400'
                    }`}>
                      {alpacaOk ? 'CONNECTED' : neverChecked ? 'Pruefe...' : sess.alpaca_error || 'CONNECTION LOST'}
                    </span>
                    {lastChecked && (
                      <span className="text-[9px] text-gray-600" title={sess.alpaca_error || ''}>
                        Check: {timeFmt.format(new Date(lastChecked))}
                      </span>
                    )}
                  </>
                }
                return <span className={`text-[10px] px-1.5 py-0.5 rounded ${isActive ? 'bg-green-500/20 text-green-400' : 'bg-red-500/20 text-red-400'}`}>
                  {isActive ? 'ACTIVE' : acctStatus}
                </span>
              })()}
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
          {(() => {
            const ap = alpacaPortfolio.positions || []
            const totalInvested = ap.reduce((s, p) => s + (p.cost_basis || p.qty * p.avg_entry_price), 0)
            const dayChange = alpacaPortfolio.account.day_change
            const dayChangePct = alpacaPortfolio.account.day_change_pct
            const dayColor = dayChange >= 0 ? 'text-green-400' : 'text-red-400'
            return (
              <div className="grid grid-cols-2 md:grid-cols-4 gap-2 mb-4">
                {[
                  { label: 'Gesamtwert', value: `$${numFmtDE.format(alpacaPortfolio.account.equity)}`, color: 'text-white' },
                  { label: 'Investiert', value: `$${numFmtDE.format(totalInvested)}`, color: 'text-accent-400' },
                  { label: 'Verfügbares Cash', value: `$${numFmtDE.format(alpacaPortfolio.account.cash)}`, color: 'text-gray-300' },
                  { label: 'Tagesänderung', value: `${dayChange >= 0 ? '+' : ''}$${numFmtDE.format(dayChange)} (${dayChangePct >= 0 ? '+' : ''}${dayChangePct.toFixed(2)}%)`, color: dayColor },
                ].map((item, i) => (
                  <div key={i} className="bg-dark-700 rounded-lg p-3">
                    <div className="text-[10px] text-gray-500 uppercase tracking-wider">{item.label}</div>
                    <div className={`text-sm font-bold mt-1 ${item.color}`}>{item.value}</div>
                  </div>
                ))}
              </div>
            )
          })()}

          {/* Alpaca Performance */}
          {(() => {
            const ap = alpacaPortfolio.positions || []
            const apTotalInvested = ap.reduce((s, p) => s + (p.cost_basis || p.qty * p.avg_entry_price), 0)
            const apTotalPnl = ap.reduce((s, p) => s + p.unrealized_pl, 0)
            const realizedPL = alpacaPortfolio.account.realized_pl || 0
            const realizedInvested = alpacaPortfolio.account.realized_invested || 0
            const realizedCount = alpacaPortfolio.account.realized_count || 0
            const totalPnl = apTotalPnl + realizedPL
            const totalInvested = apTotalInvested + realizedInvested
            const gesamtRenditePct = totalInvested > 0 ? (totalPnl / totalInvested) * 100 : 0
            const portfolioPct = apTotalInvested > 0 ? (apTotalPnl / apTotalInvested) * 100 : 0
            const apWins = ap.filter(p => p.unrealized_pl > 0).length
            const apLosses = ap.filter(p => p.unrealized_pl <= 0).length
            const totalTrades = ap.length + realizedCount
            const apWinRate = ap.length > 0 ? (apWins / ap.length) * 100 : 0
            const apAvgReturn = ap.length > 0 ? ap.reduce((s, p) => s + p.unrealized_pl_pct, 0) / ap.length : 0
            const apWinPos = ap.filter(p => p.unrealized_pl_pct > 0)
            const apLosePos = ap.filter(p => p.unrealized_pl_pct <= 0)
            const apAvgWin = apWinPos.length > 0 ? apWinPos.reduce((s, p) => s + p.unrealized_pl_pct, 0) / apWinPos.length : 0
            const apAvgLoss = apLosePos.length > 0 ? apLosePos.reduce((s, p) => s + p.unrealized_pl_pct, 0) / apLosePos.length : 0
            const apRR = apAvgLoss !== 0 ? Math.abs(apAvgWin / apAvgLoss) : 0
            return (
              <div className="grid grid-cols-3 md:grid-cols-8 gap-2 mb-4">
                {[
                  { label: 'Gesamt-Rendite', value: `${gesamtRenditePct >= 0 ? '+' : ''}${gesamtRenditePct.toFixed(2)}%`, sub: `${totalPnl >= 0 ? '+' : ''}$${totalPnl.toFixed(2)}`, color: totalPnl >= 0 ? 'text-green-400' : 'text-red-400' },
                  { label: 'Portfolio', value: `${portfolioPct >= 0 ? '+' : ''}${portfolioPct.toFixed(2)}%`, sub: `${apTotalPnl >= 0 ? '+' : ''}$${apTotalPnl.toFixed(2)} unreal.`, color: apTotalPnl >= 0 ? 'text-green-400' : 'text-red-400' },
                  { label: 'Realisiert', value: `${realizedPL >= 0 ? '+' : ''}$${realizedPL.toFixed(2)}`, sub: `${realizedCount} Trades`, color: realizedPL >= 0 ? 'text-green-400' : 'text-red-400' },
                  { label: 'Trades', value: `${totalTrades}`, sub: `${ap.length} offen`, color: 'text-white' },
                  { label: 'Win Rate', value: `${apWinRate.toFixed(0)}%`, sub: `${apWins}W / ${apLosses}L`, color: apWinRate >= 50 ? 'text-green-400' : 'text-red-400' },
                  { label: 'R/R', value: apRR > 0 ? apRR.toFixed(2) : '-', color: apRR >= 1 ? 'text-green-400' : apRR > 0 ? 'text-red-400' : 'text-gray-400' },
                  { label: 'Ø Win', value: apAvgWin > 0 ? `+${apAvgWin.toFixed(2)}%` : '-', color: 'text-green-400' },
                  { label: 'Ø Loss', value: apAvgLoss < 0 ? `${apAvgLoss.toFixed(2)}%` : '-', color: 'text-red-400' },
                ].map((item, i) => (
                  <div key={i} className="bg-dark-700/50 rounded-lg p-2 text-center">
                    <div className="text-[9px] text-gray-500 uppercase tracking-wider">{item.label}</div>
                    <div className={`text-xs font-bold mt-0.5 ${item.color}`}>{item.value}</div>
                    {item.sub && <div className={`text-[9px] mt-0.5 ${item.color} opacity-70`}>{item.sub}</div>}
                  </div>
                ))}
              </div>
            )
          })()}

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
                        <span className="text-sm font-bold text-accent-400">{p.name || p.symbol}</span>
                        <span className="text-[10px] text-gray-500">{p.symbol}</span>
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
                      <div><span className="text-gray-500">Rendite:</span> <span className={p.unrealized_pl >= 0 ? 'text-green-400' : 'text-red-400'}>{p.unrealized_pl >= 0 ? '+' : ''}${p.unrealized_pl.toFixed(2)} ({p.unrealized_pl_pct >= 0 ? '+' : ''}{p.unrealized_pl_pct.toFixed(2)}%)</span></div>
                    </div>
                  </div>
                ))}
              </div>

              {/* Desktop Table */}
              <div className="hidden md:block overflow-x-auto">
                <table className="w-full text-xs">
                  <thead>
                    <tr className="text-gray-500 text-left">
                      <th className="pb-2 pr-3">Name</th>
                      <th className="pb-2 pr-3">Seite</th>
                      <th className="pb-2 pr-3 text-right">Stück</th>
                      <th className="pb-2 pr-3 text-right">Einstieg</th>
                      <th className="pb-2 pr-3 text-right">Aktuell</th>
                      <th className="pb-2 pr-3 text-right cursor-pointer select-none hover:text-gray-300 transition-colors" onClick={() => toggleAlpacaSort('market_value')}>
                        Marktwert {alpacaPosSort.field === 'market_value' ? (alpacaPosSort.dir === 'desc' ? '↓' : '↑') : ''}
                      </th>
                      <th className="pb-2 text-right cursor-pointer select-none hover:text-gray-300 transition-colors" onClick={() => toggleAlpacaSort('rendite')}>
                        Rendite {alpacaPosSort.field === 'rendite' ? (alpacaPosSort.dir === 'desc' ? '↓' : '↑') : ''}
                      </th>
                    </tr>
                  </thead>
                  <tbody>
                    {sortedAlpacaPositions.map((p, i) => (
                      <tr key={i} className="border-t border-dark-600/50">
                        <td className="py-2 pr-3 font-medium text-accent-400">{p.name || p.symbol} <span className="text-[10px] text-gray-500 ml-1">{p.symbol}</span></td>
                        <td className={`py-2 pr-3 font-medium ${p.side === 'long' ? 'text-green-400' : 'text-red-400'}`}>{p.side.toUpperCase()}</td>
                        <td className="py-2 pr-3 text-right text-gray-300">{p.qty}</td>
                        <td className="py-2 pr-3 text-right text-gray-400">${p.avg_entry_price.toFixed(2)}</td>
                        <td className="py-2 pr-3 text-right text-gray-300">${p.current_price.toFixed(2)}</td>
                        <td className="py-2 pr-3 text-right text-gray-300">${numFmtDE.format(p.market_value)}</td>
                        <td className={`py-2 text-right font-medium ${p.unrealized_pl >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                          {p.unrealized_pl >= 0 ? '+' : ''}${p.unrealized_pl.toFixed(2)} ({p.unrealized_pl_pct >= 0 ? '+' : ''}{p.unrealized_pl_pct.toFixed(2)}%)
                        </td>
                      </tr>
                    ))}
                  </tbody>
                  <tfoot>
                    {(() => {
                      const totalMV = alpacaPortfolio.positions.reduce((s, p) => s + p.market_value, 0)
                      const totalPnl = alpacaPortfolio.positions.reduce((s, p) => s + p.unrealized_pl, 0)
                      const totalCost = alpacaPortfolio.positions.reduce((s, p) => s + (p.cost_basis || p.qty * p.avg_entry_price), 0)
                      const totalPct = totalCost > 0 ? (totalPnl / totalCost) * 100 : 0
                      const pnlColor = totalPnl >= 0 ? 'text-green-400' : 'text-red-400'
                      return (
                        <tr className="border-t border-dark-500">
                          <td colSpan={5} className="py-2 pr-3 text-gray-400 font-medium">Gesamt</td>
                          <td className="py-2 pr-3 text-right text-white font-medium">
                            ${numFmtDE.format(totalMV)}
                          </td>
                          <td className={`py-2 text-right font-bold ${pnlColor}`}>
                            {totalPnl >= 0 ? '+' : ''}${totalPnl.toFixed(2)} ({totalPct >= 0 ? '+' : ''}{totalPct.toFixed(2)}%)
                          </td>
                        </tr>
                      )
                    })()}
                  </tfoot>
                </table>
              </div>
            </div>
          )}

          {alpacaPortfolio.positions.length === 0 && (
            <div className="text-center py-4 text-gray-600 text-sm">Keine offenen Positionen</div>
          )}

          {/* Trade History */}
          {alpacaPortfolio.orders.length > 0 && (() => {
            const allOrders = alpacaPortfolio.orders.filter(o => o.status === 'filled' || o.status === 'partially_filled')
            const filtered = ordersSearch
              ? allOrders.filter(o => o.symbol?.toLowerCase().includes(ordersSearch.toLowerCase()) || o.name?.toLowerCase().includes(ordersSearch.toLowerCase()))
              : allOrders
            const perPage = 20
            const totalPages = Math.max(1, Math.ceil(filtered.length / perPage))
            const page = Math.min(ordersPage, totalPages)
            const paged = filtered.slice((page - 1) * perPage, page * perPage)

            const getSL = (o) => {
              if (o.stop_price > 0) return o.stop_price
              const leg = o.legs?.find(l => l.type === 'stop')
              return leg?.stop_price > 0 ? leg.stop_price : 0
            }
            const getTP = (o) => {
              if (o.limit_price > 0) return o.limit_price
              const leg = o.legs?.find(l => l.type === 'limit')
              return leg?.limit_price > 0 ? leg.limit_price : 0
            }
            const fmtDate = (o) => {
              const d = o.filled_at || o.created_at
              return d ? dateShortFmt.format(new Date(d)) : '-'
            }

            return (
              <div>
                <div className="flex items-center justify-between mb-2">
                  <button onClick={() => setShowAlpacaOrders(!showAlpacaOrders)} className="flex items-center gap-1 text-xs text-gray-500 hover:text-gray-300 transition-colors">
                    <svg className={`w-3 h-3 transition-transform ${showAlpacaOrders ? 'rotate-180' : ''}`} fill="none" viewBox="0 0 24 24" stroke="currentColor">
                      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 9l-7 7-7-7" />
                    </svg>
                    Trade History ({allOrders.length})
                  </button>
                  {showAlpacaOrders && (
                    <input
                      type="text"
                      placeholder="Ticker suchen..."
                      value={ordersSearch}
                      onChange={e => { setOrdersSearch(e.target.value); setOrdersPage(1) }}
                      className="bg-dark-700 border border-dark-600 rounded px-2 py-1 text-xs text-gray-300 w-32 focus:outline-none focus:border-accent-500"
                    />
                  )}
                </div>
                {showAlpacaOrders && (
                  <>
                    {/* Mobile Cards */}
                    <div className="md:hidden space-y-2 mb-2">
                      {paged.map((o, i) => {
                        const sl = getSL(o); const tp = getTP(o)
                        const plColor = o.trade_pl >= 0 ? 'text-green-400' : 'text-red-400'
                        return (
                          <div key={i} className="bg-dark-700 rounded-lg p-3">
                            <div className="flex items-center justify-between mb-1">
                              <div className="flex items-center gap-2">
                                <span className={`text-[10px] font-bold px-1.5 py-0.5 rounded ${o.side === 'buy' ? 'bg-green-500/20 text-green-400' : 'bg-red-500/20 text-red-400'}`}>{o.side?.toUpperCase()}</span>
                                <span className="text-sm font-bold text-accent-400">{o.name || o.symbol}</span>
                                <span className="text-[10px] text-gray-500">{o.symbol}</span>
                              </div>
                              <span className="text-[10px] text-gray-500">{fmtDate(o)}</span>
                            </div>
                            <div className="grid grid-cols-3 gap-2 text-[10px]">
                              <div><span className="text-gray-500">Stück:</span> <span className="text-gray-300">{o.filled_qty || o.qty}</span></div>
                              <div><span className="text-gray-500">Kurs:</span> <span className="text-gray-300">${o.filled_avg_price > 0 ? o.filled_avg_price.toFixed(2) : '-'}</span></div>
                              <div><span className="text-gray-500">Investiert:</span> <span className="text-gray-300">${o.invested > 0 ? o.invested.toFixed(2) : '-'}</span></div>
                              {sl > 0 && <div><span className="text-gray-500">SL:</span> <span className="text-red-400/70">${sl.toFixed(2)}</span></div>}
                              {tp > 0 && <div><span className="text-gray-500">TP:</span> <span className="text-green-400/70">${tp.toFixed(2)}</span></div>}
                              {o.side === 'sell' && o.trade_pl !== 0 && <div><span className="text-gray-500">Rendite:</span> <span className={plColor}>{o.trade_pl >= 0 ? '+' : ''}${o.trade_pl.toFixed(2)} ({o.trade_pl_pct >= 0 ? '+' : ''}{o.trade_pl_pct.toFixed(2)}%)</span></div>}
                            </div>
                          </div>
                        )
                      })}
                    </div>

                    {/* Desktop Table */}
                    <div className="hidden md:block overflow-x-auto mb-2">
                      <table className="w-full text-xs">
                        <thead>
                          <tr className="text-gray-500 text-left">
                            <th className="pb-2 pr-2">Datum</th>
                            <th className="pb-2 pr-2">Name</th>
                            <th className="pb-2 pr-2">Seite</th>
                            <th className="pb-2 pr-2 text-right">Stück</th>
                            <th className="pb-2 pr-2 text-right">Kurs</th>
                            <th className="pb-2 pr-2 text-right">Investiert</th>
                            <th className="pb-2 pr-2 text-right">SL</th>
                            <th className="pb-2 pr-2 text-right">TP</th>
                            <th className="pb-2 pr-2 text-right">Rendite</th>
                            <th className="pb-2 text-left">Status</th>
                          </tr>
                        </thead>
                        <tbody>
                          {paged.map((o, i) => {
                            const sl = getSL(o); const tp = getTP(o)
                            const plColor = o.trade_pl >= 0 ? 'text-green-400' : 'text-red-400'
                            const hasPL = o.side === 'sell' && (o.trade_pl !== 0 || o.trade_pl_pct !== 0)
                            return (
                              <tr key={i} className="border-t border-dark-700/50">
                                <td className="py-1.5 pr-2 text-gray-500 whitespace-nowrap">{fmtDate(o)}</td>
                                <td className="py-1.5 pr-2 text-accent-400 font-medium">{o.name || o.symbol} <span className="text-[10px] text-gray-600 ml-1">{o.symbol}</span></td>
                                <td className={`py-1.5 pr-2 font-medium ${o.side === 'buy' ? 'text-green-400' : 'text-red-400'}`}>{o.side?.toUpperCase()}</td>
                                <td className="py-1.5 pr-2 text-right text-gray-300">{o.filled_qty || o.qty}</td>
                                <td className="py-1.5 pr-2 text-right text-gray-400">{o.filled_avg_price > 0 ? `$${o.filled_avg_price.toFixed(2)}` : '-'}</td>
                                <td className="py-1.5 pr-2 text-right text-gray-300">{o.invested > 0 ? `$${o.invested.toFixed(2)}` : '-'}</td>
                                <td className="py-1.5 pr-2 text-right text-red-400/60">{sl > 0 ? `$${sl.toFixed(2)}` : '-'}</td>
                                <td className="py-1.5 pr-2 text-right text-green-400/60">{tp > 0 ? `$${tp.toFixed(2)}` : '-'}</td>
                                <td className={`py-1.5 pr-2 text-right font-medium ${hasPL ? plColor : 'text-gray-600'}`}>
                                  {hasPL ? `${o.trade_pl >= 0 ? '+' : ''}$${o.trade_pl.toFixed(2)} (${o.trade_pl_pct >= 0 ? '+' : ''}${o.trade_pl_pct.toFixed(2)}%)` : '-'}
                                </td>
                                <td className={`py-1.5 ${o.status === 'filled' ? 'text-green-400' : o.status === 'canceled' || o.status === 'cancelled' ? 'text-gray-600' : 'text-amber-400'}`}>{o.status}</td>
                              </tr>
                            )
                          })}
                        </tbody>
                      </table>
                    </div>

                    {/* Pagination */}
                    {totalPages > 1 && (
                      <div className="flex items-center justify-center gap-3 text-xs text-gray-500">
                        <button onClick={() => setOrdersPage(Math.max(1, page - 1))} disabled={page <= 1} className="px-2 py-1 rounded bg-dark-700 hover:bg-dark-600 disabled:opacity-30 disabled:cursor-not-allowed">Zurück</button>
                        <span>Seite {page} / {totalPages}</span>
                        <button onClick={() => setOrdersPage(Math.min(totalPages, page + 1))} disabled={page >= totalPages} className="px-2 py-1 rounded bg-dark-700 hover:bg-dark-600 disabled:opacity-30 disabled:cursor-not-allowed">Weiter</button>
                      </div>
                    )}
                  </>
                )}
              </div>
            )
          })()}
        </div>
      )}

      {/* Resume Banner */}
      {!status?.is_running && status?.last_session && (
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
          {status.last_session.can_resume && (isAdmin ? (
            <button
              onClick={() => resumeSession(status.last_session.id)}
              className="px-4 py-2 bg-amber-600 hover:bg-amber-500 text-white text-sm font-medium rounded-lg transition-colors"
            >
              Fortsetzen
            </button>
          ) : (
            <span className="flex items-center gap-2 px-4 py-2 bg-dark-700 border border-dark-600 rounded-lg text-sm text-gray-500 cursor-not-allowed">
              <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z" /></svg>
              Fortsetzen
              <span className="px-1.5 py-0.5 bg-amber-500/20 text-amber-400 text-[10px] font-bold rounded">PRO</span>
            </span>
          ))}
        </div>
      )}

      {/* Performance */}
      {(status?.is_running || positions.length > 0) && (
        <div className="bg-dark-800 rounded-lg border border-dark-600 p-4 mb-4">
          <div className="flex items-center justify-between mb-3">
            <h3 className="text-sm font-medium text-white">Performance</h3>
            <div className="flex items-center gap-3 text-[10px] text-gray-500">
              <span>{openPositions.length} offen</span>
              <span>{perfMetrics.totalClosed} geschlossen</span>
              <span>{symbols.length} Aktien</span>
              <span>{STRATEGY_LABELS[config?.strategy] || '-'}</span>
            </div>
          </div>
          <div className="grid grid-cols-2 md:grid-cols-4 lg:grid-cols-8 gap-2">
            {[
              { label: 'Rendite', value: `${perfMetrics.totalRenditePct >= 0 ? '+' : ''}${perfMetrics.totalRenditePct.toFixed(2)}%`, sub: `(${perfMetrics.totalPnlEur >= 0 ? '+' : ''}${perfMetrics.totalPnlEur.toFixed(2)}€)`, subColor: true, color: perfMetrics.totalPnlEur >= 0 ? 'text-green-400' : 'text-red-400' },
              { label: 'Trades', value: `${positions.length}`, sub: `${openPositions.length} offen / ${perfMetrics.totalClosed} closed`, color: 'text-white' },
              { label: 'Win Rate', value: `${perfMetrics.winRate.toFixed(0)}%`, sub: `${perfMetrics.totalWins}W / ${perfMetrics.totalLosses}L`, color: perfMetrics.winRate >= 50 ? 'text-green-400' : 'text-red-400' },
              { label: 'R/R', value: perfMetrics.riskReward > 0 ? perfMetrics.riskReward.toFixed(2) : '-', sub: 'Risk/Reward', color: perfMetrics.riskReward >= 1 ? 'text-green-400' : perfMetrics.riskReward > 0 ? 'text-red-400' : 'text-gray-400' },
              { label: 'Ø / Trade', value: `${perfMetrics.avgReturnPerTrade >= 0 ? '+' : ''}${perfMetrics.avgReturnPerTrade.toFixed(2)}%`, color: perfMetrics.avgReturnPerTrade >= 0 ? 'text-green-400' : 'text-red-400' },
              { label: 'Ø Win', value: perfMetrics.winPositions.length > 0 ? `+${perfMetrics.avgWin.toFixed(2)}%` : '-', color: 'text-green-400' },
              { label: 'Ø Loss', value: perfMetrics.losePositions.length > 0 ? `${perfMetrics.avgLoss.toFixed(2)}%` : '-', color: 'text-red-400' },
              { label: 'Investiert', value: `${perfMetrics.totalInvested.toFixed(0)}€`, color: 'text-white' },
            ].map((m, i) => (
              <div key={i} className="bg-dark-700 rounded-lg p-2.5">
                <div className="text-[10px] text-gray-500">{m.label}{m.label === 'Rendite' && openPositions.length > 0 ? ' (inkl. offen)' : ''}</div>
                <div className={`text-sm font-bold ${m.color || 'text-white'}`}>
                  {m.value}
                  {m.sub && <span className={`ml-1 ${m.subColor ? m.color + ' font-bold' : 'text-[10px] text-gray-500 font-normal'}`}>{m.sub}</span>}
                </div>
              </div>
            ))}
          </div>
        </div>
      )}

      {/* Session Statistiken (zugeklappt) */}
      {(status?.is_running || positions.length > 0) && (() => {
        const allPos = positions
        const closed = allPos.filter(p => p.is_closed)
        const open = allPos.filter(p => !p.is_closed)

        // Session duration — find current session's started_at from active_sessions or sessions list
        const currentSess = status?.active_sessions?.find(s => String(s.session_id) === String(urlSessionId))
        const sessStartRaw = currentSess?.started_at || sessions.find(s => String(s.id) === String(urlSessionId))?.started_at
        const sessStartDate = sessStartRaw ? new Date(sessStartRaw) : null
        // Guard against zero time (Go 0001-01-01)
        const sessionStart = (sessStartDate && sessStartDate.getFullYear() > 2000) ? sessStartDate : (allPos.length > 0 ? new Date(Math.min(...allPos.map(p => new Date(p.entry_time)))) : new Date())
        const sessionDaysRaw = (Date.now() - sessionStart) / 86400000
        const sessionDays = Math.max(1, sessionDaysRaw)
        const sessionWeeks = Math.max(1, sessionDays / 7)
        const sessionMonths = Math.max(1, sessionDays / 30)

        const durationStr = sessionDaysRaw < 1
          ? `${Math.floor(sessionDaysRaw * 24)}h`
          : sessionDaysRaw < 7
            ? `${Math.floor(sessionDaysRaw)} Tag${Math.floor(sessionDaysRaw) !== 1 ? 'e' : ''} ${Math.floor((sessionDaysRaw % 1) * 24)}h`
            : `${Math.floor(sessionDaysRaw)} Tage`

        // Totals
        const sPnlEur = allPos.reduce((s, p) => s + (p.profit_loss_amt || 0), 0)
        const sInvested = allPos.reduce((s, p) => s + (p.invested_amount || 0), 0)
        const sRenditePct = sInvested > 0 ? (sPnlEur / sInvested) * 100 : 0
        const sWins = allPos.filter(p => (p.profit_loss_pct || 0) > 0).length
        const sLosses = allPos.filter(p => (p.profit_loss_pct || 0) <= 0).length
        const sWinRate = allPos.length > 0 ? (sWins / allPos.length) * 100 : 0
        const sAvgReturn = allPos.length > 0 ? allPos.reduce((s, p) => s + (p.profit_loss_pct || 0), 0) / allPos.length : 0
        const sAvgReturnEur = allPos.length > 0 ? sPnlEur / allPos.length : 0
        const sWinPos = allPos.filter(p => (p.profit_loss_pct || 0) > 0)
        const sLosePos = allPos.filter(p => (p.profit_loss_pct || 0) <= 0)
        const sAvgWin = sWinPos.length > 0 ? sWinPos.reduce((s, p) => s + p.profit_loss_pct, 0) / sWinPos.length : 0
        const sAvgLoss = sLosePos.length > 0 ? sLosePos.reduce((s, p) => s + p.profit_loss_pct, 0) / sLosePos.length : 0
        const sRR = sAvgLoss !== 0 ? Math.abs(sAvgWin / sAvgLoss) : 0

        // Open positions value
        const openValue = open.reduce((s, p) => s + (p.invested_amount || 0) + (p.profit_loss_amt || 0), 0)
        const openPnl = open.reduce((s, p) => s + (p.profit_loss_amt || 0), 0)

        // Date helpers
        const todayStart = new Date(); todayStart.setHours(0,0,0,0)
        const todayEnd = new Date(todayStart); todayEnd.setDate(todayEnd.getDate() + 1)
        const yesterdayStart = new Date(todayStart); yesterdayStart.setDate(yesterdayStart.getDate() - 1)

        const getDateKey = (p) => {
          const d = p.close_time ? new Date(p.close_time) : new Date(p.entry_time)
          return d.toISOString().slice(0, 10)
        }

        // Filter: closed trades by close_time, open trades by entry_time
        const todayTrades = closed.filter(p => {
          const t = new Date(p.close_time)
          return t >= todayStart && t < todayEnd
        })
        // Include open trades opened today
        const todayOpen = open.filter(p => new Date(p.entry_time) >= todayStart)
        const todayAll = [...todayTrades, ...todayOpen]
        const todayPnl = todayAll.reduce((s, p) => s + (p.profit_loss_amt || 0), 0)
        const todayInv = todayAll.reduce((s, p) => s + (p.invested_amount || 0), 0)
        const todayPct = todayInv > 0 ? (todayPnl / todayInv) * 100 : 0
        const todayWins = todayAll.filter(p => (p.profit_loss_pct || 0) > 0).length

        const yesterdayTrades = closed.filter(p => {
          const t = new Date(p.close_time)
          return t >= yesterdayStart && t < todayStart
        })
        const yesterdayPnl = yesterdayTrades.reduce((s, p) => s + (p.profit_loss_amt || 0), 0)
        const yesterdayInv = yesterdayTrades.reduce((s, p) => s + (p.invested_amount || 0), 0)
        const yesterdayPct = yesterdayInv > 0 ? (yesterdayPnl / yesterdayInv) * 100 : 0
        const yesterdayWins = yesterdayTrades.filter(p => (p.profit_loss_pct || 0) > 0).length

        // Averages
        const avgPnlDay = sPnlEur / sessionDays
        const avgPnlWeek = sPnlEur / sessionWeeks
        const avgPnlMonth = sPnlEur / sessionMonths
        const avgPctDay = sRenditePct / sessionDays
        const avgPctWeek = sRenditePct / sessionWeeks
        const avgPctMonth = sRenditePct / sessionMonths
        const avgTradesDay = allPos.length / sessionDays
        const avgTradesWeek = allPos.length / sessionWeeks
        const avgTradesMonth = allPos.length / sessionMonths

        // Best/Worst by day (closed trades grouped by close date)
        const dailyPnl = {}
        closed.forEach(p => {
          if (!p.close_time) return
          const key = getDateKey(p)
          if (!dailyPnl[key]) dailyPnl[key] = 0
          dailyPnl[key] += p.profit_loss_amt || 0
        })
        const dailyEntries = Object.entries(dailyPnl)
        const bestDay = dailyEntries.length > 0 ? dailyEntries.reduce((a, b) => b[1] > a[1] ? b : a) : null
        const worstDay = dailyEntries.length > 0 ? dailyEntries.reduce((a, b) => b[1] < a[1] ? b : a) : null

        // Best/Worst single trade
        const bestTrade = allPos.length > 0 ? allPos.reduce((a, b) => (b.profit_loss_pct || 0) > (a.profit_loss_pct || 0) ? b : a) : null
        const worstTrade = allPos.length > 0 ? allPos.reduce((a, b) => (b.profit_loss_pct || 0) < (a.profit_loss_pct || 0) ? b : a) : null

        // Win/Loss streaks
        let maxWinStreak = 0, maxLossStreak = 0, curWin = 0, curLoss = 0
        const sortedClosed = [...closed].sort((a, b) => new Date(a.close_time) - new Date(b.close_time))
        sortedClosed.forEach(p => {
          if ((p.profit_loss_pct || 0) > 0) { curWin++; curLoss = 0; maxWinStreak = Math.max(maxWinStreak, curWin) }
          else { curLoss++; curWin = 0; maxLossStreak = Math.max(maxLossStreak, curLoss) }
        })

        const pf = (v, showSign = true) => `${showSign && v >= 0 ? '+' : ''}${v.toFixed(2)}`

        return (
          <div className="bg-dark-800 rounded-lg border border-dark-600 mb-4">
            <button
              onClick={() => setShowSessionStats(!showSessionStats)}
              className="w-full flex items-center justify-between p-4 text-left"
            >
              <div className="flex items-center gap-2">
                <h3 className="text-sm font-medium text-white">Session Statistiken</h3>
                <span className="text-[10px] text-gray-500">{durationStr} aktiv</span>
              </div>
              <div className="flex items-center gap-3">
                <span className={`text-xs font-bold ${sPnlEur >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                  {pf(sRenditePct)}% ({pf(sPnlEur)}€)
                </span>
                <svg className={`w-4 h-4 text-gray-500 transition-transform ${showSessionStats ? 'rotate-180' : ''}`} fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 9l-7 7-7-7" />
                </svg>
              </div>
            </button>

            {showSessionStats && (
              <div className="px-4 pb-4 space-y-4">

                {/* Gruppe 1: Gesamtübersicht */}
                <div>
                  <div className="text-[10px] text-gray-500 uppercase tracking-wider mb-2">Gesamtübersicht</div>
                  <div className="grid grid-cols-3 md:grid-cols-5 lg:grid-cols-9 gap-1.5">
                    <StatCard label="Rendite" value={`${pf(sRenditePct)}%`} sub={`(${pf(sPnlEur)}€)`} color={sPnlEur >= 0 ? 'text-green-400' : 'text-red-400'} />
                    <StatCard label="Offene Pos." value={`${open.length}`} sub={`(${pf(openPnl)}€)`} color={openPnl >= 0 ? 'text-green-400' : 'text-red-400'} />
                    <StatCard label="Trades" value={`${allPos.length}`} sub={`${sWins}W / ${sLosses}L`} color="text-white" />
                    <StatCard label="Win Rate" value={`${sWinRate.toFixed(0)}%`} color={sWinRate >= 50 ? 'text-green-400' : 'text-red-400'} />
                    <StatCard label="R/R" value={sRR > 0 ? sRR.toFixed(2) : '-'} color={sRR >= 1 ? 'text-green-400' : sRR > 0 ? 'text-red-400' : 'text-gray-400'} />
                    <StatCard label="Ø / Trade" value={`${pf(sAvgReturn)}%`} sub={`(${pf(sAvgReturnEur)}€)`} color={sAvgReturn >= 0 ? 'text-green-400' : 'text-red-400'} />
                    <StatCard label="Ø Win" value={sAvgWin > 0 ? `${pf(sAvgWin)}%` : '-'} color="text-green-400" />
                    <StatCard label="Ø Loss" value={sAvgLoss < 0 ? `${sAvgLoss.toFixed(2)}%` : '-'} color="text-red-400" />
                    <StatCard label="Investiert" value={`${sInvested.toFixed(0)}€`} color="text-gray-300" />
                  </div>
                </div>

                {/* Gruppe 2: Heute / Gestern */}
                <div>
                  <div className="text-[10px] text-gray-500 uppercase tracking-wider mb-2">Zeiträume</div>
                  <div className="grid grid-cols-2 gap-3">
                    <div className="bg-dark-700/30 rounded-lg p-3">
                      <div className="text-[10px] text-gray-400 font-medium mb-2">Heute</div>
                      <div className="grid grid-cols-3 gap-1.5">
                        <StatCard label="Rendite" value={todayAll.length > 0 ? `${pf(todayPct)}%` : '-'} sub={todayAll.length > 0 ? `(${pf(todayPnl)}€)` : null} color={todayPnl >= 0 ? 'text-green-400' : 'text-red-400'} />
                        <StatCard label="Trades" value={`${todayAll.length}`} color="text-white" />
                        <StatCard label="Win Rate" value={todayAll.length > 0 ? `${(todayWins / todayAll.length * 100).toFixed(0)}%` : '-'} color={todayWins / Math.max(1, todayAll.length) >= 0.5 ? 'text-green-400' : 'text-red-400'} />
                      </div>
                    </div>
                    <div className="bg-dark-700/30 rounded-lg p-3">
                      <div className="text-[10px] text-gray-400 font-medium mb-2">Gestern</div>
                      <div className="grid grid-cols-3 gap-1.5">
                        <StatCard label="Rendite" value={yesterdayTrades.length > 0 ? `${pf(yesterdayPct)}%` : '-'} sub={yesterdayTrades.length > 0 ? `(${pf(yesterdayPnl)}€)` : null} color={yesterdayPnl >= 0 ? 'text-green-400' : 'text-red-400'} />
                        <StatCard label="Trades" value={`${yesterdayTrades.length}`} color="text-white" />
                        <StatCard label="Win Rate" value={yesterdayTrades.length > 0 ? `${(yesterdayWins / yesterdayTrades.length * 100).toFixed(0)}%` : '-'} color={yesterdayWins / Math.max(1, yesterdayTrades.length) >= 0.5 ? 'text-green-400' : 'text-red-400'} />
                      </div>
                    </div>
                  </div>
                </div>

                {/* Gruppe 3: Durchschnitte */}
                <div>
                  <div className="text-[10px] text-gray-500 uppercase tracking-wider mb-2">Durchschnitte</div>
                  <div className="grid grid-cols-3 md:grid-cols-6 gap-1.5">
                    <StatCard label="Ø / Tag" value={`${pf(avgPctDay)}%`} sub={`(${pf(avgPnlDay)}€)`} color={avgPnlDay >= 0 ? 'text-green-400' : 'text-red-400'} />
                    <StatCard label="Ø / Woche" value={`${pf(avgPctWeek)}%`} sub={`(${pf(avgPnlWeek)}€)`} color={avgPnlWeek >= 0 ? 'text-green-400' : 'text-red-400'} />
                    <StatCard label="Ø / Monat" value={`${pf(avgPctMonth)}%`} sub={`(${pf(avgPnlMonth)}€)`} color={avgPnlMonth >= 0 ? 'text-green-400' : 'text-red-400'} />
                    <StatCard label="Trades / Tag" value={avgTradesDay.toFixed(1)} color="text-gray-300" />
                    <StatCard label="Trades / Woche" value={avgTradesWeek.toFixed(1)} color="text-gray-300" />
                    <StatCard label="Trades / Monat" value={avgTradesMonth.toFixed(1)} color="text-gray-300" />
                  </div>
                </div>

                {/* Gruppe 4: Bestleistungen */}
                {closed.length > 0 && (
                  <div>
                    <div className="text-[10px] text-gray-500 uppercase tracking-wider mb-2">Highlights</div>
                    <div className="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-6 gap-1.5">
                      <StatCard label="Bester Tag" value={bestDay ? new Date(bestDay[0]).toLocaleDateString('de-DE', { day: '2-digit', month: '2-digit' }) : '-'} sub={bestDay ? `(${pf(bestDay[1])}€)` : null} color="text-green-400" />
                      <StatCard label="Schlechtester Tag" value={worstDay ? new Date(worstDay[0]).toLocaleDateString('de-DE', { day: '2-digit', month: '2-digit' }) : '-'} sub={worstDay ? `(${pf(worstDay[1])}€)` : null} color="text-red-400" />
                      <StatCard label="Bester Trade" value={bestTrade ? bestTrade.symbol : '-'} sub={bestTrade ? `${pf(bestTrade.profit_loss_pct)}% (${pf(bestTrade.profit_loss_amt)}€)` : null} color="text-green-400" />
                      <StatCard label="Schlechtester Trade" value={worstTrade ? worstTrade.symbol : '-'} sub={worstTrade ? `${pf(worstTrade.profit_loss_pct)}% (${pf(worstTrade.profit_loss_amt)}€)` : null} color="text-red-400" />
                      <StatCard label="Win-Serie" value={`${maxWinStreak}`} sub="in Folge" color="text-green-400" />
                      <StatCard label="Loss-Serie" value={`${maxLossStreak}`} sub="in Folge" color="text-red-400" />
                    </div>
                  </div>
                )}
              </div>
            )}
          </div>
        )
      })()}

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
                    {p.symbol.includes('.') && <span className="text-[10px] px-1 py-0.5 rounded bg-amber-500/20 text-amber-400 border border-amber-500/30" title="Nicht über Alpaca handelbar (nur DB-Tracking)">Non-US</span>}
                    {p.strategy_name && <span className="text-[10px] px-1 py-0.5 rounded bg-cyan-500/20 text-cyan-400 border border-cyan-500/30">{p.strategy_name}</span>}
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
                  {strategies.length > 1 && <th className="pb-2 pr-3">Strat</th>}
                  <th className="pb-2 pr-3">Entry</th>
                  <th className="pb-2 pr-3">Aktuell</th>
                  <th className="pb-2 pr-3 text-right">Stk</th>
                  <th className="pb-2 pr-3 text-right">Buy-In</th>
                  <th className="pb-2 pr-3 text-right cursor-pointer select-none hover:text-gray-300 transition-colors" onClick={() => toggleAppSort('marktwert')}>
                    Marktwert {appPosSort.field === 'marktwert' ? (appPosSort.dir === 'desc' ? '↓' : '↑') : ''}
                  </th>
                  <th className="pb-2 pr-3 text-right cursor-pointer select-none hover:text-gray-300 transition-colors" onClick={() => toggleAppSort('rendite')}>
                    Rendite {appPosSort.field === 'rendite' ? (appPosSort.dir === 'desc' ? '↓' : '↑') : ''}
                  </th>
                  <th className="pb-2 text-right">SL / TP</th>
                </tr>
              </thead>
              <tbody>
                {sortedAppPositions.map(p => (
                  <tr key={p.id} className="border-b border-dark-700/50">
                    <td className="py-2 pr-3 font-medium text-accent-400">
                      {p.symbol}
                      {p.alpaca_order_id && <span className="ml-1 text-[10px] px-1 py-0.5 rounded bg-purple-500/20 text-purple-400" title={`Order: ${p.alpaca_order_id}`}>A</span>}
                      {p.symbol.includes('.') && <span className="ml-1 text-[10px] px-1 py-0.5 rounded bg-amber-500/20 text-amber-400" title="Nicht über Alpaca handelbar (nur DB-Tracking)">Non-US</span>}
                    </td>
                    <td className={`py-2 pr-3 font-medium ${p.direction === 'LONG' ? 'text-green-400' : 'text-red-400'}`}>{p.direction}</td>
                    {strategies.length > 1 && (
                      <td className="py-2 pr-3">
                        <span className="text-[10px] px-1.5 py-0.5 rounded bg-cyan-500/15 text-cyan-400">{p.strategy_name || '-'}</span>
                      </td>
                    )}
                    <td className="py-2 pr-3 text-gray-400">
                      <div>{formatPrice(p.entry_price, p.symbol)}</div>
                      <div className="text-gray-600 text-[10px]">{formatTime(p.entry_time)}</div>
                    </td>
                    <td className="py-2 pr-3 text-white font-medium">
                      {formatPrice(p.current_price, p.symbol)}
                    </td>
                    <td className="py-2 pr-3 text-right text-gray-300">{p.quantity ? `${p.quantity}x` : '-'}</td>
                    <td className="py-2 pr-3 text-right text-gray-400">{p.invested_amount?.toFixed(2)} €</td>
                    <td className="py-2 pr-3 text-right text-gray-300">{((p.invested_amount || 0) + (p.profit_loss_amt || 0)).toFixed(2)} €</td>
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

      {/* Symbols Table */}
      {symbols.length > 0 && (
        <div className="bg-dark-800 rounded-lg border border-dark-600 p-4 mb-4">
          <div className="flex items-center justify-between mb-3">
            <h3 className="text-sm font-medium text-white">Aktien ({symbolsTable.getFilteredRowModel().rows.length}{symbolSearch ? ` / ${symbols.length}` : ''})</h3>
            <div className="relative">
              <input
                type="text"
                value={symbolSearch}
                onChange={e => setSymbolSearch(e.target.value)}
                placeholder="Suche..."
                className="w-36 bg-dark-700 border border-dark-500 rounded px-2 py-1 pl-7 text-xs text-white placeholder-gray-500 focus:border-accent-500 focus:outline-none"
              />
              <svg className="absolute left-2 top-1/2 -translate-y-1/2 w-3.5 h-3.5 text-gray-500" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" /></svg>
              {symbolSearch && (
                <button onClick={() => setSymbolSearch('')} className="absolute right-1.5 top-1/2 -translate-y-1/2 text-gray-500 hover:text-white">
                  <svg className="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" /></svg>
                </button>
              )}
            </div>
          </div>
          <div className="max-h-80 overflow-y-auto">
            <table className="w-full text-xs">
              <thead className="sticky top-0 bg-dark-800 z-10">
                {symbolsTable.getHeaderGroups().map(hg => (
                  <tr key={hg.id} className="text-left text-gray-500 border-b border-dark-600">
                    {hg.headers.map(header => (
                      <th
                        key={header.id}
                        className="pb-1 pr-2 cursor-pointer hover:text-white select-none"
                        onClick={header.column.getToggleSortingHandler()}
                        style={{ width: header.getSize() }}
                      >
                        {flexRender(header.column.columnDef.header, header.getContext())}
                        {{ asc: ' ▲', desc: ' ▼' }[header.column.getIsSorted()] ?? ''}
                      </th>
                    ))}
                  </tr>
                ))}
              </thead>
              <tbody>
                {symbolsTable.getRowModel().rows.map(row => (
                  <tr
                    key={row.id}
                    className="border-b border-dark-700/50 last:border-0 cursor-pointer hover:bg-dark-700 transition-colors"
                    onClick={() => openAnalysis(row.original.symbol)}
                  >
                    {row.getVisibleCells().map(cell => (
                      <td key={cell.id} className="py-1 pr-2">
                        {flexRender(cell.column.columnDef.cell, cell.getContext())}
                      </td>
                    ))}
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
          <div className="flex items-center justify-between mb-3 flex-wrap gap-2">
            <h3 className="text-sm font-medium text-white">Trade History ({closedPositions.length})</h3>
            <div className="flex items-center gap-3">
              <div className="flex items-center gap-1">
                <input
                  type="text"
                  placeholder="Symbol suchen..."
                  value={tradeHistorySearch}
                  onChange={e => setTradeHistorySearch(e.target.value)}
                  className="bg-dark-700 border border-dark-600 rounded px-2 py-1 text-xs text-gray-300 w-32 focus:outline-none focus:border-accent-500 placeholder-gray-600"
                />
                {tradeHistorySearch && <button onClick={() => setTradeHistorySearch('')} className="text-[10px] text-gray-500 hover:text-gray-300">✕</button>}
              </div>
              <div className="hidden md:flex items-center gap-3 text-[10px] text-gray-500">
                <span><span className="inline-block w-1.5 h-1.5 rounded-full bg-green-400 mr-1" />TP</span>
                <span><span className="inline-block w-1.5 h-1.5 rounded-full bg-red-400 mr-1" />SL</span>
                <span><span className="inline-block w-1.5 h-1.5 rounded-full bg-yellow-400 mr-1" />SIGNAL</span>
                <span><span className="inline-block w-1.5 h-1.5 rounded-full bg-gray-400 mr-1" />MANUAL</span>
              </div>
            </div>
          </div>
          <div className="overflow-auto max-h-96">
            <table className="w-full text-xs">
              <thead className="sticky top-0 bg-dark-800">
                <tr className="text-left text-gray-500 border-b border-dark-600">
                  <th className="pb-2 pr-2">Symbol</th>
                  <th className="pb-2 pr-2">Dir</th>
                  {strategies.length > 1 && <th className="pb-2 pr-2">Strat</th>}
                  <th className="pb-2 pr-2">Entry</th>
                  <th className="pb-2 pr-2">Exit</th>
                  <th className="pb-2 pr-2 text-right">Stk</th>
                  <th className="pb-2 pr-2 text-right">Rendite</th>
                  <th className="pb-2 text-right">Reason</th>
                </tr>
              </thead>
              <tbody>
                {sortedClosedPositions
                  .filter(p => !tradeHistorySearch || p.symbol?.toLowerCase().includes(tradeHistorySearch.toLowerCase()) || p.close_reason?.toLowerCase().includes(tradeHistorySearch.toLowerCase()) || p.strategy_name?.toLowerCase().includes(tradeHistorySearch.toLowerCase()))
                  .map(p => (
                  <tr key={p.id} className="border-b border-dark-700/50">
                    <td className="py-1.5 pr-2 font-medium text-accent-400">
                      {p.symbol}
                      {p.symbol.includes('.') && <span className="ml-1 text-[10px] px-1 py-0.5 rounded bg-amber-500/20 text-amber-400" title="Nicht über Alpaca handelbar (nur DB-Tracking)">Non-US</span>}
                    </td>
                    <td className={`py-1.5 pr-2 ${p.direction === 'LONG' ? 'text-green-400' : 'text-red-400'}`}>{p.direction}</td>
                    {strategies.length > 1 && (
                      <td className="py-1.5 pr-2">
                        <span className="text-[10px] px-1.5 py-0.5 rounded bg-cyan-500/15 text-cyan-400">{p.strategy_name || '-'}</span>
                      </td>
                    )}
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
      {botActions.length > 0 && (() => {
        const filteredBotActions = botActions.filter(a => !botActionSearch || a.symbol?.toLowerCase().includes(botActionSearch.toLowerCase()) || a.type?.toLowerCase().includes(botActionSearch.toLowerCase()))
        return (
        <div className="bg-dark-800 rounded-lg border border-dark-600 p-4 mb-4">
          <div className="flex items-center justify-between mb-3">
            <h3 className="text-sm font-medium text-white">Letzte Bot Aktionen ({botActions.length})</h3>
            <div className="flex items-center gap-1">
              <input
                type="text"
                placeholder="Symbol suchen..."
                value={botActionSearch}
                onChange={e => setBotActionSearch(e.target.value)}
                className="bg-dark-700 border border-dark-600 rounded px-2 py-1 text-xs text-gray-300 w-32 focus:outline-none focus:border-accent-500 placeholder-gray-600"
              />
              {botActionSearch && <button onClick={() => setBotActionSearch('')} className="text-[10px] text-gray-500 hover:text-gray-300">✕</button>}
              {botActionSearch && <span className="text-[10px] text-gray-600 ml-1">{filteredBotActions.length}/{botActions.length}</span>}
            </div>
          </div>

          {/* Mobile: Cards */}
          <div className="md:hidden space-y-2">
            {filteredBotActions.map((a, i) => (
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
                {filteredBotActions.map((a, i) => (
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
        )
      })()}

      {/* Debug-Log */}
      {urlSessionId && (
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
              WARN: 'text-yellow-500',
              ERROR: 'text-orange-400',
              TRADE: 'text-purple-400',
              ALPACA: 'text-purple-400',
              REFRESH: 'text-cyan-400',
              DEBUG: 'text-teal-400',
              DATA_MISMATCH: 'text-orange-500',
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
              WARN: 'bg-yellow-500/20 border-yellow-500/30',
              ERROR: 'bg-orange-500/20 border-orange-500/30',
              TRADE: 'bg-purple-500/20 border-purple-500/30',
              ALPACA: 'bg-purple-500/20 border-purple-500/30',
              REFRESH: 'bg-cyan-500/20 border-cyan-500/30',
              DEBUG: 'bg-teal-500/20 border-teal-500/30',
              DATA_MISMATCH: 'bg-orange-500/30 border-orange-500/40',
            }
            const allLevels = [...new Set(debugLogs.map(l => l.level))].sort()
            const searchLower = debugSearch.toLowerCase()
            const allStrategies = [...new Set(debugLogs.map(l => l.strategy).filter(Boolean))].sort()
            const filteredLogs = debugLogs.filter(l => {
              if (hiddenLogLevels.has(l.level)) return false
              if (strategyFilter && l.strategy !== strategyFilter) return false
              if (searchLower) {
                const time = dateFmt.format(new Date(l.created_at))
                return l.symbol?.toLowerCase().includes(searchLower) ||
                  l.message?.toLowerCase().includes(searchLower) ||
                  l.level?.toLowerCase().includes(searchLower) ||
                  time.includes(searchLower)
              }
              return true
            })
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
                <div className="flex items-center gap-2 mt-3 mb-2">
                  <input
                    type="text"
                    placeholder="Suche: Ticker, Signal, Zeit..."
                    value={debugSearch}
                    onChange={e => setDebugSearch(e.target.value)}
                    className="bg-dark-700 border border-dark-600 rounded px-2 py-1 text-xs text-gray-300 w-48 focus:outline-none focus:border-accent-500 placeholder-gray-600"
                  />
                  {debugSearch && <button onClick={() => setDebugSearch('')} className="text-[10px] text-gray-500 hover:text-gray-300">x</button>}
                  {allStrategies.length > 1 && (
                    <select
                      value={strategyFilter}
                      onChange={e => setStrategyFilter(e.target.value)}
                      className="bg-dark-700 border border-dark-600 rounded px-2 py-1 text-xs text-gray-300 focus:outline-none focus:border-accent-500"
                    >
                      <option value="">Alle Strategien</option>
                      {allStrategies.map(s => <option key={s} value={s}>{STRATEGY_LABELS[s] || s}</option>)}
                    </select>
                  )}
                  <span className="text-[10px] text-gray-600 ml-auto">{filteredLogs.length}/{debugLogs.length}</span>
                </div>
                {allLevels.length > 0 && (
                  <div className="flex flex-wrap items-center gap-1.5 mb-2">
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
                    const time = timeFmt.format(new Date(log.created_at))
                    return (
                      <div key={log.id} className="flex gap-2 py-0.5 border-b border-dark-700/30">
                        <span className="text-gray-600 shrink-0">{time}</span>
                        <span className={`shrink-0 w-14 text-right ${levelColors[log.level] || 'text-gray-400'}`}>{log.level}</span>
                        <span className="text-gray-500 shrink-0 w-16 text-right">{log.symbol !== '-' ? log.symbol : ''}</span>
                        {log.strategy && <span className="shrink-0 text-cyan-500 text-[10px]">[{log.strategy}]</span>}
                        <span className={log.level === 'DATA_MISMATCH' ? 'text-orange-400 font-bold' : 'text-gray-300'}>{log.message}</span>
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
                          <span
                            className="text-xs font-medium text-white cursor-text"
                            contentEditable={isAdmin}
                            suppressContentEditableWarning
                            onBlur={async (e) => {
                              const newName = e.target.textContent.trim()
                              if (newName && newName !== (s.name || `#${s.id}`)) {
                                try {
                                  await fetch(`/api/trading/live/session/${s.id}/name`, {
                                    method: 'PATCH',
                                    headers: { ...headers, 'Content-Type': 'application/json' },
                                    body: JSON.stringify({ name: newName }),
                                  })
                                  fetchSessions()
                                } catch { /* ignore */ }
                              }
                            }}
                            onKeyDown={(e) => { if (e.key === 'Enter') { e.preventDefault(); e.target.blur() } }}
                            title={isAdmin ? 'Klicken zum Umbenennen' : ''}
                          >
                            {s.name || `#${s.id} ${STRATEGY_LABELS[s.strategy] || s.strategy} (${s.interval})`}
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
                          {!s.is_active && s.can_resume && (isAdmin ? (
                            <button
                              onClick={(e) => { e.stopPropagation(); resumeSession(s.id) }}
                              className="px-2.5 py-1 bg-amber-600 hover:bg-amber-500 text-white text-[10px] font-medium rounded transition-colors"
                            >
                              Fortsetzen
                            </button>
                          ) : (
                            <span className="flex items-center gap-1 px-2 py-1 text-[10px] text-gray-500 cursor-not-allowed">
                              <svg className="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z" /></svg>
                              <span className="px-1 py-0.5 bg-amber-500/20 text-amber-400 font-bold rounded">PRO</span>
                            </span>
                          ))}
                          {s.is_active && (isAdmin ? (
                            <button
                              onClick={(e) => { e.stopPropagation(); stopLive(s.id) }}
                              className="px-2.5 py-1 bg-red-600 hover:bg-red-500 text-white text-[10px] font-medium rounded transition-colors"
                            >
                              Stoppen
                            </button>
                          ) : (
                            <span className="flex items-center gap-1 px-2 py-1 text-[10px] text-gray-500 cursor-not-allowed">
                              <svg className="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z" /></svg>
                              <span className="px-1 py-0.5 bg-amber-500/20 text-amber-400 font-bold rounded">PRO</span>
                            </span>
                          ))}
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
                          {s.total_pnl >= 0 ? '+' : ''}${s.total_pnl?.toFixed(2)}
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
            und drücke "Neue Session starten" um zu beginnen.
          </div>
        </div>
      )}

      {/* Stock Analysis Overlay */}
      {analysisSymbol && (
        <div className="fixed inset-0 z-50 bg-dark-900/95 overflow-auto">
          <div className="sticky top-0 z-10 bg-dark-800 border-b border-dark-600 px-6 py-3 flex items-center justify-between">
            <div className="flex items-center gap-3">
              <h2 className="text-lg font-bold text-white">{analysisSymbol}</h2>
              {analysisData?.strategies?.filter(s => s.is_enabled).length > 1 ? (
                <select
                  value={analysisStrategyId || ''}
                  onChange={e => { const id = Number(e.target.value); setAnalysisStrategyId(id); openAnalysis(analysisSymbol, id) }}
                  className="bg-dark-700 border border-dark-500 rounded px-2 py-1 text-xs text-gray-300 focus:outline-none focus:border-accent-500"
                >
                  {analysisData.strategies.filter(s => s.is_enabled).map(s => (
                    <option key={s.id} value={s.id}>{STRATEGY_LABELS[s.name] || s.name}</option>
                  ))}
                </select>
              ) : (
                <span className="text-xs text-gray-500">
                  {analysisData?.active_strategy?.name ? (STRATEGY_LABELS[analysisData.active_strategy.name] || analysisData.active_strategy.name) : (STRATEGY_LABELS[config?.strategy] || config?.strategy)}
                </span>
              )}
              <span className="text-xs text-gray-600">|</span>
              <span className="text-xs text-gray-500">{config?.interval}</span>
              {analysisData?.comparison?.mismatches > 0 && (
                <span className="px-2 py-0.5 rounded text-[10px] font-bold bg-orange-500/20 border border-orange-500/30 text-orange-400 animate-pulse">
                  {analysisData.comparison.mismatches} MISMATCH{analysisData.comparison.mismatches !== 1 ? 'ES' : ''}
                </span>
              )}
            </div>
            <button onClick={closeAnalysis} className="text-gray-400 hover:text-white transition-colors p-1">
              <svg className="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
              </svg>
            </button>
          </div>

          <div className="max-w-7xl mx-auto px-6 py-4 space-y-4">
            {analysisLoading ? (
              <div className="flex items-center justify-center py-20">
                <div className="w-6 h-6 border-2 border-accent-500 border-t-transparent rounded-full animate-spin" />
                <span className="ml-3 text-gray-400">Analysiere {analysisSymbol}...</span>
              </div>
            ) : analysisData ? (
              <>
                {/* Chart with Bollinger Bands */}
                <ArenaChart
                  symbol={analysisSymbol}
                  interval={config?.interval || '4h'}
                  token={token}
                  markers={analysisData.markers}
                  overlays={analysisData.overlays}
                  customData={analysisData.chart_data}
                />

                {/* Indicator sub-chart */}
                {analysisData.indicators?.length > 0 && (
                  <ArenaIndicatorChart
                    indicators={analysisData.indicators}
                    markers={analysisData.markers}
                    strategyName={analysisData?.active_strategy?.name ? (STRATEGY_LABELS[analysisData.active_strategy.name] || analysisData.active_strategy.name) : (STRATEGY_LABELS[config?.strategy] || config?.strategy)}
                  />
                )}

                {/* Backtest results */}
                <ArenaBacktestPanel
                  metrics={analysisData.metrics}
                  trades={analysisData.trades}
                  formatPrice={formatPrice}
                  symbol={analysisSymbol}
                  tradeAmount={config?.trade_amount || 500}
                />

                {/* Comparison panel */}
                {analysisData.comparison && (
                  <div className={`bg-dark-800 rounded-lg border p-4 ${analysisData.comparison.mismatches > 0 ? 'border-orange-500/50' : 'border-dark-600'}`}>
                    <h3 className="text-sm font-medium text-white mb-3">Daten-Vergleich (Live vs. Backtest)</h3>
                    <div className="grid grid-cols-2 gap-2 mb-3">
                      <div className="bg-dark-700 rounded p-2 text-center">
                        <div className="text-[10px] text-gray-500">Übereinstimmungen</div>
                        <div className="text-base font-bold text-green-400">{analysisData.comparison.matches}</div>
                      </div>
                      <div className="bg-dark-700 rounded p-2 text-center">
                        <div className="text-[10px] text-gray-500">Abweichungen</div>
                        <div className={`text-base font-bold ${analysisData.comparison.mismatches > 0 ? 'text-orange-400' : 'text-green-400'}`}>
                          {analysisData.comparison.mismatches}
                        </div>
                      </div>
                    </div>
                    {analysisData.comparison.mismatches === 0 && analysisData.comparison.matches === 0 && (
                      <div className="text-xs text-gray-500 text-center py-2">Keine Trades seit Session-Start zum Vergleichen</div>
                    )}
                    {analysisData.comparison.mismatches === 0 && analysisData.comparison.matches > 0 && (
                      <div className="text-xs text-green-400/70 text-center py-2">Alle Daten stimmen überein</div>
                    )}
                    {analysisData.comparison.details?.length > 0 && (
                      <div className="max-h-48 overflow-auto mt-2">
                        <table className="w-full text-[11px]">
                          <thead>
                            <tr className="text-gray-500 border-b border-dark-600">
                              <th className="text-left py-1 pr-2">Typ</th>
                              <th className="text-left py-1 pr-2">Zeit</th>
                              <th className="text-left py-1">Beschreibung</th>
                            </tr>
                          </thead>
                          <tbody>
                            {analysisData.comparison.details.map((d, i) => (
                              <tr key={i} className="border-b border-dark-700/30">
                                <td className="py-1.5 pr-2">
                                  <span className={`px-1.5 py-0.5 rounded text-[9px] font-bold ${
                                    d.type === 'ENTRY_PRICE_DIFF' ? 'bg-yellow-500/20 text-yellow-400' :
                                    d.type === 'MISSING_POSITION' ? 'bg-red-500/20 text-red-400' :
                                    'bg-orange-500/20 text-orange-400'
                                  }`}>
                                    {d.type === 'ENTRY_PRICE_DIFF' ? 'PREIS' : d.type === 'MISSING_POSITION' ? 'FEHLT' : 'EXTRA'}
                                  </span>
                                </td>
                                <td className="py-1.5 pr-2 text-gray-400 whitespace-nowrap">
                                  {d.time ? dateShortFmt.format(new Date(d.time)) : '-'}
                                </td>
                                <td className="py-1.5 text-orange-300">{d.message}</td>
                              </tr>
                            ))}
                          </tbody>
                        </table>
                      </div>
                    )}
                  </div>
                )}
              </>
            ) : (
              <div className="text-center py-20 text-gray-500">Fehler beim Laden der Analyse</div>
            )}
          </div>
        </div>
      )}
    </div>
  )
}

export default LiveTrading
