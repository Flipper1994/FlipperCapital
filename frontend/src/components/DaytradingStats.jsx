import { useState, useEffect, useCallback, useMemo, useRef } from 'react'
import { useCurrency } from '../context/CurrencyContext'

const STRATEGY_LABELS = {
  regression_scalping: 'Regression Scalping',
  hybrid_ai_trend: 'NW Bollinger Bands',
  diamond_signals: 'Diamond Signals',
}

function EquityCurve({ trades }) {
  const canvasRef = useRef(null)

  useEffect(() => {
    const canvas = canvasRef.current
    if (!canvas || trades.length === 0) return
    const ctx = canvas.getContext('2d')
    const dpr = window.devicePixelRatio || 1
    const rect = canvas.getBoundingClientRect()
    canvas.width = rect.width * dpr
    canvas.height = rect.height * dpr
    ctx.scale(dpr, dpr)
    const w = rect.width
    const h = rect.height

    // Build equity curve
    const sorted = [...trades].filter(t => t.is_closed).sort((a, b) => new Date(a.close_time) - new Date(b.close_time))
    if (sorted.length === 0) { ctx.clearRect(0, 0, w, h); return }
    const points = [{ x: 0, y: 100 }]
    let eq = 100
    sorted.forEach((t, i) => {
      eq *= (1 + (t.profit_loss_pct || 0) / 100)
      points.push({ x: i + 1, y: eq })
    })

    const minY = Math.min(...points.map(p => p.y)) * 0.95
    const maxY = Math.max(...points.map(p => p.y)) * 1.05
    const rangeY = maxY - minY || 1
    const padL = 50, padR = 15, padT = 15, padB = 30
    const chartW = w - padL - padR
    const chartH = h - padT - padB

    const toX = i => padL + (i / (points.length - 1)) * chartW
    const toY = v => padT + (1 - (v - minY) / rangeY) * chartH

    ctx.clearRect(0, 0, w, h)

    // Grid lines
    ctx.strokeStyle = 'rgba(255,255,255,0.05)'
    ctx.lineWidth = 1
    const gridSteps = 5
    for (let i = 0; i <= gridSteps; i++) {
      const val = minY + (rangeY * i) / gridSteps
      const y = toY(val)
      ctx.beginPath(); ctx.moveTo(padL, y); ctx.lineTo(w - padR, y); ctx.stroke()
      ctx.fillStyle = 'rgba(255,255,255,0.3)'
      ctx.font = '10px monospace'
      ctx.textAlign = 'right'
      ctx.fillText(val.toFixed(1), padL - 5, y + 3)
    }

    // Baseline at 100
    const y100 = toY(100)
    ctx.strokeStyle = 'rgba(255,255,255,0.15)'
    ctx.setLineDash([4, 4])
    ctx.beginPath(); ctx.moveTo(padL, y100); ctx.lineTo(w - padR, y100); ctx.stroke()
    ctx.setLineDash([])

    // Fill gradient
    const lastY = points[points.length - 1].y
    const isUp = lastY >= 100
    const grad = ctx.createLinearGradient(0, padT, 0, padT + chartH)
    grad.addColorStop(0, isUp ? 'rgba(34,197,94,0.15)' : 'rgba(239,68,68,0.15)')
    grad.addColorStop(1, 'rgba(0,0,0,0)')
    ctx.beginPath()
    ctx.moveTo(toX(0), toY(points[0].y))
    points.forEach((p, i) => ctx.lineTo(toX(i), toY(p.y)))
    ctx.lineTo(toX(points.length - 1), padT + chartH)
    ctx.lineTo(toX(0), padT + chartH)
    ctx.closePath()
    ctx.fillStyle = grad
    ctx.fill()

    // Line
    ctx.strokeStyle = isUp ? '#22c55e' : '#ef4444'
    ctx.lineWidth = 2
    ctx.lineJoin = 'round'
    ctx.beginPath()
    points.forEach((p, i) => {
      const x = toX(i), y = toY(p.y)
      i === 0 ? ctx.moveTo(x, y) : ctx.lineTo(x, y)
    })
    ctx.stroke()

    // X-axis labels
    ctx.fillStyle = 'rgba(255,255,255,0.3)'
    ctx.font = '9px monospace'
    ctx.textAlign = 'center'
    const labelCount = Math.min(6, sorted.length)
    for (let i = 0; i < labelCount; i++) {
      const idx = Math.floor((i / (labelCount - 1)) * (sorted.length - 1))
      const t = sorted[idx]
      const date = new Date(t.close_time)
      ctx.fillText(`${date.getDate()}.${date.getMonth() + 1}`, toX(idx + 1), h - 5)
    }

    // End value label
    ctx.fillStyle = isUp ? '#22c55e' : '#ef4444'
    ctx.font = 'bold 11px monospace'
    ctx.textAlign = 'left'
    ctx.fillText(`${lastY.toFixed(1)}`, toX(points.length - 1) + 5, toY(lastY) + 4)
  }, [trades])

  return <canvas ref={canvasRef} className="w-full h-full" style={{ display: 'block' }} />
}

function DaytradingStats({ token, isAdmin }) {
  const [sessions, setSessions] = useState([])
  const [selectedId, setSelectedId] = useState(null)
  const [sessionData, setSessionData] = useState(null)
  const [loading, setLoading] = useState(true)
  const [detailLoading, setDetailLoading] = useState(false)
  const [sortField, setSortField] = useState('total_return')
  const [sortDir, setSortDir] = useState('desc')
  const [tradeSortField, setTradeSortField] = useState('time')
  const [tradeSortDir, setTradeSortDir] = useState('desc')
  const [dateFilter, setDateFilter] = useState('all') // 'all' | 'day' | 'week' | 'month' | 'custom'
  const [customDate, setCustomDate] = useState('') // YYYY-MM-DD for day
  const [customWeek, setCustomWeek] = useState('') // YYYY-Www for week
  const [customMonth, setCustomMonth] = useState('') // YYYY-MM for month
  const { formatPrice, currency } = useCurrency()

  const headers = token ? { 'Authorization': `Bearer ${token}` } : {}

  const fetchSessions = useCallback(async () => {
    try {
      const res = await fetch('/api/trading/live/sessions', { headers })
      if (res.ok) {
        const data = await res.json()
        const list = data.sessions || []
        setSessions(list)
        if (list.length > 0 && !selectedId) {
          const active = list.find(s => s.is_active)
          setSelectedId(active ? active.id : list[0].id)
        }
      }
    } catch { /* ignore */ }
    setLoading(false)
  }, [token])

  const fetchSession = useCallback(async (id) => {
    if (!id) return
    setDetailLoading(true)
    try {
      const res = await fetch(`/api/trading/live/session/${id}`, { headers })
      if (res.ok) {
        const data = await res.json()
        setSessionData(data)
      }
    } catch { /* ignore */ }
    setDetailLoading(false)
  }, [token])

  useEffect(() => { fetchSessions() }, [])
  useEffect(() => { if (selectedId) fetchSession(selectedId) }, [selectedId])

  const allPositions = sessionData?.positions || []

  // Date range filter
  const dateRange = useMemo(() => {
    if (dateFilter === 'all') return null
    let start, end
    if (dateFilter === 'day' && customDate) {
      start = new Date(customDate); start.setHours(0, 0, 0, 0)
      end = new Date(customDate); end.setHours(23, 59, 59, 999)
    } else if (dateFilter === 'week' && customWeek) {
      const [y, w] = customWeek.split('-W').map(Number)
      const jan1 = new Date(y, 0, 1)
      const days = (w - 1) * 7 - jan1.getDay() + 1
      start = new Date(y, 0, 1 + days); start.setHours(0, 0, 0, 0)
      end = new Date(start); end.setDate(end.getDate() + 6); end.setHours(23, 59, 59, 999)
    } else if (dateFilter === 'month' && customMonth) {
      const [y, m] = customMonth.split('-').map(Number)
      start = new Date(y, m - 1, 1, 0, 0, 0, 0)
      end = new Date(y, m, 0, 23, 59, 59, 999)
    } else {
      return null
    }
    return { start, end }
  }, [dateFilter, customDate, customWeek, customMonth])

  const positions = useMemo(() => {
    if (!dateRange) return allPositions
    return allPositions.filter(p => {
      const t = new Date(p.is_closed ? p.close_time : p.entry_time)
      return t >= dateRange.start && t <= dateRange.end
    })
  }, [allPositions, dateRange])

  const dateFilterLabel = useMemo(() => {
    if (dateFilter === 'all') return null
    if (dateFilter === 'day' && customDate) return new Date(customDate).toLocaleDateString('de-DE', { day: '2-digit', month: '2-digit', year: 'numeric' })
    if (dateFilter === 'week' && customWeek) return `KW ${customWeek.split('-W')[1]} / ${customWeek.split('-W')[0]}`
    if (dateFilter === 'month' && customMonth) { const [y, m] = customMonth.split('-'); return new Date(y, m - 1).toLocaleDateString('de-DE', { month: 'long', year: 'numeric' }) }
    return null
  }, [dateFilter, customDate, customWeek, customMonth])

  const closedPositions = positions.filter(p => p.is_closed)
  const openPositions = positions.filter(p => !p.is_closed && p.alpaca_order_id)

  // === KPI Calculations ===
  const stats = useMemo(() => {
    if (positions.length === 0) return null
    const all = positions.filter(p => p.is_closed || p.profit_loss_pct != null)
    const totalPnl = all.reduce((s, p) => s + (p.profit_loss_amt || 0), 0)
    const totalInvested = all.reduce((s, p) => s + (p.invested_amount || 0), 0)
    const rendite = totalInvested > 0 ? (totalPnl / totalInvested) * 100 : 0
    const wins = all.filter(p => (p.profit_loss_pct || 0) > 0)
    const losses = all.filter(p => (p.profit_loss_pct || 0) <= 0)
    const winRate = all.length > 0 ? (wins.length / all.length) * 100 : 0
    const avgWin = wins.length > 0 ? wins.reduce((s, p) => s + p.profit_loss_pct, 0) / wins.length : 0
    const avgLoss = losses.length > 0 ? losses.reduce((s, p) => s + p.profit_loss_pct, 0) / losses.length : 0
    const rr = avgLoss !== 0 ? Math.abs(avgWin / avgLoss) : 0
    const grossWin = wins.reduce((s, p) => s + (p.profit_loss_amt || 0), 0)
    const grossLoss = Math.abs(losses.reduce((s, p) => s + (p.profit_loss_amt || 0), 0))
    const profitFactor = grossLoss > 0 ? grossWin / grossLoss : grossWin > 0 ? Infinity : 0
    const avgReturn = all.length > 0 ? all.reduce((s, p) => s + (p.profit_loss_pct || 0), 0) / all.length : 0
    const best = all.length > 0 ? Math.max(...all.map(p => p.profit_loss_pct || 0)) : 0
    const worst = all.length > 0 ? Math.min(...all.map(p => p.profit_loss_pct || 0)) : 0

    // Max drawdown on equity curve
    const sorted = [...closedPositions].sort((a, b) => new Date(a.close_time) - new Date(b.close_time))
    let eq = 100, peak = 100, maxDD = 0
    sorted.forEach(t => {
      eq *= (1 + (t.profit_loss_pct || 0) / 100)
      if (eq > peak) peak = eq
      const dd = (peak - eq) / peak * 100
      if (dd > maxDD) maxDD = dd
    })

    // Avg holding duration
    const durations = closedPositions.filter(p => p.entry_time && p.close_time).map(p =>
      (new Date(p.close_time) - new Date(p.entry_time)) / (1000 * 60)
    )
    const avgDuration = durations.length > 0 ? durations.reduce((a, b) => a + b, 0) / durations.length : 0

    // Streaks
    let winStreak = 0, lossStreak = 0, maxWinStreak = 0, maxLossStreak = 0
    sorted.forEach(t => {
      if ((t.profit_loss_pct || 0) > 0) {
        winStreak++; lossStreak = 0
        if (winStreak > maxWinStreak) maxWinStreak = winStreak
      } else {
        lossStreak++; winStreak = 0
        if (lossStreak > maxLossStreak) maxLossStreak = lossStreak
      }
    })

    return {
      totalPnl, rendite, winRate, rr, profitFactor, avgReturn,
      totalTrades: all.length, totalClosed: closedPositions.length,
      wins: wins.length, losses: losses.length,
      avgWin, avgLoss, best, worst, maxDD, avgDuration,
      maxWinStreak, maxLossStreak, totalInvested, equity: eq
    }
  }, [positions])

  // === Per-Symbol Breakdown ===
  const symbolBreakdown = useMemo(() => {
    const map = {}
    positions.forEach(p => {
      if (!map[p.symbol]) map[p.symbol] = { symbol: p.symbol, trades: 0, wins: 0, totalReturn: 0, pnl: 0 }
      map[p.symbol].trades++
      map[p.symbol].totalReturn += p.profit_loss_pct || 0
      map[p.symbol].pnl += p.profit_loss_amt || 0
      if ((p.profit_loss_pct || 0) > 0) map[p.symbol].wins++
    })
    return Object.values(map).map(s => ({
      ...s,
      winRate: s.trades > 0 ? (s.wins / s.trades) * 100 : 0,
      avgReturn: s.trades > 0 ? s.totalReturn / s.trades : 0,
    }))
  }, [positions])

  const sortedSymbols = useMemo(() => {
    return [...symbolBreakdown].sort((a, b) => {
      const mul = sortDir === 'asc' ? 1 : -1
      return (a[sortField] - b[sortField]) * mul
    })
  }, [symbolBreakdown, sortField, sortDir])

  const toggleSymbolSort = (field) => {
    if (sortField === field) setSortDir(d => d === 'asc' ? 'desc' : 'asc')
    else { setSortField(field); setSortDir('desc') }
  }

  // === Trade History ===
  const tradeHistory = useMemo(() => {
    return closedPositions.map(p => ({
      ...p,
      time: p.close_time,
      duration: p.entry_time && p.close_time ? (new Date(p.close_time) - new Date(p.entry_time)) / (1000 * 60) : 0,
    })).sort((a, b) => {
      const mul = tradeSortDir === 'asc' ? 1 : -1
      if (tradeSortField === 'time') return (new Date(a.time) - new Date(b.time)) * mul
      if (tradeSortField === 'symbol') return a.symbol.localeCompare(b.symbol) * mul
      if (tradeSortField === 'pnl') return ((a.profit_loss_pct || 0) - (b.profit_loss_pct || 0)) * mul
      if (tradeSortField === 'duration') return (a.duration - b.duration) * mul
      return 0
    })
  }, [closedPositions, tradeSortField, tradeSortDir])

  const toggleTradeSort = (field) => {
    if (tradeSortField === field) setTradeSortDir(d => d === 'asc' ? 'desc' : 'asc')
    else { setTradeSortField(field); setTradeSortDir('desc') }
  }

  const formatDuration = (mins) => {
    if (mins < 60) return `${Math.round(mins)}m`
    if (mins < 1440) return `${Math.floor(mins / 60)}h ${Math.round(mins % 60)}m`
    return `${Math.floor(mins / 1440)}d ${Math.floor((mins % 1440) / 60)}h`
  }

  const formatTime = (ts) => {
    if (!ts) return '-'
    return new Date(ts).toLocaleString('de-DE', { day: '2-digit', month: '2-digit', year: '2-digit', hour: '2-digit', minute: '2-digit' })
  }

  const selectedSession = sessions.find(s => s.id === selectedId)

  // === PDF Export ===
  const exportPDF = useCallback(() => {
    if (!stats || !selectedSession) return
    const s = stats
    const session = selectedSession
    const title = `Trading Report — ${session.name || `Session #${session.id}`}`
    const subtitle = `${STRATEGY_LABELS[session.strategy] || session.strategy} | ${session.interval} | ${dateFilterLabel || 'Gesamtzeitraum'}`
    const now = new Date().toLocaleString('de-DE')

    const sorted = [...closedPositions].sort((a, b) => new Date(b.close_time) - new Date(a.close_time))

    const html = `<!DOCTYPE html>
<html><head><meta charset="utf-8"><title>${title}</title>
<style>
  * { margin: 0; padding: 0; box-sizing: border-box; }
  body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif; background: #0a0a0f; color: #e5e7eb; padding: 32px; font-size: 12px; }
  .header { text-align: center; margin-bottom: 24px; border-bottom: 1px solid #1f2937; padding-bottom: 16px; }
  .header h1 { font-size: 22px; color: #fff; margin-bottom: 4px; }
  .header p { color: #9ca3af; font-size: 11px; }
  .kpi-grid { display: grid; grid-template-columns: repeat(7, 1fr); gap: 8px; margin-bottom: 20px; }
  .kpi { background: #111827; border: 1px solid #1f2937; border-radius: 8px; padding: 10px; text-align: center; }
  .kpi .label { font-size: 9px; color: #6b7280; text-transform: uppercase; letter-spacing: 0.5px; }
  .kpi .value { font-size: 16px; font-weight: 700; margin-top: 2px; }
  .kpi .sub { font-size: 9px; opacity: 0.7; margin-top: 1px; }
  .ext-grid { display: grid; grid-template-columns: repeat(6, 1fr); gap: 6px; margin-bottom: 20px; }
  .ext { background: #111827; border: 1px solid #1f2937; border-radius: 6px; padding: 8px; text-align: center; }
  .ext .label { font-size: 8px; color: #6b7280; text-transform: uppercase; }
  .ext .value { font-size: 13px; font-weight: 700; margin-top: 2px; }
  .green { color: #22c55e; } .red { color: #ef4444; } .yellow { color: #eab308; } .white { color: #fff; } .gray { color: #9ca3af; }
  h2 { font-size: 14px; color: #fff; margin: 20px 0 10px; }
  table { width: 100%; border-collapse: collapse; font-size: 10px; }
  thead th { text-align: left; padding: 6px 8px; color: #6b7280; border-bottom: 1px solid #1f2937; font-weight: 500; }
  tbody td { padding: 5px 8px; border-bottom: 1px solid #111827; }
  tr:nth-child(even) { background: rgba(17,24,39,0.5); }
  .badge { display: inline-block; padding: 1px 5px; border-radius: 3px; font-size: 9px; font-weight: 600; margin-left: 4px; }
  .badge-alpaca { background: rgba(168,85,247,0.2); color: #a855f7; }
  .badge-tp { background: rgba(34,197,94,0.2); color: #22c55e; }
  .badge-sl { background: rgba(239,68,68,0.2); color: #ef4444; }
  .badge-signal { background: rgba(234,179,8,0.2); color: #eab308; }
  .badge-other { background: rgba(107,114,128,0.2); color: #9ca3af; }
  .footer { text-align: center; margin-top: 30px; padding-top: 12px; border-top: 1px solid #1f2937; color: #4b5563; font-size: 9px; }
  @media print { body { padding: 16px; } .kpi-grid { grid-template-columns: repeat(7, 1fr); } }
</style></head><body>
<div class="header">
  <h1>${title}</h1>
  <p>${subtitle}</p>
  <p style="margin-top:4px">Erstellt: ${now}</p>
</div>
<div class="kpi-grid">
  <div class="kpi"><div class="label">Gesamtrendite</div><div class="value ${s.rendite >= 0 ? 'green' : 'red'}">${s.rendite >= 0 ? '+' : ''}${s.rendite.toFixed(2)}%</div><div class="sub ${s.rendite >= 0 ? 'green' : 'red'}">(${s.totalPnl >= 0 ? '+' : ''}${s.totalPnl.toFixed(2)}\u20AC)</div></div>
  <div class="kpi"><div class="label">Win Rate</div><div class="value ${s.winRate >= 50 ? 'green' : 'red'}">${s.winRate.toFixed(0)}%</div><div class="sub gray">${s.wins}W / ${s.losses}L</div></div>
  <div class="kpi"><div class="label">Risk / Reward</div><div class="value ${s.rr >= 1 ? 'green' : 'red'}">${s.rr.toFixed(2)}</div></div>
  <div class="kpi"><div class="label">Profit Factor</div><div class="value ${s.profitFactor >= 1 ? 'green' : 'red'}">${s.profitFactor === Infinity ? '\u221E' : s.profitFactor.toFixed(2)}</div></div>
  <div class="kpi"><div class="label">\u00D8 / Trade</div><div class="value ${s.avgReturn >= 0 ? 'green' : 'red'}">${s.avgReturn >= 0 ? '+' : ''}${s.avgReturn.toFixed(2)}%</div></div>
  <div class="kpi"><div class="label">Trades</div><div class="value white">${s.totalTrades}</div><div class="sub gray">${s.totalClosed} closed</div></div>
  <div class="kpi"><div class="label">Investiert</div><div class="value gray">${s.totalInvested.toFixed(0)}\u20AC</div></div>
</div>
<div class="ext-grid">
  <div class="ext"><div class="label">\u00D8 Win</div><div class="value green">${s.avgWin > 0 ? '+' + s.avgWin.toFixed(2) + '%' : '-'}</div></div>
  <div class="ext"><div class="label">\u00D8 Loss</div><div class="value red">${s.avgLoss < 0 ? s.avgLoss.toFixed(2) + '%' : '-'}</div></div>
  <div class="ext"><div class="label">Best Trade</div><div class="value green">+${s.best.toFixed(2)}%</div></div>
  <div class="ext"><div class="label">Worst Trade</div><div class="value red">${s.worst.toFixed(2)}%</div></div>
  <div class="ext"><div class="label">Max Drawdown</div><div class="value ${s.maxDD > 5 ? 'red' : 'yellow'}">${s.maxDD > 0 ? '-' : ''}${s.maxDD.toFixed(2)}%</div></div>
  <div class="ext"><div class="label">\u00D8 Haltedauer</div><div class="value gray">${formatDuration(s.avgDuration)}</div></div>
</div>
<h2>Trade History (${sorted.length} Trades)</h2>
<table>
<thead><tr><th>Datum</th><th>Symbol</th><th>Dir</th><th>Entry</th><th>Exit</th><th>Stk</th><th style="text-align:right">Rendite</th><th style="text-align:right">P&L</th><th>Reason</th><th style="text-align:right">Dauer</th></tr></thead>
<tbody>
${sorted.map(p => {
  const dur = p.entry_time && p.close_time ? (new Date(p.close_time) - new Date(p.entry_time)) / (1000 * 60) : 0
  const pct = p.profit_loss_pct || 0
  const amt = p.profit_loss_amt || 0
  const reasonClass = p.close_reason === 'TP' ? 'badge-tp' : p.close_reason === 'SL' ? 'badge-sl' : p.close_reason === 'SIGNAL' ? 'badge-signal' : 'badge-other'
  return `<tr>
    <td style="color:#6b7280">${formatTime(p.close_time)}</td>
    <td style="color:#60a5fa;font-weight:600">${p.symbol}${p.alpaca_order_id ? '<span class="badge badge-alpaca">A</span>' : ''}</td>
    <td class="${p.direction === 'LONG' ? 'green' : 'red'}">${p.direction}</td>
    <td style="color:#9ca3af">${p.entry_price?.toFixed(2)}</td>
    <td style="color:#9ca3af">${p.close_price?.toFixed(2)}</td>
    <td>${p.quantity || '-'}</td>
    <td style="text-align:right;font-weight:600" class="${pct >= 0 ? 'green' : 'red'}">${pct >= 0 ? '+' : ''}${pct.toFixed(2)}%</td>
    <td style="text-align:right" class="${amt >= 0 ? 'green' : 'red'}">${amt >= 0 ? '+' : ''}${amt.toFixed(2)}\u20AC</td>
    <td><span class="badge ${reasonClass}">${p.close_reason || '-'}</span></td>
    <td style="text-align:right;color:#6b7280">${formatDuration(dur)}</td>
  </tr>`
}).join('')}
</tbody></table>
<div class="footer">FlipperCapital — Automatisiertes Trading System — ${now}</div>
</body></html>`

    const blob = new Blob([html], { type: 'text/html' })
    const url = URL.createObjectURL(blob)
    const w = window.open(url, '_blank')
    if (w) {
      w.onload = () => { setTimeout(() => w.print(), 500) }
    }
    setTimeout(() => URL.revokeObjectURL(url), 10000)
  }, [stats, selectedSession, closedPositions, dateFilterLabel])

  if (loading) {
    return (
      <div className="min-h-screen bg-dark-900 flex items-center justify-center">
        <div className="text-gray-500 text-sm animate-pulse">Lade Sessions...</div>
      </div>
    )
  }

  if (sessions.length === 0) {
    return (
      <div className="min-h-screen bg-dark-900 flex items-center justify-center p-4">
        <div className="text-center">
          <div className="text-4xl mb-3 opacity-30">&#x1F4CA;</div>
          <div className="text-gray-400 text-lg font-medium mb-1">Keine Trading Sessions</div>
          <div className="text-gray-600 text-sm">Starte eine Live Trading Session um Statistiken zu sehen.</div>
        </div>
      </div>
    )
  }

  return (
    <div className="min-h-screen bg-dark-900 p-4 md:p-6 max-w-7xl mx-auto">
      {/* Header */}
      <div className="mb-6">
        <h1 className="text-2xl font-bold text-white">Daytrading Statistiken</h1>
        <p className="text-sm text-gray-500 mt-1">Performance-Nachweis aller Live Trading Sessions</p>
      </div>

      {/* Session Selector */}
      <div className="bg-dark-800 rounded-lg border border-dark-600 p-4 mb-4">
        <div className="flex flex-col md:flex-row md:items-center gap-3">
          <label className="text-xs text-gray-400 font-medium whitespace-nowrap">Session:</label>
          <select
            value={selectedId || ''}
            onChange={e => setSelectedId(Number(e.target.value))}
            className="flex-1 bg-dark-700 border border-dark-500 rounded-lg px-3 py-2 text-sm text-white focus:border-accent-500 focus:outline-none"
          >
            {sessions.map(s => (
              <option key={s.id} value={s.id}>
                {s.is_active ? '\u25CF ' : ''}{s.name || `#${s.id}`} — {STRATEGY_LABELS[s.strategy] || s.strategy} ({s.interval}) — {s.total_trades || 0} Trades
              </option>
            ))}
          </select>
          {selectedSession && (
            <div className="flex items-center gap-2 text-[10px] text-gray-500 flex-wrap">
              <span className={`w-2 h-2 rounded-full ${selectedSession.is_active ? 'bg-green-400 animate-pulse' : 'bg-gray-600'}`} />
              <span>{selectedSession.is_active ? 'Aktiv' : 'Beendet'}</span>
              <span className="text-gray-700">|</span>
              <span>{formatTime(selectedSession.started_at)}</span>
              {selectedSession.stopped_at && <><span className="text-gray-700">—</span><span>{formatTime(selectedSession.stopped_at)}</span></>}
              <span className="text-gray-700">|</span>
              <span>{selectedSession.total_polls || 0} Scans</span>
            </div>
          )}
        </div>

        {/* Date Range Filter + PDF Export */}
        <div className="flex flex-col md:flex-row md:items-center gap-3 mt-3 pt-3 border-t border-dark-600">
          <div className="flex items-center gap-2 flex-wrap">
            <span className="text-xs text-gray-400 font-medium">Zeitraum:</span>
            {[
              { key: 'all', label: 'Gesamt' },
              { key: 'day', label: 'Tag' },
              { key: 'week', label: 'Woche' },
              { key: 'month', label: 'Monat' },
            ].map(opt => (
              <button
                key={opt.key}
                onClick={() => setDateFilter(opt.key)}
                className={`px-2.5 py-1 rounded text-xs font-medium transition-colors ${
                  dateFilter === opt.key
                    ? 'bg-accent-500/20 text-accent-400 border border-accent-500/30'
                    : 'bg-dark-700 text-gray-400 hover:text-white border border-dark-600'
                }`}
              >
                {opt.label}
              </button>
            ))}
          </div>
          {dateFilter === 'day' && (
            <input type="date" value={customDate} onChange={e => setCustomDate(e.target.value)}
              className="bg-dark-700 border border-dark-500 rounded px-2 py-1 text-xs text-white focus:border-accent-500 focus:outline-none" />
          )}
          {dateFilter === 'week' && (
            <input type="week" value={customWeek} onChange={e => setCustomWeek(e.target.value)}
              className="bg-dark-700 border border-dark-500 rounded px-2 py-1 text-xs text-white focus:border-accent-500 focus:outline-none" />
          )}
          {dateFilter === 'month' && (
            <input type="month" value={customMonth} onChange={e => setCustomMonth(e.target.value)}
              className="bg-dark-700 border border-dark-500 rounded px-2 py-1 text-xs text-white focus:border-accent-500 focus:outline-none" />
          )}
          {dateFilterLabel && (
            <span className="text-xs text-accent-400 font-medium">{dateFilterLabel}</span>
          )}
          <div className="md:ml-auto">
            <button
              onClick={exportPDF}
              disabled={!stats}
              className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-medium bg-dark-700 text-gray-300 hover:text-white hover:bg-dark-600 border border-dark-600 transition-colors disabled:opacity-40 disabled:cursor-not-allowed"
            >
              <svg className="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 10v6m0 0l-3-3m3 3l3-3m2 8H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
              </svg>
              PDF Export
            </button>
          </div>
        </div>
      </div>

      {detailLoading && (
        <div className="text-center py-8 text-gray-500 text-sm animate-pulse">Lade Session-Daten...</div>
      )}

      {!detailLoading && stats && (
        <>
          {/* Primary KPIs */}
          <div className="grid grid-cols-2 md:grid-cols-4 lg:grid-cols-7 gap-2 mb-4">
            {[
              { label: 'Gesamtrendite', value: `${stats.rendite >= 0 ? '+' : ''}${stats.rendite.toFixed(2)}%`, sub: `(${stats.totalPnl >= 0 ? '+' : ''}${stats.totalPnl.toFixed(2)}€)`, color: stats.rendite >= 0 ? 'text-green-400' : 'text-red-400' },
              { label: 'Win Rate', value: `${stats.winRate.toFixed(0)}%`, sub: `${stats.wins}W / ${stats.losses}L`, color: stats.winRate >= 50 ? 'text-green-400' : 'text-red-400' },
              { label: 'Risk / Reward', value: stats.rr > 0 ? stats.rr.toFixed(2) : '-', color: stats.rr >= 1 ? 'text-green-400' : stats.rr > 0 ? 'text-red-400' : 'text-gray-400' },
              { label: 'Profit Factor', value: stats.profitFactor === Infinity ? '∞' : stats.profitFactor > 0 ? stats.profitFactor.toFixed(2) : '-', color: stats.profitFactor >= 1 ? 'text-green-400' : stats.profitFactor > 0 ? 'text-red-400' : 'text-gray-400' },
              { label: 'Ø / Trade', value: `${stats.avgReturn >= 0 ? '+' : ''}${stats.avgReturn.toFixed(2)}%`, color: stats.avgReturn >= 0 ? 'text-green-400' : 'text-red-400' },
              { label: 'Trades', value: `${stats.totalTrades}`, sub: `${stats.totalClosed} closed`, color: 'text-white' },
              { label: 'Investiert', value: `${stats.totalInvested.toFixed(0)}€`, color: 'text-gray-300' },
            ].map((m, i) => (
              <div key={i} className="bg-dark-800 rounded-lg border border-dark-600 p-3">
                <div className="text-[10px] text-gray-500 uppercase tracking-wider">{m.label}</div>
                <div className={`text-lg font-bold mt-0.5 ${m.color}`}>{m.value}</div>
                {m.sub && <div className={`text-[10px] mt-0.5 ${m.color} opacity-70`}>{m.sub}</div>}
              </div>
            ))}
          </div>

          {/* Extended Stats */}
          <div className="grid grid-cols-2 md:grid-cols-4 lg:grid-cols-6 gap-2 mb-4">
            {[
              { label: 'Ø Win', value: stats.avgWin > 0 ? `+${stats.avgWin.toFixed(2)}%` : '-', color: 'text-green-400' },
              { label: 'Ø Loss', value: stats.avgLoss < 0 ? `${stats.avgLoss.toFixed(2)}%` : '-', color: 'text-red-400' },
              { label: 'Best Trade', value: `+${stats.best.toFixed(2)}%`, color: 'text-green-400' },
              { label: 'Worst Trade', value: `${stats.worst.toFixed(2)}%`, color: 'text-red-400' },
              { label: 'Max Drawdown', value: stats.maxDD > 0 ? `-${stats.maxDD.toFixed(2)}%` : '0%', color: stats.maxDD > 5 ? 'text-red-400' : 'text-yellow-400' },
              { label: 'Ø Haltedauer', value: formatDuration(stats.avgDuration), color: 'text-gray-300' },
              { label: 'Win Streak', value: `${stats.maxWinStreak}`, color: 'text-green-400' },
              { label: 'Loss Streak', value: `${stats.maxLossStreak}`, color: 'text-red-400' },
              { label: 'Equity', value: stats.equity.toFixed(1), sub: 'Start: 100', color: stats.equity >= 100 ? 'text-green-400' : 'text-red-400' },
            ].map((m, i) => (
              <div key={i} className="bg-dark-800 rounded-lg border border-dark-600 p-2.5">
                <div className="text-[9px] text-gray-500 uppercase tracking-wider">{m.label}</div>
                <div className={`text-sm font-bold mt-0.5 ${m.color}`}>{m.value}</div>
                {m.sub && <div className="text-[9px] text-gray-600 mt-0.5">{m.sub}</div>}
              </div>
            ))}
          </div>

          {/* Equity Curve */}
          {closedPositions.length >= 2 && (
            <div className="bg-dark-800 rounded-lg border border-dark-600 p-4 mb-4">
              <h3 className="text-sm font-medium text-white mb-3">Equity-Kurve</h3>
              <div className="h-48 md:h-64">
                <EquityCurve trades={positions} />
              </div>
            </div>
          )}

          {/* Per-Symbol Breakdown */}
          {symbolBreakdown.length > 0 && (
            <div className="bg-dark-800 rounded-lg border border-dark-600 p-4 mb-4">
              <h3 className="text-sm font-medium text-white mb-3">Per-Symbol Breakdown ({symbolBreakdown.length})</h3>
              {/* Mobile Cards */}
              <div className="md:hidden grid grid-cols-1 gap-2">
                {sortedSymbols.map(s => (
                  <div key={s.symbol} className="bg-dark-700 rounded-lg p-3 border border-dark-600">
                    <div className="flex items-center justify-between mb-1">
                      <span className="text-sm font-bold text-accent-400">{s.symbol}</span>
                      <span className={`text-sm font-bold ${s.totalReturn >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                        {s.totalReturn >= 0 ? '+' : ''}{s.totalReturn.toFixed(2)}%
                      </span>
                    </div>
                    <div className="grid grid-cols-3 gap-2 text-[10px]">
                      <div><span className="text-gray-500">Trades:</span> <span className="text-gray-300">{s.trades}</span></div>
                      <div><span className="text-gray-500">Win:</span> <span className={s.winRate >= 50 ? 'text-green-400' : 'text-red-400'}>{s.winRate.toFixed(0)}%</span></div>
                      <div><span className="text-gray-500">P&L:</span> <span className={s.pnl >= 0 ? 'text-green-400' : 'text-red-400'}>{s.pnl >= 0 ? '+' : ''}{s.pnl.toFixed(2)}€</span></div>
                    </div>
                  </div>
                ))}
              </div>
              {/* Desktop Table */}
              <div className="hidden md:block overflow-auto">
                <table className="w-full text-xs">
                  <thead>
                    <tr className="text-left text-gray-500 border-b border-dark-600">
                      {[
                        { key: 'symbol', label: 'Symbol', align: '' },
                        { key: 'trades', label: 'Trades', align: 'text-right' },
                        { key: 'winRate', label: 'Win Rate', align: 'text-right' },
                        { key: 'avgReturn', label: 'Ø Return', align: 'text-right' },
                        { key: 'totalReturn', label: 'Total Return', align: 'text-right' },
                        { key: 'pnl', label: 'P&L', align: 'text-right' },
                      ].map(col => (
                        <th key={col.key}
                          onClick={() => toggleSymbolSort(col.key)}
                          className={`pb-2 pr-3 cursor-pointer hover:text-gray-300 transition-colors select-none ${col.align}`}
                        >
                          {col.label} {sortField === col.key && (sortDir === 'asc' ? '↑' : '↓')}
                        </th>
                      ))}
                    </tr>
                  </thead>
                  <tbody>
                    {sortedSymbols.map(s => (
                      <tr key={s.symbol} className="border-b border-dark-700/50 hover:bg-dark-700/30 transition-colors">
                        <td className="py-2 pr-3 font-medium text-accent-400">
                          {s.symbol}
                          {s.symbol.includes('.') && <span className="ml-1 text-[10px] px-1 py-0.5 rounded bg-amber-500/20 text-amber-400">Non-US</span>}
                        </td>
                        <td className="py-2 pr-3 text-right text-gray-300">{s.trades}</td>
                        <td className={`py-2 pr-3 text-right font-medium ${s.winRate >= 50 ? 'text-green-400' : 'text-red-400'}`}>{s.winRate.toFixed(0)}%</td>
                        <td className={`py-2 pr-3 text-right ${s.avgReturn >= 0 ? 'text-green-400' : 'text-red-400'}`}>{s.avgReturn >= 0 ? '+' : ''}{s.avgReturn.toFixed(2)}%</td>
                        <td className={`py-2 pr-3 text-right font-medium ${s.totalReturn >= 0 ? 'text-green-400' : 'text-red-400'}`}>{s.totalReturn >= 0 ? '+' : ''}{s.totalReturn.toFixed(2)}%</td>
                        <td className={`py-2 pr-3 text-right ${s.pnl >= 0 ? 'text-green-400' : 'text-red-400'}`}>{s.pnl >= 0 ? '+' : ''}{s.pnl.toFixed(2)}€</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </div>
          )}

          {/* Open Positions (Alpaca only) */}
          {openPositions.length > 0 && (
            <div className="bg-dark-800 rounded-lg border border-dark-600 p-4 mb-4">
              <div className="flex items-center gap-2 mb-3">
                <h3 className="text-sm font-medium text-white">Offene Positionen</h3>
                <span className="text-[10px] px-1.5 py-0.5 rounded bg-purple-500/20 text-purple-400 border border-purple-500/30">ALPACA</span>
                <span className="text-[10px] text-gray-500">({openPositions.length})</span>
              </div>
              {/* Mobile Cards */}
              <div className="md:hidden grid grid-cols-1 gap-2">
                {openPositions.map(p => (
                  <div key={p.id} className="bg-dark-700 rounded-lg p-3 border border-dark-600">
                    <div className="flex items-center justify-between mb-2">
                      <div className="flex items-center gap-2">
                        <span className="text-sm font-bold text-accent-400">{p.symbol}</span>
                        <span className={`text-[10px] font-medium px-1.5 py-0.5 rounded ${p.direction === 'LONG' ? 'bg-green-500/20 text-green-400' : 'bg-red-500/20 text-red-400'}`}>{p.direction}</span>
                      </div>
                      <span className={`text-sm font-bold ${(p.profit_loss_pct || 0) >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                        {(p.profit_loss_pct || 0) >= 0 ? '+' : ''}{(p.profit_loss_pct || 0).toFixed(2)}%
                      </span>
                    </div>
                    <div className="grid grid-cols-2 gap-x-4 gap-y-1 text-[10px]">
                      <div><span className="text-gray-500">Entry:</span> <span className="text-gray-300">{formatPrice(p.entry_price, p.symbol)}</span></div>
                      <div><span className="text-gray-500">Aktuell:</span> <span className="text-white font-medium">{formatPrice(p.current_price, p.symbol)}</span></div>
                      <div><span className="text-gray-500">Seit:</span> <span className="text-gray-300">{formatTime(p.entry_time)}</span></div>
                      <div><span className="text-gray-500">P&L:</span> <span className={(p.profit_loss_amt || 0) >= 0 ? 'text-green-400' : 'text-red-400'}>{(p.profit_loss_amt || 0) >= 0 ? '+' : ''}{(p.profit_loss_amt || 0).toFixed(2)}€</span></div>
                    </div>
                  </div>
                ))}
              </div>
              {/* Desktop Table */}
              <div className="hidden md:block overflow-auto">
                <table className="w-full text-xs">
                  <thead>
                    <tr className="text-left text-gray-500 border-b border-dark-600">
                      <th className="pb-2 pr-3">Symbol</th>
                      <th className="pb-2 pr-3">Dir</th>
                      <th className="pb-2 pr-3">Entry</th>
                      <th className="pb-2 pr-3">Aktuell</th>
                      <th className="pb-2 pr-3 text-right">Stk</th>
                      <th className="pb-2 pr-3 text-right">Rendite</th>
                      <th className="pb-2 text-right">Seit</th>
                    </tr>
                  </thead>
                  <tbody>
                    {openPositions.map(p => (
                      <tr key={p.id} className="border-b border-dark-700/50">
                        <td className="py-2 pr-3 font-medium text-accent-400">{p.symbol}</td>
                        <td className={`py-2 pr-3 font-medium ${p.direction === 'LONG' ? 'text-green-400' : 'text-red-400'}`}>{p.direction}</td>
                        <td className="py-2 pr-3 text-gray-400">{formatPrice(p.entry_price, p.symbol)}</td>
                        <td className="py-2 pr-3 text-white font-medium">{formatPrice(p.current_price, p.symbol)}</td>
                        <td className="py-2 pr-3 text-right text-gray-300">{p.quantity ? `${p.quantity}x` : '-'}</td>
                        <td className={`py-2 pr-3 text-right font-medium ${(p.profit_loss_pct || 0) >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                          {(p.profit_loss_pct || 0) >= 0 ? '+' : ''}{(p.profit_loss_pct || 0).toFixed(2)}%
                          <span className="text-gray-500 font-normal ml-1">({(p.profit_loss_amt || 0) >= 0 ? '+' : ''}{(p.profit_loss_amt || 0).toFixed(2)}€)</span>
                        </td>
                        <td className="py-2 text-right text-gray-500 text-[10px]">{formatTime(p.entry_time)}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </div>
          )}

          {/* Trade History */}
          {tradeHistory.length > 0 && (
            <div className="bg-dark-800 rounded-lg border border-dark-600 p-4 mb-4">
              <h3 className="text-sm font-medium text-white mb-3">Trade History ({tradeHistory.length})</h3>
              {/* Mobile Cards */}
              <div className="md:hidden space-y-2 max-h-[500px] overflow-y-auto">
                {tradeHistory.map(p => (
                  <div key={p.id} className="bg-dark-700 rounded-lg p-3 border border-dark-600">
                    <div className="flex items-center justify-between mb-1.5">
                      <div className="flex items-center gap-1.5">
                        <span className="text-xs font-bold text-accent-400">{p.symbol}</span>
                        <span className={`text-[10px] px-1 py-0.5 rounded ${p.direction === 'LONG' ? 'bg-green-500/20 text-green-400' : 'bg-red-500/20 text-red-400'}`}>{p.direction}</span>
                        {p.alpaca_order_id && <span className="text-[10px] px-1 py-0.5 rounded bg-purple-500/20 text-purple-400">ALPACA</span>}
                        {p.symbol.includes('.') && <span className="text-[10px] px-1 py-0.5 rounded bg-amber-500/20 text-amber-400">Non-US</span>}
                        <span className={`text-[10px] px-1 py-0.5 rounded ${
                          p.close_reason === 'TP' ? 'bg-green-500/20 text-green-400' :
                          p.close_reason === 'SL' ? 'bg-red-500/20 text-red-400' :
                          p.close_reason === 'SIGNAL' ? 'bg-yellow-500/20 text-yellow-400' :
                          'bg-gray-500/20 text-gray-400'
                        }`}>{p.close_reason}</span>
                      </div>
                      <span className={`text-xs font-bold ${(p.profit_loss_pct || 0) >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                        {(p.profit_loss_pct || 0) >= 0 ? '+' : ''}{(p.profit_loss_pct || 0).toFixed(2)}%
                      </span>
                    </div>
                    <div className="grid grid-cols-2 gap-x-4 gap-y-0.5 text-[10px]">
                      <div><span className="text-gray-500">Entry:</span> <span className="text-gray-300">{formatPrice(p.entry_price, p.symbol)}</span></div>
                      <div><span className="text-gray-500">Exit:</span> <span className="text-gray-300">{formatPrice(p.close_price, p.symbol)}</span></div>
                      <div><span className="text-gray-500">P&L:</span> <span className={(p.profit_loss_amt || 0) >= 0 ? 'text-green-400' : 'text-red-400'}>{(p.profit_loss_amt || 0) >= 0 ? '+' : ''}{(p.profit_loss_amt || 0).toFixed(2)}€</span></div>
                      <div><span className="text-gray-500">Dauer:</span> <span className="text-gray-300">{formatDuration(p.duration)}</span></div>
                      <div className="col-span-2 text-gray-600">{formatTime(p.close_time)}</div>
                    </div>
                  </div>
                ))}
              </div>
              {/* Desktop Table */}
              <div className="hidden md:block overflow-auto max-h-[600px]">
                <table className="w-full text-xs">
                  <thead className="sticky top-0 bg-dark-800">
                    <tr className="text-left text-gray-500 border-b border-dark-600">
                      {[
                        { key: 'time', label: 'Datum', align: '' },
                        { key: 'symbol', label: 'Symbol', align: '' },
                        { key: null, label: 'Dir', align: '' },
                        { key: null, label: 'Entry', align: '' },
                        { key: null, label: 'Exit', align: '' },
                        { key: null, label: 'Stk', align: 'text-right' },
                        { key: 'pnl', label: 'Rendite', align: 'text-right' },
                        { key: null, label: 'P&L', align: 'text-right' },
                        { key: null, label: 'Reason', align: 'text-right' },
                        { key: 'duration', label: 'Dauer', align: 'text-right' },
                      ].map((col, i) => (
                        <th key={i}
                          onClick={col.key ? () => toggleTradeSort(col.key) : undefined}
                          className={`pb-2 pr-2 ${col.align} ${col.key ? 'cursor-pointer hover:text-gray-300 transition-colors select-none' : ''}`}
                        >
                          {col.label} {col.key && tradeSortField === col.key && (tradeSortDir === 'asc' ? '↑' : '↓')}
                        </th>
                      ))}
                    </tr>
                  </thead>
                  <tbody>
                    {tradeHistory.map(p => (
                      <tr key={p.id} className="border-b border-dark-700/50 hover:bg-dark-700/30 transition-colors">
                        <td className="py-1.5 pr-2 text-gray-500 text-[10px]">{formatTime(p.close_time)}</td>
                        <td className="py-1.5 pr-2 font-medium text-accent-400">
                          {p.symbol}
                          {p.alpaca_order_id && <span className="ml-1 text-[10px] px-1 py-0.5 rounded bg-purple-500/20 text-purple-400">A</span>}
                          {p.symbol.includes('.') && <span className="ml-1 text-[10px] px-1 py-0.5 rounded bg-amber-500/20 text-amber-400">Non-US</span>}
                        </td>
                        <td className={`py-1.5 pr-2 ${p.direction === 'LONG' ? 'text-green-400' : 'text-red-400'}`}>{p.direction}</td>
                        <td className="py-1.5 pr-2 text-gray-400">{formatPrice(p.entry_price, p.symbol)}</td>
                        <td className="py-1.5 pr-2 text-gray-400">{formatPrice(p.close_price, p.symbol)}</td>
                        <td className="py-1.5 pr-2 text-right text-gray-300">{p.quantity ? `${p.quantity}x` : '-'}</td>
                        <td className={`py-1.5 pr-2 text-right font-medium ${(p.profit_loss_pct || 0) >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                          {(p.profit_loss_pct || 0) >= 0 ? '+' : ''}{(p.profit_loss_pct || 0).toFixed(2)}%
                        </td>
                        <td className={`py-1.5 pr-2 text-right ${(p.profit_loss_amt || 0) >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                          {(p.profit_loss_amt || 0) >= 0 ? '+' : ''}{(p.profit_loss_amt || 0).toFixed(2)}€
                        </td>
                        <td className="py-1.5 pr-2 text-right">
                          <span className={`px-1.5 py-0.5 rounded text-[10px] font-medium ${
                            p.close_reason === 'TP' ? 'bg-green-500/20 text-green-400' :
                            p.close_reason === 'SL' ? 'bg-red-500/20 text-red-400' :
                            p.close_reason === 'SIGNAL' ? 'bg-yellow-500/20 text-yellow-400' :
                            'bg-gray-500/20 text-gray-400'
                          }`}>{p.close_reason}</span>
                        </td>
                        <td className="py-1.5 text-right text-gray-500">{formatDuration(p.duration)}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </div>
          )}
        </>
      )}

      {!detailLoading && !stats && selectedId && (
        <div className="text-center py-12">
          <div className="text-gray-500 text-sm">Keine Trades in dieser Session.</div>
        </div>
      )}
    </div>
  )
}

export default DaytradingStats
