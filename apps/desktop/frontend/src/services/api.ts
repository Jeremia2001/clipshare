import axios from 'axios'

const apiUrl = localStorage.getItem('api_url') || 'http://localhost:8080'

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

// Handle token refresh on 401
api.interceptors.response.use(
  (response) => response,
  async (error) => {
    const originalRequest = error.config
    
    if (error.response?.status === 401 && !originalRequest._retry) {
      originalRequest._retry = true
      
      const refreshToken = localStorage.getItem('refresh_token')
      if (refreshToken) {
        try {
          const response = await axios.post(`${apiUrl}/api/v1/auth/refresh`, {
            refresh_token: refreshToken
          })
          
          const { access_token } = response.data
          localStorage.setItem('access_token', access_token)
          
          originalRequest.headers.Authorization = `Bearer ${access_token}`
          return api(originalRequest)
        } catch (refreshError) {
          localStorage.removeItem('access_token')
          localStorage.removeItem('refresh_token')
          window.location.href = '/login'
        }
      }
    }
    
    return Promise.reject(error)
  }
)

export const setApiUrl = (url: string) => {
  localStorage.setItem('api_url', url)
  api.defaults.baseURL = `${url}/api/v1`
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

export const clipApi = {
  uploadFile: (file: File): Promise<{ data: { clip: Clip; object_key: string } }> => {
    const apiUrl = localStorage.getItem('api_url') || 'http://localhost:8080'
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
    const apiUrl = localStorage.getItem('api_url') || 'http://localhost:8080'
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
