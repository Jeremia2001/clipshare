import { useCallback, useEffect, useState } from 'react'
import { useAuth } from '../hooks/useAuth'
import { Settings as SettingsIcon, User, Server, Shield, Ticket, LogOut, Copy, Trash2, Plus, Database } from 'lucide-react'
import { api } from '../services/api'

interface InviteRow {
  id: string
  note?: string
  redeemed_by?: string
  redeemed_at?: string
  expires_at?: string
  created_at: string
}

interface StorageStatus {
  limit_bytes: number
  used_bytes: number
}

function SettingsPage() {
  const { user, isDevMode, bootstrap, logout } = useAuth()
  const isAdmin = !!user?.is_admin

  return (
    <div className="space-y-6">
      <div className="flex items-center space-x-3">
        <div className="h-10 w-10 rounded-lg bg-forest-800/50 flex items-center justify-center">
          <SettingsIcon className="h-5 w-5 text-forest-400" />
        </div>
        <div>
          <h1 className="text-2xl font-bold text-sand-100">Settings</h1>
          <p className="text-sm text-sand-500">Manage your app and account preferences</p>
        </div>
      </div>

      {/* Server info (read-only; change via re-login) */}
      <div className="card">
        <div className="section-header flex items-center space-x-2.5">
          <Server className="h-4 w-4 text-sand-500" />
          <h2 className="text-base font-semibold text-sand-200">Server</h2>
        </div>
        <div className="card-body space-y-2">
          <p className="text-sm text-sand-300">{bootstrap?.serverURL || '—'}</p>
          <p className="text-xs text-sand-600">
            To connect to a different server, sign out and sign back in.
          </p>
        </div>
      </div>

      {/* Account */}
      {user && (
        <div className="card">
          <div className="section-header flex items-center space-x-2.5">
            <Shield className="h-4 w-4 text-sand-500" />
            <h2 className="text-base font-semibold text-sand-200">Account</h2>
          </div>
          <div className="card-body space-y-5">
            <div className="flex items-center space-x-4">
              <div className="h-10 w-10 rounded-lg bg-earth-800/50 flex items-center justify-center shrink-0">
                <User className="h-5 w-5 text-earth-400" />
              </div>
              <div className="flex-1">
                <label className="text-xs font-medium text-sand-600 uppercase tracking-wider">Username</label>
                <p className="text-sand-200 text-sm">{user.username}{user.is_admin && <span className="ml-2 text-xs text-forest-400">(admin)</span>}</p>
              </div>
            </div>

            {!isDevMode && (
              <div className="border-t border-forest-800/40 pt-5">
                <button
                  onClick={logout}
                  className="flex items-center space-x-2 text-sm text-red-300 hover:text-red-200"
                >
                  <LogOut className="h-4 w-4" />
                  <span>Sign out &amp; forget this device</span>
                </button>
              </div>
            )}
          </div>
        </div>
      )}

      {!isDevMode && <ServerStorage isAdmin={isAdmin} />}

      {isAdmin && !isDevMode && <InviteManager />}

      {isDevMode && (
        <div className="card border-earth-700/50">
          <div className="card-body flex items-center space-x-3">
            <div className="h-2.5 w-2.5 rounded-full bg-earth-500 animate-pulse" />
            <span className="text-sm text-earth-300">
              Running in development mode — authentication is disabled
            </span>
          </div>
        </div>
      )}
    </div>
  )
}

function InviteManager() {
  const [invites, setInvites] = useState<InviteRow[]>([])
  const [loading, setLoading] = useState(false)
  const [note, setNote] = useState('')
  const [lastCode, setLastCode] = useState<string | null>(null)
  const [error, setError] = useState('')

  const refresh = useCallback(async () => {
    setLoading(true)
    try {
      const resp = await api.get<{ invites: InviteRow[] }>('/auth/invites')
      setInvites(resp.data.invites || [])
    } catch (err: any) {
      setError(err?.response?.data?.error || String(err))
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => { refresh() }, [refresh])

  const handleCreate = async () => {
    setError('')
    try {
      const resp = await api.post<{ code: string; invite: InviteRow }>('/auth/invites', {
        note: note.trim() || undefined,
      })
      setLastCode(resp.data.code)
      setNote('')
      await refresh()
    } catch (err: any) {
      setError(err?.response?.data?.error || String(err))
    }
  }

  const handleDelete = async (id: string) => {
    try {
      await api.delete(`/auth/invites/${id}`)
      await refresh()
    } catch (err: any) {
      setError(err?.response?.data?.error || 'Failed to delete invite')
    }
  }

  return (
    <div className="card">
      <div className="section-header flex items-center space-x-2.5">
        <Ticket className="h-4 w-4 text-sand-500" />
        <h2 className="text-base font-semibold text-sand-200">Invites</h2>
      </div>
      <div className="card-body space-y-4">
        <p className="text-xs text-sand-500">
          Generate single-use invite codes. Share one with a user along with the server URL.
          Each code registers the first device used to redeem it.
        </p>

        <div className="flex space-x-2">
          <input
            value={note}
            onChange={(e) => setNote(e.target.value)}
            className="input-field flex-1"
            placeholder="Optional note (who is this for?)"
          />
          <button onClick={handleCreate} className="btn-primary flex items-center space-x-2">
            <Plus className="h-4 w-4" /> <span>New code</span>
          </button>
        </div>

        {lastCode && (
          <div className="px-3 py-3 rounded-lg bg-forest-900/70 border border-forest-700">
            <div className="text-xs text-sand-500 mb-1">New invite code (shown once):</div>
            <div className="flex items-center justify-between">
              <code className="font-mono text-sand-100 text-lg tracking-wider">{lastCode}</code>
              <button
                className="text-sand-400 hover:text-sand-200"
                onClick={() => navigator.clipboard.writeText(lastCode)}
                title="Copy to clipboard"
              >
                <Copy className="h-4 w-4" />
              </button>
            </div>
          </div>
        )}

        {error && (
          <div className="text-sm text-red-300">{error}</div>
        )}

        <div className="border-t border-forest-800/40 pt-4">
          <div className="text-xs text-sand-600 mb-2">{loading ? 'Loading…' : `${invites.length} invite${invites.length === 1 ? '' : 's'}`}</div>
          <div className="space-y-2">
            {invites.map((inv) => (
              <div key={inv.id} className="flex items-center justify-between px-3 py-2 rounded-lg bg-forest-900/40">
                <div className="text-sm">
                  <div className="text-sand-200">{inv.note || <span className="text-sand-600 italic">no note</span>}</div>
                  <div className="text-xs text-sand-600">
                    {inv.redeemed_at
                      ? `redeemed ${new Date(inv.redeemed_at).toLocaleString()}`
                      : `created ${new Date(inv.created_at).toLocaleString()}`}
                    {inv.expires_at && !inv.redeemed_at && ` · expires ${new Date(inv.expires_at).toLocaleString()}`}
                  </div>
                </div>
                <button
                  onClick={() => handleDelete(inv.id)}
                  className="text-sand-500 hover:text-red-300"
                  title={inv.redeemed_at ? 'Remove entry' : 'Revoke'}
                >
                  <Trash2 className="h-4 w-4" />
                </button>
              </div>
            ))}
          </div>
        </div>
      </div>
    </div>
  )
}

function ServerStorage({ isAdmin }: { isAdmin: boolean }) {
  const [status, setStatus] = useState<StorageStatus | null>(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const [limitInput, setLimitInput] = useState('')
  const [unit, setUnit] = useState<'GB' | 'MB'>('GB')
  const [saved, setSaved] = useState(false)

  const refresh = useCallback(async () => {
    setLoading(true)
    try {
      const resp = await api.get<StorageStatus>('/instance/storage')
      setStatus(resp.data)
      if (resp.data.limit_bytes > 0 && !limitInput) {
        const gb = resp.data.limit_bytes / 1024 ** 3
        if (gb >= 1) { setLimitInput(String(Math.round(gb * 10) / 10)); setUnit('GB') }
        else { setLimitInput(String(Math.round(resp.data.limit_bytes / 1024 ** 2))); setUnit('MB') }
      }
    } catch (err: any) {
      setError(err?.response?.data?.error || String(err))
    } finally {
      setLoading(false)
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  useEffect(() => { refresh() }, [refresh])

  const handleSave = async () => {
    setError('')
    const n = parseFloat(limitInput)
    if (Number.isNaN(n) || n < 0) {
      setError('Enter a non-negative number (or 0 for unlimited)')
      return
    }
    const bytes = Math.round(n * (unit === 'GB' ? 1024 ** 3 : 1024 ** 2))
    try {
      const resp = await api.put<StorageStatus>('/instance/storage', { limit_bytes: bytes })
      setStatus(resp.data)
      setSaved(true)
      setTimeout(() => setSaved(false), 2000)
    } catch (err: any) {
      setError(err?.response?.data?.error || String(err))
    }
  }

  const used = status?.used_bytes || 0
  const limit = status?.limit_bytes || 0
  const percent = limit > 0 ? Math.min(100, (used / limit) * 100) : 0
  const isFull = limit > 0 && used >= limit
  const barColor = isFull ? 'bg-red-500' : percent > 85 ? 'bg-earth-500' : 'bg-forest-500'

  return (
    <div className="card">
      <div className="section-header flex items-center space-x-2.5">
        <Database className="h-4 w-4 text-sand-500" />
        <h2 className="text-base font-semibold text-sand-200">Server storage</h2>
      </div>
      <div className="card-body space-y-4">
        {loading && !status ? (
          <div className="text-sm text-sand-500">Loading…</div>
        ) : (
          <>
            <div>
              <div className="flex justify-between text-sm text-sand-300">
                <span>{formatBytes(used)} used</span>
                <span>{limit > 0 ? `of ${formatBytes(limit)}` : 'no limit'}</span>
              </div>
              {limit > 0 && (
                <div className="mt-2 h-2 bg-forest-900 rounded-full overflow-hidden">
                  <div className={`h-full ${barColor} rounded-full transition-all duration-500`} style={{ width: `${percent}%` }} />
                </div>
              )}
              {isFull && (
                <p className="mt-2 text-xs text-red-300">Server is full. New uploads will be rejected until space is freed.</p>
              )}
            </div>

            {isAdmin && (
              <div className="border-t border-forest-800/40 pt-4">
                <label className="block text-xs font-medium text-sand-600 uppercase tracking-wider mb-2">
                  Storage limit (0 = unlimited)
                </label>
                <div className="flex space-x-2">
                  <input
                    type="number"
                    min="0"
                    step="0.1"
                    value={limitInput}
                    onChange={(e) => setLimitInput(e.target.value)}
                    className="input-field flex-1"
                    placeholder="e.g. 50"
                  />
                  <select
                    value={unit}
                    onChange={(e) => setUnit(e.target.value as 'GB' | 'MB')}
                    className="input-field w-24"
                  >
                    <option value="GB">GB</option>
                    <option value="MB">MB</option>
                  </select>
                  <button onClick={handleSave} className="btn-primary whitespace-nowrap">Save</button>
                </div>
                {saved && <span className="text-xs text-forest-400 mt-2 inline-block">Saved</span>}
              </div>
            )}

            {error && <div className="text-sm text-red-300">{error}</div>}
          </>
        )}
      </div>
    </div>
  )
}

function formatBytes(n: number): string {
  if (n <= 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.min(units.length - 1, Math.floor(Math.log(n) / Math.log(1024)))
  const v = n / Math.pow(1024, i)
  return `${v < 10 ? v.toFixed(2) : v.toFixed(1)} ${units[i]}`
}

export default SettingsPage
