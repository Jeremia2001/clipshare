import { Routes, Route, Navigate } from 'react-router-dom'
import { AuthProvider, useAuth } from './hooks/useAuth'
import Layout from './components/Layout'
import LoginPage from './pages/LoginPage'
import LibraryPage from './pages/LibraryPage'
import EditorPage from './pages/EditorPage'
import ClipDetailPage from './pages/ClipDetailPage'
import SettingsPage from './pages/SettingsPage'
import './styles/App.css'

function ProtectedRoute({ children }: { children: React.ReactNode }) {
  const { isAuthenticated, isLoading } = useAuth()
  
  if (isLoading) {
    return <div className="min-h-screen bg-forest-950 flex items-center justify-center text-sand-500">Loading...</div>
  }

  if (!isAuthenticated) {
    return <Navigate to="/login" replace />
  }

  return children
}

function AuthRoute({ children }: { children: React.ReactNode }) {
  const { isAuthenticated, isLoading } = useAuth()

  if (isLoading) {
    return <div className="min-h-screen bg-forest-950 flex items-center justify-center text-sand-500">Loading...</div>
  }
  
  if (isAuthenticated) {
    return <Navigate to="/" replace />
  }
  
  return children
}

function App() {
  return (
    <AuthProvider>
      <Routes>
        <Route path="/" element={
          <ProtectedRoute>
            <Layout />
          </ProtectedRoute>
        }>
          <Route index element={<LibraryPage />} />
          <Route path="editor" element={<EditorPage />} />
          <Route path="clips/:id" element={<ClipDetailPage />} />
          <Route path="settings" element={<SettingsPage />} />
        </Route>
        <Route path="/login" element={
          <AuthRoute>
            <LoginPage />
          </AuthRoute>
        } />
      </Routes>
    </AuthProvider>
  )
}

export default App