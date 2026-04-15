import React, { createContext, useContext, useState, useEffect, useCallback } from 'react'
import { api } from '../services/api'
import { IsDevMode } from '../../wailsjs/go/main/App'

interface User {
  id: string
  email: string
  username?: string
  display_name?: string
  avatar_url?: string
  is_verified: boolean
  storage_used_bytes: number
  storage_quota_bytes: number
}

const devUser: User = {
  id: 'dev-user-id',
  email: 'dev@localhost',
  username: 'devuser',
  display_name: 'Developer',
  is_verified: true,
  storage_used_bytes: 0,
  storage_quota_bytes: 10737418240,
}

interface AuthContextType {
  user: User | null
  isLoading: boolean
  isAuthenticated: boolean
  isDevMode: boolean
  login: (email: string) => Promise<void>
  verifyToken: (token: string) => Promise<void>
  logout: () => Promise<void>
}

const AuthContext = createContext<AuthContextType | undefined>(undefined)

export const AuthProvider: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const [user, setUser] = useState<User | null>(null)
  const [isLoading, setIsLoading] = useState(true)
  const [devMode, setDevMode] = useState(false)

  const checkDevMode = useCallback(async () => {
    try {
      const result = await IsDevMode()
      setDevMode(result)
      return result
    } catch {
      const fallback = false
      setDevMode(fallback)
      return fallback
    }
  }, [])

  const fetchUser = useCallback(async () => {
    try {
      const response = await api.get('/auth/me')
      setUser(response.data)
    } catch {
      localStorage.removeItem('access_token')
    } finally {
      setIsLoading(false)
    }
  }, [])

  useEffect(() => {
    const init = async () => {
      const isDev = await checkDevMode()

      if (isDev) {
        setUser(devUser)
        setIsLoading(false)
        return
      }

      const token = localStorage.getItem('access_token')
      if (token) {
        await fetchUser()
      } else {
        setIsLoading(false)
      }
    }

    init()
  }, [checkDevMode, fetchUser])

  const login = async (email: string) => {
    if (devMode) {
      setUser(devUser)
      return
    }
    setIsLoading(true)
    try {
      await api.post('/auth/magic-link', { email })
    } finally {
      setIsLoading(false)
    }
  }

  const verifyToken = async (token: string) => {
    setIsLoading(true)
    try {
      const response = await api.post('/auth/verify', { token })
      localStorage.setItem('access_token', response.data.access_token)
      if (response.data.refresh_token) {
        localStorage.setItem('refresh_token', response.data.refresh_token)
      }
      setUser(response.data.user)
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
      await api.delete('/auth/logout')
    } finally {
      localStorage.removeItem('access_token')
      localStorage.removeItem('refresh_token')
      setUser(null)
    }
  }

  return (
    <AuthContext.Provider value={{
      user,
      isLoading,
      isAuthenticated: !!user,
      isDevMode: devMode,
      login,
      verifyToken,
      logout
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