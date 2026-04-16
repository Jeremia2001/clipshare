import { useState, useRef, useCallback, useEffect } from 'react'
import { useNavigate } from 'react-router-dom'
import { useAuth } from '../hooks/useAuth'
import {
  Upload, Film, Scissors, Globe,
  MessageSquare, X, Loader2,
  Play, Pause, SkipBack, SkipForward, FolderOpen
} from 'lucide-react'
import { clipApi } from '../services/api'
import {
  FFmpegIsAvailable, InstallFFmpeg, GetMediaServerURL,
  OpenFileDialog, ProbeVideo, TrimVideo, ServeLocalFile, CleanupServe, UploadFile
} from '../../wailsjs/go/main/App'
import { EventsOn, EventsOff } from '../../wailsjs/runtime/runtime'

type EditorStep = 'select' | 'editing' | 'processing' | 'uploading'

function formatTime(seconds: number): string {
  if (!seconds || !isFinite(seconds)) return '0:00.0'
  const m = Math.floor(seconds / 60)
  const s = Math.floor(seconds % 60)
  const ms = Math.floor((seconds % 1) * 10)
  return `${m}:${s.toString().padStart(2, '0')}.${ms}`
}

interface VideoPlayerProps {
  src: string
  onDurationChange: (duration: number) => void
  onDimensionsChange: (width: number, height: number) => void
  onTrimChange: (start: number, end: number) => void
}

function VideoPlayer({ src, onDurationChange, onDimensionsChange, onTrimChange }: VideoPlayerProps) {
  const videoEl = useRef<HTMLVideoElement>(null)
  const [playing, setPlaying] = useState(false)
  const [currentTime, setCurrentTime] = useState(0)
  const [duration, setDuration] = useState(0)
  const [trimStart, setTrimStart] = useState(0)
  const [trimEnd, setTrimEnd] = useState(0)
  const onTrimChangeRef = useRef(onTrimChange)
  onTrimChangeRef.current = onTrimChange
  const onDurationChangeRef = useRef(onDurationChange)
  onDurationChangeRef.current = onDurationChange
  const onDimensionsChangeRef = useRef(onDimensionsChange)
  onDimensionsChangeRef.current = onDimensionsChange

  useEffect(() => {
    const v = videoEl.current
    if (!v) return

    const onTime = () => setCurrentTime(v.currentTime)
    const onDur = () => {
      const dur = v.duration
      if (!dur || !isFinite(dur)) return
      setDuration(dur)
      onDurationChangeRef.current(dur)
      // videoWidth/videoHeight may be 0 at loadedmetadata in WebView2; canPlay handles that case
      if (v.videoWidth > 0 && v.videoHeight > 0) {
        onDimensionsChangeRef.current(v.videoWidth, v.videoHeight)
      }
      setTrimEnd(dur)
      onTrimChangeRef.current(0, dur)
    }
    // canplay fires after enough data is decoded — dimensions are reliably available here
    const onCanPlay = () => {
      if (v.videoWidth > 0 && v.videoHeight > 0) {
        onDimensionsChangeRef.current(v.videoWidth, v.videoHeight)
      }
    }
    const onPlay = () => setPlaying(true)
    const onPause = () => setPlaying(false)
    const onEnded = () => setPlaying(false)

    v.addEventListener('timeupdate', onTime)
    v.addEventListener('loadedmetadata', onDur)
    v.addEventListener('canplay', onCanPlay)
    v.addEventListener('play', onPlay)
    v.addEventListener('pause', onPause)
    v.addEventListener('ended', onEnded)
    return () => {
      v.removeEventListener('timeupdate', onTime)
      v.removeEventListener('loadedmetadata', onDur)
      v.removeEventListener('canplay', onCanPlay)
      v.removeEventListener('play', onPlay)
      v.removeEventListener('pause', onPause)
      v.removeEventListener('ended', onEnded)
    }
  }, [src])

  const togglePlay = useCallback(() => {
    const v = videoEl.current
    if (!v) return
    if (v.paused) { v.play().catch(() => {}) } else { v.pause() }
  }, [])

  const seek = useCallback((time: number) => {
    const v = videoEl.current
    if (!v || !duration) return
    const wasPlaying = !v.paused
    v.currentTime = Math.max(0, Math.min(time, duration))
    if (wasPlaying && v.paused) {
      v.play().catch(() => {})
    }
  }, [duration])

  const handleProgressClick = (e: React.MouseEvent<HTMLDivElement>) => {
    const rect = e.currentTarget.getBoundingClientRect()
    const pct = (e.clientX - rect.left) / rect.width
    seek(pct * duration)
  }

  const handleTrimStartChange = (val: number) => {
    setTrimStart(val)
    if (val >= trimEnd) setTrimEnd(Math.min(duration, val + 0.5))
    seek(val)
    onTrimChange(val, trimEnd)
  }

  const handleTrimEndChange = (val: number) => {
    setTrimEnd(val)
    if (val <= trimStart) setTrimStart(Math.max(0, val - 0.5))
    seek(val)
    onTrimChange(trimStart, val)
  }

  return (
    <div className="space-y-2">
      <div className="relative bg-black rounded-xl overflow-hidden" style={{ maxHeight: '480px' }}>
        <video
          ref={videoEl}
          src={src}
          className="w-full"
          style={{ maxHeight: '480px', display: 'block', backgroundColor: '#000' }}
          playsInline
          preload="auto"
          controls
        />
      </div>

      {duration > 0 && (
        <div className="relative h-6 bg-forest-900 rounded cursor-pointer mx-1" onClick={handleProgressClick}>
          <div
            className="absolute top-0 bottom-0"
            style={{
              left: `${(trimStart / duration) * 100}%`,
              width: `${((trimEnd - trimStart) / duration) * 100}%`,
              background: 'rgba(74, 222, 128, 0.15)',
              borderTop: '2px solid rgb(74, 222, 128)',
              borderBottom: '2px solid rgb(74, 128, 255)',
              pointerEvents: 'none',
            }}
          />
          <div
            className="absolute top-0 bottom-0 w-0.5 bg-sand-300 z-10"
            style={{ left: `${(currentTime / duration) * 100}%` }}
          />
        </div>
      )}

      <div className="flex items-center justify-between px-1">
        <div className="flex items-center space-x-1">
          <button onClick={() => seek(currentTime - 5)} className="btn-ghost p-1.5" title="Back 5s">
            <SkipBack className="h-4 w-4" />
          </button>
          <button onClick={togglePlay} className="btn-primary p-1.5" title={playing ? 'Pause' : 'Play'}>
            {playing ? <Pause className="h-4 w-4" /> : <Play className="h-4 w-4 ml-0.5" />}
          </button>
          <button onClick={() => seek(currentTime + 5)} className="btn-ghost p-1.5" title="Forward 5s">
            <SkipForward className="h-4 w-4" />
          </button>
          <span className="text-xs text-sand-400 font-mono ml-2">
            {formatTime(currentTime)} / {formatTime(duration)}
          </span>
          <span className="text-xs text-forest-400 font-mono ml-2">
            Trim: {formatTime(trimStart)} - {formatTime(trimEnd)} ({formatTime(trimEnd - trimStart)})
          </span>
        </div>
      </div>

      {duration > 0 && (
        <div className="card">
          <div className="section-header flex items-center space-x-2.5 py-2.5">
            <Scissors className="h-3.5 w-3.5 text-sand-500" />
            <span className="text-sm font-semibold text-sand-200">Trim</span>
          </div>
          <div className="card-body py-3 space-y-3">
            <div className="grid grid-cols-2 gap-4">
              <div>
                <label className="block text-xs text-sand-500 mb-1">Start at {formatTime(trimStart)}</label>
                <input
                  type="range"
                  min={0}
                  max={duration || 1}
                  step={0.1}
                  value={trimStart}
                  onChange={e => handleTrimStartChange(parseFloat(e.target.value))}
                  className="w-full accent-forest-500"
                />
              </div>
              <div>
                <label className="block text-xs text-sand-500 mb-1">End at {formatTime(trimEnd)}</label>
                <input
                  type="range"
                  min={0}
                  max={duration || 1}
                  step={0.1}
                  value={trimEnd}
                  onChange={e => handleTrimEndChange(parseFloat(e.target.value))}
                  className="w-full accent-forest-500"
                />
              </div>
            </div>
            <div className="text-xs text-sand-500">
              Trimmed duration: {formatTime(trimEnd - trimStart)} from {formatTime(duration)}
            </div>
          </div>
        </div>
      )}
    </div>
  )
}

function EditorPage() {
  const { user } = useAuth()
  const navigate = useNavigate()

  const selectingRef = useRef(false)

  const [step, setStep] = useState<EditorStep>('select')
  const [selectedFilePath, setSelectedFilePath] = useState<string>('')
  const [fileName, setFileName] = useState('')
  const [videoMediaUrl, setVideoMediaUrl] = useState<string | null>(null)
  const [servedName, setServedName] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)

  const [title, setTitle] = useState('')
  const [description, setDescription] = useState('')
  const [isPublic, setIsPublic] = useState(true)
  const [allowComments, setAllowComments] = useState(true)
  const [trimStart, setTrimStart] = useState(0)
  const [trimEnd, setTrimEnd] = useState(0)
  const [videoDuration, setVideoDuration] = useState(0)
  const [videoWidth, setVideoWidth] = useState(0)
  const [videoHeight, setVideoHeight] = useState(0)

  const [mediaServerURL, setMediaServerURL] = useState('')
  const [ffmpegAvailable, setFfmpegAvailable] = useState(false)
  const [ffmpegInstalling, setFfmpegInstalling] = useState(false)
  const [ffmpegInstallStage, setFfmpegInstallStage] = useState('')
  const [ffmpegInstallPct, setFfmpegInstallPct] = useState(0)
  const [ffmpegInstallError, setFfmpegInstallError] = useState('')
  const [processing, setProcessing] = useState(false)
  const [processingProgress, setProcessingProgress] = useState('')

  useEffect(() => {
    GetMediaServerURL().then(setMediaServerURL)
    FFmpegIsAvailable().then(setFfmpegAvailable).catch(() => setFfmpegAvailable(false))
  }, [])

  const handleInstallFFmpeg = useCallback(async () => {
    setFfmpegInstalling(true)
    setFfmpegInstallError('')
    setFfmpegInstallStage('Starting…')
    setFfmpegInstallPct(0)

    const handler = (data: { stage: string; pct: number }) => {
      setFfmpegInstallStage(data.stage)
      setFfmpegInstallPct(data.pct)
    }
    EventsOn('ffmpeg:install:progress', handler)

    try {
      await InstallFFmpeg()
      setFfmpegAvailable(true)
    } catch (err: any) {
      setFfmpegInstallError(err?.message ?? String(err))
    } finally {
      EventsOff('ffmpeg:install:progress')
      setFfmpegInstalling(false)
    }
  }, [])

  const handleSelectFile = useCallback(async () => {
    if (selectingRef.current) return
    selectingRef.current = true
    try {
      const filePath = await OpenFileDialog()
      if (!filePath) return

      setSelectedFilePath(filePath)
      const name = filePath.replace(/^.*[/\\]/, '')
      setFileName(name)
      setTitle(name.replace(/\.[^/.]+$/, ''))

      const serveName = await ServeLocalFile(filePath)
      setServedName(serveName)
      setVideoMediaUrl(`${mediaServerURL}/media/${serveName}`)
      setStep('editing')

      if (ffmpegAvailable) {
        try {
          const probe = await ProbeVideo(filePath)
          if (probe) {
            setVideoDuration(probe.duration)
            setVideoWidth(probe.width)
            setVideoHeight(probe.height)
            setTrimEnd(probe.duration)
            setTrimStart(0)
          }
        } catch (e) {
          console.error('[EditorPage] Probe failed:', e)
        }
      }
    } catch (err: any) {
      setError(err.message || 'Failed to select file')
    } finally {
      selectingRef.current = false
    }
  }, [ffmpegAvailable, mediaServerURL])

  const handleDurationChange = (duration: number) => {
    setVideoDuration(duration)
  }

  const handleDimensionsChange = (width: number, height: number) => {
    setVideoWidth(width)
    setVideoHeight(height)
  }

  const handleTrimChange = (start: number, end: number) => {
    setTrimStart(start)
    setTrimEnd(end)
  }

  const handleUpload = async () => {
    if (!selectedFilePath || !servedName) return

    const isTrimmed = trimStart > 0.5 || trimEnd < videoDuration - 0.5

    if (isTrimmed && ffmpegAvailable) {
      setProcessing(true)
      setProcessingProgress('Trimming video with native ffmpeg...')
      try {
        const trimmedServeName = await TrimVideo({
          input_path: selectedFilePath,
          start_time: trimStart,
          duration: Math.max(0.1, trimEnd - trimStart),
        })

        setProcessingProgress('Uploading trimmed clip...')
        const result = await UploadFile(trimmedServeName)
        setProcessing(false)
        await finalizeUpload(result.clip_id, result.file_size_bytes)
      } catch (err: any) {
        console.error('[EditorPage] Native ffmpeg trim error:', err)
        setProcessing(false)
        setError('Trimming failed. Uploading full clip instead.')
        try {
          const result = await UploadFile(servedName)
          await finalizeUpload(result.clip_id, result.file_size_bytes)
        } catch (uploadErr: any) {
          setError(uploadErr.message || 'Upload failed')
          setStep('editing')
        }
      }
    } else {
      try {
        setProcessing(true)
        setProcessingProgress('Uploading clip...')
        const result = await UploadFile(servedName)
        setProcessing(false)
        await finalizeUpload(result.clip_id, result.file_size_bytes)
      } catch (err: any) {
        setError(err.message || 'Upload failed')
        setStep('editing')
      } finally {
        setProcessing(false)
      }
    }
  }

  const finalizeUpload = async (clipId: string, fileSize: number) => {
    try {
      await clipApi.finalizeUpload(clipId, {
        title,
        description: description || undefined,
        original_filename: fileName,
        file_size_bytes: fileSize,
        duration_seconds: Math.round(videoDuration),
        width: videoWidth,
        height: videoHeight,
        is_public: isPublic,
        allow_comments: allowComments,
        trim_start_seconds: trimStart,
        trim_end_seconds: trimEnd,
      })

      navigate(`/clips/${clipId}`)
    } catch (err: any) {
      console.error('[EditorPage] Finalize error:', err)
      setError(err.response?.data?.error || err.message || 'Finalize failed')
      setStep('editing')
    }
  }

  const reset = () => {
    if (servedName) {
      CleanupServe(servedName).catch(() => {})
    }
    setStep('select')
    setSelectedFilePath('')
    setFileName('')
    setVideoMediaUrl(null)
    setServedName(null)
    setError(null)
    setTitle('')
    setDescription('')
    setTrimStart(0)
    setTrimEnd(0)
    setVideoDuration(0)
    setVideoWidth(0)
    setVideoHeight(0)
    setProcessing(false)
    setProcessingProgress('')
  }

  if (!user) return null

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-sand-100">Clip Editor</h1>
          <p className="text-sm text-sand-500 mt-1">Trim and edit locally, then upload to share</p>
        </div>
        {step !== 'select' && (
          <button onClick={reset} className="btn-ghost inline-flex items-center space-x-2 text-sm">
            <X className="h-4 w-4" />
            <span>New Clip</span>
          </button>
        )}
      </div>

      {error && (
        <div className="bg-earth-900/40 border border-earth-700/50 text-earth-300 px-4 py-3 rounded-lg text-sm">
          {error}
        </div>
      )}

      {step === 'select' && (
        <div className="card">
          <div
            className="card-body text-center py-16 border-2 border-dashed border-forest-700/60 rounded-xl hover:border-forest-600/80 transition-colors cursor-pointer"
            onClick={handleSelectFile}
          >
            <div className="mx-auto h-16 w-16 rounded-2xl bg-forest-800/50 flex items-center justify-center mb-4">
              <Film className="h-8 w-8 text-forest-400" />
            </div>
            <h3 className="text-lg font-semibold text-sand-200">Select a video to edit</h3>
            <p className="mt-2 text-sand-500 max-w-md mx-auto">
              Choose a video file from your computer to trim and edit locally
            </p>
            <button className="btn-primary mt-6 inline-flex items-center space-x-2" onClick={(e) => { e.stopPropagation(); handleSelectFile() }}>
              <FolderOpen className="h-4 w-4" />
              <span>Browse Files</span>
            </button>
            {ffmpegAvailable ? (
              <p className="mt-3 text-xs text-moss-400">Native ffmpeg detected — fast trimming enabled</p>
            ) : (
              <div className="mt-3 space-y-2">
                {ffmpegInstalling ? (
                  <div className="space-y-1">
                    <p className="text-xs text-sand-400">{ffmpegInstallStage}</p>
                    <div className="w-48 mx-auto h-1.5 bg-stone-700 rounded-full overflow-hidden">
                      <div
                        className="h-full bg-forest-500 transition-all duration-300"
                        style={{ width: `${ffmpegInstallPct}%` }}
                      />
                    </div>
                  </div>
                ) : (
                  <>
                    {ffmpegInstallError && (
                      <p className="text-xs text-earth-400">{ffmpegInstallError}</p>
                    )}
                    <button
                      className="btn-secondary text-xs px-3 py-1.5"
                      onClick={handleInstallFFmpeg}
                    >
                      Install FFmpeg
                    </button>
                    <p className="text-xs text-sand-600">Required for video trimming</p>
                  </>
                )}
              </div>
            )}
          </div>
        </div>
      )}

      {step === 'editing' && videoMediaUrl && (
        <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
          <div className="lg:col-span-2 space-y-4">
            <div className="card">
              <div className="card-body p-0 overflow-hidden">
                <VideoPlayer
                  src={videoMediaUrl}
                  onDurationChange={handleDurationChange}
                  onDimensionsChange={handleDimensionsChange}
                  onTrimChange={handleTrimChange}
                />
              </div>
            </div>

            <div className="card">
              <div className="section-header flex items-center space-x-2.5">
                <Scissors className="h-4 w-4 text-sand-500" />
                <h2 className="text-base font-semibold text-sand-200">Clip Details</h2>
              </div>
              <div className="card-body space-y-4">
                <div>
                  <label className="block text-sm font-medium text-sand-400 mb-2">Title</label>
                  <input
                    type="text"
                    value={title}
                    onChange={(e) => setTitle(e.target.value)}
                    className="input-field"
                    placeholder="Give your clip a title..."
                  />
                </div>
                <div>
                  <label className="block text-sm font-medium text-sand-400 mb-2">Description</label>
                  <textarea
                    value={description}
                    onChange={(e) => setDescription(e.target.value)}
                    className="input-field min-h-[80px] resize-y"
                    placeholder="Describe your clip..."
                  />
                </div>
                <div className="flex items-center space-x-6">
                  <label className="flex items-center space-x-2 cursor-pointer">
                    <input
                      type="checkbox"
                      checked={isPublic}
                      onChange={(e) => setIsPublic(e.target.checked)}
                      className="h-4 w-4 rounded border-forest-700 bg-forest-950 text-forest-500 focus:ring-forest-500"
                    />
                    <Globe className="h-4 w-4 text-sand-500" />
                    <span className="text-sm text-sand-300">Public</span>
                  </label>
                  <label className="flex items-center space-x-2 cursor-pointer">
                    <input
                      type="checkbox"
                      checked={allowComments}
                      onChange={(e) => setAllowComments(e.target.checked)}
                      className="h-4 w-4 rounded border-forest-700 bg-forest-950 text-forest-500 focus:ring-forest-500"
                    />
                    <MessageSquare className="h-4 w-4 text-sand-500" />
                    <span className="text-sm text-sand-300">Allow Comments</span>
                  </label>
                </div>
                {selectedFilePath && (
                  <div className="grid grid-cols-2 gap-4 text-sm text-sand-500">
                    <div>Duration: {videoDuration > 0 ? `${Math.round(videoDuration)}s` : 'loading...'}</div>
                    <div>Resolution: {videoWidth && videoHeight ? `${videoWidth}x${videoHeight}` : 'loading...'}</div>
                    <div>File: {fileName}</div>
                    <div>Engine: {ffmpegAvailable ? 'Native ffmpeg' : 'None'}</div>
                  </div>
                )}
              </div>
            </div>

            <div className="flex items-center space-x-3">
              <button onClick={handleUpload} disabled={processing} className="btn-primary inline-flex items-center space-x-2">
                {processing ? <Loader2 className="h-4 w-4 animate-spin" /> : <Upload className="h-4 w-4" />}
                <span>{processing ? 'Processing...' : 'Upload & Save'}</span>
              </button>
              <button onClick={reset} disabled={processing} className="btn-ghost inline-flex items-center space-x-2 text-earth-500 hover:text-earth-400">
                <X className="h-4 w-4" />
                <span>Cancel</span>
              </button>
            </div>

            {processing && processingProgress && (
              <div className="card">
                <div className="card-body text-center py-4">
                  <Loader2 className="h-8 w-8 text-forest-400 animate-spin mx-auto mb-2" />
                  <p className="text-sm text-sand-400">{processingProgress}</p>
                </div>
              </div>
            )}
          </div>

          <div className="space-y-4">
            <div className="card">
              <div className="section-header flex items-center space-x-2.5">
                <Scissors className="h-4 w-4 text-sand-500" />
                <h2 className="text-base font-semibold text-sand-200">
                  {ffmpegAvailable ? 'Native Processing' : 'Trimming Unavailable'}
                </h2>
              </div>
              <div className="card-body space-y-3">
                {ffmpegAvailable ? (
                  <>
                    <p className="text-sm text-sand-400">
                      Trim your clip using the controls above. When you click <strong>Upload &amp; Save</strong>, the trimmed segment is cut with native ffmpeg and uploaded.
                    </p>
                    <div className="text-xs text-sand-600 space-y-1">
                      <div className="flex items-center space-x-1.5">
                        <span className="h-1.5 w-1.5 rounded-full bg-moss-500 inline-block" />
                        <span>Native ffmpeg for fast, high-quality trimming</span>
                      </div>
                      <div className="flex items-center space-x-1.5">
                        <span className="h-1.5 w-1.5 rounded-full bg-moss-500 inline-block" />
                        <span>Only the trimmed portion is uploaded</span>
                      </div>
                      <div className="flex items-center space-x-1.5">
                        <span className="h-1.5 w-1.5 rounded-full bg-moss-500 inline-block" />
                        <span>Hardware-accelerated encoding when available</span>
                      </div>
                    </div>
                  </>
                ) : ffmpegInstalling ? (
                  <div className="space-y-2">
                    <p className="text-sm text-sand-400">{ffmpegInstallStage}</p>
                    <div className="h-1.5 bg-stone-700 rounded-full overflow-hidden">
                      <div
                        className="h-full bg-forest-500 transition-all duration-300"
                        style={{ width: `${ffmpegInstallPct}%` }}
                      />
                    </div>
                    <p className="text-xs text-sand-600">{Math.round(ffmpegInstallPct)}%</p>
                  </div>
                ) : (
                  <div className="space-y-3">
                    <p className="text-sm text-sand-400">
                      FFmpeg is required for trimming. It will be downloaded once (~70 MB) and cached locally.
                    </p>
                    {ffmpegInstallError && (
                      <p className="text-xs text-earth-400">{ffmpegInstallError}</p>
                    )}
                    <button className="btn-secondary w-full" onClick={handleInstallFFmpeg}>
                      Install FFmpeg
                    </button>
                  </div>
                )}
              </div>
            </div>
          </div>
        </div>
      )}

      {step === 'uploading' && (
        <div className="card">
          <div className="card-body text-center py-12">
            <Loader2 className="h-12 w-12 text-forest-400 animate-spin mx-auto mb-4" />
            <h3 className="text-lg font-semibold text-sand-200">Uploading...</h3>
            <p className="text-sm text-sand-500 mt-2">{fileName}</p>
            <p className="text-xs text-sand-600 mt-1">You will be redirected automatically</p>
          </div>
        </div>
      )}
    </div>
  )
}

export default EditorPage