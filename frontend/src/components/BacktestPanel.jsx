function BacktestPanel({ trades, metrics }) {
  if (!trades || trades.length === 0 || !metrics) {
    return (
      <div className="p-4">
        <h3 className="text-sm font-medium text-white mb-2">System Performance</h3>
        <p className="text-gray-500 text-xs">Select a stock to see backtest results.</p>
      </div>
    )
  }

  const formatDate = (timestamp) => {
    if (!timestamp) return '-'
    const date = new Date(timestamp * 1000)
    return date.toLocaleDateString('en-US', { year: '2-digit', month: 'short' })
  }

  const formatPrice = (price) => {
    if (price === null || price === undefined) return '-'
    return `$${price.toFixed(2)}`
  }

  const formatReturn = (returnPct) => {
    if (returnPct === null || returnPct === undefined) return '-'
    const sign = returnPct >= 0 ? '+' : ''
    return `${sign}${returnPct.toFixed(1)}%`
  }

  return (
    <div className="flex flex-col h-full">
      {/* Header */}
      <div className="px-4 py-3 border-b border-dark-600">
        <h3 className="text-sm font-medium text-white">System Performance</h3>
        <p className="text-xs text-gray-500">BX Trender Strategy</p>
      </div>

      {/* Performance Metrics - Compact Grid */}
      <div className="grid grid-cols-3 gap-2 p-3 border-b border-dark-600">
        <div className="bg-dark-700 rounded p-2 text-center">
          <div className="text-xs text-gray-500">Win Rate</div>
          <div className={`text-base font-bold ${metrics.winRate >= 50 ? 'text-green-400' : 'text-red-400'}`}>
            {metrics.winRate.toFixed(0)}%
          </div>
        </div>
        <div className="bg-dark-700 rounded p-2 text-center">
          <div className="text-xs text-gray-500">R/R</div>
          <div className={`text-base font-bold ${metrics.riskReward >= 1 ? 'text-green-400' : 'text-red-400'}`}>
            {metrics.riskReward.toFixed(1)}
          </div>
        </div>
        <div className="bg-dark-700 rounded p-2 text-center">
          <div className="text-xs text-gray-500">Total</div>
          <div className={`text-base font-bold ${metrics.totalReturn >= 0 ? 'text-green-400' : 'text-red-400'}`}>
            {metrics.totalReturn >= 0 ? '+' : ''}{metrics.totalReturn.toFixed(0)}%
          </div>
        </div>
        <div className="bg-dark-700 rounded p-2 text-center">
          <div className="text-xs text-gray-500">Trades</div>
          <div className="text-base font-bold text-white">{metrics.totalTrades}</div>
        </div>
        <div className="bg-dark-700 rounded p-2 text-center">
          <div className="text-xs text-gray-500">Wins</div>
          <div className="text-base font-bold text-green-400">{metrics.wins}</div>
        </div>
        <div className="bg-dark-700 rounded p-2 text-center">
          <div className="text-xs text-gray-500">Losses</div>
          <div className="text-base font-bold text-red-400">{metrics.losses}</div>
        </div>
      </div>

      {/* Trade History Table - Scrollable */}
      <div className="flex-1 overflow-auto p-3">
        <h4 className="text-xs font-medium text-gray-500 mb-2">TRADE HISTORY</h4>
        <table className="w-full text-xs">
          <thead>
            <tr className="text-left text-gray-500">
              <th className="pb-1 pr-1">BUY</th>
              <th className="pb-1 pr-1">SELL</th>
              <th className="pb-1 text-right">Return</th>
            </tr>
          </thead>
          <tbody>
            {trades.map((trade, idx) => (
              <tr key={idx} className="border-b border-dark-700/50 last:border-0">
                <td className="py-1.5 pr-1">
                  <div className="text-gray-400">{formatDate(trade.entryDate)}</div>
                  <div className="text-green-400 font-medium">{formatPrice(trade.entryPrice)}</div>
                </td>
                <td className="py-1.5 pr-1">
                  {trade.isOpen ? (
                    <div>
                      <span className="px-1.5 py-0.5 rounded bg-blue-500/20 text-blue-400 text-xs">OPEN</span>
                      <div className="text-gray-500 text-xs">{formatPrice(trade.currentPrice)}</div>
                    </div>
                  ) : (
                    <div>
                      <div className="text-gray-400">{formatDate(trade.exitDate)}</div>
                      <div className="text-red-400 font-medium">{formatPrice(trade.exitPrice)}</div>
                    </div>
                  )}
                </td>
                <td className={`py-1.5 text-right font-medium ${
                  trade.returnPct >= 0 ? 'text-green-400' : 'text-red-400'
                }`}>
                  {formatReturn(trade.returnPct)}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  )
}

export default BacktestPanel
