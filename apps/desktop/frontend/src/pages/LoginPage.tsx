import { useAuth } from '../hooks/useAuth'
import { useEffect, useState } from 'react'
import { Loader2, Video, Server, KeyRound, User, Lock } from 'lucide-react'

type Mode = 'login' | 'redeem' | 'setup'

function LoginPage() {
  const { bootstrap, refreshBootstrap, setupAdmin, loginAdmin, redeemInvite } = useAuth()
  const [mode, setMode] = useState<Mode>('login')
  const [serverURL, setServerURL] = useState('http://127.0.0.1:8080')
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [setupToken, setSetupToken] = useState('')
  const [inviteCode, setInviteCode] = useState('')
  const [isLoading, setIsLoading] = useState(false)
  const [error, setError] = useState('')

  useEffect(() => {
    if (bootstrap?.serverURL) setServerURL(bootstrap.serverURL)
    if (bootstrap?.accountUsername) setUsername(bootstrap.accountUsername)
    if (bootstrap?.needsSetup) setMode('setup')
  }, [bootstrap])

  const handleProbeServer = async () => {
    setIsLoading(true)
    setError('')
    try {
      // Persist URL and re-probe so the UI can tell us if setup is needed.
      const b = await refreshBootstrap()
      if (!b?.reachable) {
        setError(`Could not reach ${serverURL}`)
      } else if (b.needsSetup) {
        setMode('setup')
      }
    } finally {
      setIsLoading(false)
    }
  }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setIsLoading(true)
    setError('')
    try {
      if (mode === 'setup') {
        await setupAdmin({ serverURL, setupToken, username, password })
      } else if (mode === 'login') {
        await loginAdmin({ serverURL, username, password })
      } else {
        await redeemInvite({ serverURL, code: inviteCode, username })
      }
    } catch (err: any) {
      setError(err?.message || String(err))
    } finally {
      setIsLoading(false)
    }
  }

  const needsSetupHint = bootstrap?.needsSetup && mode !== 'setup'

  return (
    <div className="min-h-screen bg-forest-950 flex items-center justify-center">
      <div className="w-full max-w-sm px-6">
        <div className="text-center mb-8">
          <div className="mx-auto h-14 w-14 rounded-2xl bg-forest-600 flex items-center justify-center mb-4">
            <Video className="h-7 w-7 text-forest-50" />
          </div>
          <h1 className="text-2xl font-bold text-sand-100 tracking-tight">ClipShare</h1>
          <p className="mt-2 text-sand-500 text-sm">
            {mode === 'setup' && 'First-time admin setup'}
            {mode === 'login' && 'Sign in as the admin'}
            {mode === 'redeem' && 'Redeem your invite code'}
          </p>
        </div>

        {/* Mode tabs */}
        <div className="grid grid-cols-2 gap-2 mb-4">
          <button
            type="button"
            className={`px-3 py-2 rounded-lg text-sm ${mode === 'login' ? 'bg-forest-700 text-sand-100' : 'bg-forest-900 text-sand-400 hover:text-sand-200'}`}
            onClick={() => setMode('login')}
          >Admin login</button>
          <button
            type="button"
            className={`px-3 py-2 rounded-lg text-sm ${mode === 'redeem' ? 'bg-forest-700 text-sand-100' : 'bg-forest-900 text-sand-400 hover:text-sand-200'}`}
            onClick={() => setMode('redeem')}
          >Invite code</button>
        </div>

        <div className="card">
          <form className="p-6 space-y-4" onSubmit={handleSubmit}>
            <FieldWithIcon icon={<Server className="h-4 w-4 text-sand-600" />} label="Server URL">
              <input
                type="url"
                required
                value={serverURL}
                onChange={(e) => setServerURL(e.target.value)}
                onBlur={handleProbeServer}
                className="input-field pl-10"
                placeholder="https://clipshare.example.com"
              />
            </FieldWithIcon>

            <FieldWithIcon icon={<User className="h-4 w-4 text-sand-600" />} label="Username">
              <input
                type="text"
                required
                autoCapitalize="none"
                autoCorrect="off"
                value={username}
                onChange={(e) => setUsername(e.target.value)}
                className="input-field pl-10"
                placeholder="alice"
              />
            </FieldWithIcon>

            {mode === 'setup' && (
              <FieldWithIcon icon={<KeyRound className="h-4 w-4 text-sand-600" />} label="One-time setup token">
                <input
                  type="text"
                  required
                  value={setupToken}
                  onChange={(e) => setSetupToken(e.target.value)}
                  className="input-field pl-10 font-mono"
                  placeholder="Printed in the server console"
                />
              </FieldWithIcon>
            )}

            {mode === 'redeem' && (
              <FieldWithIcon icon={<KeyRound className="h-4 w-4 text-sand-600" />} label="Invite code">
                <input
                  type="text"
                  required
                  value={inviteCode}
                  onChange={(e) => setInviteCode(e.target.value)}
                  className="input-field pl-10 font-mono"
                  placeholder="XXXX-XXXX-XXXX"
                />
              </FieldWithIcon>
            )}

            {(mode === 'setup' || mode === 'login') && (
              <FieldWithIcon icon={<Lock className="h-4 w-4 text-sand-600" />} label="Password">
                <input
                  type="password"
                  required
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  className="input-field pl-10"
                  placeholder={mode === 'setup' ? 'Pick a strong password' : '••••••••'}
                />
              </FieldWithIcon>
            )}

            {needsSetupHint && (
              <div className="text-xs px-3 py-2 rounded-lg bg-forest-900/70 text-forest-300">
                This server hasn&apos;t been set up yet. <button type="button" className="underline" onClick={() => setMode('setup')}>Run setup</button> first.
              </div>
            )}

            {error && (
              <div className="text-sm text-center px-3 py-2 rounded-lg bg-red-900/30 text-red-300">
                {error}
              </div>
            )}

            <button
              type="submit"
              disabled={isLoading}
              className="btn-primary w-full flex justify-center items-center space-x-2 py-3"
            >
              {isLoading ? (
                <Loader2 className="animate-spin h-5 w-5" />
              ) : (
                <span>
                  {mode === 'setup' && 'Create admin & continue'}
                  {mode === 'login' && 'Sign in'}
                  {mode === 'redeem' && 'Redeem invite'}
                </span>
              )}
            </button>
          </form>
        </div>

        <p className="mt-6 text-center text-xs text-sand-600">
          Credentials are stored in your OS keyring — no login next time.
        </p>
      </div>
    </div>
  )
}

function FieldWithIcon({ icon, label, children }: { icon: React.ReactNode; label: string; children: React.ReactNode }) {
  return (
    <div>
      <label className="block text-sm font-medium text-sand-400 mb-2">{label}</label>
      <div className="relative">
        <div className="absolute inset-y-0 left-0 pl-3.5 flex items-center pointer-events-none">{icon}</div>
        {children}
      </div>
    </div>
  )
}

export default LoginPage
