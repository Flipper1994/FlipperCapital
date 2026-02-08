import { useState, useEffect, useRef, useMemo } from 'react'
import { createChart } from 'lightweight-charts'
import { useNavigate } from 'react-router-dom'
import { useCurrency } from '../context/CurrencyContext'

const MODES = [
  { key: 'defensive', title: 'Defensiv' },
  { key: 'aggressive', title: 'Aggressiv' },
  { key: 'quant', title: 'Quant' },
  { key: 'ditz', title: 'Ditz' },
  { key: 'trader', title: 'Trader' },
]

const S = {
  defensive: { dot: 'bg-blue-500', headerBg: 'bg-gradient-to-r from-blue-500/15 to-transparent' },
  aggressive: { dot: 'bg-orange-500', headerBg: 'bg-gradient-to-r from-orange-500/15 to-transparent' },
  quant: { dot: 'bg-violet-500', headerBg: 'bg-gradient-to-r from-violet-500/15 to-transparent' },
  ditz: { dot: 'bg-cyan-500', headerBg: 'bg-gradient-to-r from-cyan-500/15 to-transparent' },
  trader: { dot: 'bg-emerald-500', headerBg: 'bg-gradient-to-r from-emerald-500/15 to-transparent' },
}

// Self-contained equity curve chart
function EquityCurveChart({ data, eigenkapital, gewinn }) {
  const cRef = useRef(null)
  const chRef = useRef(null)

  useEffect(() => {
    if (!data?.length || !cRef.current) return
    if (chRef.current) { chRef.current.remove(); chRef.current = null }

    const chart = createChart(cRef.current, {
      layout: { background: { color: '#12121a' }, textColor: '#9ca3af' },
      grid: { vertLines: { visible: false }, horzLines: { visible: false } },
      width: cRef.current.clientWidth,
      height: 180,
      timeScale: { borderColor: '#2a2a34', timeVisible: false },
      rightPriceScale: { borderColor: '#2a2a34', scaleMargins: { top: 0.1, bottom: 0.1 } },
      crosshair: { mode: 1, vertLine: { color: '#6366f1', width: 1, style: 2 }, horzLine: { color: '#6366f1', width: 1, style: 2 } },
    })
    chRef.current = chart

    const deduped = new Map()
    for (const p of data) deduped.set(p.time, p.value)
    const sorted = Array.from(deduped, ([time, value]) => ({ time, value })).sort((a, b) => a.time - b.time)

    const pos = gewinn >= 0
    chart.addAreaSeries({
      lineColor: pos ? '#22c55e' : '#ef4444',
      topColor: pos ? 'rgba(34,197,94,0.3)' : 'rgba(239,68,68,0.3)',
      bottomColor: pos ? 'rgba(34,197,94,0.02)' : 'rgba(239,68,68,0.02)',
      lineWidth: 2,
      priceFormat: { type: 'price', precision: 2, minMove: 0.01 },
      lastValueVisible: true, priceLineVisible: false,
    }).setData(sorted)

    chart.addLineSeries({
      color: '#4b5563', lineWidth: 1, lineStyle: 2,
      lastValueVisible: false, priceLineVisible: false, crosshairMarkerVisible: false,
    }).setData(sorted.map(d => ({ time: d.time, value: eigenkapital })))

    chart.timeScale().fitContent()

    const onResize = () => {
      if (cRef.current && chRef.current) chRef.current.applyOptions({ width: cRef.current.clientWidth })
    }
    window.addEventListener('resize', onResize)
    return () => {
      window.removeEventListener('resize', onResize)
      if (chRef.current) { chRef.current.remove(); chRef.current = null }
    }
  }, [data, eigenkapital, gewinn])

  return <div ref={cRef} className="h-[180px] rounded-lg border border-dark-600 overflow-hidden" />
}

// Sortable table header
function SortTH({ field, sort, onSort, children, right }) {
  const active = sort.field === field
  return (
    <th className={`px-2 py-2 font-medium whitespace-nowrap ${right ? 'text-right' : 'text-left'}`}>
      <button
        onClick={() => onSort(active ? { field, dir: sort.dir === 'asc' ? 'desc' : 'asc' } : { field, dir: 'desc' })}
        className="inline-flex items-center gap-1 hover:text-white transition-colors"
        style={right ? { marginLeft: 'auto' } : undefined}
      >
        {children}
        {active && <span className="text-accent-400">{sort.dir === 'asc' ? '↑' : '↓'}</span>}
      </button>
    </th>
  )
}

function Performance({ token }) {
  const navigate = useNavigate()
  const { formatPrice, currencySymbol } = useCurrency()

  const [trades, setTrades] = useState([])
  const [loading, setLoading] = useState(true)
  const [timeRange, setTimeRange] = useState('1y')

  // Filters
  const [filtersOpen, setFiltersOpen] = useState(false)
  const [filters, setFilters] = useState({
    minWinrate: '', maxWinrate: '', minRR: '', maxRR: '',
    minAvgReturn: '', maxAvgReturn: '', minMarketCap: ''
  })

  // Section & trade table collapse
  const [openSections, setOpenSections] = useState({})
  const [openTradeTables, setOpenTradeTables] = useState({})

  // Simulation
  const [simAmount, setSimAmount] = useState('100')
  const [simResults, setSimResults] = useState({})
  const [simSorts, setSimSorts] = useState({})
  const [tradeSorts, setTradeSorts] = useState({})

  // Optimierung
  const [optOpen, setOptOpen] = useState(false)
  const [optMode, setOptMode] = useState('defensive')
  const [optLocked, setOptLocked] = useState({})
  const [filterSymbols, setFilterSymbols] = useState(null) // null = kein Filter, Set = nur diese Symbole

  useEffect(() => { if (!token) navigate('/') }, [token, navigate])

  useEffect(() => {
    if (!token) return
    setLoading(true)
    fetch('/api/performance/history', { headers: { 'Authorization': `Bearer ${token}` } })
      .then(r => r.json())
      .then(d => setTrades(Array.isArray(d) ? d : []))
      .catch(() => {})
      .finally(() => setLoading(false))
  }, [token])

  const cutoffDate = useMemo(() => {
    const now = Math.floor(Date.now() / 1000)
    const y = 365 * 24 * 60 * 60
    const map = { '1m': 30 * 86400, '3m': 90 * 86400, '6m': 180 * 86400, '1y': y, '2y': 2 * y, '3y': 3 * y, '4y': 4 * y, '5y': 5 * y, '10y': 10 * y }
    return map[timeRange] ? now - map[timeRange] : 0
  }, [timeRange])

  // Per-mode data
  const modeData = useMemo(() => {
    const result = {}
    for (const m of MODES) {
      const filtered = trades
        .filter(t => t.mode === m.key && t.entry_date >= cutoffDate)
        .filter(t => {
          if (filterSymbols && !filterSymbols.has(t.symbol)) return false
          if (filters.minWinrate && t.win_rate < parseFloat(filters.minWinrate)) return false
          if (filters.maxWinrate && t.win_rate > parseFloat(filters.maxWinrate)) return false
          if (filters.minRR && t.risk_reward < parseFloat(filters.minRR)) return false
          if (filters.maxRR && t.risk_reward > parseFloat(filters.maxRR)) return false
          if (filters.minAvgReturn && t.avg_return < parseFloat(filters.minAvgReturn)) return false
          if (filters.maxAvgReturn && t.avg_return > parseFloat(filters.maxAvgReturn)) return false
          if (filters.minMarketCap && t.market_cap < parseFloat(filters.minMarketCap) * 1e9) return false
          return true
        })
      const wins = filtered.filter(t => (t.return_pct || 0) > 0)
      const losses = filtered.filter(t => (t.return_pct || 0) < 0)
      const totalReturn = filtered.reduce((s, t) => s + (t.return_pct || 0), 0)
      const aw = wins.length > 0 ? wins.reduce((s, t) => s + t.return_pct, 0) / wins.length : 0
      const al = losses.length > 0 ? Math.abs(losses.reduce((s, t) => s + t.return_pct, 0) / losses.length) : 0
      result[m.key] = {
        trades: filtered,
        stats: {
          tradeCount: filtered.length,
          winRate: filtered.length > 0 ? (wins.length / filtered.length) * 100 : 0,
          riskReward: al > 0 ? aw / al : aw > 0 ? Infinity : 0,
          totalReturn,
          avgReturn: filtered.length > 0 ? totalReturn / filtered.length : 0,
          wins: wins.length,
          losses: losses.length,
        }
      }
    }
    return result
  }, [trades, cutoffDate, filters, filterSymbols])

  // Optimierung: Stock-Aggregation pro optMode (ohne User-Filter)
  const stockPool = useMemo(() => {
    const map = new Map()
    for (const t of trades) {
      if (t.mode !== optMode || t.entry_date < cutoffDate) continue
      const s = map.get(t.symbol)
      if (s) {
        s.total_trades++
        s.total_return += (t.return_pct || 0)
        if (t.entry_date < s.first_trade) s.first_trade = t.entry_date
        if (t.entry_date > s.last_trade) s.last_trade = t.entry_date
      } else {
        map.set(t.symbol, {
          symbol: t.symbol,
          win_rate: t.win_rate || 0,
          risk_reward: t.risk_reward || 0,
          avg_return: t.avg_return || 0,
          market_cap: t.market_cap || 0,
          total_trades: 1,
          total_return: t.return_pct || 0,
          first_trade: t.entry_date,
          last_trade: t.entry_date,
        })
      }
    }
    return Array.from(map.values())
  }, [trades, optMode, cutoffDate])

  // Optimierung: Filtern + Mediane berechnen
  const optResult = useMemo(() => {
    const median = (arr) => {
      if (!arr.length) return 0
      const sorted = [...arr].sort((a, b) => a - b)
      const mid = Math.floor(sorted.length / 2)
      return sorted.length % 2 ? sorted[mid] : (sorted[mid - 1] + sorted[mid]) / 2
    }

    // Ranges aus gesamtem Pool
    const ranges = {
      trades: { min: 0, max: Math.max(1, ...stockPool.map(s => s.total_trades)), step: 1 },
      winrate: { min: 0, max: 100, step: 1 },
      rr: { min: 0, max: Math.min(5, Math.max(1, Math.ceil(Math.max(...stockPool.map(s => s.risk_reward), 0)))), step: 0.1 },
      totalReturn: {
        min: Math.floor(Math.min(0, ...stockPool.map(s => s.total_return))),
        max: Math.ceil(Math.max(1, ...stockPool.map(s => s.total_return))),
        step: 1,
      },
      marketCap: { min: 0, max: Math.max(1, Math.ceil(Math.max(...stockPool.map(s => s.market_cap / 1e9), 0))), step: 1 },
    }

    // Filter mit locked Sliders
    const filtered = stockPool.filter(s => {
      if (optLocked.trades != null && s.total_trades < optLocked.trades) return false
      if (optLocked.winrate != null && s.win_rate < optLocked.winrate) return false
      if (optLocked.rr != null && s.risk_reward < optLocked.rr) return false
      if (optLocked.totalReturn != null && s.total_return < optLocked.totalReturn) return false
      if (optLocked.marketCap != null && s.market_cap < optLocked.marketCap * 1e9) return false
      return true
    })

    const medians = {
      trades: median(filtered.map(s => s.total_trades)),
      winrate: median(filtered.map(s => s.win_rate)),
      rr: Math.round(median(filtered.map(s => s.risk_reward)) * 10) / 10,
      totalReturn: Math.round(median(filtered.map(s => s.total_return))),
      marketCap: Math.round(median(filtered.map(s => s.market_cap / 1e9))),
    }

    // Trade-level Stats (exakt wie modeData) für qualifizierende Symbole
    const qualSymbols = new Set(filtered.map(s => s.symbol))
    const matchingTrades = trades.filter(t =>
      t.mode === optMode && t.entry_date >= cutoffDate && qualSymbols.has(t.symbol))
    const wins = matchingTrades.filter(t => (t.return_pct || 0) > 0)
    const losses = matchingTrades.filter(t => (t.return_pct || 0) < 0)
    const totalReturn = matchingTrades.reduce((s, t) => s + (t.return_pct || 0), 0)
    const tradeCount = matchingTrades.length
    const winRate = tradeCount > 0 ? (wins.length / tradeCount) * 100 : 0
    const aw = wins.length > 0 ? wins.reduce((s, t) => s + t.return_pct, 0) / wins.length : 0
    const al = losses.length > 0 ? Math.abs(losses.reduce((s, t) => s + t.return_pct, 0) / losses.length) : 0
    const riskReward = al > 0 ? aw / al : aw > 0 ? Infinity : 0

    // p.a. Rendite aus Trades (gleiche Basis wie Simulation CAGR)
    const now = Math.floor(Date.now() / 1000)
    const earliest = filtered.length ? Math.min(...filtered.map(s => s.first_trade)) : now
    const years = Math.max(0.25, (now - earliest) / (365 * 86400))
    const returnPa = filtered.length > 0 ? (totalReturn / filtered.length) / years : 0

    return { filtered, medians, ranges, count: filtered.length,
      tradeCount, winRate, riskReward, totalReturn, returnPa }
  }, [stockPool, optLocked, trades, optMode, cutoffDate])

  // Optimierung: Vorschläge per Grid-Search
  const optPresets = useMemo(() => {
    if (stockPool.length < 2) return []

    // Zeitspanne für p.a.-Berechnung
    const now = Math.floor(Date.now() / 1000)
    const earliest = Math.min(...stockPool.map(s => s.first_trade))
    const poolYears = Math.max(0.25, (now - earliest) / (365 * 86400))

    const calcReturnPa = (stocks) => {
      // Durchschnittliche Rendite pro Aktie, annualisiert
      // = wenn ich gleichmäßig in diese Aktien investiere, was kommt p.a. raus?
      const avgRet = stocks.reduce((s, x) => s + x.total_return, 0) / stocks.length
      return avgRet / poolYears
    }

    const percentiles = (arr, pcts) => {
      const s = [...arr].sort((a, b) => a - b)
      return pcts.map(p => s[Math.min(Math.floor(p * s.length), s.length - 1)])
    }
    const trSteps = [0, ...percentiles(stockPool.map(s => s.total_trades), [0.25, 0.5, 0.75])]
    const wrSteps = [0, ...percentiles(stockPool.map(s => s.win_rate), [0.25, 0.5, 0.75, 0.9])]
    const rrSteps = [0, ...percentiles(stockPool.map(s => s.risk_reward), [0.25, 0.5, 0.75])]
    const retSteps = [
      Math.floor(Math.min(0, ...stockPool.map(s => s.total_return))),
      0,
      ...percentiles(stockPool.map(s => s.total_return), [0.25, 0.5, 0.75])
    ]
    const unique = arr => [...new Set(arr.map(v => Math.round(v * 100) / 100))]
    const uTr = unique(trSteps), uWr = unique(wrSteps), uRr = unique(rrSteps), uRet = unique(retSteps)

    const applyF = (pool, f) => pool.filter(s => {
      if (f.trades && s.total_trades < f.trades) return false
      if (f.winrate && s.win_rate < f.winrate) return false
      if (f.rr && s.risk_reward < f.rr) return false
      if (f.totalReturn != null && s.total_return < f.totalReturn) return false
      return true
    })

    const strategies = [
      {
        name: 'Max Rendite',
        desc: 'Beste p.a. Rendite',
        color: 'text-green-400 border-green-500/30 bg-green-500/10',
        icon: '📈',
        // Reine p.a.-Maximierung, min 3 Aktien damit nicht nur 2 Ausreißer
        score: (stocks) => {
          if (stocks.length < 3) return -Infinity
          return calcReturnPa(stocks)
        },
        minStocks: 3,
      },
      {
        name: 'Top Picks',
        desc: 'Wenige Top-Performer',
        color: 'text-amber-400 border-amber-500/30 bg-amber-500/10',
        icon: '🎯',
        // Höchste p.a. mit kleiner, fokussierter Auswahl (5-15 Aktien)
        score: (stocks) => {
          if (stocks.length < 5 || stocks.length > Math.max(15, Math.floor(stockPool.length * 0.25))) return -Infinity
          return calcReturnPa(stocks)
        },
        minStocks: 5,
      },
      {
        name: 'Breit & Stabil',
        desc: 'Viele Aktien, gute p.a.',
        color: 'text-blue-400 border-blue-500/30 bg-blue-500/10',
        icon: '🛡',
        // Beste p.a. mit breiter Diversifikation (mind. 40% des Pools)
        score: (stocks) => {
          if (stocks.length < Math.max(5, Math.floor(stockPool.length * 0.4))) return -Infinity
          return calcReturnPa(stocks)
        },
        minStocks: Math.max(5, Math.floor(stockPool.length * 0.4)),
      },
      {
        name: 'Risiko-Optimiert',
        desc: 'Stabile p.a. Rendite',
        color: 'text-violet-400 border-violet-500/30 bg-violet-500/10',
        icon: '⚖',
        // Maximiere p.a. / Streuung — hohe Rendite bei geringer Varianz
        score: (stocks) => {
          if (stocks.length < 3) return -Infinity
          const pa = calcReturnPa(stocks)
          const rets = stocks.map(s => s.total_return / poolYears)
          const avg = rets.reduce((a, b) => a + b, 0) / rets.length
          const variance = rets.reduce((s, r) => s + (r - avg) ** 2, 0) / rets.length
          const std = Math.sqrt(variance) || 1
          // Sharpe-ähnlich, aber skaliert mit p.a. damit hohe Rendite bevorzugt wird
          return pa * (avg / std)
        },
        minStocks: 3,
      },
    ]

    return strategies.map(strat => {
      let best = null, bestScore = -Infinity
      for (const tr of uTr) {
        for (const wr of uWr) {
          for (const rr of uRr) {
            for (const ret of uRet) {
              const f = { trades: tr, winrate: wr, rr, totalReturn: ret }
              const filtered = applyF(stockPool, f)
              if (filtered.length < strat.minStocks) continue
              const sc = strat.score(filtered)
              if (sc > bestScore) {
                bestScore = sc
                best = { ...f, count: filtered.length }
              }
            }
          }
        }
      }
      if (!best) return null
      const locked = {}
      if (best.trades > 0) locked.trades = best.trades
      if (best.winrate > 0) locked.winrate = Math.round(best.winrate * 10) / 10
      if (best.rr > 0) locked.rr = Math.round(best.rr * 10) / 10
      if (best.totalReturn !== 0) locked.totalReturn = Math.round(best.totalReturn)
      const filtered = applyF(stockPool, best)
      const returnPa = calcReturnPa(filtered)
      return { ...strat, locked, count: best.count, returnPa }
    }).filter(Boolean)
  }, [stockPool])

  // Helpers
  const fmtSim = (val) => {
    if (val == null) return '--'
    const prefix = currencySymbol === 'CHF' ? 'CHF ' : currencySymbol
    const f = Math.abs(val).toLocaleString(currencySymbol === 'CHF' ? 'de-CH' : 'en-US', { minimumFractionDigits: 2, maximumFractionDigits: 2 })
    return val < 0 ? `-${prefix}${f}` : `${prefix}${f}`
  }

  const fmtDate = (ts) => !ts ? '-' : new Date(ts * 1000).toLocaleDateString('de-DE', { day: '2-digit', month: '2-digit', year: 'numeric' })

  // Simulation for one mode
  const runSimForMode = (filteredTrades) => {
    const amount = parseFloat(simAmount)
    if (!amount || amount <= 0 || !filteredTrades.length) return null

    const simTrades = []
    let totalProfit = 0, openCount = 0
    for (const t of filteredTrades) {
      const pct = t.return_pct || 0
      const isOpen = t.status === 'OPEN' || !t.exit_date
      const profit = Math.round(amount * (pct / 100) * 100) / 100
      if (isOpen) openCount++
      totalProfit += profit
      simTrades.push({
        symbol: t.symbol, name: t.name,
        entryDate: t.entry_date, exitDate: isOpen ? null : t.exit_date,
        entryPrice: t.entry_price, exitPrice: isOpen ? t.current_price : t.exit_price,
        profit, received: Math.round((amount + profit) * 100) / 100,
        returnPct: pct, status: t.status,
      })
    }
    simTrades.sort((a, b) => a.entryDate - b.entryDate)

    // Max concurrent
    const events = []
    for (const t of simTrades) {
      events.push({ time: t.entryDate, type: 1 })
      events.push({ time: t.exitDate || Math.floor(Date.now() / 1000), type: -1 })
    }
    events.sort((a, b) => a.time - b.time || a.type - b.type)
    let conc = 0, maxConc = 0
    for (const e of events) { conc += e.type; if (conc > maxConc) maxConc = conc }

    const ek = maxConc * amount
    const gew = Math.round(totalProfit * 100) / 100
    const endk = Math.round((ek + gew) * 100) / 100
    const rendite = ek > 0 ? (gew / ek) * 100 : 0

    // CAGR
    const first = simTrades[0]?.entryDate || 0
    const last = simTrades.reduce((m, t) => Math.max(m, t.exitDate || Math.floor(Date.now() / 1000)), 0)
    const years = first > 0 ? (last - first) / (365 * 86400) : 0
    let cagr = 0
    if (ek > 0 && endk > 0 && years >= 0.1) cagr = (Math.pow(endk / ek, 1 / years) - 1) * 100
    else if (ek > 0) cagr = rendite

    const allW = simTrades.filter(t => t.profit > 0)
    const allL = simTrades.filter(t => t.profit < 0)
    const wr = simTrades.length > 0 ? (allW.length / simTrades.length) * 100 : 0
    const avgW = allW.length > 0 ? allW.reduce((s, t) => s + t.profit, 0) / allW.length : 0
    const avgL = allL.length > 0 ? Math.abs(allL.reduce((s, t) => s + t.profit, 0) / allL.length) : 0
    const avgWP = allW.length > 0 ? allW.reduce((s, t) => s + t.returnPct, 0) / allW.length : 0
    const avgLP = allL.length > 0 ? Math.abs(allL.reduce((s, t) => s + t.returnPct, 0) / allL.length) : 0
    const rr = avgL > 0 ? avgW / avgL : avgW > 0 ? Infinity : 0

    // Equity curve
    const curve = []
    if (simTrades.length) {
      const now = Math.floor(Date.now() / 1000)
      const start = simTrades[0].entryDate
      const end = simTrades.reduce((m, t) => Math.max(m, t.exitDate || now), 0)
      const DAY = 86400
      const days = Math.ceil((end - start) / DAY)
      const step = days > 1000 ? Math.ceil(days / 1000) * DAY : DAY
      const calc = (ts) => {
        let real = 0, unreal = 0
        for (const t of simTrades) {
          const exit = t.exitDate || now
          if (ts >= exit) real += t.profit
          else if (ts >= t.entryDate) {
            const dur = exit - t.entryDate
            unreal += amount * (t.returnPct / 100) * (dur > 0 ? (ts - t.entryDate) / dur : 0)
          }
        }
        return Math.round((ek + real + unreal) * 100) / 100
      }
      for (let ts = start; ts <= end; ts += step) curve.push({ time: ts, value: calc(ts) })
      if (!curve.length || curve[curve.length - 1].time < end) curve.push({ time: end, value: calc(end) })
    }

    return {
      trades: simTrades, eigenkapital: ek, endkapital: endk, gewinn: gew, rendite, cagr, years,
      maxConcurrent: maxConc, amount, tradeCount: simTrades.length, openCount,
      winRate: wr, riskReward: rr, wins: allW.length, losses: allL.length,
      avgWin: avgW, avgLoss: avgL, avgWinPct: avgWP, avgLossPct: avgLP, equityCurve: curve,
    }
  }

  const runAllSimulations = () => {
    const results = {}
    for (const m of MODES) results[m.key] = runSimForMode(modeData[m.key].trades)
    setSimResults(results)
  }

  // Auto-run simulation when data changes (initial load, filter/time change)
  useEffect(() => {
    const amount = parseFloat(simAmount)
    if (!amount || amount <= 0 || !trades.length) { setSimResults({}); return }
    const results = {}
    for (const m of MODES) results[m.key] = runSimForMode(modeData[m.key].trades)
    setSimResults(results)
  }, [modeData, simAmount])

  // Filter UI
  const handleFilterChange = (f, v) => setFilters(p => ({ ...p, [f]: v }))
  const clearFilters = () => { setFilters({ minWinrate: '', maxWinrate: '', minRR: '', maxRR: '', minAvgReturn: '', maxAvgReturn: '', minMarketCap: '' }); setFilterSymbols(null) }
  const hasActiveFilters = Object.values(filters).some(v => v !== '') || filterSymbols !== null

  const toggleSection = (k) => setOpenSections(p => ({ ...p, [k]: !p[k] }))
  const toggleTradeTable = (k) => setOpenTradeTables(p => ({ ...p, [k]: !p[k] }))

  // Optimierung handlers
  const handleOptSlider = (key, value) => {
    const v = parseFloat(value)
    const testLocked = { ...optLocked, [key]: v }
    const remaining = stockPool.filter(s => {
      if (testLocked.trades != null && s.total_trades < testLocked.trades) return false
      if (testLocked.winrate != null && s.win_rate < testLocked.winrate) return false
      if (testLocked.rr != null && s.risk_reward < testLocked.rr) return false
      if (testLocked.totalReturn != null && s.total_return < testLocked.totalReturn) return false
      if (testLocked.marketCap != null && s.market_cap < testLocked.marketCap * 1e9) return false
      return true
    })
    if (remaining.length < 1) return
    setOptLocked(p => ({ ...p, [key]: v }))
  }
  const resetOptSlider = (key) => setOptLocked(p => { const n = { ...p }; delete n[key]; return n })
  const applyOptFilters = () => {
    // Exakte Symbolliste aus Optimierung übernehmen → modeData zeigt genau diese Trades
    setFilterSymbols(new Set(optResult.filtered.map(s => s.symbol)))
    setFilters({ minWinrate: '', maxWinrate: '', minRR: '', maxRR: '', minAvgReturn: '', maxAvgReturn: '', minMarketCap: '' })
    setFiltersOpen(true)
  }

  // Sort helpers
  const getSimSort = (k) => simSorts[k] || { field: 'entryDate', dir: 'desc' }
  const doSetSimSort = (k, s) => setSimSorts(p => ({ ...p, [k]: s }))
  const getTradeSort = (k) => tradeSorts[k] || { field: 'entry_date', dir: 'desc' }
  const doSetTradeSort = (k, s) => setTradeSorts(p => ({ ...p, [k]: s }))

  const sortSimTrades = (list, sort) => [...list].sort((a, b) => {
    if (sort.field === 'symbol') return sort.dir === 'asc' ? a.symbol.localeCompare(b.symbol) : b.symbol.localeCompare(a.symbol)
    const av = a[sort.field] ?? (sort.field === 'exitDate' ? Infinity : 0)
    const bv = b[sort.field] ?? (sort.field === 'exitDate' ? Infinity : 0)
    return sort.dir === 'asc' ? av - bv : bv - av
  })

  const sortBasicTrades = (list, sort) => [...list].sort((a, b) => {
    if (sort.field === 'symbol') return sort.dir === 'asc' ? (a.symbol || '').localeCompare(b.symbol || '') : (b.symbol || '').localeCompare(a.symbol || '')
    const av = a[sort.field] ?? 0
    const bv = b[sort.field] ?? 0
    return sort.dir === 'asc' ? av - bv : bv - av
  })

  const totalTradesAll = MODES.reduce((s, m) => s + (modeData[m.key]?.trades.length || 0), 0)

  const timeRangeOptions = [
    { value: '1m', label: '1M' }, { value: '3m', label: '3M' }, { value: '6m', label: '6M' },
    { value: '1y', label: '1J' }, { value: '2y', label: '2J' }, { value: '3y', label: '3J' },
    { value: '4y', label: '4J' }, { value: '5y', label: '5J' }, { value: '10y', label: '10J' },
    { value: 'all', label: 'Max' },
  ]

  return (
    <div className="p-4 md:p-6 h-full overflow-y-auto max-w-5xl mx-auto w-full">
      {/* Title */}
      <div className="mb-4 text-center">
        <h1 className="text-2xl font-bold text-white">Performance</h1>
        <p className="text-gray-400 mt-1">Trade-Historie der Watchlist — Alle Modi</p>
      </div>

      {/* Time Range */}
      <div className="flex justify-center mb-4">
        <div className="inline-flex bg-dark-800 rounded-lg p-1 border border-dark-600 flex-wrap justify-center">
          {timeRangeOptions.map(o => (
            <button key={o.value} onClick={() => setTimeRange(o.value)}
              className={`px-3 py-1.5 text-sm font-medium rounded-md transition-all ${timeRange === o.value ? 'bg-accent-500 text-white' : 'text-gray-400 hover:text-white hover:bg-dark-700'}`}>
              {o.label}
            </button>
          ))}
        </div>
      </div>

      {/* Filters */}
      <div className="mb-3 bg-dark-800 rounded-xl border border-dark-600 overflow-hidden">
        <button onClick={() => setFiltersOpen(!filtersOpen)}
          className="w-full px-4 py-2.5 flex items-center justify-between text-left hover:bg-dark-700/50 transition-colors">
          <div className="flex items-center gap-2">
            <svg className="w-4 h-4 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M3 4a1 1 0 011-1h16a1 1 0 011 1v2.586a1 1 0 01-.293.707l-6.414 6.414a1 1 0 00-.293.707V17l-4 4v-6.586a1 1 0 00-.293-.707L3.293 7.293A1 1 0 013 6.586V4z" />
            </svg>
            <span className="text-white font-medium text-sm">Filter</span>
            {hasActiveFilters && <span className="px-1.5 py-0.5 text-xs bg-accent-500 text-white rounded-full">Aktiv</span>}
          </div>
          <svg className={`w-4 h-4 text-gray-400 transition-transform ${filtersOpen ? 'rotate-180' : ''}`} fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 9l-7 7-7-7" />
          </svg>
        </button>
        {filtersOpen && (
          <div className="px-4 pb-3 border-t border-dark-600">
            <div className="grid grid-cols-2 md:grid-cols-4 gap-3 mt-3">
              <div>
                <label className="block text-xs text-gray-400 mb-1">Winrate (%)</label>
                <div className="flex gap-2">
                  <input type="number" placeholder="Min" value={filters.minWinrate} onChange={e => handleFilterChange('minWinrate', e.target.value)}
                    className="w-full px-2 py-1.5 text-sm bg-dark-700 border border-dark-600 rounded text-white placeholder-gray-500" />
                  <input type="number" placeholder="Max" value={filters.maxWinrate} onChange={e => handleFilterChange('maxWinrate', e.target.value)}
                    className="w-full px-2 py-1.5 text-sm bg-dark-700 border border-dark-600 rounded text-white placeholder-gray-500" />
                </div>
              </div>
              <div>
                <label className="block text-xs text-gray-400 mb-1">Risk/Reward</label>
                <div className="flex gap-2">
                  <input type="number" step="0.1" placeholder="Min" value={filters.minRR} onChange={e => handleFilterChange('minRR', e.target.value)}
                    className="w-full px-2 py-1.5 text-sm bg-dark-700 border border-dark-600 rounded text-white placeholder-gray-500" />
                  <input type="number" step="0.1" placeholder="Max" value={filters.maxRR} onChange={e => handleFilterChange('maxRR', e.target.value)}
                    className="w-full px-2 py-1.5 text-sm bg-dark-700 border border-dark-600 rounded text-white placeholder-gray-500" />
                </div>
              </div>
              <div>
                <label className="block text-xs text-gray-400 mb-1">Ø Rendite (%)</label>
                <div className="flex gap-2">
                  <input type="number" step="0.1" placeholder="Min" value={filters.minAvgReturn} onChange={e => handleFilterChange('minAvgReturn', e.target.value)}
                    className="w-full px-2 py-1.5 text-sm bg-dark-700 border border-dark-600 rounded text-white placeholder-gray-500" />
                  <input type="number" step="0.1" placeholder="Max" value={filters.maxAvgReturn} onChange={e => handleFilterChange('maxAvgReturn', e.target.value)}
                    className="w-full px-2 py-1.5 text-sm bg-dark-700 border border-dark-600 rounded text-white placeholder-gray-500" />
                </div>
              </div>
              <div>
                <label className="block text-xs text-gray-400 mb-1">Min Market Cap (Mrd)</label>
                <input type="number" step="0.1" placeholder="z.B. 10" value={filters.minMarketCap} onChange={e => handleFilterChange('minMarketCap', e.target.value)}
                  className="w-full px-2 py-1.5 text-sm bg-dark-700 border border-dark-600 rounded text-white placeholder-gray-500" />
              </div>
            </div>
            {filterSymbols && (
              <div className="mt-3 flex items-center gap-2 px-2 py-1.5 bg-accent-500/10 border border-accent-500/30 rounded-lg">
                <span className="text-xs text-accent-400 font-medium">Optimierung aktiv: {filterSymbols.size} Aktien ausgewählt</span>
                <button onClick={() => setFilterSymbols(null)} className="ml-auto text-xs text-gray-400 hover:text-white transition-colors">Aufheben</button>
              </div>
            )}
            {hasActiveFilters && (
              <div className="mt-2 flex justify-end">
                <button onClick={clearFilters} className="px-3 py-1 text-xs text-gray-400 hover:text-white transition-colors">Filter zurücksetzen</button>
              </div>
            )}
          </div>
        )}
      </div>

      {/* Simulation Config Bar */}
      <div className="mb-4 bg-dark-800 rounded-xl border border-dark-600 px-4 py-3 flex items-center gap-3 flex-wrap">
        <svg className="w-4 h-4 text-gray-400 flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 7h6m0 10v-3m-3 3h.01M9 17h.01M9 14h.01M12 14h.01M15 11h.01M12 11h.01M9 11h.01M7 21h10a2 2 0 002-2V5a2 2 0 00-2-2H7a2 2 0 00-2 2v14a2 2 0 002 2z" />
        </svg>
        <span className="text-sm text-gray-400">Simulation:</span>
        <div className="relative">
          <input type="number" min="1" step="10" value={simAmount} onChange={e => setSimAmount(e.target.value)}
            onKeyDown={e => e.key === 'Enter' && runAllSimulations()}
            className="w-28 px-3 py-1.5 text-sm bg-dark-700 border border-dark-600 rounded-lg text-white pr-8" />
          <span className="absolute right-2.5 top-1/2 -translate-y-1/2 text-xs text-gray-500">{currencySymbol}</span>
        </div>
        <span className="text-xs text-gray-500">pro Trade</span>
        <button onClick={runAllSimulations}
          disabled={!simAmount || parseFloat(simAmount) <= 0 || totalTradesAll === 0}
          className="px-4 py-1.5 text-sm bg-accent-500 text-white rounded-lg hover:bg-accent-600 disabled:opacity-50 disabled:cursor-not-allowed transition-colors font-medium">
          Berechnen
        </button>
        {Object.keys(simResults).length > 0 && (
          <span className="text-xs text-gray-500 ml-auto">
            {MODES.filter(m => simResults[m.key]).length} Modi berechnet
          </span>
        )}
      </div>

      {/* Loading */}
      {loading && (
        <div className="flex justify-center py-12">
          <div className="animate-spin w-8 h-8 border-2 border-accent-500 border-t-transparent rounded-full" />
        </div>
      )}

      {/* Mode Sections */}
      {!loading && (
        <div className="space-y-3">
          {MODES.map(mc => {
            const data = modeData[mc.key]
            const sim = simResults[mc.key]
            const isOpen = openSections[mc.key]
            const tradesOpen = openTradeTables[mc.key]
            const st = data.stats
            const style = S[mc.key]
            const hasTrades = st.tradeCount > 0

            return (
              <div key={mc.key} className={`bg-dark-800 rounded-xl border border-dark-600 overflow-hidden ${!hasTrades ? 'opacity-60' : ''}`}>
                {/* Section Header */}
                <button onClick={() => hasTrades && toggleSection(mc.key)}
                  className={`w-full px-4 py-3 text-left transition-colors ${hasTrades ? 'hover:bg-dark-700/30 cursor-pointer' : 'cursor-default'} ${isOpen ? style.headerBg : ''}`}>
                  {/* Row 1: Mode name + sim summary + chevron */}
                  <div className="flex items-center justify-between gap-3">
                    <div className="flex items-center gap-2.5 min-w-0">
                      <span className={`w-3 h-3 rounded-full flex-shrink-0 ${style.dot}`} />
                      <span className="font-semibold text-white text-base">{mc.title}</span>
                      {hasTrades ? (
                        <span className="text-sm text-gray-400 flex-shrink-0">{st.tradeCount} Trades</span>
                      ) : (
                        <span className="text-sm text-gray-500">Keine Trades</span>
                      )}
                    </div>
                    <div className="flex items-center gap-3 flex-shrink-0">
                      {sim && hasTrades && (
                        <div className="hidden sm:flex items-center gap-2">
                          <span className={`text-sm font-medium ${sim.gewinn >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                            {sim.gewinn >= 0 ? '+' : ''}{fmtSim(sim.gewinn)}
                          </span>
                          <span className={`text-xs ${sim.rendite >= 0 ? 'text-green-400/70' : 'text-red-400/70'}`}>
                            ({sim.rendite >= 0 ? '+' : ''}{sim.rendite.toFixed(1)}%)
                          </span>
                          <span className={`text-xs ${sim.cagr >= 0 ? 'text-green-400/70' : 'text-red-400/70'}`}>
                            ({sim.cagr >= 0 ? '+' : ''}{sim.cagr.toFixed(1)}% p.a.)
                          </span>
                        </div>
                      )}
                      {hasTrades && (
                        <svg className={`w-5 h-5 text-gray-400 transition-transform ${isOpen ? 'rotate-180' : ''}`} fill="none" stroke="currentColor" viewBox="0 0 24 24">
                          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 9l-7 7-7-7" />
                        </svg>
                      )}
                    </div>
                  </div>
                  {/* Row 2: Stats bar - consistent across all sections */}
                  {hasTrades && (
                    <div className="flex items-center gap-3 md:gap-5 mt-1.5 pl-[22px] overflow-x-auto scrollbar-none">
                      <span className={`text-sm font-medium whitespace-nowrap ${st.winRate >= 50 ? 'text-green-400' : 'text-red-400'}`}>
                        Win {st.winRate.toFixed(1)}%
                      </span>
                      <span className={`text-sm font-medium whitespace-nowrap ${st.riskReward >= 1 ? 'text-green-400' : 'text-orange-400'}`}>
                        R/R {st.riskReward === Infinity ? '∞' : st.riskReward.toFixed(2)}
                      </span>
                      <span className={`text-sm font-medium whitespace-nowrap ${st.totalReturn >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                        Σ {st.totalReturn >= 0 ? '+' : ''}{st.totalReturn.toFixed(1)}%
                      </span>
                      <span className={`text-sm whitespace-nowrap ${st.avgReturn >= 0 ? 'text-green-400/70' : 'text-red-400/70'}`}>
                        Ø {st.avgReturn >= 0 ? '+' : ''}{st.avgReturn.toFixed(2)}%
                      </span>
                      {sim && (
                        <span className={`text-sm font-medium whitespace-nowrap sm:hidden ${sim.gewinn >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                          Sim: {sim.gewinn >= 0 ? '+' : ''}{fmtSim(sim.gewinn)} ({sim.rendite >= 0 ? '+' : ''}{sim.rendite.toFixed(1)}%)
                        </span>
                      )}
                    </div>
                  )}
                </button>

                {/* Section Body */}
                {isOpen && hasTrades && (
                  <div className="border-t border-dark-600 px-4 py-3 space-y-3">

                    {/* Sim Stats */}
                    {sim && (
                      <>
                        <div className="grid grid-cols-2 md:grid-cols-4 gap-3">
                          <div className="bg-dark-700 rounded-lg p-3">
                            <div className="text-xs text-gray-400">Startkapital</div>
                            <div className="text-lg font-bold text-amber-400">{fmtSim(sim.eigenkapital)}</div>
                            <div className="text-xs text-gray-500">{sim.maxConcurrent} x {fmtSim(sim.amount)}</div>
                          </div>
                          <div className="bg-dark-700 rounded-lg p-3">
                            <div className="text-xs text-gray-400">Endkapital</div>
                            <div className="text-lg font-bold text-white">{fmtSim(sim.endkapital)}</div>
                            <div className="text-xs text-gray-500">{sim.openCount > 0 ? `inkl. ${sim.openCount} offene` : `${sim.tradeCount} Trades`}</div>
                          </div>
                          <div className="bg-dark-700 rounded-lg p-3">
                            <div className="text-xs text-gray-400">Gewinn / Verlust</div>
                            <div className={`text-lg font-bold ${sim.gewinn >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                              {sim.gewinn >= 0 ? '+' : ''}{fmtSim(sim.gewinn)}
                            </div>
                            <div className={`text-xs ${sim.rendite >= 0 ? 'text-green-400/70' : 'text-red-400/70'}`}>
                              {sim.rendite >= 0 ? '+' : ''}{sim.rendite.toFixed(1)}%
                            </div>
                          </div>
                          <div className="bg-dark-700 rounded-lg p-3">
                            <div className="text-xs text-gray-400">Rendite p.a.</div>
                            <div className={`text-lg font-bold ${sim.cagr >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                              {sim.cagr >= 0 ? '+' : ''}{sim.cagr.toFixed(1)}%
                            </div>
                            <div className="text-xs text-gray-500">
                              {sim.years >= 1 ? `${sim.years.toFixed(1)} Jahre` : `${Math.round(sim.years * 12)} Mon.`}
                            </div>
                          </div>
                        </div>

                        <div className="grid grid-cols-2 md:grid-cols-4 gap-3">
                          <div className="bg-dark-700 rounded-lg p-3">
                            <div className="text-xs text-gray-400">Win Rate</div>
                            <div className={`text-lg font-bold ${sim.winRate >= 50 ? 'text-green-400' : 'text-red-400'}`}>
                              {sim.winRate.toFixed(1)}%
                            </div>
                            <div className="text-xs text-gray-500">{sim.wins}W / {sim.losses}L</div>
                          </div>
                          <div className="bg-dark-700 rounded-lg p-3">
                            <div className="text-xs text-gray-400">Risiko-Rendite</div>
                            <div className={`text-lg font-bold ${sim.riskReward >= 1 ? 'text-green-400' : 'text-red-400'}`}>
                              {sim.riskReward === Infinity ? '∞' : sim.riskReward.toFixed(2)}
                            </div>
                          </div>
                          <div className="bg-dark-700 rounded-lg p-3">
                            <div className="text-xs text-gray-400">Ø pro Win-Trade</div>
                            <div className="text-lg font-bold text-green-400">
                              {fmtSim(sim.avgWin)}
                              <span className="text-sm ml-1 text-green-400/70">({sim.avgWinPct.toFixed(1)}%)</span>
                            </div>
                          </div>
                          <div className="bg-dark-700 rounded-lg p-3">
                            <div className="text-xs text-gray-400">Ø pro Loss-Trade</div>
                            <div className="text-lg font-bold text-red-400">
                              {fmtSim(-sim.avgLoss)}
                              <span className="text-sm ml-1 text-red-400/70">({sim.avgLossPct.toFixed(1)}%)</span>
                            </div>
                          </div>
                        </div>

                        {sim.equityCurve.length > 1 && (
                          <div>
                            <div className="text-xs text-gray-400 uppercase tracking-wider mb-1">Kapitalverlauf</div>
                            <EquityCurveChart data={sim.equityCurve} eigenkapital={sim.eigenkapital} gewinn={sim.gewinn} />
                          </div>
                        )}
                      </>
                    )}

                    {/* Trade Table - collapsible */}
                    <div>
                      <button onClick={() => toggleTradeTable(mc.key)}
                        className="w-full flex items-center justify-between py-2 text-left hover:text-white transition-colors group">
                        <span className="text-sm text-gray-400 uppercase tracking-wider group-hover:text-gray-300">
                          {sim ? 'Einzelne Trades' : 'Trades'} ({st.tradeCount})
                          <span className="text-gray-600 ml-2 normal-case tracking-normal">{st.wins}W / {st.losses}L</span>
                        </span>
                        <svg className={`w-4 h-4 text-gray-500 transition-transform ${tradesOpen ? 'rotate-180' : ''}`} fill="none" stroke="currentColor" viewBox="0 0 24 24">
                          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 9l-7 7-7-7" />
                        </svg>
                      </button>

                      {tradesOpen && (
                        <div className="overflow-x-auto max-h-[400px] overflow-y-auto rounded-lg border border-dark-600">
                          {sim ? (
                            /* Sim Trades Table */
                            <table className="w-full text-sm">
                              <thead className="bg-dark-700 sticky top-0 text-gray-400 z-10">
                                <tr>
                                  <SortTH field="symbol" sort={getSimSort(mc.key)} onSort={s => doSetSimSort(mc.key, s)}>Aktie</SortTH>
                                  <SortTH field="entryDate" sort={getSimSort(mc.key)} onSort={s => doSetSimSort(mc.key, s)}>Kauf</SortTH>
                                  <SortTH field="entryPrice" sort={getSimSort(mc.key)} onSort={s => doSetSimSort(mc.key, s)} right>Kaufkurs</SortTH>
                                  <SortTH field="exitDate" sort={getSimSort(mc.key)} onSort={s => doSetSimSort(mc.key, s)}>Verkauf</SortTH>
                                  <SortTH field="exitPrice" sort={getSimSort(mc.key)} onSort={s => doSetSimSort(mc.key, s)} right>Verkaufskurs</SortTH>
                                  <SortTH field="returnPct" sort={getSimSort(mc.key)} onSort={s => doSetSimSort(mc.key, s)} right>Rendite</SortTH>
                                  <SortTH field="profit" sort={getSimSort(mc.key)} onSort={s => doSetSimSort(mc.key, s)} right>Gewinn</SortTH>
                                  <SortTH field="received" sort={getSimSort(mc.key)} onSort={s => doSetSimSort(mc.key, s)} right>Erhalten</SortTH>
                                </tr>
                              </thead>
                              <tbody className="divide-y divide-dark-600/50">
                                {sortSimTrades(sim.trades, getSimSort(mc.key)).map((t, i) => (
                                  <tr key={i} className="hover:bg-dark-700/30 transition-colors">
                                    <td className="px-2 py-1.5">
                                      <span className="font-medium text-white">{t.symbol}</span>
                                    </td>
                                    <td className="px-2 py-1.5 text-gray-300">{fmtDate(t.entryDate)}</td>
                                    <td className="px-2 py-1.5 text-right text-gray-300">{formatPrice(t.entryPrice, t.symbol)}</td>
                                    <td className="px-2 py-1.5">
                                      {t.exitDate ? (
                                        <span className="text-gray-300">{fmtDate(t.exitDate)}</span>
                                      ) : (
                                        <span className="inline-flex items-center gap-1 px-1.5 py-0.5 rounded bg-blue-500/15 text-blue-400 font-medium text-xs">
                                          <span className="w-1 h-1 rounded-full bg-blue-400 animate-pulse" />
                                          OPEN
                                        </span>
                                      )}
                                    </td>
                                    <td className="px-2 py-1.5 text-right text-gray-300">{formatPrice(t.exitPrice, t.symbol)}</td>
                                    <td className={`px-2 py-1.5 text-right font-medium ${t.returnPct >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                                      {t.returnPct >= 0 ? '+' : ''}{t.returnPct.toFixed(1)}%
                                    </td>
                                    <td className={`px-2 py-1.5 text-right font-medium ${t.profit >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                                      {t.profit >= 0 ? '+' : ''}{fmtSim(t.profit)}
                                    </td>
                                    <td className="px-2 py-1.5 text-right text-white font-medium">{fmtSim(t.received)}</td>
                                  </tr>
                                ))}
                              </tbody>
                              <tfoot>
                                <tr className="bg-dark-700/50 font-medium border-t border-dark-500 text-sm">
                                  <td className="px-2 py-2 text-gray-400" colSpan={5}>Gesamt</td>
                                  <td className={`px-2 py-2 text-right ${sim.rendite >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                                    {sim.rendite >= 0 ? '+' : ''}{sim.rendite.toFixed(1)}%
                                  </td>
                                  <td className={`px-2 py-2 text-right ${sim.gewinn >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                                    {sim.gewinn >= 0 ? '+' : ''}{fmtSim(sim.gewinn)}
                                  </td>
                                  <td className="px-2 py-2 text-right text-white">
                                    {fmtSim(sim.trades.reduce((s, t) => s + t.received, 0))}
                                  </td>
                                </tr>
                              </tfoot>
                            </table>
                          ) : (
                            /* Basic Trades Table */
                            <table className="w-full text-sm">
                              <thead className="bg-dark-700 sticky top-0 text-gray-400 z-10">
                                <tr>
                                  <SortTH field="symbol" sort={getTradeSort(mc.key)} onSort={s => doSetTradeSort(mc.key, s)}>Symbol</SortTH>
                                  <SortTH field="entry_date" sort={getTradeSort(mc.key)} onSort={s => doSetTradeSort(mc.key, s)}>BUY</SortTH>
                                  <th className="px-2 py-2 text-right font-medium">Einstieg</th>
                                  <th className="px-2 py-2 font-medium">SELL / OPEN</th>
                                  <th className="px-2 py-2 text-right font-medium">Ausstieg</th>
                                  <SortTH field="return_pct" sort={getTradeSort(mc.key)} onSort={s => doSetTradeSort(mc.key, s)} right>Rendite</SortTH>
                                </tr>
                              </thead>
                              <tbody className="divide-y divide-dark-600/50">
                                {sortBasicTrades(data.trades, getTradeSort(mc.key)).map((t, i) => (
                                  <tr key={i} className="hover:bg-dark-700/30 transition-colors">
                                    <td className="px-2 py-1.5">
                                      <div className="font-medium text-white">{t.symbol}</div>
                                      <div className="text-xs text-gray-500 truncate max-w-[80px]">{t.name}</div>
                                    </td>
                                    <td className="px-2 py-1.5">
                                      <span className="px-1.5 py-0.5 text-xs rounded bg-green-500/20 text-green-400 font-medium">BUY</span>
                                      <div className="text-xs text-gray-400 mt-0.5">{fmtDate(t.entry_date)}</div>
                                    </td>
                                    <td className="px-2 py-1.5 text-right text-gray-300">{formatPrice(t.entry_price, t.symbol)}</td>
                                    <td className="px-2 py-1.5">
                                      {t.status === 'OPEN' ? (
                                        <span className="inline-flex items-center gap-1 px-1.5 py-0.5 rounded bg-blue-500/15 text-blue-400 font-medium text-xs">
                                          <span className="w-1 h-1 rounded-full bg-blue-400 animate-pulse" />
                                          OPEN
                                        </span>
                                      ) : (
                                        <>
                                          <span className="px-1.5 py-0.5 text-xs rounded bg-red-500/20 text-red-400 font-medium">SELL</span>
                                          <div className="text-xs text-gray-400 mt-0.5">{fmtDate(t.exit_date)}</div>
                                        </>
                                      )}
                                    </td>
                                    <td className="px-2 py-1.5 text-right text-gray-300">
                                      {formatPrice(t.status === 'OPEN' ? t.current_price : t.exit_price, t.symbol)}
                                    </td>
                                    <td className={`px-2 py-1.5 text-right font-medium ${(t.return_pct || 0) >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                                      {(t.return_pct || 0) >= 0 ? '+' : ''}{(t.return_pct || 0).toFixed(2)}%
                                    </td>
                                  </tr>
                                ))}
                              </tbody>
                            </table>
                          )}
                        </div>
                      )}
                    </div>
                  </div>
                )}
              </div>
            )
          })}
        </div>
      )}

      {/* Optimierung Section */}
      {!loading && trades.length > 0 && (
        <div className="mt-6 bg-dark-800 rounded-xl border border-dark-600 overflow-hidden">
          <button onClick={() => setOptOpen(!optOpen)}
            className="w-full px-4 py-3 flex items-center justify-between text-left hover:bg-dark-700/30 transition-colors">
            <div className="flex items-center gap-2.5">
              <span className="text-lg">⚙</span>
              <span className="font-semibold text-white text-base">Optimierung</span>
              <span className="text-sm text-gray-500">Filter-Kombinationen finden</span>
            </div>
            <svg className={`w-5 h-5 text-gray-400 transition-transform ${optOpen ? 'rotate-180' : ''}`} fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 9l-7 7-7-7" />
            </svg>
          </button>

          {optOpen && (
            <div className="border-t border-dark-600 p-4 space-y-4">
              {/* Mode Tabs */}
              <div className="flex gap-2 flex-wrap">
                {MODES.map(m => (
                  <button key={m.key} onClick={() => { setOptMode(m.key); setOptLocked({}) }}
                    className={`px-3 py-1.5 text-sm font-medium rounded-lg transition-all ${optMode === m.key
                      ? 'bg-accent-500 text-white'
                      : 'bg-dark-700 text-gray-400 hover:text-white hover:bg-dark-600'}`}>
                    <span className={`inline-block w-2 h-2 rounded-full mr-1.5 ${S[m.key].dot}`} />
                    {m.title}
                  </button>
                ))}
              </div>

              {stockPool.length === 0 ? (
                <p className="text-sm text-gray-500 text-center py-4">Keine Daten für diesen Modus im gewählten Zeitraum.</p>
              ) : (
                <>
                  {/* Presets */}
                  {optPresets.length > 0 && (
                    <div>
                      <div className="text-xs text-gray-500 uppercase tracking-wider mb-2">Vorschläge</div>
                      <div className="grid grid-cols-2 md:grid-cols-4 gap-2">
                        {optPresets.map(p => (
                          <button key={p.name} onClick={() => setOptLocked(p.locked)}
                            className={`text-left px-3 py-2 rounded-lg border transition-all hover:scale-[1.02] ${p.color}`}>
                            <div className="flex items-center gap-1.5 mb-1">
                              <span className="text-base">{p.icon}</span>
                              <span className="text-sm font-semibold">{p.name}</span>
                            </div>
                            <div className="text-xs opacity-70">{p.desc}</div>
                            <div className="flex items-center gap-2 mt-1.5 text-xs font-medium">
                              <span>{p.count} Aktien</span>
                              <span>·</span>
                              <span>{p.returnPa >= 0 ? '+' : ''}{p.returnPa.toFixed(1)}% p.a.</span>
                            </div>
                          </button>
                        ))}
                      </div>
                    </div>
                  )}

                  {/* Sliders */}
                  {[
                    { key: 'trades', label: 'Min Trades', fmt: v => `${Math.round(v)} Trades` },
                    { key: 'winrate', label: 'Min Winrate', fmt: v => `${Math.round(v)}%` },
                    { key: 'rr', label: 'Min R/R', fmt: v => v.toFixed(1) },
                    { key: 'totalReturn', label: 'Min Gesamtrendite', fmt: v => `${v >= 0 ? '+' : ''}${Math.round(v)}%` },
                    { key: 'marketCap', label: 'Min Market Cap', fmt: v => `${Math.round(v)} Mrd` },
                  ].map(sl => {
                    const locked = optLocked[sl.key] != null
                    const range = optResult.ranges[sl.key]
                    const val = locked ? optLocked[sl.key] : optResult.medians[sl.key]
                    return (
                      <div key={sl.key}>
                        <div className="flex items-center justify-between mb-1">
                          <label className="text-sm text-gray-400">{sl.label}</label>
                          <div className="flex items-center gap-2">
                            <span className={`text-sm font-medium ${locked ? 'text-orange-400' : 'text-blue-400'}`}>
                              {locked && <span className="mr-1">🔒</span>}
                              {sl.fmt(val)}
                            </span>
                            {locked && (
                              <button onClick={() => resetOptSlider(sl.key)}
                                className="w-5 h-5 flex items-center justify-center rounded text-gray-500 hover:text-white hover:bg-dark-600 transition-colors text-xs">
                                ×
                              </button>
                            )}
                          </div>
                        </div>
                        <input type="range" min={range.min} max={range.max} step={range.step}
                          value={val}
                          onChange={e => handleOptSlider(sl.key, e.target.value)}
                          className={`w-full h-1.5 rounded-lg appearance-none cursor-pointer ${locked ? 'accent-orange-500' : 'accent-blue-500'}`}
                          style={{ background: `linear-gradient(to right, ${locked ? '#f97316' : '#3b82f6'} ${range.max > range.min ? ((val - range.min) / (range.max - range.min)) * 100 : 0}%, #374151 0%)` }}
                        />
                      </div>
                    )
                  })}

                  {/* Summary */}
                  <div className="grid grid-cols-2 md:grid-cols-5 gap-3">
                    <div className="bg-dark-700 rounded-lg p-3">
                      <div className="text-xs text-gray-400">Aktien / Trades</div>
                      <div className="text-lg font-bold text-white">{optResult.count}</div>
                      <div className="text-xs text-gray-500">{optResult.tradeCount} Trades</div>
                    </div>
                    <div className="bg-dark-700 rounded-lg p-3">
                      <div className="text-xs text-gray-400">Win Rate</div>
                      <div className={`text-lg font-bold ${optResult.winRate >= 50 ? 'text-green-400' : 'text-red-400'}`}>
                        {optResult.winRate.toFixed(1)}%
                      </div>
                    </div>
                    <div className="bg-dark-700 rounded-lg p-3">
                      <div className="text-xs text-gray-400">R/R</div>
                      <div className={`text-lg font-bold ${optResult.riskReward >= 1 ? 'text-green-400' : 'text-orange-400'}`}>
                        {optResult.riskReward === Infinity ? '∞' : optResult.riskReward.toFixed(2)}
                      </div>
                    </div>
                    <div className="bg-dark-700 rounded-lg p-3">
                      <div className="text-xs text-gray-400">Σ Rendite</div>
                      <div className={`text-lg font-bold ${optResult.totalReturn >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                        {optResult.totalReturn >= 0 ? '+' : ''}{optResult.totalReturn.toFixed(1)}%
                      </div>
                    </div>
                    <div className="bg-dark-700 rounded-lg p-3">
                      <div className="text-xs text-gray-400">Rendite p.a.</div>
                      <div className={`text-lg font-bold ${optResult.returnPa >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                        {optResult.returnPa >= 0 ? '+' : ''}{optResult.returnPa.toFixed(1)}%
                      </div>
                    </div>
                  </div>

                  {/* Apply Button */}
                  <button onClick={applyOptFilters}
                    disabled={Object.keys(optLocked).length === 0}
                    className="w-full py-2 text-sm font-medium rounded-lg transition-all bg-accent-500 text-white hover:bg-accent-600 disabled:opacity-40 disabled:cursor-not-allowed">
                    Filter übernehmen
                  </button>
                </>
              )}
            </div>
          )}
        </div>
      )}
    </div>
  )
}

export default Performance
