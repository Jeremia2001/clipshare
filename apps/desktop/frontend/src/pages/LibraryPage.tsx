import { useState, useEffect, useCallback } from 'react'
import { useNavigate } from 'react-router-dom'
import { useAuth } from '../hooks/useAuth'
import {
  Video, Plus, Search, Grid3X3, List, Eye, Trash2, Share2,
  Globe, Lock, Clock, HardDrive, Loader2
} from 'lucide-react'
import { Clip, clipApi, shareApi, ShareResponse } from '../services/api'

type ViewMode = 'grid' | 'list'

function LibraryPage() {
  const { user } = useAuth()
  const navigate = useNavigate()

  const [clips, setClips] = useState<Clip[]>([])
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [loading, setLoading] = useState(true)
  const [viewMode, setViewMode] = useState<ViewMode>('grid')
  const [search, setSearch] = useState('')
  const [shareClipId, setShareClipId] = useState<string | null>(null)
  const [shares, setShares] = useState<ShareResponse[]>([])
  const [deleting, setDeleting] = useState<string | null>(null)

  const fetchClips = useCallback(async () => {
    setLoading(true)
    try {
      const { data } = await clipApi.list(page, 20)
      setClips(data.clips || [])
      setTotal(data.total)
    } catch {
      setClips([])
    }
    setLoading(false)
  }, [page])

  useEffect(() => {
    fetchClips()
  }, [fetchClips])

  const handleDelete = async (clipId: string) => {
    setDeleting(clipId)
    try {
      await clipApi.delete(clipId)
      setClips(prev => prev.filter(c => c.id !== clipId))
      setTotal(prev => prev - 1)
    } catch {}
    setDeleting(null)
  }

  const handleShare = async (clipId: string) => {
    setShareClipId(clipId)
    try {
      const { data } = await shareApi.list(clipId)
      const mapped = (data.shares || []).map((s: any) => ({
        share_code: s.share_code,
        share_url: `/s/${s.share_code}`,
        share: s,
      }))
      setShares(mapped)
    } catch {
      setShares([])
    }
  }

  const handleCreateShare = async () => {
    if (!shareClipId) return
    try {
      const { data: resp } = await shareApi.create(shareClipId)
      setShares(prev => [resp, ...prev])
    } catch {}
  }

  const handleCloseShares = () => {
    setShareClipId(null)
    setShares([])
  }

  const formatDuration = (seconds: number) => {
    const m = Math.floor(seconds / 60)
    const s = seconds % 60
    return `${m}:${s.toString().padStart(2, '0')}`
  }

  const formatDate = (dateStr: string) => {
    const d = new Date(dateStr)
    return d.toLocaleDateString('en-US', { month: 'short', day: 'numeric', year: 'numeric' })
  }

  const filteredClips = search
    ? clips.filter(c => c.title.toLowerCase().includes(search.toLowerCase()))
    : clips

  if (!user) return null

  const storageUsedMB = ((user.storage_used_bytes || 0) / 1024 / 1024).toFixed(0)
  const storageQuotaGB = ((user.storage_quota_bytes || 0) / 1024 / 1024 / 1024).toFixed(0)
  const totalPages = Math.ceil(total / 20)

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-sand-100">My Clips</h1>
          <p className="text-sm text-sand-500 mt-1">
            {storageUsedMB} MB used of {storageQuotaGB} GB &middot; {total} clip{total !== 1 ? 's' : ''}
          </p>
        </div>
        <button onClick={() => navigate('/editor')} className="btn-primary inline-flex items-center space-x-2">
          <Plus className="h-4 w-4" />
          <span>New Clip</span>
        </button>
      </div>

      <div className="flex items-center space-x-3">
        <div className="relative flex-1 max-w-md">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-sand-600" />
          <input
            type="text"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder="Search clips..."
            className="input-field pl-10"
          />
        </div>
        <div className="flex items-center bg-forest-900/50 border border-forest-800/60 rounded-lg p-0.5">
          <button
            onClick={() => setViewMode('grid')}
            className={`p-2 rounded-md transition-colors ${viewMode === 'grid' ? 'bg-forest-800/70 text-forest-300' : 'text-sand-600 hover:text-sand-400'}`}
          >
            <Grid3X3 className="h-4 w-4" />
          </button>
          <button
            onClick={() => setViewMode('list')}
            className={`p-2 rounded-md transition-colors ${viewMode === 'list' ? 'bg-forest-800/70 text-forest-300' : 'text-sand-600 hover:text-sand-400'}`}
          >
            <List className="h-4 w-4" />
          </button>
        </div>
      </div>

      {loading && (
        <div className="card">
          <div className="card-body text-center py-16">
            <Loader2 className="h-8 w-8 text-forest-500 animate-spin mx-auto" />
            <p className="text-sand-500 mt-3">Loading clips...</p>
          </div>
        </div>
      )}

      {!loading && filteredClips.length === 0 && (
        <div className="card">
          <div className="card-body text-center py-16">
            <div className="mx-auto h-16 w-16 rounded-2xl bg-forest-800/50 flex items-center justify-center mb-4">
              <Video className="h-8 w-8 text-forest-500" />
            </div>
            <h3 className="text-lg font-semibold text-sand-200">
              {search ? 'No clips match your search' : 'No clips yet'}
            </h3>
            <p className="mt-2 text-sand-500 max-w-sm mx-auto">
              {search
                ? 'Try a different search term'
                : 'Upload your first gaming clip and it will appear here. Use the editor to trim and share your best moments.'}
            </p>
            {!search && (
              <button onClick={() => navigate('/editor')} className="btn-primary mt-6 inline-flex items-center space-x-2">
                <Plus className="h-4 w-4" />
                <span>Create Your First Clip</span>
              </button>
            )}
          </div>
        </div>
      )}

      {!loading && filteredClips.length > 0 && viewMode === 'grid' && (
        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-4">
          {filteredClips.map(clip => (
            <div key={clip.id} className="card group hover:border-forest-700/80 transition-colors cursor-pointer" onClick={() => navigate(`/clips/${clip.id}`)}>
              <div className="relative aspect-video bg-forest-900 rounded-t-xl overflow-hidden">
                <div className="absolute inset-0 flex items-center justify-center">
                  <Video className="h-8 w-8 text-forest-700" />
                </div>
                <div className="absolute bottom-2 right-2 bg-forest-950/80 text-sand-300 text-xs px-1.5 py-0.5 rounded">
                  {formatDuration(clip.duration_seconds)}
                </div>
                {!clip.is_public && (
                  <div className="absolute top-2 left-2">
                    <Lock className="h-3.5 w-3.5 text-earth-400" />
                  </div>
                )}
              </div>
              <div className="p-3">
                <h4 className="text-sm font-medium text-sand-200 truncate">{clip.title}</h4>
                <div className="flex items-center space-x-3 mt-1.5 text-xs text-sand-600">
                  <span className="flex items-center space-x-1">
                    <Eye className="h-3 w-3" />
                    <span>{clip.view_count}</span>
                  </span>
                  <span className="flex items-center space-x-1">
                    <Clock className="h-3 w-3" />
                    <span>{formatDate(clip.created_at)}</span>
                  </span>
                  <span className="flex items-center space-x-1">
                    <HardDrive className="h-3 w-3" />
                    <span>{(clip.file_size_bytes / 1024 / 1024).toFixed(1)} MB</span>
                  </span>
                </div>
                <div className="flex items-center space-x-1.5 mt-3 opacity-0 group-hover:opacity-100 transition-opacity">
                  <button
                    onClick={(e) => { e.stopPropagation(); navigate(`/clips/${clip.id}`) }}
                    className="p-1.5 text-sand-500 hover:text-sand-300 rounded transition-colors"
                    title="View"
                  >
                    <Eye className="h-3.5 w-3.5" />
                  </button>
                  <button
                    onClick={(e) => { e.stopPropagation(); handleShare(clip.id) }}
                    className="p-1.5 text-sand-500 hover:text-sand-300 rounded transition-colors"
                    title="Share"
                  >
                    <Globe className="h-3.5 w-3.5" />
                  </button>
                  <button
                    onClick={(e) => { e.stopPropagation(); handleDelete(clip.id) }}
                    disabled={deleting === clip.id}
                    className="p-1.5 text-earth-700 hover:text-earth-500 rounded transition-colors ml-auto"
                    title="Delete"
                  >
                    {deleting === clip.id ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <Trash2 className="h-3.5 w-3.5" />}
                  </button>
                </div>
              </div>
            </div>
          ))}
        </div>
      )}

      {!loading && filteredClips.length > 0 && viewMode === 'list' && (
        <div className="card overflow-hidden">
          <div className="divide-y divide-forest-800/40">
            {filteredClips.map(clip => (
              <div key={clip.id} className="flex items-center space-x-4 px-5 py-3.5 hover:bg-forest-900/40 transition-colors group cursor-pointer" onClick={() => navigate(`/clips/${clip.id}`)}>
                <div className="w-24 h-14 bg-forest-900 rounded-lg flex items-center justify-center shrink-0 overflow-hidden relative">
                  <Video className="h-5 w-5 text-forest-700" />
                  <div className="absolute bottom-1 right-1 bg-forest-950/80 text-sand-300 text-[10px] px-1 py-0.5 rounded">
                    {formatDuration(clip.duration_seconds)}
                  </div>
                </div>
                <div className="flex-1 min-w-0">
                  <h4 className="text-sm font-medium text-sand-200 truncate">{clip.title}</h4>
                  <div className="flex items-center space-x-3 mt-0.5 text-xs text-sand-600">
                    <span className="flex items-center space-x-1">
                      <Eye className="h-3 w-3" />
                      <span>{clip.view_count}</span>
                    </span>
                    <span>{formatDate(clip.created_at)}</span>
                    <span>{(clip.file_size_bytes / 1024 / 1024).toFixed(1)} MB</span>
                    {!clip.is_public && <Lock className="h-3 w-3 text-earth-400" />}
                  </div>
                </div>
                <div className="flex items-center space-x-1 opacity-0 group-hover:opacity-100 transition-opacity">
                  <button
                    onClick={(e) => { e.stopPropagation(); handleShare(clip.id) }}
                    className="p-2 text-sand-500 hover:text-sand-300 rounded-lg transition-colors"
                    title="Share"
                  >
                    <Share2 className="h-4 w-4" />
                  </button>
                  <button
                    onClick={(e) => { e.stopPropagation(); handleDelete(clip.id) }}
                    disabled={deleting === clip.id}
                    className="p-2 text-earth-700 hover:text-earth-500 rounded-lg transition-colors"
                    title="Delete"
                  >
                    {deleting === clip.id ? <Loader2 className="h-4 w-4 animate-spin" /> : <Trash2 className="h-4 w-4" />}
                  </button>
                </div>
              </div>
            ))}
          </div>
        </div>
      )}

      {!loading && totalPages > 1 && (
        <div className="flex items-center justify-center space-x-2">
          <button
            onClick={() => setPage(p => Math.max(1, p - 1))}
            disabled={page <= 1}
            className="btn-ghost text-sm disabled:opacity-30"
          >
            Previous
          </button>
          <span className="text-sm text-sand-500">
            Page {page} of {totalPages}
          </span>
          <button
            onClick={() => setPage(p => Math.min(totalPages, p + 1))}
            disabled={page >= totalPages}
            className="btn-ghost text-sm disabled:opacity-30"
          >
            Next
          </button>
        </div>
      )}

      {shareClipId && (
        <div className="fixed inset-0 bg-forest-950/70 flex items-center justify-center z-50" onClick={handleCloseShares}>
          <div className="bg-forest-950 border border-forest-800/60 rounded-xl w-full max-w-md mx-4 shadow-xl" onClick={e => e.stopPropagation()}>
            <div className="flex items-center justify-between px-5 py-4 border-b border-forest-800/60">
              <h3 className="text-base font-semibold text-sand-200">Share Links</h3>
              <button onClick={handleCloseShares} className="p-1 text-sand-500 hover:text-sand-300">&times;</button>
            </div>
            <div className="p-5 space-y-4">
              <button onClick={handleCreateShare} className="btn-primary w-full inline-flex items-center justify-center space-x-2">
                <Share2 className="h-4 w-4" />
                <span>Create New Share Link</span>
              </button>

              {shares.length === 0 && (
                <p className="text-sm text-sand-600 text-center">No share links yet</p>
              )}

              {shares.map(s => (
                <div key={s.share_code} className="flex items-center space-x-2 p-2.5 bg-forest-900/50 rounded-lg">
                  <code className="text-sm text-forest-300 flex-1 truncate">{s.share_code}</code>
                  <button
                    onClick={() => { navigator.clipboard.writeText(s.share_code) }}
                    className="p-1.5 text-sand-500 hover:text-sand-300 rounded transition-colors shrink-0"
                    title="Copy"
                  >
                    <Share2 className="h-3.5 w-3.5" />
                  </button>
                </div>
              ))}
            </div>
          </div>
        </div>
      )}
    </div>
  )
}

export default LibraryPage