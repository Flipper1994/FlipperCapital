function ArenaBacktestPanel({ metrics, trades, formatPrice, symbol, timeRange, tradeAmount }) {
  if (!metrics || !trades || trades.length === 0) return null

  const formatTime = (ts) => {
    if (!ts) return '-'
    const d = new Date(ts * 1000)
    return d.toLocaleDateString('de-DE', { day: '2-digit', month: '2-digit', year: '2-digit' }) +
      ' ' + d.toLocaleTimeString('de-DE', { hour: '2-digit', minute: '2-digit' })
  }

  const formatReturn = (v) => {
    if (v == null) return '-'
    return `${v >= 0 ? '+' : ''}${v.toFixed(1)}%`
  }

  const fmtPrice = (price) => {
    if (formatPrice && symbol) return formatPrice(price, symbol)
    return `$${price.toFixed(2)}`
  }

  const reasonColor = {
    TP: 'bg-green-500/20 text-green-400',
    SL: 'bg-red-500/20 text-red-400',
    SIGNAL: 'bg-yellow-500/20 text-yellow-400',
    END: 'bg-gray-500/20 text-gray-400',
    TSL: 'bg-red-500/20 text-red-400',
  }

  // Portfolio return with equal position sizing (compounding)
  const closedTrades = trades.filter(t => !t.is_open)
  const sortedClosed = [...closedTrades].sort((a, b) => a.entry_time - b.entry_time)
  let equity = 1.0
  for (const t of sortedClosed) {
    equity *= (1 + t.return_pct / 100)
  }
  const portfolioReturn = (equity - 1) * 100
  const positionSize = tradeAmount > 0 ? tradeAmount : 100
  const portfolioProfit = positionSize * (equity - 1)

  return (
    <div className="bg-dark-800 rounded-lg border border-dark-600 p-4">
      <div className="flex items-center justify-between mb-3">
        <h3 className="text-sm font-medium text-white">Backtest Ergebnisse</h3>
        {timeRange && <span className="text-[10px] text-gray-500">{formatTime(timeRange.start)} — {formatTime(timeRange.end)}</span>}
      </div>

      {/* Metrics Grid */}
      <div className="grid grid-cols-4 gap-2 mb-4">
        <div className="bg-dark-700 rounded p-2 text-center">
          <div className="text-xs text-gray-500">Win Rate</div>
          <div className={`text-base font-bold ${metrics.win_rate >= 50 ? 'text-green-400' : 'text-red-400'}`}>
            {metrics.win_rate.toFixed(0)}%
          </div>
        </div>
        <div className="bg-dark-700 rounded p-2 text-center">
          <div className="text-xs text-gray-500">R/R</div>
          <div className={`text-base font-bold ${metrics.risk_reward >= 1 ? 'text-green-400' : 'text-red-400'}`}>
            {metrics.risk_reward.toFixed(1)}
          </div>
        </div>
        <div className="bg-dark-700 rounded p-2 text-center">
          <div className="text-xs text-gray-500">Total (Summe)</div>
          <div className={`text-base font-bold ${metrics.total_return >= 0 ? 'text-green-400' : 'text-red-400'}`}>
            {metrics.total_return >= 0 ? '+' : ''}{metrics.total_return.toFixed(0)}%
          </div>
        </div>
        <div className="bg-dark-700 rounded p-2 text-center">
          <div className="text-xs text-gray-500">&Oslash;/Trade</div>
          <div className={`text-base font-bold ${metrics.avg_return >= 0 ? 'text-green-400' : 'text-red-400'}`}>
            {formatReturn(metrics.avg_return)}
          </div>
        </div>
        <div className="bg-dark-700 rounded p-2 text-center">
          <div className="text-xs text-gray-500">Trades</div>
          <div className="text-base font-bold text-white">{metrics.total_trades}</div>
        </div>
        <div className="bg-dark-700 rounded p-2 text-center">
          <div className="text-xs text-gray-500">Wins</div>
          <div className="text-base font-bold text-green-400">{metrics.wins}</div>
        </div>
        <div className="bg-dark-700 rounded p-2 text-center">
          <div className="text-xs text-gray-500">Losses</div>
          <div className="text-base font-bold text-red-400">{metrics.losses}</div>
        </div>
        <div className="bg-dark-700 rounded p-2 text-center">
          <div className="text-xs text-gray-500">Max DD</div>
          <div className="text-base font-bold text-red-400">-{metrics.max_drawdown.toFixed(1)}%</div>
        </div>
        <div className="bg-indigo-500/10 border border-indigo-500/30 rounded p-2 text-center col-span-2">
          <div className="text-xs text-indigo-300">Portfolio-Rendite (gleiche Pos.)</div>
          <div className={`text-lg font-bold ${portfolioReturn >= 0 ? 'text-green-400' : 'text-red-400'}`}>
            {portfolioReturn >= 0 ? '+' : ''}{portfolioReturn.toFixed(1)}%
          </div>
        </div>
        <div className="bg-indigo-500/10 border border-indigo-500/30 rounded p-2 text-center col-span-2">
          <div className="text-xs text-indigo-300">Gewinn bei {positionSize}€/Trade</div>
          <div className={`text-lg font-bold ${portfolioProfit >= 0 ? 'text-green-400' : 'text-red-400'}`}>
            {portfolioProfit >= 0 ? '+' : ''}{portfolioProfit.toFixed(2)} €
          </div>
        </div>
      </div>

      {/* Trade Table */}
      <div className="max-h-64 overflow-auto">
        <table className="w-full text-xs">
          <thead>
            <tr className="text-left text-gray-500 border-b border-dark-600">
              <th className="pb-1 pr-2">Dir</th>
              <th className="pb-1 pr-2">Entry</th>
              <th className="pb-1 pr-2">Exit</th>
              <th className="pb-1 pr-2 text-right">Return</th>
              <th className="pb-1 text-right">Reason</th>
            </tr>
          </thead>
          <tbody>
            {[...trades].reverse().map((t, i) => (
              <tr key={i} className="border-b border-dark-700/50 last:border-0">
                <td className="py-1.5 pr-2">
                  <span className={`font-medium ${t.direction === 'LONG' ? 'text-green-400' : 'text-red-400'}`}>
                    {t.direction}
                  </span>
                </td>
                <td className="py-1.5 pr-2 text-gray-400">
                  <div>{fmtPrice(t.entry_price)}</div>
                  <div className="text-gray-600 text-[10px]">{formatTime(t.entry_time)}</div>
                </td>
                <td className="py-1.5 pr-2 text-gray-400">
                  {t.is_open ? (
                    <span className="px-1.5 py-0.5 rounded bg-blue-500/20 text-blue-400">OPEN</span>
                  ) : (
                    <>
                      <div>{fmtPrice(t.exit_price)}</div>
                      <div className="text-gray-600 text-[10px]">{formatTime(t.exit_time)}</div>
                    </>
                  )}
                </td>
                <td className={`py-1.5 pr-2 text-right font-medium ${t.return_pct >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                  {formatReturn(t.return_pct)}
                </td>
                <td className="py-1.5 text-right">
                  <span className={`px-1.5 py-0.5 rounded text-[10px] font-medium ${reasonColor[t.exit_reason] || ''}`}>
                    {t.exit_reason}
                  </span>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  )
}

export default ArenaBacktestPanel
