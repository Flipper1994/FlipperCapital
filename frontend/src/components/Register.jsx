import { useState } from 'react'
import { useNavigate, Link } from 'react-router-dom'

function Register({ onLogin }) {
  const [inviteCode, setInviteCode] = useState('')
  const [email, setEmail] = useState('')
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [confirmPassword, setConfirmPassword] = useState('')
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)
  const navigate = useNavigate()

  const handleSubmit = async (e) => {
    e.preventDefault()
    setError('')

    if (password !== confirmPassword) {
      setError('Passworter stimmen nicht uberein')
      return
    }

    if (password.length < 6) {
      setError('Passwort muss mindestens 6 Zeichen lang sein')
      return
    }

    setLoading(true)

    try {
      const res = await fetch('/api/register', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ email, username, password, invite_code: inviteCode })
      })

      const data = await res.json()

      if (res.ok && data.token) {
        onLogin(data.token, data.user)
        navigate('/')
      } else {
        setError(data.error || 'Registrierung fehlgeschlagen')
      }
    } catch (err) {
      setError('Verbindungsfehler. Bitte versuche es erneut.')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="flex-1 flex items-center justify-center p-6">
      <div className="w-full max-w-md">
        <div className="bg-dark-800 rounded-xl border border-dark-600 p-8">
          <div className="text-center mb-8">
            <h1 className="text-2xl font-bold text-white mb-2">Konto erstellen</h1>
            <p className="text-gray-500">Registriere dich fur FlipperCapital</p>
          </div>

          <form onSubmit={handleSubmit} className="space-y-5">
            {error && (
              <div className="p-3 bg-red-500/10 border border-red-500/20 rounded-lg text-red-400 text-sm">
                {error}
              </div>
            )}

            <div>
              <label className="block text-sm font-medium text-gray-400 mb-2">
                Invite-Code
              </label>
              <input
                type="text"
                value={inviteCode}
                onChange={(e) => setInviteCode(e.target.value)}
                className="w-full px-4 py-3 bg-dark-700 border border-dark-600 rounded-lg text-white placeholder-gray-500 focus:outline-none focus:border-accent-500 transition-colors"
                placeholder="Invite-Code eingeben"
                required
              />
              <p className="text-xs text-gray-500 mt-1">
                Du benötigst einen Invite-Code. Diesen kannst du kaufen oder beim Inhaber anfragen.
              </p>
            </div>

            <div>
              <label className="block text-sm font-medium text-gray-400 mb-2">
                E-Mail
              </label>
              <input
                type="email"
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                className="w-full px-4 py-3 bg-dark-700 border border-dark-600 rounded-lg text-white placeholder-gray-500 focus:outline-none focus:border-accent-500 transition-colors"
                placeholder="email@beispiel.de"
                required
              />
            </div>

            <div>
              <label className="block text-sm font-medium text-gray-400 mb-2">
                Benutzername
              </label>
              <input
                type="text"
                value={username}
                onChange={(e) => setUsername(e.target.value)}
                className="w-full px-4 py-3 bg-dark-700 border border-dark-600 rounded-lg text-white placeholder-gray-500 focus:outline-none focus:border-accent-500 transition-colors"
                placeholder="dein_benutzername"
                required
              />
            </div>

            <div>
              <label className="block text-sm font-medium text-gray-400 mb-2">
                Passwort
              </label>
              <input
                type="password"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                className="w-full px-4 py-3 bg-dark-700 border border-dark-600 rounded-lg text-white placeholder-gray-500 focus:outline-none focus:border-accent-500 transition-colors"
                placeholder="Mindestens 6 Zeichen"
                required
              />
            </div>

            <div>
              <label className="block text-sm font-medium text-gray-400 mb-2">
                Passwort bestatigen
              </label>
              <input
                type="password"
                value={confirmPassword}
                onChange={(e) => setConfirmPassword(e.target.value)}
                className="w-full px-4 py-3 bg-dark-700 border border-dark-600 rounded-lg text-white placeholder-gray-500 focus:outline-none focus:border-accent-500 transition-colors"
                placeholder="Passwort wiederholen"
                required
              />
            </div>

            <button
              type="submit"
              disabled={loading}
              className="w-full py-3 bg-accent-500 hover:bg-accent-600 disabled:bg-accent-500/50 text-white font-medium rounded-lg transition-colors"
            >
              {loading ? 'Wird registriert...' : 'Registrieren'}
            </button>
          </form>

          <div className="mt-6 text-center">
            <p className="text-gray-500 text-sm">
              Bereits ein Konto?{' '}
              <Link to="/login" className="text-accent-400 hover:text-accent-300 transition-colors">
                Jetzt anmelden
              </Link>
            </p>
          </div>
        </div>
      </div>
    </div>
  )
}

export default Register
