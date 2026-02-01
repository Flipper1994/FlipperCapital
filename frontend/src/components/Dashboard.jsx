import { useState, useEffect, useCallback } from 'react'
import ChartArea from './ChartArea'
import Watchlist from './Watchlist'
import BacktestPanel from './BacktestPanel'
import { processStock } from '../utils/bxtrender'

function Dashboard({ isAdmin, token }) {
  const [stocks, setStocks] = useState([])
  const [loading, setLoading] = useState(true)
  const [selectedStock, setSelectedStock] = useState(null)
  const [backtestData, setBacktestData] = useState({ trades: [], metrics: null })

  // Mobile collapsible state for watchlist (default collapsed)
  const [watchlistExpanded, setWatchlistExpanded] = useState(false)

  // PC collapsible state for System Performance (default collapsed, opens on stock click)
  const [performanceExpanded, setPerformanceExpanded] = useState(false)

  const fetchStocks = useCallback(async (showLoading = false) => {
    if (showLoading) setLoading(true)
    try {
      const res = await fetch('/api/stocks')
      const data = await res.json()
      setStocks(data)

      if (selectedStock) {
        const updated = data.find(s => s.id === selectedStock.id)
        if (updated) setSelectedStock(updated)
      }
    } catch (err) {
      console.error('Failed to fetch stocks:', err)
    } finally {
      setLoading(false)
    }
  }, [selectedStock])

  useEffect(() => {
    fetchStocks(true)
  }, [])

  useEffect(() => {
    const interval = setInterval(() => {
      fetchStocks(false)
    }, 30000)

    return () => clearInterval(interval)
  }, [fetchStocks])

  const addStock = async (stock) => {
    try {
      // Check if stock already exists in watchlist - if so, just select it
      const existingStock = stocks.find(s => s.symbol.toUpperCase() === stock.symbol.toUpperCase())
      if (existingStock) {
        handleSelectStock(existingStock)
        return { success: true, alreadyExists: true }
      }

      // First validate that chart data is available for BX Trender analysis
      const histRes = await fetch(`/api/history/${stock.symbol}?period=max&interval=1mo`)
      if (!histRes.ok) {
        return { success: false, error: `Keine Kursdaten für ${stock.symbol} verfügbar. BX Trender Analyse nicht möglich.` }
      }
      const histData = await histRes.json()
      if (!histData.data || histData.data.length < 40) {
        return { success: false, error: `Nicht genügend historische Daten für ${stock.symbol}. Mindestens 40 Monate benötigt für BX Trender Analyse.` }
      }

      const res = await fetch('/api/stocks', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'Authorization': `Bearer ${token}`
        },
        body: JSON.stringify(stock)
      })
      if (res.ok) {
        const addedStock = await res.json()

        // Calculate and save BX-Trender performance for both modes (defensive & aggressive)
        processStock(addedStock.symbol, addedStock.name).catch(err => {
          console.warn('Failed to process stock performance:', err)
        })

        // Fetch updated stocks list with prices
        const stocksRes = await fetch('/api/stocks')
        const updatedStocks = await stocksRes.json()
        setStocks(updatedStocks)

        // Find the newly added stock with current price data and select it
        const stockWithPrice = updatedStocks.find(s => s.symbol === addedStock.symbol)
        if (stockWithPrice) {
          handleSelectStock(stockWithPrice)
        }
        return { success: true }
      }
      return { success: false, error: 'Fehler beim Hinzufügen zur Watchlist.' }
    } catch (err) {
      return { success: false, error: err.message || 'Unbekannter Fehler' }
    }
  }

  const deleteStock = async (id) => {
    try {
      const res = await fetch(`/api/stocks/${id}`, {
        method: 'DELETE',
        headers: {
          'Authorization': `Bearer ${token}`
        }
      })
      if (res.ok) {
        if (selectedStock?.id === id) {
          setSelectedStock(null)
        }
        fetchStocks(false)
      }
    } catch (err) {
      console.error('Failed to delete stock:', err)
    }
  }

  const handleBacktestUpdate = useCallback((data) => {
    setBacktestData(data)
  }, [])

  // Handle stock selection - also expand performance panel
  const handleSelectStock = useCallback((stock) => {
    // Save scroll position to prevent jumping to top
    const scrollY = window.scrollY
    const scrollX = window.scrollX

    setSelectedStock(stock)
    if (stock) {
      setPerformanceExpanded(true)
      // Calculate and save BX-Trender performance for both modes when stock is viewed
      processStock(stock.symbol, stock.name).catch(err => {
        console.warn('Failed to process stock performance:', err)
      })
    }

    // Restore scroll position after a short delay
    requestAnimationFrame(() => {
      window.scrollTo(scrollX, scrollY)
    })
  }, [])

  // Chevron icon component for collapsible sections
  const ChevronIcon = ({ expanded }) => (
    <svg
      className={`w-5 h-5 transition-transform duration-200 ${expanded ? 'rotate-180' : ''}`}
      fill="none"
      stroke="currentColor"
      viewBox="0 0 24 24"
    >
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 9l-7 7-7-7" />
    </svg>
  )

  return (
    <main className="flex flex-col md:flex-row flex-1 overflow-hidden">
      {/* Chart Area */}
      <div className="flex-1 p-4 md:p-6 overflow-auto">
        <ChartArea
          stock={selectedStock}
          stocks={stocks}
          onBacktestUpdate={handleBacktestUpdate}
          onSelectStock={handleSelectStock}
          backtestData={backtestData}
        />
      </div>

      {/* Right Sidebar - Desktop: fixed sidebar, Mobile: collapsible sections */}
      <aside className="md:w-80 bg-dark-800 border-t md:border-t-0 md:border-l border-dark-600 flex flex-col">
        {/* System Performance Section - Now ABOVE Watchlist, collapsible on both mobile and PC */}
        <div className="border-b border-dark-600 flex flex-col">
          {/* Collapsible Header */}
          <button
            onClick={() => setPerformanceExpanded(!performanceExpanded)}
            className="flex items-center justify-between w-full px-4 py-3 bg-dark-700 border-b border-dark-600 hover:bg-dark-600 transition-colors"
          >
            <div className="flex items-center gap-2">
              <svg className="w-5 h-5 text-accent-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M9 19v-6a2 2 0 00-2-2H5a2 2 0 00-2 2v6a2 2 0 002 2h2a2 2 0 002-2zm0 0V9a2 2 0 012-2h2a2 2 0 012 2v10m-6 0a2 2 0 002 2h2a2 2 0 002-2m0 0V5a2 2 0 012-2h2a2 2 0 012 2v14a2 2 0 01-2 2h-2a2 2 0 01-2-2z" />
              </svg>
              <span className="text-white font-medium">System Performance</span>
              {selectedStock && (
                <span className="text-xs text-accent-400">({selectedStock.symbol})</span>
              )}
            </div>
            <ChevronIcon expanded={performanceExpanded} />
          </button>

          {/* Performance Content */}
          <div className={`
            overflow-hidden transition-all duration-300 ease-in-out
            ${performanceExpanded ? 'max-h-[400px]' : 'max-h-0'}
          `}>
            <div className="overflow-auto max-h-[400px]">
              <BacktestPanel
                trades={backtestData.trades}
                metrics={backtestData.metrics}
                symbol={selectedStock?.symbol}
              />
            </div>
          </div>
        </div>

        {/* Watchlist Section - Now BELOW System Performance */}
        <div className="flex-1 overflow-hidden flex flex-col">
          {/* Collapsible Header - Mobile only */}
          <button
            onClick={() => setWatchlistExpanded(!watchlistExpanded)}
            className="md:hidden flex items-center justify-between w-full px-4 py-3 bg-dark-700 border-b border-dark-600"
          >
            <div className="flex items-center gap-2">
              <svg className="w-5 h-5 text-accent-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M13 7h8m0 0v8m0-8l-8 8-4-4-6 6" />
              </svg>
              <span className="text-white font-medium">Watchlist</span>
              <span className="text-xs text-gray-500">({stocks.length})</span>
            </div>
            <ChevronIcon expanded={watchlistExpanded} />
          </button>

          {/* Watchlist Content */}
          <div className={`
            overflow-hidden transition-all duration-300 ease-in-out
            ${watchlistExpanded ? 'max-h-[400px]' : 'max-h-0'}
            md:max-h-none md:flex-1
          `}>
            <Watchlist
              stocks={stocks}
              loading={loading}
              isAdmin={isAdmin}
              onAdd={addStock}
              onDelete={deleteStock}
              onSelectStock={handleSelectStock}
              onCategoryChange={() => fetchStocks(false)}
            />
          </div>
        </div>
      </aside>
    </main>
  )
}

export default Dashboard
