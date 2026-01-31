import { useState, useEffect, useRef } from 'react'
import { Link } from 'react-router-dom'
import { useCurrency } from '../context/CurrencyContext'
import { CURRENCY_SYMBOLS } from '../utils/currency'
import PortfolioChart from './PortfolioChart'

function PortfolioManage() {
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
              Um dein Portfolio zu verwalten, musst du angemeldet sein.
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

  return <PortfolioContent token={token} />
}

function PortfolioContent({ token }) {
  const [positions, setPositions] = useState([])
  const [trades, setTrades] = useState([])
  const [performance, setPerformance] = useState(null)
  const [loading, setLoading] = useState(true)
  const [showForm, setShowForm] = useState(false)
  const [editingPosition, setEditingPosition] = useState(null)
  const [sellingPosition, setSellingPosition] = useState(null)
  const [sellPrice, setSellPrice] = useState('')
  const [searchQuery, setSearchQuery] = useState('')
  const [searchResults, setSearchResults] = useState([])
  const [searching, setSearching] = useState(false)
  const [showDropdown, setShowDropdown] = useState(false)
  const [showHistory, setShowHistory] = useState(false)
  const [formData, setFormData] = useState({
    symbol: '',
    name: '',
    purchase_date: '',
    avg_price: '',
    currency: 'EUR',
    quantity: ''
  })
  const [submitting, setSubmitting] = useState(false)
  const searchRef = useRef(null)
  const debounceRef = useRef(null)
  const { formatPrice, currency } = useCurrency()

  useEffect(() => {
    fetchPortfolio()
    fetchPerformance()
    fetchTrades()
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

  const fetchPortfolio = async () => {
    try {
      const res = await fetch('/api/portfolio', {
        headers: { 'Authorization': `Bearer ${token}` }
      })
      const data = await res.json()
      setPositions(data)
    } catch (err) {
      console.error('Failed to fetch portfolio:', err)
    } finally {
      setLoading(false)
    }
  }

  const fetchPerformance = async () => {
    try {
      const res = await fetch('/api/portfolio/performance', {
        headers: { 'Authorization': `Bearer ${token}` }
      })
      const data = await res.json()
      setPerformance(data)
    } catch (err) {
      console.error('Failed to fetch performance:', err)
    }
  }

  const fetchTrades = async () => {
    try {
      const res = await fetch('/api/portfolio/trades', {
        headers: { 'Authorization': `Bearer ${token}` }
      })
      const data = await res.json()
      setTrades(data || [])
    } catch (err) {
      console.error('Failed to fetch trades:', err)
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

  const handleSearchChange = (e) => {
    const value = e.target.value
    setSearchQuery(value)
    if (debounceRef.current) clearTimeout(debounceRef.current)
    debounceRef.current = setTimeout(() => {
      searchStocks(value)
    }, 300)
  }

  const handleSelectStock = (stock) => {
    setFormData({
      ...formData,
      symbol: stock.symbol,
      name: stock.name
    })
    setSearchQuery('')
    setSearchResults([])
    setShowDropdown(false)
    setShowForm(true)
  }

  const handleSubmit = async (e) => {
    e.preventDefault()
    if (!formData.symbol || !formData.avg_price) return

    setSubmitting(true)
    try {
      const payload = {
        symbol: formData.symbol,
        name: formData.name,
        avg_price: parseFloat(formData.avg_price),
        currency: formData.currency || 'EUR',
        purchase_date: formData.purchase_date || null,
        quantity: formData.quantity ? parseFloat(formData.quantity) : null
      }

      const url = editingPosition
        ? `/api/portfolio/${editingPosition.id}`
        : '/api/portfolio'
      const method = editingPosition ? 'PUT' : 'POST'

      const res = await fetch(url, {
        method,
        headers: {
          'Content-Type': 'application/json',
          'Authorization': `Bearer ${token}`
        },
        body: JSON.stringify(payload)
      })

      if (res.ok) {
        setShowForm(false)
        setEditingPosition(null)
        setFormData({ symbol: '', name: '', purchase_date: '', avg_price: '', currency: 'EUR', quantity: '' })
        fetchPortfolio()
        fetchPerformance()
      }
    } catch (err) {
      console.error('Failed to save position:', err)
    } finally {
      setSubmitting(false)
    }
  }

  const handleEdit = (position) => {
    setEditingPosition(position)
    setFormData({
      symbol: position.symbol,
      name: position.name,
      purchase_date: position.purchase_date ? position.purchase_date.split('T')[0] : '',
      avg_price: position.avg_price.toString(),
      currency: position.currency || 'EUR',
      quantity: position.quantity ? position.quantity.toString() : ''
    })
    setShowForm(true)
  }

  const handleDelete = async (id) => {
    if (!confirm('Position wirklich löschen? (Wird NICHT in der History gespeichert)')) return
    try {
      const res = await fetch(`/api/portfolio/${id}`, {
        method: 'DELETE',
        headers: { 'Authorization': `Bearer ${token}` }
      })
      if (res.ok) {
        fetchPortfolio()
        fetchPerformance()
      }
    } catch (err) {
      console.error('Failed to delete position:', err)
    }
  }

  const handleSell = async (e) => {
    e.preventDefault()
    if (!sellingPosition || !sellPrice) return

    setSubmitting(true)
    try {
      const res = await fetch(`/api/portfolio/${sellingPosition.id}/sell`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'Authorization': `Bearer ${token}`
        },
        body: JSON.stringify({
          sell_price: parseFloat(sellPrice),
          quantity: sellingPosition.quantity
        })
      })
      if (res.ok) {
        setSellingPosition(null)
        setSellPrice('')
        fetchPortfolio()
        fetchPerformance()
        fetchTrades()
      }
    } catch (err) {
      console.error('Failed to sell position:', err)
    } finally {
      setSubmitting(false)
    }
  }

  const openSellModal = (pos) => {
    setSellingPosition(pos)
    setSellPrice(pos.current_price?.toFixed(2) || '')
  }

  const handleCancel = () => {
    setShowForm(false)
    setEditingPosition(null)
    setFormData({ symbol: '', name: '', purchase_date: '', avg_price: '', currency: 'EUR', quantity: '' })
  }

  const formatPercent = (value) => {
    if (value === undefined || value === null || isNaN(value)) return '--'
    const sign = value >= 0 ? '+' : ''
    return `${sign}${value.toFixed(2)}%`
  }

  const formatDate = (dateStr) => {
    if (!dateStr) return '-'
    const date = new Date(dateStr)
    return date.toLocaleDateString('de-DE', { day: '2-digit', month: '2-digit', year: 'numeric' })
  }

  return (
    <div className="flex-1 p-4 md:p-6 overflow-auto">
      <div className="max-w-6xl mx-auto">
        {/* Header */}
        <div className="mb-4 md:mb-6">
          <h1 className="text-xl md:text-2xl font-bold text-white">Mein Portfolio</h1>
          <p className="text-gray-500 text-sm">Verwalte deine Aktien und Investitionen</p>
        </div>

        {/* Portfolio Performance Chart */}
        {positions.length > 0 && (
          <div className="mb-4 md:mb-6">
            <PortfolioChart token={token} height={250} />
          </div>
        )}

        {/* Performance Stats */}
        {performance && (
          <div className="bg-dark-800 rounded-xl border border-dark-600 p-4 md:p-6 mb-4 md:mb-6">
            <h2 className="text-lg font-semibold text-white mb-4">Performance Übersicht</h2>

            {/* Main Stats */}
            <div className="grid grid-cols-2 md:grid-cols-4 gap-2 md:gap-4 mb-4">
              <div className="bg-dark-700 rounded-lg p-3 md:p-4">
                <div className="text-xs text-gray-500 mb-1">Gesamtwert</div>
                <div className="text-base md:text-xl font-bold text-white">
                  {formatPrice(performance.total_value)}
                </div>
              </div>
              <div className="bg-dark-700 rounded-lg p-3 md:p-4">
                <div className="text-xs text-gray-500 mb-1">Investiert</div>
                <div className="text-base md:text-xl font-bold text-white">
                  {formatPrice(performance.total_invested)}
                </div>
              </div>
              <div className="bg-dark-700 rounded-lg p-3 md:p-4">
                <div className="text-xs text-gray-500 mb-1">Gewinn/Verlust</div>
                <div className={`text-base md:text-xl font-bold ${performance.total_return >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                  {formatPrice(performance.total_return)}
                </div>
              </div>
              <div className="bg-dark-700 rounded-lg p-3 md:p-4">
                <div className="text-xs text-gray-500 mb-1">Rendite</div>
                <div className={`text-base md:text-xl font-bold ${performance.total_return_pct >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                  {formatPercent(performance.total_return_pct)}
                </div>
              </div>
            </div>

            {!performance.has_quantities && performance.positions_count > 0 && (
              <p className="text-xs text-gray-500 mt-3">
                * Ohne Stückzahlen wird eine Gleichverteilung angenommen
              </p>
            )}
          </div>
        )}

        {/* Search & Add Section */}
        <div className="bg-dark-800 rounded-xl border border-dark-600 p-4 md:p-6 mb-4 md:mb-6">
          <div className="flex flex-col md:flex-row md:items-center justify-between gap-3 md:gap-4 mb-4">
            <h2 className="text-lg font-semibold text-white">Positionen ({positions.length})</h2>

            {/* Search Input */}
            <div className="relative flex-1 max-w-full md:max-w-md" ref={searchRef}>
              <input
                type="text"
                placeholder="Aktie suchen (z.B. AAPL, Tesla)"
                value={searchQuery}
                onChange={handleSearchChange}
                onFocus={() => searchResults.length > 0 && setShowDropdown(true)}
                className="w-full px-4 py-2.5 bg-dark-700 border border-dark-600 rounded-lg text-sm text-white placeholder-gray-500 focus:outline-none focus:border-accent-500"
              />
              {searching && (
                <div className="absolute right-3 top-1/2 -translate-y-1/2">
                  <div className="w-4 h-4 border-2 border-accent-500 border-t-transparent rounded-full animate-spin"></div>
                </div>
              )}

              {showDropdown && searchResults.length > 0 && (
                <div className="absolute z-50 w-full mt-1 bg-dark-700 border border-dark-600 rounded-lg shadow-xl max-h-64 overflow-auto">
                  {searchResults.map((result) => (
                    <button
                      key={result.symbol}
                      onClick={() => handleSelectStock(result)}
                      className="w-full px-4 py-3 text-left hover:bg-dark-600 transition-colors flex items-center justify-between"
                    >
                      <div>
                        <div className="flex items-center gap-2">
                          <span className="font-medium text-white">{result.symbol}</span>
                          <span className="text-xs px-1.5 py-0.5 bg-dark-800 text-gray-400 rounded">
                            {result.exchange}
                          </span>
                        </div>
                        <p className="text-xs text-gray-500 truncate">{result.name}</p>
                      </div>
                      <svg className="w-4 h-4 text-accent-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 4v16m8-8H4" />
                      </svg>
                    </button>
                  ))}
                </div>
              )}
            </div>
          </div>

          {/* Add/Edit Form */}
          {showForm && (
            <form onSubmit={handleSubmit} className="bg-dark-700 rounded-lg p-4 mb-4">
              <div className="flex items-center gap-2 mb-4">
                <span className="font-semibold text-white">{formData.symbol}</span>
                <span className="text-gray-500 text-sm truncate">{formData.name}</span>
              </div>

              <div className="grid grid-cols-2 md:grid-cols-4 gap-3 md:gap-4 mb-4">
                <div>
                  <label className="block text-xs text-gray-500 mb-1">
                    Kaufkurs *
                  </label>
                  <input
                    type="number"
                    step="0.01"
                    required
                    value={formData.avg_price}
                    onChange={(e) => setFormData({ ...formData, avg_price: e.target.value })}
                    placeholder="0.00"
                    className="w-full px-3 py-2 bg-dark-800 border border-dark-600 rounded-lg text-white placeholder-gray-600 focus:outline-none focus:border-accent-500"
                  />
                </div>
                <div>
                  <label className="block text-xs text-gray-500 mb-1">
                    Währung
                  </label>
                  <select
                    value={formData.currency}
                    onChange={(e) => setFormData({ ...formData, currency: e.target.value })}
                    className="w-full px-3 py-2 bg-dark-800 border border-dark-600 rounded-lg text-white focus:outline-none focus:border-accent-500"
                  >
                    <option value="EUR">EUR</option>
                    <option value="USD">USD</option>
                    <option value="GBP">GBP</option>
                    <option value="CHF">CHF</option>
                  </select>
                </div>
                <div>
                  <label className="block text-xs text-gray-500 mb-1">
                    Anzahl
                  </label>
                  <input
                    type="number"
                    step="0.0001"
                    value={formData.quantity}
                    onChange={(e) => setFormData({ ...formData, quantity: e.target.value })}
                    placeholder="z.B. 10"
                    className="w-full px-3 py-2 bg-dark-800 border border-dark-600 rounded-lg text-white placeholder-gray-600 focus:outline-none focus:border-accent-500"
                  />
                </div>
                <div>
                  <label className="block text-xs text-gray-500 mb-1">
                    Kaufdatum
                  </label>
                  <input
                    type="date"
                    value={formData.purchase_date}
                    onChange={(e) => setFormData({ ...formData, purchase_date: e.target.value })}
                    className="w-full px-3 py-2 bg-dark-800 border border-dark-600 rounded-lg text-white focus:outline-none focus:border-accent-500"
                  />
                </div>
              </div>

              <div className="flex gap-2">
                <button
                  type="submit"
                  disabled={submitting}
                  className="px-4 py-2 bg-accent-500 text-white rounded-lg hover:bg-accent-400 transition-colors disabled:opacity-50 text-sm"
                >
                  {submitting ? 'Speichern...' : (editingPosition ? 'Aktualisieren' : 'Hinzufügen')}
                </button>
                <button
                  type="button"
                  onClick={handleCancel}
                  className="px-4 py-2 bg-dark-600 text-gray-300 rounded-lg hover:bg-dark-500 transition-colors text-sm"
                >
                  Abbrechen
                </button>
              </div>
            </form>
          )}

          {/* Positions List */}
          {loading ? (
            <div className="text-center py-8">
              <div className="w-8 h-8 border-2 border-accent-500 border-t-transparent rounded-full animate-spin mx-auto"></div>
            </div>
          ) : positions.length === 0 ? (
            <div className="text-center py-8 md:py-12">
              <div className="w-16 h-16 mx-auto mb-4 bg-dark-700 rounded-full flex items-center justify-center">
                <svg className="w-8 h-8 text-gray-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M12 4v16m8-8H4" />
                </svg>
              </div>
              <p className="text-gray-500">Keine Positionen vorhanden</p>
              <p className="text-gray-600 text-sm mt-1">Suche oben nach einer Aktie</p>
            </div>
          ) : (
            <>
              {/* Mobile Card View */}
              <div className="md:hidden space-y-3">
                {positions.map((pos) => (
                  <div key={pos.id} className="bg-dark-700 rounded-lg p-4">
                    <div className="flex justify-between items-start mb-3">
                      <div>
                        <div className="font-semibold text-white">{pos.symbol}</div>
                        <div className="text-xs text-gray-500 truncate max-w-[180px]">{pos.name}</div>
                      </div>
                      <div className="flex gap-1">
                        <button
                          onClick={() => openSellModal(pos)}
                          className="p-2 text-green-400 hover:text-green-300"
                          title="Verkaufen"
                        >
                          <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 8c-1.657 0-3 .895-3 2s1.343 2 3 2 3 .895 3 2-1.343 2-3 2m0-8c1.11 0 2.08.402 2.599 1M12 8V7m0 1v8m0 0v1m0-1c-1.11 0-2.08-.402-2.599-1M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
                          </svg>
                        </button>
                        <button
                          onClick={() => handleEdit(pos)}
                          className="p-2 text-gray-400 hover:text-white"
                        >
                          <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15.232 5.232l3.536 3.536m-2.036-5.036a2.5 2.5 0 113.536 3.536L6.5 21.036H3v-3.572L16.732 3.732z" />
                          </svg>
                        </button>
                        <button
                          onClick={() => handleDelete(pos.id)}
                          className="p-2 text-gray-400 hover:text-red-400"
                        >
                          <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
                          </svg>
                        </button>
                      </div>
                    </div>

                    <div className="grid grid-cols-2 gap-3 text-sm">
                      <div>
                        <div className="text-xs text-gray-500">Kaufkurs</div>
                        <div className="text-white">{CURRENCY_SYMBOLS[pos.currency] || '€'}{pos.avg_price.toFixed(2)}</div>
                      </div>
                      <div>
                        <div className="text-xs text-gray-500">Aktuell</div>
                        <div className="text-white">{formatPrice(pos.current_price, pos.symbol)}</div>
                      </div>
                      <div>
                        <div className="text-xs text-gray-500">Rendite</div>
                        <div className={`font-medium ${pos.total_return_pct >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                          {formatPercent(pos.total_return_pct)}
                        </div>
                      </div>
                      <div>
                        <div className="text-xs text-gray-500">Anzahl</div>
                        <div className="text-gray-400">{pos.quantity || '-'}</div>
                      </div>
                    </div>

                    {pos.purchase_date && (
                      <div className="mt-2 pt-2 border-t border-dark-600 text-xs text-gray-500">
                        Gekauft: {formatDate(pos.purchase_date)}
                      </div>
                    )}
                  </div>
                ))}
              </div>

              {/* Desktop Table View */}
              <div className="hidden md:block overflow-x-auto">
                <table className="w-full">
                  <thead>
                    <tr className="text-left text-xs text-gray-500 border-b border-dark-600">
                      <th className="pb-3 pr-4">Symbol</th>
                      <th className="pb-3 pr-4">Kaufkurs</th>
                      <th className="pb-3 pr-4">Anzahl</th>
                      <th className="pb-3 pr-4">Aktuell</th>
                      <th className="pb-3 pr-4">Rendite</th>
                      <th className="pb-3 pr-4">Kaufdatum</th>
                      <th className="pb-3"></th>
                    </tr>
                  </thead>
                  <tbody>
                    {positions.map((pos) => (
                      <tr key={pos.id} className="border-b border-dark-700/50 last:border-0">
                        <td className="py-3 pr-4">
                          <div className="font-medium text-white">{pos.symbol}</div>
                          <div className="text-xs text-gray-500 truncate max-w-[150px]">{pos.name}</div>
                        </td>
                        <td className="py-3 pr-4">
                          <div className="text-white">
                            {CURRENCY_SYMBOLS[pos.currency] || '€'}{pos.avg_price.toFixed(2)}
                          </div>
                        </td>
                        <td className="py-3 pr-4">
                          <div className="text-gray-400">{pos.quantity || '-'}</div>
                        </td>
                        <td className="py-3 pr-4">
                          <div className="text-white">{formatPrice(pos.current_price, pos.symbol)}</div>
                          <div className={`text-xs ${pos.change_percent >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                            {formatPercent(pos.change_percent)} heute
                          </div>
                        </td>
                        <td className="py-3 pr-4">
                          <div className={`font-medium ${pos.total_return_pct >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                            {formatPercent(pos.total_return_pct)}
                          </div>
                          {pos.quantity && (
                            <div className={`text-xs ${pos.total_return >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                              {formatPrice(pos.current_value - pos.invested_value)}
                            </div>
                          )}
                        </td>
                        <td className="py-3 pr-4">
                          <div className="text-gray-400 text-sm">{formatDate(pos.purchase_date)}</div>
                        </td>
                        <td className="py-3">
                          <div className="flex items-center gap-1">
                            <button
                              onClick={() => openSellModal(pos)}
                              className="p-1.5 text-green-400 hover:text-green-300 transition-colors"
                              title="Verkaufen"
                            >
                              <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 8c-1.657 0-3 .895-3 2s1.343 2 3 2 3 .895 3 2-1.343 2-3 2m0-8c1.11 0 2.08.402 2.599 1M12 8V7m0 1v8m0 0v1m0-1c-1.11 0-2.08-.402-2.599-1M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
                              </svg>
                            </button>
                            <button
                              onClick={() => handleEdit(pos)}
                              className="p-1.5 text-gray-400 hover:text-white transition-colors"
                              title="Bearbeiten"
                            >
                              <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15.232 5.232l3.536 3.536m-2.036-5.036a2.5 2.5 0 113.536 3.536L6.5 21.036H3v-3.572L16.732 3.732z" />
                              </svg>
                            </button>
                            <button
                              onClick={() => handleDelete(pos.id)}
                              className="p-1.5 text-gray-400 hover:text-red-400 transition-colors"
                              title="Löschen"
                            >
                              <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
                              </svg>
                            </button>
                          </div>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </>
          )}
        </div>

        {/* Trade History Section */}
        <div className="bg-dark-800 rounded-xl border border-dark-600 p-4 md:p-6">
          <button
            onClick={() => setShowHistory(!showHistory)}
            className="w-full flex items-center justify-between"
          >
            <h2 className="text-lg font-semibold text-white">Trade History ({trades.length})</h2>
            <svg className={`w-5 h-5 text-gray-400 transition-transform ${showHistory ? 'rotate-180' : ''}`} fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 9l-7 7-7-7" />
            </svg>
          </button>

          {showHistory && trades.length > 0 && (
            <div className="mt-4 overflow-x-auto">
              <table className="w-full text-sm">
                <thead>
                  <tr className="text-left text-xs text-gray-500 border-b border-dark-600">
                    <th className="pb-2 pr-4">Symbol</th>
                    <th className="pb-2 pr-4">Kauf</th>
                    <th className="pb-2 pr-4">Verkauf</th>
                    <th className="pb-2 pr-4">Menge</th>
                    <th className="pb-2 pr-4 text-right">Rendite</th>
                    <th className="pb-2 text-right">Datum</th>
                  </tr>
                </thead>
                <tbody>
                  {trades.map((trade) => (
                    <tr key={trade.id} className="border-b border-dark-700/50 last:border-0">
                      <td className="py-2 pr-4">
                        <div className="font-medium text-white">{trade.symbol}</div>
                        <div className="text-xs text-gray-500 truncate max-w-[100px]">{trade.name}</div>
                      </td>
                      <td className="py-2 pr-4 text-green-400">
                        {CURRENCY_SYMBOLS[trade.currency] || '€'}{trade.buy_price.toFixed(2)}
                      </td>
                      <td className="py-2 pr-4 text-red-400">
                        {CURRENCY_SYMBOLS[trade.currency] || '€'}{trade.sell_price.toFixed(2)}
                      </td>
                      <td className="py-2 pr-4 text-gray-400">
                        {trade.quantity}
                      </td>
                      <td className={`py-2 pr-4 text-right font-medium ${trade.profit_loss_pct >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                        {formatPercent(trade.profit_loss_pct)}
                      </td>
                      <td className="py-2 text-right text-gray-500">
                        {formatDate(trade.sell_date)}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}

          {showHistory && trades.length === 0 && (
            <p className="mt-4 text-gray-500 text-sm">Noch keine abgeschlossenen Trades.</p>
          )}
        </div>
      </div>

      {/* Sell Modal */}
      {sellingPosition && (
        <div className="fixed inset-0 bg-black/70 z-50 flex items-center justify-center p-4" onClick={() => setSellingPosition(null)}>
          <div className="bg-dark-800 rounded-xl border border-dark-600 max-w-md w-full p-6" onClick={e => e.stopPropagation()}>
            <h3 className="text-lg font-semibold text-white mb-4">Position verkaufen</h3>

            <div className="mb-4 p-3 bg-dark-700 rounded-lg">
              <div className="flex items-center gap-2 mb-2">
                <span className="font-bold text-white">{sellingPosition.symbol}</span>
                <span className="text-gray-500 text-sm">{sellingPosition.name}</span>
              </div>
              <div className="grid grid-cols-2 gap-2 text-sm">
                <div>
                  <span className="text-gray-500">Kaufkurs:</span>
                  <span className="text-white ml-2">{CURRENCY_SYMBOLS[sellingPosition.currency] || '€'}{sellingPosition.avg_price.toFixed(2)}</span>
                </div>
                <div>
                  <span className="text-gray-500">Aktuell:</span>
                  <span className="text-white ml-2">{formatPrice(sellingPosition.current_price, sellingPosition.symbol)}</span>
                </div>
              </div>
            </div>

            <form onSubmit={handleSell}>
              <div className="mb-4">
                <label className="block text-sm text-gray-400 mb-1">Verkaufspreis ({CURRENCY_SYMBOLS[sellingPosition.currency] || '€'})</label>
                <input
                  type="number"
                  step="0.01"
                  required
                  value={sellPrice}
                  onChange={(e) => setSellPrice(e.target.value)}
                  className="w-full px-3 py-2 bg-dark-700 border border-dark-600 rounded-lg text-white focus:outline-none focus:border-accent-500"
                  placeholder="0.00"
                />
              </div>

              {sellPrice && (
                <div className="mb-4 p-3 bg-dark-700 rounded-lg">
                  <div className="text-sm text-gray-400">Voraussichtliche Rendite:</div>
                  <div className={`text-xl font-bold ${parseFloat(sellPrice) >= sellingPosition.avg_price ? 'text-green-400' : 'text-red-400'}`}>
                    {formatPercent(((parseFloat(sellPrice) - sellingPosition.avg_price) / sellingPosition.avg_price) * 100)}
                  </div>
                </div>
              )}

              <div className="flex gap-3">
                <button
                  type="submit"
                  disabled={submitting}
                  className="flex-1 px-4 py-2 bg-green-500 text-white rounded-lg hover:bg-green-400 transition-colors disabled:opacity-50"
                >
                  {submitting ? 'Verkaufe...' : 'Verkaufen'}
                </button>
                <button
                  type="button"
                  onClick={() => setSellingPosition(null)}
                  className="px-4 py-2 bg-dark-600 text-gray-300 rounded-lg hover:bg-dark-500 transition-colors"
                >
                  Abbrechen
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  )
}

export default PortfolioManage
