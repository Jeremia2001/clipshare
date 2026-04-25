import { useState, useEffect } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import {
  ArrowLeft, Eye, Trash2, Share2, Globe, Lock, Clock,
  MessageSquare, HardDrive, Loader2, Copy, Check, Film, User, RefreshCw
} from 'lucide-react'
import { Clip, Comment, clipApi, commentApi, shareApi, ShareResponse, normalizeApiUrl } from '../services/api'
import { ProxyVideoURL } from '../../wailsjs/go/main/App'

function ClipDetailPage() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()

  const [clip, setClip] = useState<Clip | null>(null)
  const [viewUrl, setViewUrl] = useState<string | null>(null)
  const [videoError, setVideoError] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const [editing, setEditing] = useState(false)
  const [title, setTitle] = useState('')
  const [description, setDescription] = useState('')
  const [isPublic, setIsPublic] = useState(true)
  const [allowComments, setAllowComments] = useState(true)
  const [saving, setSaving] = useState(false)
  const [deleting, setDeleting] = useState(false)
  const [confirmingDelete, setConfirmingDelete] = useState(false)

  const [shares, setShares] = useState<ShareResponse[]>([])
  const [copiedCode, setCopiedCode] = useState<string | null>(null)

  const [comments, setComments] = useState<Comment[]>([])
  const [commentsLoading, setCommentsLoading] = useState(true)

  const loadComments = async (clipId: string) => {
    setCommentsLoading(true)
    try {
      const { data } = await commentApi.listByClip(clipId)
      setComments(data.comments || [])
    } catch {
      setComments([])
    } finally {
      setCommentsLoading(false)
    }
  }

  useEffect(() => {
    if (!id) return
    setLoading(true)
    clipApi.get(id).then(async ({ data }) => {
      setClip(data.clip)
      // Use the API download endpoint for desktop playback — presigned storage
      // URLs (e.g. http://minio:9000/...) are internal and unreachable from the
      // host process. The download endpoint is always accessible via the API URL.
      const apiUrl = normalizeApiUrl(localStorage.getItem('api_url'))
      const downloadUrl = `${apiUrl}/api/v1/clips/${id}/download`
      const token = localStorage.getItem('access_token') || ''
      try {
        const proxyUrl = await ProxyVideoURL(downloadUrl, token)
        setViewUrl(proxyUrl)
      } catch {
        setViewUrl(null)
      }
      setTitle(data.clip.title)
      setDescription(data.clip.description || '')
      setIsPublic(data.clip.is_public)
      setAllowComments(data.clip.allow_comments)
    }).catch(() => {
      setError('Clip not found')
    }).finally(() => setLoading(false))

    shareApi.list(id).then(({ data }) => {
      const mapped = (data.shares || []).map((s: any) => ({
        share_code: s.share_code,
        share_url: s.share_url,
        share: s,
      }))
      setShares(mapped)
    }).catch(() => {})

    loadComments(id)
  }, [id])

  const handleSave = async () => {
    if (!id) return
    setSaving(true)
    try {
      const { data: updated } = await clipApi.update(id, {
        title,
        description: description || undefined,
        is_public: isPublic,
        allow_comments: allowComments,
      })
      setClip(updated)
      setEditing(false)
    } catch {
      setError('Failed to save')
    }
    setSaving(false)
  }

  const handleDelete = async () => {
    if (!id) return
    setDeleting(true)
    try {
      await clipApi.delete(id)
      navigate('/')
    } catch {
      setConfirmingDelete(false)
      setError('Failed to delete clip. Please try again.')
    }
    setDeleting(false)
  }

  const handleCreateShare = async () => {
    if (!id) return
    try {
      const { data: resp } = await shareApi.create(id)
      setShares(prev => [resp, ...prev])
    } catch {}
  }

  const handleCopyCode = (code: string) => {
    navigator.clipboard.writeText(code)
    setCopiedCode(code)
    setTimeout(() => setCopiedCode(null), 2000)
  }

  const formatDuration = (seconds: number) => {
    const m = Math.floor(seconds / 60)
    const s = seconds % 60
    return `${m}:${s.toString().padStart(2, '0')}`
  }

  if (loading) {
    return (
      <div className="flex items-center justify-center py-20">
        <Loader2 className="h-8 w-8 text-forest-500 animate-spin" />
      </div>
    )
  }

  if (error || !clip) {
    return (
      <div className="text-center py-20">
        <p className="text-sand-500">{error || 'Clip not found'}</p>
        <button onClick={() => navigate('/')} className="btn-ghost mt-4 inline-flex items-center space-x-2">
          <ArrowLeft className="h-4 w-4" />
          <span>Back to Library</span>
        </button>
      </div>
    )
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center space-x-3">
        <button onClick={() => navigate('/')} className="p-2 text-sand-500 hover:text-sand-300 rounded-lg transition-colors">
          <ArrowLeft className="h-5 w-5" />
        </button>
        <div className="flex-1 min-w-0">
          <h1 className="text-2xl text-sand-100 xbox-title truncate">{clip.title}</h1>
          <div className="flex items-center space-x-3 mt-1 text-sm text-sand-500">
            <span className="flex items-center space-x-1"><Eye className="h-3.5 w-3.5" /><span>{clip.view_count} views</span></span>
            <span className="flex items-center space-x-1"><Clock className="h-3.5 w-3.5" /><span>{new Date(clip.created_at).toLocaleDateString()}</span></span>
            <span className="flex items-center space-x-1"><HardDrive className="h-3.5 w-3.5" /><span>{(clip.file_size_bytes / 1024 / 1024).toFixed(1)} MB</span></span>
            {clip.is_public ? <Globe className="h-3.5 w-3.5 text-forest-400" /> : <Lock className="h-3.5 w-3.5 text-earth-400" />}
          </div>
        </div>
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        <div className="lg:col-span-2 space-y-4">
          <div className="card">
            <div className="card-body p-0 overflow-hidden">
              {viewUrl ? (
                <video
                  src={viewUrl}
                  controls
                  preload="auto"
                  playsInline
                  className="w-full max-h-[500px] bg-black"
                  onError={() => setVideoError('Failed to load video')}
                />
              ) : (
                <div className="bg-forest-900 aspect-video flex items-center justify-center">
                  <p className="text-sand-600 text-sm">Video preview unavailable</p>
                </div>
              )}
              {videoError && viewUrl && (
                <div className="xbox-error rounded-none">
                  {videoError}
                </div>
              )}
            </div>
          </div>

          <div className="card">
            <div className="section-header flex items-center justify-between">
              <div className="flex items-center space-x-2.5">
                <Film className="h-4 w-4 text-sand-500" />
                <h2 className="text-base font-semibold text-sand-200">Details</h2>
              </div>
              <button onClick={() => setEditing(!editing)} className="btn-ghost text-sm">
                {editing ? 'Cancel' : 'Edit'}
              </button>
            </div>
            <div className="card-body space-y-4">
              {editing ? (
                <>
                  <div>
                    <label className="block text-sm font-medium text-sand-400 mb-2">Title</label>
                    <input type="text" value={title} onChange={e => setTitle(e.target.value)} className="input-field" />
                  </div>
                  <div>
                    <label className="block text-sm font-medium text-sand-400 mb-2">Description</label>
                    <textarea value={description} onChange={e => setDescription(e.target.value)} className="input-field min-h-[80px] resize-y" />
                  </div>
                  <div className="flex items-center space-x-6">
                    <label className="flex items-center space-x-2 cursor-pointer">
                      <input type="checkbox" checked={isPublic} onChange={e => setIsPublic(e.target.checked)} className="h-4 w-4 rounded border-forest-700 bg-forest-950 text-forest-500 focus:ring-forest-500" />
                      <Globe className="h-4 w-4 text-sand-500" />
                      <span className="text-sm text-sand-300">Public</span>
                    </label>
                    <label className="flex items-center space-x-2 cursor-pointer">
                      <input type="checkbox" checked={allowComments} onChange={e => setAllowComments(e.target.checked)} className="h-4 w-4 rounded border-forest-700 bg-forest-950 text-forest-500 focus:ring-forest-500" />
                      <MessageSquare className="h-4 w-4 text-sand-500" />
                      <span className="text-sm text-sand-300">Allow Comments</span>
                    </label>
                  </div>
                  <button onClick={handleSave} disabled={saving} className="btn-primary inline-flex items-center space-x-2">
                    {saving && <Loader2 className="h-4 w-4 animate-spin" />}
                    <span>{saving ? 'Saving...' : 'Save'}</span>
                  </button>
                </>
              ) : (
                <>
                  {clip.description && <p className="text-sand-400 text-sm">{clip.description}</p>}
                  <div className="grid grid-cols-2 gap-3 text-sm text-sand-500">
                    {clip.duration_seconds > 0 && <div>Duration: {formatDuration(clip.duration_seconds)}</div>}
                    {(clip.width > 0 && clip.height > 0) && <div>Resolution: {clip.width}x{clip.height}</div>}
                    <div>Size: {(clip.file_size_bytes / 1024 / 1024).toFixed(1)} MB</div>
                  </div>
                </>
              )}
            </div>
          </div>

          <div className="card">
            <div className="section-header flex items-center justify-between">
              <div className="flex items-center space-x-2.5">
                <MessageSquare className="h-4 w-4 text-sand-500" />
                <h2 className="text-base font-semibold text-sand-200">Comments</h2>
                {!commentsLoading && (
                  <span className="text-xs text-sand-600">({comments.length})</span>
                )}
              </div>
              <button
                onClick={() => id && loadComments(id)}
                disabled={commentsLoading}
                className="btn-ghost text-sm inline-flex items-center space-x-1.5"
                title="Refresh"
              >
                <RefreshCw className={`h-3.5 w-3.5 ${commentsLoading ? 'animate-spin' : ''}`} />
                <span>Refresh</span>
              </button>
            </div>
            <div className="card-body">
              {!clip.allow_comments ? (
                <p className="text-sm text-sand-600 italic">
                  Comments are disabled for this clip.
                </p>
              ) : commentsLoading && comments.length === 0 ? (
                <div className="flex items-center justify-center py-6">
                  <Loader2 className="h-5 w-5 text-forest-500 animate-spin" />
                </div>
              ) : comments.length === 0 ? (
                <p className="text-sm text-sand-600 italic">
                  No comments yet. Share this clip to start the conversation.
                </p>
              ) : (
                <ul className="space-y-3">
                  {comments.map(comment => (
                    <li key={comment.id} className="flex items-start space-x-3">
                      <div className="mt-0.5 h-8 w-8 rounded-sm bg-forest-900/70 border border-forest-700/40 flex items-center justify-center shrink-0">
                        <User className="h-4 w-4 text-sand-500" />
                      </div>
                      <div className="flex-1 min-w-0">
                        <div className="flex items-baseline space-x-2">
                          <span className="text-sm font-semibold text-sand-200 truncate">
                            {comment.display_name || 'Guest'}
                          </span>
                          <span className="text-xs text-sand-600">
                            {new Date(comment.created_at).toLocaleString()}
                          </span>
                        </div>
                        <p className="text-sm text-sand-300 whitespace-pre-wrap break-words mt-0.5">
                          {comment.content}
                        </p>
                      </div>
                    </li>
                  ))}
                </ul>
              )}
            </div>
          </div>

          <div className="flex items-center space-x-3">
            {confirmingDelete ? (
              <>
                <span className="text-sm text-sand-500">Delete this clip permanently?</span>
                <button
                  onClick={handleDelete}
                  disabled={deleting}
                  className="btn-danger inline-flex items-center space-x-2"
                >
                  {deleting ? <Loader2 className="h-4 w-4 animate-spin" /> : <Trash2 className="h-4 w-4" />}
                  <span>{deleting ? 'Deleting...' : 'Confirm Delete'}</span>
                </button>
                <button onClick={() => setConfirmingDelete(false)} className="btn-cancel">
                  Cancel
                </button>
              </>
            ) : (
              <button
                onClick={() => setConfirmingDelete(true)}
                className="btn-ghost inline-flex items-center space-x-2 text-earth-500 hover:text-earth-400"
              >
                <Trash2 className="h-4 w-4" />
                <span>Delete</span>
              </button>
            )}
          </div>
        </div>

        <div className="space-y-4">
          <div className="card">
            <div className="section-header flex items-center space-x-2.5">
              <Share2 className="h-4 w-4 text-sand-500" />
              <h2 className="text-base font-semibold text-sand-200">Shares</h2>
            </div>
            <div className="card-body space-y-3">
              <button onClick={handleCreateShare} className="btn-primary w-full inline-flex items-center justify-center space-x-2">
                <Share2 className="h-4 w-4" />
                <span>Create Share Link</span>
              </button>

              {shares.length === 0 && (
                <p className="text-sm text-sand-600 text-center">No share links yet</p>
              )}

              {shares.map(s => (
                <div key={s.share_code} className="flex items-center space-x-2 p-2 bg-forest-900/40 border border-forest-800/40 border-l-forest-600/40" style={{ borderLeftWidth: '2px' }}>
                  <code className="text-xs text-forest-300 flex-1 truncate">{s.share_url}</code>
                  {s.share.has_password && <Lock className="h-3 w-3 text-earth-500 shrink-0" />}
                  <button onClick={() => handleCopyCode(s.share_url)} className="p-1 text-sand-500 hover:text-sand-300 shrink-0 transition-colors" title="Copy link">
                    {copiedCode === s.share_url ? <Check className="h-3 w-3 text-moss-400" /> : <Copy className="h-3 w-3" />}
                  </button>
                </div>
              ))}
            </div>
          </div>
        </div>
      </div>
    </div>
  )
}

export default ClipDetailPage