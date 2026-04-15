import { useAuth } from '../hooks/useAuth'
import { useState } from 'react'
import { Loader2, Mail, Video } from 'lucide-react'

function LoginPage() {
  const { login } = useAuth()
  const [email, setEmail] = useState('')
  const [isLoading, setIsLoading] = useState(false)
  const [message, setMessage] = useState('')

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setIsLoading(true)
    setMessage('')

    try {
      await login(email)
      setMessage('Check your email for a magic link!')
    } catch (error) {
      setMessage('Failed to send magic link. Please try again.')
    } finally {
      setIsLoading(false)
    }
  }

  return (
    <div className="min-h-screen bg-forest-950 flex items-center justify-center">
      <div className="w-full max-w-sm px-6">
        {/* Logo */}
        <div className="text-center mb-10">
          <div className="mx-auto h-14 w-14 rounded-2xl bg-forest-600 flex items-center justify-center mb-4">
            <Video className="h-7 w-7 text-forest-50" />
          </div>
          <h1 className="text-2xl font-bold text-sand-100 tracking-tight">Welcome to ClipShare</h1>
          <p className="mt-2 text-sand-500 text-sm">Sign in with your email to continue</p>
        </div>

        {/* Login form */}
        <div className="card">
          <form className="p-6 space-y-5" onSubmit={handleSubmit}>
            <div>
              <label htmlFor="email" className="block text-sm font-medium text-sand-400 mb-2">
                Email address
              </label>
              <div className="relative">
                <div className="absolute inset-y-0 left-0 pl-3.5 flex items-center pointer-events-none">
                  <Mail className="h-4 w-4 text-sand-600" />
                </div>
                <input
                  id="email"
                  name="email"
                  type="email"
                  required
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                  className="input-field pl-10"
                  placeholder="you@example.com"
                />
              </div>
            </div>

            {message && (
              <div className={`text-sm text-center px-3 py-2 rounded-lg ${
                message.includes('Check')
                  ? 'bg-forest-900/50 text-forest-300'
                  : 'bg-red-900/30 text-red-300'
              }`}>
                {message}
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
                <span>Send Magic Link</span>
              )}
            </button>
          </form>
        </div>

        <p className="mt-6 text-center text-xs text-sand-600">
          We will send you a login link — no password needed.
        </p>
      </div>
    </div>
  )
}

export default LoginPage