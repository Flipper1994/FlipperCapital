import { useMemo, useState, useEffect } from 'react'

const WEEKDAYS_SHORT = ['So', 'Mo', 'Di', 'Mi', 'Do', 'Fr', 'Sa']
const MONTHS_SHORT = ['Jan', 'Feb', 'Mär', 'Apr', 'Mai', 'Jun', 'Jul', 'Aug', 'Sep', 'Okt', 'Nov', 'Dez']

function isoWeekNumber(d) {
  const tmp = new Date(Date.UTC(d.getFullYear(), d.getMonth(), d.getDate()))
  tmp.setUTCDate(tmp.getUTCDate() + 4 - (tmp.getUTCDay() || 7))
  const yearStart = new Date(Date.UTC(tmp.getUTCFullYear(), 0, 1))
  return Math.ceil(((tmp - yearStart) / 86400000 + 1) / 7)
}

function isoWeekKey(d) {
  const tmp = new Date(Date.UTC(d.getFullYear(), d.getMonth(), d.getDate()))
  tmp.setUTCDate(tmp.getUTCDate() + 4 - (tmp.getUTCDay() || 7))
  return `${tmp.getUTCFullYear()}-W${String(isoWeekNumber(d)).padStart(2, '0')}`
}

function fmtNum(val) {
  return val.toLocaleString('de-DE', { minimumFractionDigits: 0, maximumFractionDigits: 0 })
}

function formatEUR(val) {
  const sign = val >= 0 ? '+' : ''
  return `${sign}${val.toLocaleString('de-DE', { minimumFractionDigits: 2, maximumFractionDigits: 2 })} €`
}

function formatPct(val) {
  const sign = val >= 0 ? '+' : ''
  return `${sign}${val.toFixed(2)}%`
}

function formatTime(ts) {
  return new Date(ts * 1000).toLocaleString('de-DE', { day: '2-digit', month: '2-digit', year: '2-digit', hour: '2-digit', minute: '2-digit' })
}

function ReturnCell({ value, className = '' }) {
  return (
    <span className={`font-medium ${value >= 0 ? 'text-green-400' : 'text-red-400'} ${className}`}>
      {formatPct(value)}
    </span>
  )
}

function EURCell({ value, className = '' }) {
  return (
    <span className={`${value >= 0 ? 'text-green-400' : 'text-red-400'} ${className}`}>
      {formatEUR(value)}
    </span>
  )
}

function ProfitBar({ value, maxAbs }) {
  if (maxAbs === 0) return null
  const pct = Math.min(Math.abs(value) / maxAbs, 1) * 100
  const color = value >= 0 ? 'bg-green-500/30' : 'bg-red-500/30'
  return (
    <div className="w-16 h-1.5 bg-dark-600 rounded-full overflow-hidden">
      <div className={`h-full rounded-full ${color}`} style={{ width: `${pct}%` }} />
    </div>
  )
}

// --- Trade Detail Overlay ---
function TradeOverlay({ title, trades, tradeAmount, onClose }) {
  const amt = tradeAmount || 100

  // Close on Escape
  useEffect(() => {
    const handler = (e) => { if (e.key === 'Escape') onClose() }
    window.addEventListener('keydown', handler)
    return () => window.removeEventListener('keydown', handler)
  }, [onClose])

  const sorted = useMemo(() =>
    [...trades].sort((a, b) => b.entry_time - a.entry_time),
    [trades]
  )

  const summary = useMemo(() => {
    let invested = 0, profit = 0, wins = 0
    for (const t of trades) {
      invested += amt
      profit += amt * (t.return_pct / 100)
      if (t.return_pct >= 0) wins++
    }
    return { invested, profit, returnPct: invested > 0 ? (profit / invested) * 100 : 0, wins, losses: trades.length - wins }
  }, [trades, amt])

  const reasonColor = {
    'signal': 'bg-blue-500/20 text-blue-400',
    'tsl': 'bg-orange-500/20 text-orange-400',
    'TSL': 'bg-orange-500/20 text-orange-400',
    'ma_filter': 'bg-purple-500/20 text-purple-400',
  }

  return (
    <div className="fixed inset-0 z-[100] flex items-center justify-center" onClick={onClose}>
      <div className="absolute inset-0 bg-black/60 backdrop-blur-sm" />
      <div
        className="relative bg-dark-800 border border-dark-600 rounded-xl shadow-2xl w-[90vw] max-w-3xl max-h-[80vh] flex flex-col"
        onClick={e => e.stopPropagation()}
      >
        {/* Header */}
        <div className="flex items-center justify-between px-4 py-3 border-b border-dark-600">
          <div className="flex items-center gap-3">
            <span className="text-sm font-medium text-white">{title}</span>
            <span className="text-[10px] text-gray-500">{trades.length} Trades</span>
          </div>
          <div className="flex items-center gap-4">
            {/* Summary chips */}
            <div className="flex items-center gap-2 text-[10px]">
              <span className="text-gray-500">Inv: {fmtNum(summary.invested)} €</span>
              <EURCell value={summary.profit} className="text-[10px] font-medium" />
              <ReturnCell value={summary.returnPct} className="text-[10px]" />
              <span className="text-green-400">{summary.wins}W</span>
              <span className="text-red-400">{summary.losses}L</span>
            </div>
            <button onClick={onClose} className="text-gray-500 hover:text-white transition-colors">
              <svg className="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
              </svg>
            </button>
          </div>
        </div>

        {/* Trade table */}
        <div className="overflow-auto flex-1 px-4 py-2">
          <table className="w-full text-xs">
            <thead className="sticky top-0 bg-dark-800">
              <tr className="text-left text-gray-500 border-b border-dark-600">
                <th className="pb-1.5 pr-2 font-normal">Symbol</th>
                <th className="pb-1.5 pr-2 font-normal">Dir</th>
                <th className="pb-1.5 pr-2 font-normal">Entry</th>
                <th className="pb-1.5 pr-2 font-normal">Exit</th>
                <th className="pb-1.5 pr-2 font-normal text-right">Return</th>
                <th className="pb-1.5 pr-2 font-normal text-right">Profit</th>
                <th className="pb-1.5 font-normal text-right">Grund</th>
              </tr>
            </thead>
            <tbody>
              {sorted.map((t, i) => (
                <tr key={i} className="border-b border-dark-700/50 last:border-0 hover:bg-dark-700/30 transition-colors">
                  <td className="py-1.5 pr-2 font-medium text-accent-400">{t.symbol}</td>
                  <td className="py-1.5 pr-2">
                    <span className={`font-medium ${t.direction === 'LONG' ? 'text-green-400' : 'text-red-400'}`}>
                      {t.direction}
                    </span>
                  </td>
                  <td className="py-1.5 pr-2 text-gray-400">
                    <div>${t.entry_price?.toFixed(2)}</div>
                    <div className="text-gray-600 text-[10px]">{formatTime(t.entry_time)}</div>
                  </td>
                  <td className="py-1.5 pr-2 text-gray-400">
                    <div>${t.exit_price?.toFixed(2)}</div>
                    <div className="text-gray-600 text-[10px]">{formatTime(t.exit_time)}</div>
                  </td>
                  <td className={`py-1.5 pr-2 text-right font-medium ${t.return_pct >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                    {formatPct(t.return_pct)}
                  </td>
                  <td className="py-1.5 pr-2 text-right">
                    <EURCell value={amt * (t.return_pct / 100)} className="text-[11px]" />
                  </td>
                  <td className="py-1.5 text-right">
                    <span className={`px-1.5 py-0.5 rounded text-[10px] font-medium ${reasonColor[t.exit_reason] || 'bg-dark-600 text-gray-400'}`}>
                      {t.exit_reason}
                    </span>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  )
}

// --- Main Component ---
export default function ArenaCalendarHeatmap({ trades, tradeAmount = 100 }) {
  const [collapsed, setCollapsed] = useState({})
  const [overlay, setOverlay] = useState(null) // { title, trades }

  const allTrades = useMemo(() =>
    (trades || []).filter(t => t.entry_time),
    [trades]
  )

  const { days, weeks, months, totals, maxAbsEUR } = useMemo(() => {
    if (!allTrades.length) return { days: {}, weeks: {}, months: {}, totals: null, maxAbsEUR: 0 }

    const dayMap = {}
    const amt = tradeAmount || 100

    // Group trades by entry day
    const dayTrades = {}
    for (const t of allTrades) {
      const d = new Date(t.entry_time * 1000)
      const key = `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`
      if (!dayTrades[key]) dayTrades[key] = []
      dayTrades[key].push(t)
    }

    for (const [key, trades] of Object.entries(dayTrades)) {
      const profitEUR = trades.reduce((sum, t) => sum + amt * (t.return_pct / 100), 0)
      const invested = trades.length * amt

      dayMap[key] = {
        date: new Date(trades[0].entry_time * 1000),
        trades: trades.length,
        invested,
        profitEUR,
        returns: trades.map(t => t.return_pct),
      }
    }

    for (const k in dayMap) {
      dayMap[k].returnPct = dayMap[k].invested > 0 ? (dayMap[k].profitEUR / dayMap[k].invested) * 100 : 0
    }

    const weekMap = {}
    for (const [key, val] of Object.entries(dayMap)) {
      const wk = isoWeekKey(val.date)
      if (!weekMap[wk]) weekMap[wk] = { trades: 0, invested: 0, profitEUR: 0, days: [] }
      weekMap[wk].trades += val.trades
      weekMap[wk].invested += val.invested
      weekMap[wk].profitEUR += val.profitEUR
      weekMap[wk].days.push(key)
    }
    for (const k in weekMap) {
      weekMap[k].returnPct = weekMap[k].invested > 0 ? (weekMap[k].profitEUR / weekMap[k].invested) * 100 : 0
    }

    const monthMap = {}
    for (const [key, val] of Object.entries(dayMap)) {
      const mk = key.substring(0, 7)
      if (!monthMap[mk]) monthMap[mk] = { trades: 0, invested: 0, profitEUR: 0, weeks: new Set() }
      monthMap[mk].trades += val.trades
      monthMap[mk].invested += val.invested
      monthMap[mk].profitEUR += val.profitEUR
      monthMap[mk].weeks.add(isoWeekKey(val.date))
    }
    for (const k in monthMap) {
      monthMap[k].returnPct = monthMap[k].invested > 0 ? (monthMap[k].profitEUR / monthMap[k].invested) * 100 : 0
      monthMap[k].weeks = [...monthMap[k].weeks].sort()
    }

    let totalTrades = 0, totalInvested = 0, totalProfit = 0
    for (const k in dayMap) {
      totalTrades += dayMap[k].trades
      totalInvested += dayMap[k].invested
      totalProfit += dayMap[k].profitEUR
    }

    let mx = 0
    for (const k in dayMap) {
      const abs = Math.abs(dayMap[k].profitEUR)
      if (abs > mx) mx = abs
    }

    const numDays = Object.keys(dayMap).length
    const numWeeks = Object.keys(weekMap).length
    const numMonths = Object.keys(monthMap).length

    // Average EUR per period
    const avgDayEUR = numDays > 0 ? totalProfit / numDays : 0
    const avgWeekEUR = numWeeks > 0 ? totalProfit / numWeeks : 0
    const avgMonthEUR = numMonths > 0 ? totalProfit / numMonths : 0

    // Average return % per period (mean of individual period returns)
    const avgDayPct = numDays > 0 ? Object.values(dayMap).reduce((s, d) => s + d.returnPct, 0) / numDays : 0
    const avgWeekPct = numWeeks > 0 ? Object.values(weekMap).reduce((s, w) => s + w.returnPct, 0) / numWeeks : 0
    const avgMonthPct = numMonths > 0 ? Object.values(monthMap).reduce((s, m) => s + m.returnPct, 0) / numMonths : 0

    // Annualized: compound monthly average over 12 months
    const annualPct = avgMonthPct !== 0 ? (Math.pow(1 + avgMonthPct / 100, 12) - 1) * 100 : 0
    const annualEUR = avgMonthEUR * 12

    return {
      days: dayMap, weeks: weekMap, months: monthMap,
      totals: {
        trades: totalTrades, invested: totalInvested, profitEUR: totalProfit,
        returnPct: totalInvested > 0 ? (totalProfit / totalInvested) * 100 : 0,
        avgDayEUR, avgDayPct, avgWeekEUR, avgWeekPct, avgMonthEUR, avgMonthPct, annualPct, annualEUR,
        numDays, numWeeks, numMonths,
      },
      maxAbsEUR: mx,
    }
  }, [allTrades, tradeAmount])

  const sortedMonths = useMemo(() => {
    return Object.entries(months).sort(([a], [b]) => b.localeCompare(a)).map(([mk, mData]) => {
      const [y, m] = mk.split('-')
      const label = `${MONTHS_SHORT[parseInt(m) - 1]} ${y}`

      const sortedWeeks = [...mData.weeks].reverse().map(wk => {
        const wData = weeks[wk]
        // Only show days belonging to THIS month (weeks can span month boundaries)
        const monthDays = wData.days.filter(dk => dk.startsWith(mk))
        const sortedDays = [...monthDays].sort().reverse().map(dk => ({
          key: dk,
          ...days[dk],
          dayLabel: `${WEEKDAYS_SHORT[days[dk].date.getDay()]}, ${days[dk].date.getDate()}.${parseInt(dk.split('-')[1])}.`,
        }))
        // Recalculate week metrics for only the days in this month
        let wTrades = 0, wInvested = 0, wProfitEUR = 0
        for (const dk of monthDays) {
          const d = days[dk]
          wTrades += d.trades
          wInvested += d.invested
          wProfitEUR += d.profitEUR
        }
        const wReturnPct = wInvested > 0 ? (wProfitEUR / wInvested) * 100 : 0
        return { key: `${mk}-${wk}`, trades: wTrades, invested: wInvested, profitEUR: wProfitEUR, returnPct: wReturnPct, days: sortedDays, label: `KW ${wk.split('-W')[1]}` }
      })

      return { key: mk, ...mData, weeks: sortedWeeks, label }
    })
  }, [months, weeks, days])

  // Filter trades for a time period
  const getTradesForDay = (dayKey) => {
    const start = new Date(dayKey)
    start.setHours(0, 0, 0, 0)
    const end = new Date(dayKey)
    end.setHours(23, 59, 59, 999)
    const startUnix = start.getTime() / 1000
    const endUnix = end.getTime() / 1000
    return allTrades.filter(t => t.entry_time >= startUnix && t.entry_time <= endUnix)
  }

  const getTradesForWeek = (weekKey, weekDays) => {
    const dayKeys = new Set(weekDays.map(d => typeof d === 'string' ? d : d.key))
    return allTrades.filter(t => {
      const d = new Date(t.entry_time * 1000)
      const key = `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`
      return dayKeys.has(key)
    })
  }

  const getTradesForMonth = (monthKey) => {
    return allTrades.filter(t => {
      const d = new Date(t.entry_time * 1000)
      const mk = `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}`
      return mk === monthKey
    })
  }

  const openOverlay = (title, tradeList) => {
    if (tradeList.length > 0) setOverlay({ title, trades: tradeList })
  }

  if (!allTrades.length || !totals) return null

  const toggleMonth = (mk) => setCollapsed(prev => ({ ...prev, [mk]: !prev[mk] }))

  return (
    <div>
      {/* Header */}
      <div className="mb-3 space-y-1.5">
        <div className="flex items-center justify-between">
          <span className="text-[10px] text-gray-500">Positionsgröße: {fmtNum(tradeAmount)} €</span>
          <div className="flex items-center gap-3">
            <span className="text-[10px] text-gray-500">{fmtNum(totals.trades)} Trades</span>
            <EURCell value={totals.profitEUR} className="text-xs font-semibold" />
            <ReturnCell value={totals.returnPct} className="text-[10px]" />
          </div>
        </div>
        <div className="flex flex-wrap items-center gap-x-4 gap-y-1 text-[10px] text-gray-500">
          <span>Ø Tag: <EURCell value={totals.avgDayEUR} className="text-[10px]" /> / <ReturnCell value={totals.avgDayPct} className="text-[10px]" /></span>
          <span>Ø Woche: <EURCell value={totals.avgWeekEUR} className="text-[10px]" /> / <ReturnCell value={totals.avgWeekPct} className="text-[10px]" /></span>
          <span>Ø Monat: <EURCell value={totals.avgMonthEUR} className="text-[10px]" /> / <ReturnCell value={totals.avgMonthPct} className="text-[10px]" /></span>
          <span>p.a.: <EURCell value={totals.annualEUR} className="text-[10px]" /> / <ReturnCell value={totals.annualPct} className="text-[10px]" /></span>
        </div>
      </div>

      {/* Table */}
      <div className="overflow-x-auto">
        <table className="w-full text-xs">
          <thead>
            <tr className="text-left text-gray-500 border-b border-dark-600">
              <th className="pb-1.5 pr-2 font-normal">Zeitraum</th>
              <th className="pb-1.5 pr-2 font-normal text-right">Trades</th>
              <th className="pb-1.5 pr-2 font-normal text-right">Investiert</th>
              <th className="pb-1.5 pr-2 font-normal text-right">Gewinn/Verlust</th>
              <th className="pb-1.5 pr-2 font-normal text-right">Rendite</th>
              <th className="pb-1.5 font-normal w-16"></th>
            </tr>
          </thead>
          <tbody>
            {sortedMonths.map(month => (
              <MonthBlock
                key={month.key}
                month={month}
                collapsed={!!collapsed[month.key]}
                onToggle={() => toggleMonth(month.key)}
                onClickMonth={() => openOverlay(month.label, getTradesForMonth(month.key))}
                onClickWeek={(wk) => openOverlay(wk.label, getTradesForWeek(wk.key, wk.days))}
                onClickDay={(day) => openOverlay(day.dayLabel, getTradesForDay(day.key))}
                maxAbsEUR={maxAbsEUR}
              />
            ))}
          </tbody>
          <tfoot>
            <tr className="border-t-2 border-dark-500 font-semibold">
              <td className="pt-2 pr-2 text-white">Gesamt</td>
              <td className="pt-2 pr-2 text-right text-gray-300">{totals.trades}</td>
              <td className="pt-2 pr-2 text-right text-gray-300">{fmtNum(totals.invested)} €</td>
              <td className="pt-2 pr-2 text-right"><EURCell value={totals.profitEUR} /></td>
              <td className="pt-2 pr-2 text-right"><ReturnCell value={totals.returnPct} /></td>
              <td className="pt-2"><ProfitBar value={totals.profitEUR} maxAbs={maxAbsEUR} /></td>
            </tr>
          </tfoot>
        </table>
      </div>

      {/* Trade Detail Overlay */}
      {overlay && (
        <TradeOverlay
          title={overlay.title}
          trades={overlay.trades}
          tradeAmount={tradeAmount}
          onClose={() => setOverlay(null)}
        />
      )}
    </div>
  )
}

function MonthBlock({ month, collapsed, onToggle, onClickMonth, onClickWeek, onClickDay, maxAbsEUR }) {
  return (
    <>
      <tr className="border-t border-dark-600 bg-dark-700/30 cursor-pointer hover:bg-dark-700/60 transition-colors">
        <td className="py-1.5 pr-2" onClick={onToggle}>
          <div className="flex items-center gap-1.5">
            <svg className={`w-3 h-3 text-gray-500 transition-transform ${collapsed ? '' : 'rotate-90'}`} fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 5l7 7-7 7" />
            </svg>
            <span className="font-medium text-white">{month.label}</span>
          </div>
        </td>
        <td className="py-1.5 pr-2 text-right text-gray-300" onClick={onClickMonth}>{month.trades}</td>
        <td className="py-1.5 pr-2 text-right text-gray-300" onClick={onClickMonth}>{fmtNum(month.invested)} €</td>
        <td className="py-1.5 pr-2 text-right" onClick={onClickMonth}><EURCell value={month.profitEUR} /></td>
        <td className="py-1.5 pr-2 text-right" onClick={onClickMonth}><ReturnCell value={month.returnPct} /></td>
        <td className="py-1.5" onClick={onClickMonth}><ProfitBar value={month.profitEUR} maxAbs={maxAbsEUR} /></td>
      </tr>

      {!collapsed && month.weeks.map(week => (
        <WeekBlock key={week.key} week={week} onClickWeek={() => onClickWeek(week)} onClickDay={onClickDay} maxAbsEUR={maxAbsEUR} />
      ))}
    </>
  )
}

function WeekBlock({ week, onClickWeek, onClickDay, maxAbsEUR }) {
  return (
    <>
      <tr className="bg-dark-700/10 cursor-pointer hover:bg-dark-700/30 transition-colors" onClick={onClickWeek}>
        <td className="py-1 pr-2 pl-6 text-gray-400 font-medium">{week.label}</td>
        <td className="py-1 pr-2 text-right text-gray-400">{week.trades}</td>
        <td className="py-1 pr-2 text-right text-gray-400">{fmtNum(week.invested)} €</td>
        <td className="py-1 pr-2 text-right"><EURCell value={week.profitEUR} className="text-[11px]" /></td>
        <td className="py-1 pr-2 text-right"><ReturnCell value={week.returnPct} className="text-[11px]" /></td>
        <td className="py-1"><ProfitBar value={week.profitEUR} maxAbs={maxAbsEUR} /></td>
      </tr>

      {week.days.map(day => (
        <tr key={day.key} className="hover:bg-dark-700/30 transition-colors cursor-pointer" onClick={() => onClickDay(day)}>
          <td className="py-0.5 pr-2 pl-10 text-gray-500">{day.dayLabel}</td>
          <td className="py-0.5 pr-2 text-right text-gray-500">{day.trades}</td>
          <td className="py-0.5 pr-2 text-right text-gray-500">{fmtNum(day.invested)} €</td>
          <td className="py-0.5 pr-2 text-right"><EURCell value={day.profitEUR} className="text-[11px]" /></td>
          <td className="py-0.5 pr-2 text-right"><ReturnCell value={day.returnPct} className="text-[11px]" /></td>
          <td className="py-0.5"><ProfitBar value={day.profitEUR} maxAbs={maxAbsEUR} /></td>
        </tr>
      ))}
    </>
  )
}
