import { useState, useEffect, useCallback } from 'react'
import { Link } from 'react-router-dom'
import { useCurrency } from '../context/CurrencyContext'
import MultiPortfolioChart, { getPortfolioColor } from './MultiPortfolioChart'

function PortfolioCompare() {
  const token = localStorage.getItem('authToken')

  // Show login prompt if not authenticated
  if (!token) {
    return (
      <div className="flex-1 p-4 md:p-6 overflow-auto">
        <div className="max-w-md mx-auto mt-12">
          <div className="bg-dark-800 rounded-xl border border-dark-600 p-8 text-center">
            <div className="w-16 h-16 mx-auto mb-6 bg-dark-700 rounded-full flex items-center justify-center">
              <svg className="w-8 h-8 text-accent-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z" />
              </svg>
            </div>
            <h2 className="text-xl font-semibold text-white mb-2">Anmeldung erforderlich</h2>
            <p className="text-gray-500 mb-6">
              Um Portfolios zu vergleichen, musst du angemeldet sein.
            </p>
            <div className="flex flex-col sm:flex-row gap-3 justify-center">
              <Link
                to="/login"
                className="px-6 py-2.5 bg-accent-500 text-white rounded-lg hover:bg-accent-400 transition-colors font-medium"
              >
                Anmelden
              </Link>
              <Link
                to="/register"
                className="px-6 py-2.5 bg-dark-700 text-gray-300 rounded-lg hover:bg-dark-600 transition-colors font-medium"
              >
                Registrieren
              </Link>
            </div>
          </div>
        </div>
      </div>
    )
  }

  return <PortfolioCompareContent token={token} />
}

function PortfolioCompareContent({ token }) {
  const [portfolios, setPortfolios] = useState([])
  const [loading, setLoading] = useState(true)
  const [expandedPortfolio, setExpandedPortfolio] = useState(null)
  const [colorMap, setColorMap] = useState({})
  const { formatPrice, currency } = useCurrency()

  // Callback to receive color mapping from chart
  const handleColorMap = useCallback((map) => {
    setColorMap(map)
  }, [])

  useEffect(() => {
    fetchPortfolios()
  }, [])

  const fetchPortfolios = async () => {
    try {
      const res = await fetch('/api/portfolios/compare', {
        headers: { 'Authorization': `Bearer ${token}` }
      })
      const data = await res.json()
      // Sort by total return descending
      data.sort((a, b) => b.total_return_pct - a.total_return_pct)
      setPortfolios(data)
    } catch (err) {
      console.error('Failed to fetch portfolios:', err)
    } finally {
      setLoading(false)
    }
  }

  const formatPercent = (value) => {
    if (value === undefined || value === null || isNaN(value)) return '--'
    const sign = value >= 0 ? '+' : ''
    return `${sign}${value.toFixed(2)}%`
  }

  // Get max return for chart scaling
  const maxReturn = Math.max(...portfolios.map(p => Math.abs(p.total_return_pct)), 10)

  // Calculate bar width percentage
  const getBarWidth = (returnPct) => {
    return Math.min(100, (Math.abs(returnPct) / maxReturn) * 100)
  }

  return (
    <div className="flex-1 p-4 md:p-6 overflow-auto">
      <div className="max-w-6xl mx-auto">
        {/* Header */}
        <div className="mb-4 md:mb-6">
          <h1 className="text-xl md:text-2xl font-bold text-white">Portfolio Vergleich</h1>
          <p className="text-gray-500 text-sm">Vergleiche alle Nutzer-Portfolios nach Performance</p>
        </div>

        {loading ? (
          <div className="text-center py-12">
            <div className="w-8 h-8 border-2 border-accent-500 border-t-transparent rounded-full animate-spin mx-auto"></div>
            <p className="text-gray-500 mt-4">Lade Portfolios...</p>
          </div>
        ) : portfolios.length === 0 ? (
          <div className="bg-dark-800 rounded-xl border border-dark-600 p-12 text-center">
            <div className="w-20 h-20 mx-auto mb-6 bg-dark-700 rounded-full flex items-center justify-center">
              <svg className="w-10 h-10 text-gray-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M9 19v-6a2 2 0 00-2-2H5a2 2 0 00-2 2v6a2 2 0 002 2h2a2 2 0 002-2zm0 0V9a2 2 0 012-2h2a2 2 0 012 2v10m-6 0a2 2 0 002 2h2a2 2 0 002-2m0 0V5a2 2 0 012-2h2a2 2 0 012 2v14a2 2 0 01-2 2h-2a2 2 0 01-2-2z" />
              </svg>
            </div>
            <h2 className="text-xl font-semibold text-gray-400 mb-3">Keine Portfolios vorhanden</h2>
            <p className="text-gray-600 max-w-md mx-auto">
              Es gibt noch keine Nutzer mit eingepflegten Aktien.
            </p>
          </div>
        ) : (
          <>
            {/* Multi Portfolio Chart - shown by default */}
            <div className="mb-4 md:mb-6">
              <MultiPortfolioChart
                token={token}
                height={300}
                portfolios={portfolios}
                onColorMap={handleColorMap}
              />
            </div>

            {/* Performance Ranking */}
            <div className="bg-dark-800 rounded-xl border border-dark-600 p-4 md:p-6 mb-4 md:mb-6">
              <div className="flex items-center justify-between mb-4">
                <h2 className="text-lg font-semibold text-white">Performance Ranking</h2>
                <p className="text-xs text-gray-500 hidden md:block">Farben entsprechen dem Chart oben</p>
              </div>
              <div className="space-y-3">
                {portfolios.map((portfolio, index) => {
                  // Get color from colorMap or fallback to index-based color
                  const lineColor = colorMap[portfolio.user_id] || getPortfolioColor(index)

                  return (
                    <div
                      key={portfolio.user_id}
                      className="flex items-center gap-3 p-2 -m-2 rounded-lg"
                    >
                      {/* Color indicator */}
                      <div
                        className="w-4 h-4 rounded-full shrink-0"
                        style={{ backgroundColor: lineColor }}
                        title={`Linienfarbe: ${portfolio.username}`}
                      />

                      {/* Rank */}
                      <div className={`w-8 h-8 rounded-full flex items-center justify-center text-sm font-bold shrink-0 ${
                        index === 0 ? 'bg-yellow-500/20 text-yellow-400' :
                        index === 1 ? 'bg-gray-400/20 text-gray-300' :
                        index === 2 ? 'bg-orange-500/20 text-orange-400' :
                        'bg-dark-700 text-gray-500'
                      }`}>
                        {index + 1}
                      </div>

                      {/* Username */}
                      <div className="w-24 md:w-32 truncate text-sm text-white font-medium text-left">
                        {portfolio.username}
                      </div>

                      {/* Bar Chart */}
                      <div className="flex-1 h-8 bg-dark-700 rounded-lg overflow-hidden relative">
                        <div
                          className="h-full transition-all duration-500"
                          style={{
                            width: `${getBarWidth(portfolio.total_return_pct)}%`,
                            backgroundColor: `${lineColor}99` // Add some transparency
                          }}
                        />
                        <div className="absolute inset-0 flex items-center justify-end pr-2">
                          <span className={`text-sm font-bold ${
                            portfolio.total_return_pct >= 0 ? 'text-green-400' : 'text-red-400'
                          }`}>
                            {formatPercent(portfolio.total_return_pct)}
                          </span>
                        </div>
                      </div>

                      {/* Position count - hidden on mobile */}
                      <div className="hidden md:block w-20 text-right text-xs text-gray-500">
                        {portfolio.position_count} Aktie{portfolio.position_count !== 1 ? 'n' : ''}
                      </div>
                    </div>
                  )
                })}
              </div>
            </div>

            {/* Portfolio Details List */}
            <div className="bg-dark-800 rounded-xl border border-dark-600 overflow-hidden">
              <div className="p-4 border-b border-dark-600">
                <h2 className="text-lg font-semibold text-white">Portfolio Details</h2>
                <p className="text-xs text-gray-500">Klicke auf ein Portfolio um die Positionen zu sehen</p>
              </div>

              {/* Mobile Card View */}
              <div className="md:hidden">
                {portfolios.map((portfolio, index) => {
                  const lineColor = colorMap[portfolio.user_id] || getPortfolioColor(index)
                  return (
                  <div key={portfolio.user_id} className="border-b border-dark-700 last:border-0">
                    <button
                      onClick={() => setExpandedPortfolio(
                        expandedPortfolio === portfolio.user_id ? null : portfolio.user_id
                      )}
                      className="w-full p-4 flex items-center justify-between hover:bg-dark-700/50 transition-colors"
                    >
                      <div className="flex items-center gap-3">
                        {/* Color indicator */}
                        <div
                          className="w-3 h-10 rounded-full shrink-0"
                          style={{ backgroundColor: lineColor }}
                        />
                        <div className="w-10 h-10 bg-accent-500/20 rounded-full flex items-center justify-center">
                          <span className="text-accent-400 font-bold">
                            {portfolio.username.charAt(0).toUpperCase()}
                          </span>
                        </div>
                        <div className="text-left">
                          <div className="text-white font-medium">{portfolio.username}</div>
                          <div className="text-xs text-gray-500">
                            {portfolio.position_count} Position{portfolio.position_count !== 1 ? 'en' : ''}
                          </div>
                        </div>
                      </div>
                      <div className="flex items-center gap-2">
                        <span className={`text-lg font-bold ${
                          portfolio.total_return_pct >= 0 ? 'text-green-400' : 'text-red-400'
                        }`}>
                          {formatPercent(portfolio.total_return_pct)}
                        </span>
                        <svg
                          className={`w-5 h-5 text-gray-400 transition-transform ${
                            expandedPortfolio === portfolio.user_id ? 'rotate-180' : ''
                          }`}
                          fill="none"
                          stroke="currentColor"
                          viewBox="0 0 24 24"
                        >
                          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 9l-7 7-7-7" />
                        </svg>
                      </div>
                    </button>

                    {/* Expanded Positions */}
                    {expandedPortfolio === portfolio.user_id && (
                      <div className="px-4 pb-4 space-y-2">
                        {portfolio.positions.map((pos, idx) => (
                          <div key={idx} className="bg-dark-700 rounded-lg p-3">
                            <div className="flex justify-between items-start mb-2">
                              <div>
                                <span className="font-medium text-white">{pos.symbol}</span>
                                <p className="text-xs text-gray-500 truncate max-w-[150px]">{pos.name}</p>
                              </div>
                              <span className={`text-sm font-bold ${
                                pos.total_return_pct >= 0 ? 'text-green-400' : 'text-red-400'
                              }`}>
                                {formatPercent(pos.total_return_pct)}
                              </span>
                            </div>
                            <div className="grid grid-cols-2 gap-2 text-xs">
                              <div>
                                <span className="text-gray-500">Kaufkurs:</span>
                                <span className="text-gray-300 ml-1">{formatPrice(pos.avg_price_usd)}</span>
                              </div>
                              <div>
                                <span className="text-gray-500">Aktuell:</span>
                                <span className="text-gray-300 ml-1">{formatPrice(pos.current_price)}</span>
                              </div>
                            </div>
                          </div>
                        ))}
                      </div>
                    )}
                  </div>
                )})}
              </div>

              {/* Desktop Table View */}
              <div className="hidden md:block overflow-x-auto">
                <table className="w-full">
                  <thead>
                    <tr className="text-left text-xs text-gray-500 border-b border-dark-600">
                      <th className="p-4 w-4"></th>
                      <th className="p-4">Nutzer</th>
                      <th className="p-4">Positionen</th>
                      <th className="p-4">Aktien</th>
                      <th className="p-4 text-right">Rendite</th>
                    </tr>
                  </thead>
                  <tbody>
                    {portfolios.map((portfolio, index) => {
                      const lineColor = colorMap[portfolio.user_id] || getPortfolioColor(index)
                      return (
                      <>
                        <tr
                          key={portfolio.user_id}
                          onClick={() => setExpandedPortfolio(
                            expandedPortfolio === portfolio.user_id ? null : portfolio.user_id
                          )}
                          className="border-b border-dark-700/50 hover:bg-dark-700/30 transition-colors cursor-pointer"
                        >
                          {/* Color indicator */}
                          <td className="p-4 pr-0">
                            <div
                              className="w-3 h-8 rounded-full"
                              style={{ backgroundColor: lineColor }}
                            />
                          </td>
                          <td className="p-4">
                            <div className="flex items-center gap-3">
                              <div className="w-10 h-10 bg-accent-500/20 rounded-full flex items-center justify-center">
                                <span className="text-accent-400 font-bold">
                                  {portfolio.username.charAt(0).toUpperCase()}
                                </span>
                              </div>
                              <span className="text-white font-medium">{portfolio.username}</span>
                            </div>
                          </td>
                          <td className="p-4 text-gray-400">
                            {portfolio.position_count} Position{portfolio.position_count !== 1 ? 'en' : ''}
                          </td>
                          <td className="p-4">
                            <div className="flex flex-wrap gap-1">
                              {portfolio.positions.slice(0, 5).map((pos, idx) => (
                                <span
                                  key={idx}
                                  className="px-2 py-0.5 bg-dark-700 text-gray-300 text-xs rounded"
                                >
                                  {pos.symbol}
                                </span>
                              ))}
                              {portfolio.positions.length > 5 && (
                                <span className="px-2 py-0.5 bg-dark-600 text-gray-500 text-xs rounded">
                                  +{portfolio.positions.length - 5}
                                </span>
                              )}
                            </div>
                          </td>
                          <td className="p-4 text-right">
                            <div className="flex items-center justify-end gap-2">
                              <span className={`text-lg font-bold ${
                                portfolio.total_return_pct >= 0 ? 'text-green-400' : 'text-red-400'
                              }`}>
                                {formatPercent(portfolio.total_return_pct)}
                              </span>
                              <svg
                                className={`w-5 h-5 text-gray-400 transition-transform ${
                                  expandedPortfolio === portfolio.user_id ? 'rotate-180' : ''
                                }`}
                                fill="none"
                                stroke="currentColor"
                                viewBox="0 0 24 24"
                              >
                                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 9l-7 7-7-7" />
                              </svg>
                            </div>
                          </td>
                        </tr>

                        {/* Expanded Row */}
                        {expandedPortfolio === portfolio.user_id && (
                          <tr key={`${portfolio.user_id}-expanded`}>
                            <td colSpan={5} className="p-0">
                              <div className="bg-dark-900/50 p-4">
                                <table className="w-full">
                                  <thead>
                                    <tr className="text-left text-xs text-gray-500">
                                      <th className="pb-2">Symbol</th>
                                      <th className="pb-2">Name</th>
                                      <th className="pb-2">Kaufkurs</th>
                                      <th className="pb-2">Aktuell</th>
                                      <th className="pb-2 text-right">Rendite</th>
                                    </tr>
                                  </thead>
                                  <tbody>
                                    {portfolio.positions.map((pos, idx) => (
                                      <tr key={idx} className="border-t border-dark-700/50">
                                        <td className="py-2 font-medium text-white">{pos.symbol}</td>
                                        <td className="py-2 text-gray-400 text-sm truncate max-w-[200px]">{pos.name}</td>
                                        <td className="py-2 text-gray-300">{formatPrice(pos.avg_price_usd)}</td>
                                        <td className="py-2 text-white">{formatPrice(pos.current_price)}</td>
                                        <td className={`py-2 text-right font-bold ${
                                          pos.total_return_pct >= 0 ? 'text-green-400' : 'text-red-400'
                                        }`}>
                                          {formatPercent(pos.total_return_pct)}
                                        </td>
                                      </tr>
                                    ))}
                                  </tbody>
                                </table>
                              </div>
                            </td>
                          </tr>
                        )}
                      </>
                    )})}
                  </tbody>
                </table>
              </div>
            </div>
          </>
        )}
      </div>
    </div>
  )
}

export default PortfolioCompare
