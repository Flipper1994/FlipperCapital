import { useState, useEffect } from 'react'

const cache = { data: null, fetched: 0 }

export function useBlockedStocks() {
  const [blockedStocks, setBlockedStocks] = useState(cache.data || {})

  useEffect(() => {
    if (cache.data && Date.now() - cache.fetched < 5 * 60 * 1000) {
      setBlockedStocks(cache.data)
      return
    }
    const token = localStorage.getItem('authToken')
    if (!token) return

    fetch('/api/bot-blocked-stocks', {
      headers: { 'Authorization': `Bearer ${token}` }
    })
      .then(r => r.json())
      .then(data => {
        cache.data = data
        cache.fetched = Date.now()
        setBlockedStocks(data)
      })
      .catch(() => {})
  }, [])

  return blockedStocks
}
