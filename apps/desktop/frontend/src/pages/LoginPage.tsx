import { useAuth } from '../hooks/useAuth'
import { useEffect, useState } from 'react'
import { Loader2, Server, KeyRound, User, Lock } from 'lucide-react'
import { ProbeServer } from '../../wailsjs/go/main/App'

type Mode = 'login' | 'redeem' | 'setup'

function LoginPage() {
  const { bootstrap, setupAdmin, loginAdmin, redeemInvite } = useAuth()
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
      const b = await ProbeServer(serverURL)
      if (!b?.reachable) {
        setError(`Could not reach ${serverURL}`)
      } else if (b.needs_setup) {
        setMode('setup')
      } else {
        setMode('login')
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

  const modeLabel =
    mode === 'setup' ? 'First-time setup' :
    mode === 'login' ? 'Admin login' :
    'Redeem invite'

  return (
    <div className="min-h-screen bg-forest-950 text-sand-100 flex flex-col">
      {/* Top accent line */}
      <div className="h-[2px] bg-gradient-to-r from-transparent via-forest-500/70 to-transparent shrink-0" />

      <div className="flex-1 flex items-center justify-center py-12 px-6">
        <div className="w-full max-w-sm">
          {/* Logo */}
          <div className="flex items-center gap-3 mb-7 justify-center">
            <div
              className="h-9 w-9 bg-forest-600 flex items-center justify-center rounded-sm shrink-0"
              style={{ boxShadow: '0 0 10px rgba(82, 176, 67, 0.3)' }}
            >
              <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 -960 960 960" className="h-5 w-5 text-white" fill="currentColor">
                <path d="M160-240v-320 320Zm0 80q-33 0-56.5-23.5T80-240v-480q0-33 23.5-56.5T160-800l80 160h120l-80-160h80l80 160h120l-80-160h80l80 160h120l-80-160h120q33 0 56.5 23.5T880-720v160H160v320h320v80H160Zm400 40v-123l221-220q9-9 20-13t22-4q12 0 23 4.5t20 13.5l37 37q8 9 12.5 20t4.5 22q0 11-4 22.5T903-340L683-120H560Zm300-263-37-37 37 37ZM620-180h38l121-122-18-19-19-18-122 121v38Zm141-141-19-18 37 37-18-19Z" />
              </svg>
            </div>
            <span
              className="font-black text-white text-xl tracking-[0.18em] uppercase nav-font"
              style={{ textShadow: '0 0 12px rgba(82, 176, 67, 0.4)' }}
            >
              ClipShare
            </span>
          </div>

          {/* Mode tabs */}
          <div className="grid grid-cols-2 gap-2 mb-3">
            <button
              type="button"
              className={`xbox-tab ${mode === 'login' ? 'xbox-tab-active' : 'xbox-tab-inactive'}`}
              onClick={() => setMode('login')}
            >Admin login</button>
            <button
              type="button"
              className={`xbox-tab ${mode === 'redeem' ? 'xbox-tab-active' : 'xbox-tab-inactive'}`}
              onClick={() => setMode('redeem')}
            >Invite code</button>
          </div>

          <div className="card">
            <div className="section-header flex items-center gap-2.5">
              <Lock className="h-4 w-4 text-sand-500 shrink-0" />
              <h2 className="text-base font-semibold text-sand-200">{modeLabel}</h2>
            </div>

            <form className="card-body space-y-4" onSubmit={handleSubmit}>
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
                <div className="xbox-info text-xs">
                  This server hasn&apos;t been set up yet.{' '}
                  <button type="button" className="underline" onClick={() => setMode('setup')}>Run setup</button> first.
                </div>
              )}

              {error && (
                <div className="xbox-error text-center">
                  {error}
                </div>
              )}

              <button
                type="submit"
                disabled={isLoading}
                className="btn-primary w-full flex justify-center items-center gap-2 py-3"
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

          <p className="mt-5 text-center text-xs text-sand-600">
            Credentials stored in OS keyring — no login next time.
          </p>
        </div>
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
