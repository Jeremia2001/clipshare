import { useState } from 'react'
import { useAuth } from '../hooks/useAuth'
import { Settings as SettingsIcon, Mail, HardDrive, Server, Shield } from 'lucide-react'

function SettingsPage() {
  const { user, isDevMode } = useAuth()
  const [apiUrl, setApiUrl] = useState(localStorage.getItem('api_url') || 'http://localhost:8080')
  const [saved, setSaved] = useState(false)

  const handleSave = () => {
    localStorage.setItem('api_url', apiUrl)
    setSaved(true)
    setTimeout(() => setSaved(false), 2000)
  }

  const storageUsedMB = ((user?.storage_used_bytes || 0) / 1024 / 1024).toFixed(1)
  const storageQuotaGB = ((user?.storage_quota_bytes || 0) / 1024 / 1024 / 1024).toFixed(0)
  const storagePercent = user ? (user.storage_used_bytes / user.storage_quota_bytes) * 100 : 0

  return (
    <div className="space-y-6">
      {/* Page header */}
      <div className="flex items-center space-x-3">
        <div className="h-10 w-10 rounded-lg bg-forest-800/50 flex items-center justify-center">
          <SettingsIcon className="h-5 w-5 text-forest-400" />
        </div>
        <div>
          <h1 className="text-2xl font-bold text-sand-100">Settings</h1>
          <p className="text-sm text-sand-500">Manage your app and account preferences</p>
        </div>
      </div>

      {/* API Configuration */}
      <div className="card">
        <div className="section-header flex items-center space-x-2.5">
          <Server className="h-4 w-4 text-sand-500" />
          <h2 className="text-base font-semibold text-sand-200">API Configuration</h2>
        </div>
        <div className="card-body space-y-4">
          <div>
            <label className="block text-sm font-medium text-sand-400 mb-2">
              API Server URL
            </label>
            <div className="flex space-x-2">
              <input
                type="url"
                value={apiUrl}
                onChange={(e) => setApiUrl(e.target.value)}
                className="input-field"
                placeholder="http://localhost:8080"
              />
              <button onClick={handleSave} className="btn-primary whitespace-nowrap">
                Save
              </button>
            </div>
            {saved && (
              <span className="text-sm text-forest-400 mt-2 inline-block">Saved successfully</span>
            )}
          </div>
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
                <Mail className="h-5 w-5 text-earth-400" />
              </div>
              <div>
                <label className="text-xs font-medium text-sand-600 uppercase tracking-wider">Email</label>
                <p className="text-sand-200 text-sm">{user.email}</p>
              </div>
            </div>

            <div className="border-t border-forest-800/40 pt-5">
              <div className="flex items-start space-x-4">
                <div className="h-10 w-10 rounded-lg bg-forest-800/50 flex items-center justify-center shrink-0">
                  <HardDrive className="h-5 w-5 text-forest-400" />
                </div>
                <div className="flex-1 min-w-0">
                  <label className="text-xs font-medium text-sand-600 uppercase tracking-wider">Storage</label>
                  <p className="text-sand-200 text-sm">
                    {storageUsedMB} MB of {storageQuotaGB} GB used
                  </p>
                  {/* Storage progress bar */}
                  <div className="mt-2 h-2 bg-forest-900 rounded-full overflow-hidden">
                    <div
                      className="h-full bg-forest-500 rounded-full transition-all duration-500"
                      style={{ width: `${Math.min(storagePercent, 100)}%` }}
                    />
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>
      )}

      {/* Dev mode indicator */}
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

export default SettingsPage