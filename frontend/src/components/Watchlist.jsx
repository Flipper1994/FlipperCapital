import { useState, useEffect, useRef } from 'react'
import { formatPrice, formatChange } from '../utils/currency'

function Watchlist({ stocks, loading, isAdmin, onAdd, onDelete, onSelectStock }) {
  const [query, setQuery] = useState('')
  const [searchResults, setSearchResults] = useState([])
  const [searching, setSearching] = useState(false)
  const [adding, setAdding] = useState(false)
  const [showDropdown, setShowDropdown] = useState(false)
  const [canAdd, setCanAdd] = useState(false)
  const [addMessage, setAddMessage] = useState('')
  const [, forceUpdate] = useState(0)
  const searchRef = useRef(null)
  const debounceRef = useRef(null)

  useEffect(() => {
    const handleCurrencyChange = () => forceUpdate(n => n + 1)
    window.addEventListener('currencyChanged', handleCurrencyChange)
    return () => window.removeEventListener('currencyChanged', handleCurrencyChange)
  }, [])

  useEffect(() => {
    checkCanAddStocks()
  }, [])

  useEffect(() => {
    const handleClickOutside = (e) => {
      if (searchRef.current && !searchRef.current.contains(e.target)) {
        setShowDropdown(false)
      }
    }
    document.addEventListener('mousedown', handleClickOutside)
    return () => document.removeEventListener('mousedown', handleClickOutside)
  }, [])

  const checkCanAddStocks = async () => {
    try {
      const token = localStorage.getItem('authToken')
      const res = await fetch('/api/can-add-stocks', {
        headers: token ? { 'Authorization': `Bearer ${token}` } : {}
      })
      const data = await res.json()
      setCanAdd(data.can_add)
      setAddMessage(data.message || '')
    } catch {
      setCanAdd(false)
    }
  }

  const searchStocks = async (q) => {
    if (!q || q.length < 1) {
      setSearchResults([])
      return
    }
    setSearching(true)
    try {
      const res = await fetch(`/api/search?q=${encodeURIComponent(q)}`)
      const data = await res.json()
      setSearchResults(data)
      setShowDropdown(true)
    } catch {
      setSearchResults([])
    } finally {
      setSearching(false)
    }
  }

  const handleQueryChange = (e) => {
    const value = e.target.value
    setQuery(value)

    if (debounceRef.current) clearTimeout(debounceRef.current)
    debounceRef.current = setTimeout(() => {
      searchStocks(value)
    }, 300)
  }

  const handleSelectStock = async (stock) => {
    setAdding(true)
    const success = await onAdd({
      symbol: stock.symbol,
      name: stock.name
    })
    if (success) {
      setQuery('')
      setSearchResults([])
      setShowDropdown(false)
    }
    setAdding(false)
  }

  return (
    <div className="p-4 h-full flex flex-col">
      <div className="flex items-center justify-between mb-4">
        <h2 className="text-lg font-semibold text-white">Watchlist</h2>
        <span className="text-xs text-gray-500">{stocks.length} stocks</span>
      </div>

      {/* Search Box - shown if user can add stocks */}
      {canAdd && (
        <div className="mb-4 relative" ref={searchRef}>
          <div className="relative">
            <input
              type="text"
              placeholder="Aktie suchen (z.B. AAPL, Tesla)"
              value={query}
              onChange={handleQueryChange}
              onFocus={() => searchResults.length > 0 && setShowDropdown(true)}
              className="w-full px-3 py-2.5 bg-dark-700 border border-dark-600 rounded-lg text-sm text-white placeholder-gray-500 focus:outline-none focus:border-accent-500 pr-10"
            />
            <div className="absolute right-3 top-1/2 -translate-y-1/2">
              {searching ? (
                <div className="w-4 h-4 border-2 border-accent-500 border-t-transparent rounded-full animate-spin"></div>
              ) : (
                <svg className="w-4 h-4 text-gray-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
                </svg>
              )}
            </div>
          </div>

          {showDropdown && searchResults.length > 0 && (
            <div className="absolute z-50 w-full mt-1 bg-dark-700 border border-dark-600 rounded-lg shadow-xl max-h-64 overflow-auto">
              {searchResults.map((result) => (
                <button
                  key={result.symbol}
                  onClick={() => handleSelectStock(result)}
                  disabled={adding}
                  className="w-full px-3 py-2.5 text-left hover:bg-dark-600 transition-colors flex items-center justify-between group disabled:opacity-50"
                >
                  <div>
                    <div className="flex items-center gap-2">
                      <span className="font-medium text-white">{result.symbol}</span>
                      <span className="text-xs px-1.5 py-0.5 bg-dark-800 text-gray-400 rounded">
                        {result.exchange}
                      </span>
                    </div>
                    <p className="text-xs text-gray-500 truncate max-w-[200px]">{result.name}</p>
                  </div>
                  <svg className="w-4 h-4 text-accent-400 opacity-0 group-hover:opacity-100 transition-opacity" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 4v16m8-8H4" />
                  </svg>
                </button>
              ))}
            </div>
          )}
        </div>
      )}

      {/* Info message if user cannot add stocks */}
      {!canAdd && addMessage && (
        <div className="mb-4 p-3 bg-dark-700 rounded-lg border border-dark-600">
          <div className="flex items-start gap-2">
            <svg className="w-5 h-5 text-accent-400 shrink-0 mt-0.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
            </svg>
            <p className="text-xs text-gray-400">{addMessage}</p>
          </div>
        </div>
      )}

      <div className="flex-1 overflow-auto">
        {loading ? (
          <div className="text-center py-8">
            <div className="w-8 h-8 border-2 border-accent-500 border-t-transparent rounded-full animate-spin mx-auto"></div>
            <p className="text-gray-500 text-sm mt-2">Loading...</p>
          </div>
        ) : stocks.length === 0 ? (
          <div className="text-center py-8">
            <div className="w-16 h-16 mx-auto mb-4 bg-dark-700 rounded-full flex items-center justify-center">
              <svg className="w-8 h-8 text-gray-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M13 7h8m0 0v8m0-8l-8 8-4-4-6 6" />
              </svg>
            </div>
            <p className="text-gray-500 text-sm">Keine Aktien in der Watchlist</p>
            {!canAdd && (
              <p className="text-gray-600 text-xs mt-1">Melde dich an und pflege dein Portfolio</p>
            )}
          </div>
        ) : (
          <div className="space-y-2">
            {stocks.map((stock) => {
              const changeData = formatChange(stock.change, stock.change_percent)
              return (
                <div
                  key={stock.id}
                  onClick={() => onSelectStock && onSelectStock(stock)}
                  className="p-3 bg-dark-700 rounded-lg hover:bg-dark-600 transition-colors group cursor-pointer"
                >
                  <div className="flex items-start justify-between">
                    <div className="flex-1 min-w-0">
                      <div className="flex items-center gap-2">
                        <span className="font-semibold text-white">{stock.symbol}</span>
                      </div>
                      <p className="text-xs text-gray-500 mt-0.5 truncate">{stock.name}</p>
                    </div>
                    <div className="text-right ml-2">
                      <div className="font-medium text-white">
                        {formatPrice(stock.price)}
                      </div>
                      {changeData && (
                        <div className={`text-xs ${changeData.isPositive ? 'text-green-400' : 'text-red-400'}`}>
                          {changeData.text}
                        </div>
                      )}
                    </div>
                  </div>
                  {isAdmin && (
                    <div className="mt-2 pt-2 border-t border-dark-600 opacity-0 group-hover:opacity-100 transition-opacity">
                      <button
                        onClick={(e) => {
                          e.stopPropagation()
                          onDelete(stock.id)
                        }}
                        className="text-xs text-red-400 hover:text-red-300 flex items-center gap-1"
                      >
                        <svg className="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
                        </svg>
                        Entfernen
                      </button>
                    </div>
                  )}
                </div>
              )
            })}
          </div>
        )}
      </div>
    </div>
  )
}

export default Watchlist
