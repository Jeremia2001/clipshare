import React, { createContext, useContext, useState, useEffect, useCallback } from 'react'
import { api, setApiUrl } from '../services/api'
import {
  IsDevMode,
  GetAuthStatus,
  GetAuthToken,
  SetupAdmin,
  LoginAdmin,
  RedeemInvite,
  LogoutDevice,
} from '../../wailsjs/go/main/App'

interface User {
  id: string
  username: string
  display_name?: string
  avatar_url?: string
  is_admin: boolean
}

const devUser: User = {
  id: '00000000-0000-0000-0000-000000000001',
  username: 'dev',
  display_name: 'Developer',
  is_admin: true,
}

export interface AuthBootstrap {
  serverURL: string
  accountUsername: string
  hasToken: boolean
  needsSetup: boolean
  reachable: boolean
}

interface AuthContextType {
  user: User | null
  isLoading: boolean
  isAuthenticated: boolean
  isDevMode: boolean
  bootstrap: AuthBootstrap | null
  refreshBootstrap: () => Promise<AuthBootstrap | null>
  setupAdmin: (args: { serverURL: string; setupToken: string; username: string; password: string }) => Promise<void>
  loginAdmin: (args: { serverURL: string; username: string; password: string }) => Promise<void>
  redeemInvite: (args: { serverURL: string; code: string; username: string }) => Promise<void>
  logout: () => Promise<void>
}

const AuthContext = createContext<AuthContextType | undefined>(undefined)

async function loadBootstrap(): Promise<AuthBootstrap | null> {
  try {
    const s = await GetAuthStatus()
    return {
      serverURL: s.server_url,
      accountUsername: s.account_username || '',
      hasToken: s.has_token,
      needsSetup: s.needs_setup,
      reachable: s.reachable,
    }
  } catch {
    return null
  }
}

// Pull the token from the keyring and make it available to axios. Called
// whenever we finish a login/setup/redeem or detect an existing session.
async function attachStoredToken(): Promise<string> {
  try {
    const token = await GetAuthToken()
    if (token) {
      localStorage.setItem('access_token', token)
    } else {
      localStorage.removeItem('access_token')
    }
    return token
  } catch {
    return ''
  }
}

export const AuthProvider: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const [user, setUser] = useState<User | null>(null)
  const [isLoading, setIsLoading] = useState(true)
  const [devMode, setDevMode] = useState(false)
  const [bootstrap, setBootstrap] = useState<AuthBootstrap | null>(null)

  const refreshBootstrap = useCallback(async () => {
    const b = await loadBootstrap()
    setBootstrap(b)
    if (b?.serverURL) {
      setApiUrl(b.serverURL)
    }
    return b
  }, [])

  const fetchUser = useCallback(async () => {
    try {
      const response = await api.get('/auth/me')
      setUser(response.data)
    } catch {
      localStorage.removeItem('access_token')
      setUser(null)
    }
  }, [])

  useEffect(() => {
    const init = async () => {
      try {
        const isDev = await IsDevMode()
        setDevMode(isDev)
        if (isDev) {
          setUser(devUser)
          return
        }
        const b = await refreshBootstrap()
        if (b?.hasToken) {
          await attachStoredToken()
          await fetchUser()
        }
      } finally {
        setIsLoading(false)
      }
    }
    init()
  }, [fetchUser, refreshBootstrap])

  const setupAdmin: AuthContextType['setupAdmin'] = async ({ serverURL, setupToken, username, password }) => {
    setIsLoading(true)
    try {
      await SetupAdmin(serverURL, setupToken, username, password)
      await refreshBootstrap()
      await attachStoredToken()
      await fetchUser()
    } finally {
      setIsLoading(false)
    }
  }

  const loginAdmin: AuthContextType['loginAdmin'] = async ({ serverURL, username, password }) => {
    setIsLoading(true)
    try {
      await LoginAdmin(serverURL, username, password)
      await refreshBootstrap()
      await attachStoredToken()
      await fetchUser()
    } finally {
      setIsLoading(false)
    }
  }

  const redeemInvite: AuthContextType['redeemInvite'] = async ({ serverURL, code, username }) => {
    setIsLoading(true)
    try {
      await RedeemInvite(serverURL, code, username)
      await refreshBootstrap()
      await attachStoredToken()
      await fetchUser()
    } finally {
      setIsLoading(false)
    }
  }

  const logout = async () => {
    if (devMode) {
      setUser(devUser)
      return
    }
    try {
      await LogoutDevice()
    } finally {
      localStorage.removeItem('access_token')
      setUser(null)
      await refreshBootstrap()
    }
  }

  return (
    <AuthContext.Provider value={{
      user,
      isLoading,
      isAuthenticated: !!user,
      isDevMode: devMode,
      bootstrap,
      refreshBootstrap,
      setupAdmin,
      loginAdmin,
      redeemInvite,
      logout,
    }}>
      {children}
    </AuthContext.Provider>
  )
}

export const useAuth = () => {
  const context = useContext(AuthContext)
  if (context === undefined) {
    throw new Error('useAuth must be used within an AuthProvider')
  }
  return context
}
