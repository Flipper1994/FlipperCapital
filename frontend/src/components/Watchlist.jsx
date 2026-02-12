import { useState, useEffect, useRef, useMemo } from 'react'
import { formatPrice, formatChange } from '../utils/currency'
import { useTradingMode } from '../context/TradingModeContext'
import { useBlockedStocks } from '../hooks/useBlockedStocks'
import BlockedBadge from './BlockedBadge'

function Watchlist({ stocks, loading, isAdmin, onAdd, onDelete, onSelectStock, onCategoryChange }) {
  const blockedStocks = useBlockedStocks()
  const [query, setQuery] = useState('')
  const [searchResults, setSearchResults] = useState([])
  const [searching, setSearching] = useState(false)
  const [adding, setAdding] = useState(false)
  const [showDropdown, setShowDropdown] = useState(false)
  const [canAdd, setCanAdd] = useState(false)
  const [addMessage, setAddMessage] = useState('')
  const [addError, setAddError] = useState('')
  const [collapsedCategories, setCollapsedCategories] = useState({})
  const [signals, setSignals] = useState({})
  const [, forceUpdate] = useState(0)
  const [categories, setCategories] = useState([])
  const [showCategoryManager, setShowCategoryManager] = useState(false)
  const [newCategoryName, setNewCategoryName] = useState('')
  const [editingCategory, setEditingCategory] = useState(null)
  const [draggedStock, setDraggedStock] = useState(null)
  const [dragOverCategory, setDragOverCategory] = useState(null)
  const [contextMenu, setContextMenu] = useState(null) // { x, y, stock }
  const [showMoveSubmenu, setShowMoveSubmenu] = useState(false)
  const searchRef = useRef(null)
  const contextMenuRef = useRef(null)
  const debounceRef = useRef(null)
  const { isAggressive, isQuant, isDitz, isTrader } = useTradingMode()

  // Fetch categories
  useEffect(() => {
    fetchCategories()
  }, [])

  const fetchCategories = async () => {
    try {
      const res = await fetch('/api/categories')
      const data = await res.json()
      setCategories(data)
    } catch {
      // Ignore
    }
  }

  // Group stocks by category
  const groupedStocks = useMemo(() => {
    const groups = {}

    // Create groups for all categories
    categories.forEach(cat => {
      groups[cat.id] = { name: cat.name, stocks: [], sortOrder: cat.sort_order }
    })

    // Add stocks to their categories
    stocks.forEach(stock => {
      const catId = stock.category_id
      if (catId && groups[catId]) {
        groups[catId].stocks.push(stock)
      } else {
        // Use category_name if available, otherwise fallback to sector or "Sonstiges"
        const fallbackName = stock.category_name || stock.sector || 'Sonstiges'
        // Find or create fallback group
        let fallbackGroup = Object.values(groups).find(g => g.name === fallbackName)
        if (!fallbackGroup) {
          // Find Sonstiges category
          const sonstigesCat = categories.find(c => c.name === 'Sonstiges')
          if (sonstigesCat && groups[sonstigesCat.id]) {
            groups[sonstigesCat.id].stocks.push(stock)
          }
        } else {
          const catEntry = Object.entries(groups).find(([, v]) => v.name === fallbackName)
          if (catEntry) {
            groups[catEntry[0]].stocks.push(stock)
          }
        }
      }
    })

    // Sort stocks within each category by market cap
    Object.values(groups).forEach(group => {
      group.stocks.sort((a, b) => (b.market_cap || 0) - (a.market_cap || 0))
    })

    // Sort categories by sort_order
    const sortedCategoryIds = Object.keys(groups).sort((a, b) => {
      return (groups[a].sortOrder || 0) - (groups[b].sortOrder || 0)
    })

    return { groups, sortedCategoryIds }
  }, [stocks, categories])

  const toggleCategory = (categoryId) => {
    setCollapsedCategories(prev => ({ ...prev, [categoryId]: !prev[categoryId] }))
  }

  useEffect(() => {
    const handleCurrencyChange = () => forceUpdate(n => n + 1)
    window.addEventListener('currencyChanged', handleCurrencyChange)
    return () => window.removeEventListener('currencyChanged', handleCurrencyChange)
  }, [])

  useEffect(() => {
    checkCanAddStocks()
  }, [])

  // Fetch signals for all stocks
  useEffect(() => {
    const fetchSignals = async () => {
      try {
        const endpoint = isTrader
          ? '/api/performance/trader'
          : isDitz
            ? '/api/performance/ditz'
            : isQuant
              ? '/api/performance/quant'
              : isAggressive
                ? '/api/performance/aggressive'
                : '/api/performance'
        const res = await fetch(endpoint)
        const data = await res.json()
        const signalMap = {}
        data.forEach(p => {
          signalMap[p.symbol] = p.signal
        })
        setSignals(signalMap)
      } catch {
        // Ignore errors
      }
    }
    fetchSignals()
  }, [isAggressive, isQuant, isDitz, isTrader, stocks])

  useEffect(() => {
    const handleClickOutside = (e) => {
      if (searchRef.current && !searchRef.current.contains(e.target)) {
        setShowDropdown(false)
      }
      if (contextMenuRef.current && !contextMenuRef.current.contains(e.target)) {
        closeContextMenu()
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

  const getSignalStyle = (signal) => {
    switch (signal) {
      case 'BUY': return 'bg-green-500/20 text-green-400'
      case 'HOLD': return 'bg-blue-500/20 text-blue-400'
      case 'SELL': return 'bg-red-500/20 text-red-400'
      case 'WAIT': return 'bg-yellow-500/20 text-yellow-400'
      case 'NO_DATA': return 'bg-gray-500/20 text-gray-400'
      default: return 'bg-gray-500/20 text-gray-400'
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
    setAddError('')
    const result = await onAdd({
      symbol: stock.symbol,
      name: stock.name
    })
    if (result?.success || result === true) {
      setQuery('')
      setSearchResults([])
      setShowDropdown(false)
    } else if (result?.error) {
      setAddError(result.error)
      setTimeout(() => setAddError(''), 8000)
    }
    setAdding(false)
  }

  // Category management
  const createCategory = async () => {
    if (!newCategoryName.trim()) return
    const token = localStorage.getItem('authToken')
    try {
      const res = await fetch('/api/categories', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'Authorization': `Bearer ${token}`
        },
        body: JSON.stringify({ name: newCategoryName.trim() })
      })
      if (res.ok) {
        setNewCategoryName('')
        fetchCategories()
        if (onCategoryChange) onCategoryChange()
      }
    } catch {
      // Ignore
    }
  }

  const updateCategoryName = async (categoryId) => {
    if (!editingCategory?.name?.trim()) return
    const token = localStorage.getItem('authToken')
    try {
      const res = await fetch(`/api/categories/${categoryId}`, {
        method: 'PUT',
        headers: {
          'Content-Type': 'application/json',
          'Authorization': `Bearer ${token}`
        },
        body: JSON.stringify({ name: editingCategory.name.trim() })
      })
      if (res.ok) {
        setEditingCategory(null)
        fetchCategories()
        if (onCategoryChange) onCategoryChange()
      }
    } catch {
      // Ignore
    }
  }

  const deleteCategory = async (categoryId) => {
    const token = localStorage.getItem('authToken')
    try {
      const res = await fetch(`/api/categories/${categoryId}`, {
        method: 'DELETE',
        headers: { 'Authorization': `Bearer ${token}` }
      })
      if (res.ok) {
        fetchCategories()
        if (onCategoryChange) onCategoryChange()
      }
    } catch {
      // Ignore
    }
  }

  // Drag & Drop handlers
  const handleDragStart = (e, stock) => {
    setDraggedStock(stock)
    e.dataTransfer.effectAllowed = 'move'
  }

  const handleDragOver = (e, categoryId) => {
    e.preventDefault()
    e.dataTransfer.dropEffect = 'move'
    setDragOverCategory(categoryId)
  }

  const handleDragLeave = () => {
    setDragOverCategory(null)
  }

  const handleDrop = async (e, categoryId) => {
    e.preventDefault()
    setDragOverCategory(null)

    if (!draggedStock || draggedStock.category_id === categoryId) {
      setDraggedStock(null)
      return
    }

    const token = localStorage.getItem('authToken')
    try {
      const res = await fetch(`/api/stocks/${draggedStock.id}/category`, {
        method: 'PUT',
        headers: {
          'Content-Type': 'application/json',
          'Authorization': `Bearer ${token}`
        },
        body: JSON.stringify({ category_id: parseInt(categoryId) })
      })
      if (res.ok && onCategoryChange) {
        onCategoryChange()
      }
    } catch {
      // Ignore
    }
    setDraggedStock(null)
  }

  const handleDragEnd = () => {
    setDraggedStock(null)
    setDragOverCategory(null)
  }

  // Context menu handlers
  const handleContextMenu = (e, stock) => {
    if (!isAdmin) return
    e.preventDefault()
    e.stopPropagation()
    setShowMoveSubmenu(false)
    setContextMenu({
      x: e.pageX,
      y: e.pageY,
      stock
    })
  }

  const moveStockToCategory = async (stockId, categoryId) => {
    const token = localStorage.getItem('authToken')
    try {
      const res = await fetch(`/api/stocks/${stockId}/category`, {
        method: 'PUT',
        headers: {
          'Content-Type': 'application/json',
          'Authorization': `Bearer ${token}`
        },
        body: JSON.stringify({ category_id: categoryId })
      })
      if (res.ok && onCategoryChange) {
        onCategoryChange()
      }
    } catch {
      // Ignore
    }
    closeContextMenu()
  }

  const handleDeleteStock = (stockId) => {
    onDelete(stockId)
    setContextMenu(null)
    setShowMoveSubmenu(false)
  }

  const closeContextMenu = () => {
    setContextMenu(null)
    setShowMoveSubmenu(false)
  }

  return (
    <div className="p-4 h-full flex flex-col">
      <div className="flex items-center justify-between mb-4">
        <h2 className="text-lg font-semibold text-white">Watchlist</h2>
        <div className="flex items-center gap-2">
          <span className="text-xs text-gray-500">{stocks.length} stocks</span>
          {isAdmin && (
            <button
              onClick={() => setShowCategoryManager(!showCategoryManager)}
              className="p-1 text-gray-400 hover:text-accent-400 transition-colors"
              title="Kategorien verwalten"
            >
              <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z" />
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
              </svg>
            </button>
          )}
        </div>
      </div>

      {/* Category Manager (Admin only) */}
      {isAdmin && showCategoryManager && (
        <div className="mb-4 p-3 bg-dark-700 rounded-lg border border-dark-600">
          <div className="flex items-center justify-between mb-2">
            <span className="text-sm font-medium text-white">Kategorien verwalten</span>
            <button
              onClick={() => setShowCategoryManager(false)}
              className="text-gray-400 hover:text-white"
            >
              <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
              </svg>
            </button>
          </div>

          {/* New category input */}
          <div className="flex gap-2 mb-3">
            <input
              type="text"
              value={newCategoryName}
              onChange={(e) => setNewCategoryName(e.target.value)}
              placeholder="Neue Kategorie..."
              className="flex-1 px-2 py-1.5 bg-dark-600 border border-dark-500 rounded text-sm text-white placeholder-gray-500 focus:outline-none focus:border-accent-500"
              onKeyDown={(e) => e.key === 'Enter' && createCategory()}
            />
            <button
              onClick={createCategory}
              className="px-2 py-1.5 bg-accent-500 text-white text-sm rounded hover:bg-accent-600 transition-colors"
            >
              +
            </button>
          </div>

          {/* Category list */}
          <div className="space-y-1 max-h-40 overflow-auto">
            {categories.map(cat => (
              <div key={cat.id} className="flex items-center gap-2 py-1">
                {editingCategory?.id === cat.id ? (
                  <>
                    <input
                      type="text"
                      value={editingCategory.name}
                      onChange={(e) => setEditingCategory({ ...editingCategory, name: e.target.value })}
                      className="flex-1 px-2 py-1 bg-dark-600 border border-dark-500 rounded text-xs text-white focus:outline-none focus:border-accent-500"
                      onKeyDown={(e) => {
                        if (e.key === 'Enter') updateCategoryName(cat.id)
                        if (e.key === 'Escape') setEditingCategory(null)
                      }}
                      autoFocus
                    />
                    <button
                      onClick={() => updateCategoryName(cat.id)}
                      className="text-green-400 hover:text-green-300"
                    >
                      <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 13l4 4L19 7" />
                      </svg>
                    </button>
                  </>
                ) : (
                  <>
                    <span className="flex-1 text-xs text-gray-300">{cat.name}</span>
                    {cat.name !== 'Sonstiges' && (
                      <>
                        <button
                          onClick={() => setEditingCategory({ id: cat.id, name: cat.name })}
                          className="text-gray-400 hover:text-accent-400"
                        >
                          <svg className="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15.232 5.232l3.536 3.536m-2.036-5.036a2.5 2.5 0 113.536 3.536L6.5 21.036H3v-3.572L16.732 3.732z" />
                          </svg>
                        </button>
                        <button
                          onClick={() => deleteCategory(cat.id)}
                          className="text-gray-400 hover:text-red-400"
                        >
                          <svg className="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
                          </svg>
                        </button>
                      </>
                    )}
                  </>
                )}
              </div>
            ))}
          </div>

          {isAdmin && (
            <p className="mt-2 text-[10px] text-gray-500">
              Tipp: Aktien per Drag & Drop in Kategorien verschieben
            </p>
          )}
        </div>
      )}

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

          {addError && (
            <div className="mt-2 p-3 bg-red-500/10 border border-red-500/30 rounded-lg">
              <div className="flex items-start gap-2">
                <svg className="w-5 h-5 text-red-400 shrink-0 mt-0.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
                </svg>
                <p className="text-xs text-red-400">{addError}</p>
              </div>
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
          <div className="space-y-3">
            {groupedStocks.sortedCategoryIds.map((categoryId) => {
              const group = groupedStocks.groups[categoryId]
              // Show empty categories only for admins (so they can drag stocks into them)
              if (!group || (group.stocks.length === 0 && !isAdmin)) return null

              return (
                <div
                  key={categoryId}
                  className={`border rounded-lg overflow-hidden transition-colors ${
                    dragOverCategory === categoryId
                      ? 'border-accent-500 bg-accent-500/10'
                      : 'border-dark-600'
                  }`}
                  onDragOver={(e) => isAdmin && handleDragOver(e, categoryId)}
                  onDragLeave={handleDragLeave}
                  onDrop={(e) => isAdmin && handleDrop(e, categoryId)}
                >
                  {/* Category Header */}
                  <button
                    onClick={() => toggleCategory(categoryId)}
                    className="w-full flex items-center justify-between px-3 py-2 bg-dark-700 hover:bg-dark-600 transition-colors"
                  >
                    <span className="text-sm font-medium text-gray-300">{group.name}</span>
                    <div className="flex items-center gap-2">
                      <span className="text-xs text-gray-500">({group.stocks.length})</span>
                      <svg
                        className={`w-4 h-4 text-gray-400 transition-transform ${collapsedCategories[categoryId] ? '' : 'rotate-180'}`}
                        fill="none"
                        stroke="currentColor"
                        viewBox="0 0 24 24"
                      >
                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 9l-7 7-7-7" />
                      </svg>
                    </div>
                  </button>

                  {/* Category Stocks */}
                  {!collapsedCategories[categoryId] && (
                    <div className="divide-y divide-dark-600">
                      {group.stocks.length === 0 && isAdmin && (
                        <div className="px-3 py-4 text-center text-xs text-gray-500 italic">
                          Aktien hierher ziehen
                        </div>
                      )}
                      {group.stocks.map((stock) => {
                        const changeData = formatChange(stock.change, stock.change_percent, stock.symbol)
                        const signal = signals[stock.symbol]
                        return (
                          <div
                            key={stock.id}
                            onClick={() => onSelectStock && onSelectStock(stock)}
                            onContextMenu={(e) => handleContextMenu(e, stock)}
                            draggable={isAdmin}
                            onDragStart={(e) => isAdmin && handleDragStart(e, stock)}
                            onDragEnd={handleDragEnd}
                            className={`px-2 py-1.5 hover:bg-dark-700 transition-colors group cursor-pointer ${
                              isAdmin ? 'cursor-grab active:cursor-grabbing' : ''
                            } ${draggedStock?.id === stock.id ? 'opacity-50' : ''}`}
                          >
                            <div className="flex items-center justify-between gap-2">
                              <div className="flex items-center gap-2 min-w-0 flex-1">
                                {isAdmin && (
                                  <svg className="w-3 h-3 text-gray-600 shrink-0" fill="currentColor" viewBox="0 0 20 20">
                                    <path d="M7 2a2 2 0 1 0 .001 4.001A2 2 0 0 0 7 2zm0 6a2 2 0 1 0 .001 4.001A2 2 0 0 0 7 8zm0 6a2 2 0 1 0 .001 4.001A2 2 0 0 0 7 14zm6-8a2 2 0 1 0-.001-4.001A2 2 0 0 0 13 6zm0 2a2 2 0 1 0 .001 4.001A2 2 0 0 0 13 8zm0 6a2 2 0 1 0 .001 4.001A2 2 0 0 0 13 14z" />
                                  </svg>
                                )}
                                <span className="font-semibold text-white text-sm">{stock.symbol}</span>
                                <BlockedBadge symbol={stock.symbol} blockedStocks={blockedStocks} />
                                {stock.market_cap > 0 && (
                                  <span className="text-[10px] text-gray-500">
                                    {stock.market_cap >= 1e12 ? `${(stock.market_cap / 1e12).toFixed(1)}T`
                                      : stock.market_cap >= 1e9 ? `${(stock.market_cap / 1e9).toFixed(0)}B`
                                      : `${(stock.market_cap / 1e6).toFixed(0)}M`}
                                  </span>
                                )}
                                {signal && (
                                  <span className={`px-1 py-0.5 text-[10px] font-bold rounded ${getSignalStyle(signal)}`}>
                                    {signal}
                                  </span>
                                )}
                              </div>
                              <div className="flex items-center gap-2">
                                <div className="text-right">
                                  <div className="text-sm font-medium text-white">
                                    {formatPrice(stock.price, stock.symbol)}
                                  </div>
                                </div>
                                {changeData && (
                                  <div className={`text-xs min-w-[50px] text-right ${changeData.isPositive ? 'text-green-400' : 'text-red-400'}`}>
                                    {changeData.text}
                                  </div>
                                )}
                              </div>
                            </div>
                          </div>
                        )
                      })}
                    </div>
                  )}
                </div>
              )
            })}
          </div>
        )}
      </div>

      {/* Context Menu for Admin */}
      {contextMenu && isAdmin && (
        <div
          ref={contextMenuRef}
          className="fixed z-[100] bg-dark-700 border border-dark-500 rounded-lg shadow-xl py-1 min-w-[180px]"
          style={{
            left: contextMenu.x,
            top: contextMenu.y
          }}
        >
          {/* Move to category - with submenu */}
          <div
            className="relative"
            onMouseEnter={() => setShowMoveSubmenu(true)}
            onMouseLeave={() => setShowMoveSubmenu(false)}
          >
            <button
              onClick={() => setShowMoveSubmenu(!showMoveSubmenu)}
              className="w-full px-3 py-2 text-left text-sm text-gray-300 hover:bg-dark-600 flex items-center justify-between"
            >
              <span className="flex items-center gap-2">
                <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M7 16V4m0 0L3 8m4-4l4 4m6 0v12m0 0l4-4m-4 4l-4-4" />
                </svg>
                Verschieben
              </span>
              <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 5l7 7-7 7" />
              </svg>
            </button>

            {/* Submenu */}
            {showMoveSubmenu && (
              <div className="absolute left-full top-0 bg-dark-700 border border-dark-500 rounded-lg shadow-xl py-1 min-w-[150px]">
                {categories.filter(cat => cat.id !== contextMenu.stock.category_id).map(cat => (
                  <button
                    key={cat.id}
                    onClick={() => moveStockToCategory(contextMenu.stock.id, cat.id)}
                    className="w-full px-3 py-2 text-left text-sm text-gray-300 hover:bg-dark-600"
                  >
                    {cat.name}
                  </button>
                ))}
                {categories.filter(cat => cat.id !== contextMenu.stock.category_id).length === 0 && (
                  <span className="block px-3 py-2 text-xs text-gray-500 italic">Keine anderen Kategorien</span>
                )}
              </div>
            )}
          </div>

          <div className="border-t border-dark-600 my-1"></div>

          {/* Delete option */}
          <button
            onClick={() => handleDeleteStock(contextMenu.stock.id)}
            onMouseEnter={() => setShowMoveSubmenu(false)}
            className="w-full px-3 py-2 text-left text-sm text-red-400 hover:bg-dark-600 flex items-center gap-2"
          >
            <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
            </svg>
            Löschen
          </button>
        </div>
      )}
    </div>
  )
}

export default Watchlist
