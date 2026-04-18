import axios from 'axios'

export const defaultApiUrl = 'http://127.0.0.1:8080'

export function normalizeApiUrl(url: string | null): string {
  if (!url) return defaultApiUrl
  return url.replace('//localhost:', '//127.0.0.1:').replace(/\/+$/, '')
}

const apiUrl = normalizeApiUrl(localStorage.getItem('api_url'))

export const api = axios.create({
  baseURL: `${apiUrl}/api/v1`,
  headers: {
    'Content-Type': 'application/json'
  }
})

// Add auth token to requests
api.interceptors.request.use((config) => {
  const token = localStorage.getItem('access_token')
  if (token) {
    config.headers.Authorization = `Bearer ${token}`
  }
  return config
})

// On 401 the stored token is either stale (admin JWT expired) or revoked
// (device removed from the server). Drop it and let the router bounce the
// user back to the login screen.
api.interceptors.response.use(
  (response) => response,
  (error) => {
    if (error.response?.status === 401) {
      localStorage.removeItem('access_token')
      if (window.location.pathname !== '/login') {
        window.location.href = '/login'
      }
    }
    return Promise.reject(error)
  }
)

export const setApiUrl = (url: string) => {
  const normalized = normalizeApiUrl(url)
  localStorage.setItem('api_url', normalized)
  api.defaults.baseURL = `${normalized}/api/v1`
}

export interface Clip {
  id: string
  user_id: string
  title: string
  description?: string
  rustfs_bucket: string
  rustfs_object_key: string
  original_filename: string
  file_size_bytes: number
  duration_seconds: number
  width: number
  height: number
  fps?: number
  bitrate_kbps?: number
  thumbnail_key?: string
  processed_variant_keys?: string[]
  codec?: string
  trim_start_seconds: number
  trim_end_seconds: number
  is_public: boolean
  allow_comments: boolean
  expires_at?: string
  view_count: number
  created_at: string
  updated_at: string
  view_url?: string
  thumbnail_url?: string
}

export interface Share {
  id: string
  clip_id: string
  user_id: string
  share_code: string
  custom_slug?: string
  has_password?: boolean
  expires_at?: string
  max_views?: number
  view_count: number
  is_active: boolean
  created_at: string
}

export interface ClipListResponse {
  clips: Clip[]
  total: number
  page: number
  per_page: number
}

export interface ShareResponse {
  share_code: string
  share_url: string
  share: Share
}

export interface Comment {
  id: string
  clip_id: string
  user_id?: string
  parent_id?: string
  display_name?: string
  content: string
  is_edited: boolean
  edited_at?: string
  created_at: string
}

export const clipApi = {
  uploadFile: (file: File): Promise<{ data: { clip: Clip; object_key: string } }> => {
    const apiUrl = normalizeApiUrl(localStorage.getItem('api_url'))
    const url = `${apiUrl}/api/v1/clips/upload`
    return new Promise((resolve, reject) => {
      const xhr = new XMLHttpRequest()
      xhr.open('POST', url)
      const token = localStorage.getItem('access_token')
      if (token) xhr.setRequestHeader('Authorization', `Bearer ${token}`)
      xhr.onload = () => {
        if (xhr.status >= 200 && xhr.status < 300) {
          resolve({ data: JSON.parse(xhr.responseText) })
        } else {
          try {
            reject({ response: { data: JSON.parse(xhr.responseText) } })
          } catch {
            reject(new Error(`Upload failed: ${xhr.status} ${xhr.statusText}`))
          }
        }
      }
      xhr.onerror = () => reject(new Error('Network error during upload'))
      xhr.ontimeout = () => reject(new Error('Upload timed out'))
      const formData = new FormData()
      formData.append('file', file)
      xhr.send(formData)
    })
  },

  finalizeUpload: (clipId: string, data: {
    title: string
    description?: string
    original_filename: string
    file_size_bytes: number
    duration_seconds: number
    width: number
    height: number
    fps?: number
    bitrate_kbps?: number
    codec?: string
    is_public: boolean
    allow_comments: boolean
    trim_start_seconds?: number
    trim_end_seconds?: number
  }) => api.post<Clip>(`/clips/${clipId}/finalize`, data),

  list: (page = 1, perPage = 20) =>
    api.get<ClipListResponse>('/clips', { params: { page, per_page: perPage } }),

  get: (id: string) =>
    api.get<{ clip: Clip; view_url?: string }>(`/clips/${id}`),

  update: (id: string, data: {
    title?: string
    description?: string
    is_public?: boolean
    allow_comments?: boolean
    thumbnail_key?: string
    trim_start_seconds?: number
    trim_end_seconds?: number
  }) => api.put<Clip>(`/clips/${id}`, data),

  delete: (id: string) =>
    api.delete(`/clips/${id}`),

  uploadThumbnail: (clipId: string, blob: Blob): Promise<void> => {
    const apiUrl = normalizeApiUrl(localStorage.getItem('api_url'))
    const url = `${apiUrl}/api/v1/clips/${clipId}/thumbnail`
    return new Promise((resolve, reject) => {
      const xhr = new XMLHttpRequest()
      xhr.open('POST', url)
      const token = localStorage.getItem('access_token')
      if (token) xhr.setRequestHeader('Authorization', `Bearer ${token}`)
      xhr.onload = () => {
        if (xhr.status >= 200 && xhr.status < 300) resolve()
        else reject(new Error(`Thumbnail upload failed: ${xhr.status}`))
      }
      xhr.onerror = () => reject(new Error('Network error'))
      const formData = new FormData()
      formData.append('thumbnail', blob, 'thumbnail.jpg')
      xhr.send(formData)
    })
  },
}

export const shareApi = {
  create: (clipId: string, data?: {
    custom_slug?: string
    password?: string
    expires_at?: string
    max_views?: number
  }) => api.post<ShareResponse>(`/clips/${clipId}/shares`, data || {}),

  list: (clipId: string) =>
    api.get<{ shares: Share[] }>(`/clips/${clipId}/shares`),

  delete: (clipId: string, shareId: string) =>
    api.delete(`/clips/${clipId}/shares/${shareId}`),

  getShared: (code: string, password?: string) =>
    api.get<{ clip: Clip; view_url: string }>(`/s/${code}`, { params: password ? { password } : {} }),
}

export const commentApi = {
  listByClip: (clipId: string) =>
    api.get<{ comments: Comment[] }>(`/clips/${clipId}/comments`),
}
