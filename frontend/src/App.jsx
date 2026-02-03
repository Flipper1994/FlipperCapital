import { useState, useEffect } from 'react'
import { Routes, Route } from 'react-router-dom'
import { CurrencyProvider } from './context/CurrencyContext'
import { TradingModeProvider } from './context/TradingModeContext'
import Header from './components/Header'
import Sidebar from './components/Sidebar'
import Dashboard from './components/Dashboard'
import Login from './components/Login'
import Register from './components/Register'
import PortfolioManage from './components/PortfolioManage'
import PortfolioCompare from './components/PortfolioCompare'
import StockTracker from './components/StockTracker'
import AdminPanel from './components/AdminPanel'
import FlipperBot from './components/FlipperBot'
import FlipperBotLab from './components/FlipperBotLab'
import LutzLab from './components/LutzLab'
import QuantLab from './components/QuantLab'
import Performance from './components/Performance'
import Help from './components/Help'

function App() {
  const [isAdmin, setIsAdmin] = useState(false)
  const [isLoggedIn, setIsLoggedIn] = useState(false)
  const [user, setUser] = useState(null)
  const [token, setToken] = useState(localStorage.getItem('authToken') || '')
  const [sidebarOpen, setSidebarOpen] = useState(false)

  useEffect(() => {
    if (token) {
      verifyToken()
    }
  }, [])

  const verifyToken = async () => {
    try {
      const res = await fetch('/api/verify', {
        headers: { 'Authorization': `Bearer ${token}` }
      })
      const data = await res.json()
      if (data.valid) {
        setIsLoggedIn(true)
        setIsAdmin(data.user?.is_admin || false)
        setUser(data.user || null)
      } else {
        clearAuth()
      }
    } catch {
      clearAuth()
    }
  }

  const clearAuth = () => {
    localStorage.removeItem('authToken')
    setToken('')
    setIsLoggedIn(false)
    setIsAdmin(false)
    setUser(null)
  }

  const handleLogin = (newToken, userData) => {
    setToken(newToken)
    localStorage.setItem('authToken', newToken)
    setIsLoggedIn(true)
    setIsAdmin(userData?.is_admin || false)
    setUser(userData)
  }

  const handleLogout = async () => {
    try {
      await fetch('/api/logout', {
        method: 'POST',
        headers: { 'Authorization': `Bearer ${token}` }
      })
    } catch {
      // Ignore errors, just clear local state
    }
    clearAuth()
  }

  return (
    <CurrencyProvider>
      <TradingModeProvider>
        <div className="min-h-screen bg-dark-900 flex flex-col">
          <Header
            isLoggedIn={isLoggedIn}
            isAdmin={isAdmin}
            user={user}
            onLogout={handleLogout}
            sidebarOpen={sidebarOpen}
            setSidebarOpen={setSidebarOpen}
          />
          <div className="flex flex-1 overflow-x-hidden">
            <Sidebar
              isLoggedIn={isLoggedIn}
              isAdmin={isAdmin}
              user={user}
              isOpen={sidebarOpen}
              onClose={() => setSidebarOpen(false)}
            />
            <Routes>
              <Route
                path="/"
                element={
                  <Dashboard
                    isAdmin={isAdmin}
                    token={token}
                  />
                }
              />
              <Route
                path="/login"
                element={<Login onLogin={handleLogin} />}
              />
              <Route
                path="/register"
                element={<Register onLogin={handleLogin} />}
              />
              <Route
                path="/portfolio"
                element={<PortfolioManage />}
              />
              <Route
                path="/compare"
                element={<PortfolioCompare />}
              />
              <Route
                path="/tracker"
                element={<StockTracker />}
              />
              <Route
                path="/performance"
                element={<Performance token={token} />}
              />
              <Route
                path="/admin"
                element={<AdminPanel />}
              />
              <Route
                path="/flipperbot"
                element={<FlipperBot />}
              />
              <Route
                path="/help"
                element={<Help />}
              />
              <Route
                path="/flipperbot-lab"
                element={<FlipperBotLab isAdmin={isAdmin} isLoggedIn={isLoggedIn} token={token} />}
              />
              <Route
                path="/lutz-lab"
                element={<LutzLab isAdmin={isAdmin} isLoggedIn={isLoggedIn} token={token} />}
              />
              <Route
                path="/quant-lab"
                element={<QuantLab isAdmin={isAdmin} isLoggedIn={isLoggedIn} token={token} />}
              />
            </Routes>
          </div>
        </div>
      </TradingModeProvider>
    </CurrencyProvider>
  )
}

export default App
