import { Link, Outlet, useLocation } from 'react-router-dom'
import { useAuth } from '../hooks/useAuth'
import { Video, Settings, LogOut, Library, Scissors } from 'lucide-react'

function Layout() {
  const { user, logout, isDevMode } = useAuth()
  const location = useLocation()

  const navItems = [
    { path: '/', label: 'Library', icon: Library },
    { path: '/editor', label: 'Editor', icon: Scissors },
    { path: '/settings', label: 'Settings', icon: Settings },
  ]

  const isActive = (path: string) => {
    if (path === '/') return location.pathname === '/'
    return location.pathname.startsWith(path)
  }

  return (
    <div className="min-h-screen bg-forest-950 text-sand-100 flex">
      {/* Sidebar */}
      <aside className="w-60 bg-forest-950/90 border-r border-forest-800/50 flex flex-col shrink-0">
        {/* Logo */}
        <div className="px-5 py-5 border-b border-forest-800/50">
          <Link to="/" className="flex items-center space-x-2.5 group">
            <div className="h-9 w-9 rounded-lg bg-forest-600 flex items-center justify-center group-hover:bg-forest-500 transition-colors">
              <Video className="h-5 w-5 text-forest-50" />
            </div>
            <span className="text-lg font-bold text-sand-100 tracking-tight">ClipShare</span>
          </Link>
        </div>

        {/* Navigation */}
        <nav className="flex-1 px-3 py-4 space-y-1">
          {navItems.map(({ path, label, icon: Icon }) => (
            <Link
              key={path}
              to={path}
              className={`flex items-center space-x-3 px-3 py-2.5 rounded-lg text-sm font-medium transition-all duration-200 ${
                isActive(path)
                  ? 'bg-forest-800/70 text-forest-200'
                  : 'text-sand-500 hover:text-sand-300 hover:bg-forest-900/50'
              }`}
            >
              <Icon className="h-[18px] w-[18px]" />
              <span>{label}</span>
            </Link>
          ))}
        </nav>

        {/* User section */}
        {user && (
          <div className="px-3 pb-4 border-t border-forest-800/50 pt-4">
            <div className="flex items-center space-x-3 px-3 py-2">
              <div className="h-8 w-8 rounded-full bg-earth-700 flex items-center justify-center text-xs font-semibold text-earth-200 shrink-0">
                {user.username?.[0]?.toUpperCase()}
              </div>
              <div className="min-w-0 flex-1">
                <p className="text-sm text-sand-200 truncate">{user.display_name || user.username}</p>
                <div className="flex items-center space-x-1.5">
                  {isDevMode && (
                    <span className="px-1.5 py-0.5 text-[10px] font-semibold bg-earth-600/40 text-earth-300 rounded">
                      DEV
                    </span>
                  )}
                  <span className="text-xs text-sand-600 truncate">@{user.username}</span>
                </div>
              </div>
              {!isDevMode && (
                <button
                  onClick={logout}
                  className="text-sand-600 hover:text-sand-400 p-1 rounded transition-colors shrink-0"
                  title="Sign out"
                >
                  <LogOut className="h-4 w-4" />
                </button>
              )}
            </div>
          </div>
        )}
      </aside>

      {/* Main content */}
      <main className="flex-1 min-w-0 overflow-auto">
        <div className="max-w-6xl mx-auto px-8 py-8">
          <Outlet />
        </div>
      </main>
    </div>
  )
}

export default Layout