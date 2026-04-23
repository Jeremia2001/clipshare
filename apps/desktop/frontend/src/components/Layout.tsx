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
    <div className="min-h-screen bg-forest-950 text-sand-100 flex flex-col">
      {/* Top accent line — Xbox green stripe */}
      <div className="h-[2px] bg-gradient-to-r from-transparent via-forest-500/70 to-transparent shrink-0" />

      {/* Top Navigation Bar */}
      <header className="h-12 bg-[#080c08] border-b border-forest-800/30 flex items-stretch shrink-0 relative z-10">
        {/* Logo */}
        <Link
          to="/"
          className="flex items-center gap-2.5 px-5 border-r border-forest-800/30 hover:bg-forest-900/40 transition-colors shrink-0"
        >
          <div className="h-7 w-7 bg-forest-600 flex items-center justify-center rounded-sm"
            style={{ boxShadow: '0 0 10px rgba(82, 176, 67, 0.3)' }}>
            <Video className="h-4 w-4 text-white" />
          </div>
          <span
            className="font-black text-white text-sm tracking-[0.18em] uppercase nav-font"
            style={{ textShadow: '0 0 12px rgba(82, 176, 67, 0.4)' }}
          >
            ClipShare
          </span>
        </Link>

        {/* Navigation blades */}
        <nav className="flex items-stretch">
          {navItems.map(({ path, label, icon: Icon }) => {
            const active = isActive(path)
            return (
              <Link
                key={path}
                to={path}
                className="group relative flex items-center h-full px-7 transition-colors"
              >
                {/* Parallelogram blade background */}
                <span
                  className={`absolute inset-y-[3px] inset-x-1 -skew-x-[10deg] rounded-[2px] transition-all duration-200 ${
                    active
                      ? 'bg-forest-700/70'
                      : 'bg-transparent group-hover:bg-forest-900/60'
                  }`}
                  style={active ? {
                    boxShadow: '0 0 14px rgba(82, 176, 67, 0.25), inset 0 1px 0 rgba(82, 176, 67, 0.15)',
                  } : {}}
                />
                {/* Bottom active indicator */}
                {active && (
                  <span
                    className="absolute bottom-0 left-1 right-1 h-[2px] bg-forest-500"
                    style={{ boxShadow: '0 0 8px rgba(82, 176, 67, 0.9)' }}
                  />
                )}
                {/* Content */}
                <span className={`relative flex items-center gap-2 text-[11px] font-bold uppercase tracking-[0.16em] nav-font transition-colors duration-200 ${
                  active
                    ? 'text-forest-400'
                    : 'text-sand-500 group-hover:text-sand-300'
                }`}>
                  <Icon className="h-3.5 w-3.5 shrink-0" />
                  <span>{label}</span>
                </span>
              </Link>
            )
          })}
        </nav>

        {/* User section — right aligned */}
        {user && (
          <div className="ml-auto flex items-center gap-3 px-4 border-l border-forest-800/30">
            {isDevMode && (
              <span className="px-1.5 py-0.5 text-[9px] font-bold bg-earth-900/60 text-earth-400 rounded-sm tracking-[0.15em] uppercase">
                DEV
              </span>
            )}
            <div
              className="h-6 w-6 rounded-sm bg-forest-800 border border-forest-700/60 flex items-center justify-center text-[10px] font-bold text-forest-400 shrink-0"
            >
              {user.username?.[0]?.toUpperCase()}
            </div>
            <span className="text-xs text-sand-500 font-medium tracking-wide">
              {user.display_name || user.username}
            </span>
            {!isDevMode && (
              <button
                onClick={logout}
                className="text-sand-600 hover:text-forest-400 p-1 rounded transition-colors shrink-0"
                title="Sign out"
              >
                <LogOut className="h-3.5 w-3.5" />
              </button>
            )}
          </div>
        )}
      </header>

      {/* Bottom nav accent */}
      <div className="h-px bg-gradient-to-r from-transparent via-forest-600/20 to-transparent shrink-0" />

      {/* Main content */}
      <main className="flex-1 min-w-0 overflow-auto">
        <div className="max-w-[1600px] mx-auto px-4 py-6">
          <Outlet />
        </div>
      </main>
    </div>
  )
}

export default Layout
