import { useState, useRef, useCallback, useEffect } from 'react'
import { useNavigate } from 'react-router-dom'
import { useAuth } from '../hooks/useAuth'
import {
  Upload, Film, Scissors, Globe,
  MessageSquare, X, Loader2,
  Play, Pause, SkipBack, SkipForward, FolderOpen,
  ChevronLeft, ChevronRight, ArrowRightFromLine, ArrowLeftFromLine
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

function formatTimePrecise(seconds: number): string {
  if (!seconds || !isFinite(seconds)) return '00:00.0'
  const m = Math.floor(seconds / 60)
  const s = Math.floor(seconds % 60)
  const d = Math.floor((seconds % 1) * 10)
  return `${m.toString().padStart(2, '0')}:${s.toString().padStart(2, '0')}.${d}`
}

interface VideoPlayerProps {
  src: string
  onDurationChange: (duration: number) => void
  onDimensionsChange: (width: number, height: number) => void
  onTrimChange: (start: number, end: number) => void
}

function VideoPlayer({ src, onDurationChange, onDimensionsChange, onTrimChange }: VideoPlayerProps) {
  const videoEl = useRef<HTMLVideoElement>(null)
  const timelineRef = useRef<HTMLDivElement>(null)
  const [playing, setPlaying] = useState(false)
  const [currentTime, setCurrentTime] = useState(0)
  const [duration, setDuration] = useState(0)
  const [trimStart, setTrimStart] = useState(0)
  const [trimEnd, setTrimEnd] = useState(0)
  const [dragging, setDragging] = useState<'start' | 'end' | 'seek' | null>(null)
  const [editingStart, setEditingStart] = useState(false)
  const [editingEnd, setEditingEnd] = useState(false)
  const [startInput, setStartInput] = useState('')
  const [endInput, setEndInput] = useState('')

  const onTrimChangeRef = useRef(onTrimChange)
  onTrimChangeRef.current = onTrimChange
  const onDurationChangeRef = useRef(onDurationChange)
  onDurationChangeRef.current = onDurationChange
  const onDimensionsChangeRef = useRef(onDimensionsChange)
  onDimensionsChangeRef.current = onDimensionsChange

  const trimStartRef = useRef(trimStart)
  const trimEndRef = useRef(trimEnd)
  const durationRef = useRef(duration)
  trimStartRef.current = trimStart
  trimEndRef.current = trimEnd
  durationRef.current = duration

  useEffect(() => {
    const v = videoEl.current
    if (!v) return

    const onTime = () => setCurrentTime(v.currentTime)
    const onDur = () => {
      const dur = v.duration
      if (!dur || !isFinite(dur)) return
      setDuration(dur)
      onDurationChangeRef.current(dur)
      if (v.videoWidth > 0 && v.videoHeight > 0) {
        onDimensionsChangeRef.current(v.videoWidth, v.videoHeight)
      }
      setTrimEnd(dur)
      onTrimChangeRef.current(0, dur)
    }
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

  const handleKeyDown = useCallback((e: React.KeyboardEvent) => {
    const v = videoEl.current
    if (!v || !durationRef.current) return
    if (editingStart || editingEnd) return

    switch (e.key) {
      case ' ':
        e.preventDefault()
        if (v.paused) { v.play().catch(() => {}) } else { v.pause() }
        break
      case 'ArrowLeft':
        e.preventDefault()
        v.currentTime = Math.max(0, v.currentTime - (e.shiftKey ? 1 : 5))
        break
      case 'ArrowRight':
        e.preventDefault()
        v.currentTime = Math.min(durationRef.current, v.currentTime + (e.shiftKey ? 1 : 5))
        break
      case 'Home':
        e.preventDefault()
        v.currentTime = trimStartRef.current
        break
      case 'End':
        e.preventDefault()
        v.currentTime = trimEndRef.current
        break
      case 's': case 'S': {
        e.preventDefault()
        const newStart = Math.max(0, Math.min(v.currentTime, trimEndRef.current - 0.5))
        setTrimStart(newStart)
        onTrimChangeRef.current(newStart, trimEndRef.current)
        break
      }
      case 'e': case 'E': {
        e.preventDefault()
        const newEnd = Math.min(durationRef.current, Math.max(v.currentTime, trimStartRef.current + 0.5))
        setTrimEnd(newEnd)
        onTrimChangeRef.current(trimStartRef.current, newEnd)
        break
      }
    }
  }, [editingStart, editingEnd])

  const markIn = useCallback(() => {
    const v = videoEl.current
    if (!v) return
    const t = v.currentTime
    const newStart = Math.max(0, Math.min(t, trimEndRef.current - 0.5))
    setTrimStart(newStart)
    onTrimChange(newStart, trimEnd)
  }, [trimEnd, onTrimChange])

  const markOut = useCallback(() => {
    const v = videoEl.current
    if (!v) return
    const t = v.currentTime
    const newEnd = Math.min(duration, Math.max(t, trimStartRef.current + 0.5))
    setTrimEnd(newEnd)
    onTrimChange(trimStart, newEnd)
  }, [duration, trimStart, onTrimChange])

  const seek = useCallback((time: number) => {
    const v = videoEl.current
    if (!v || !duration) return
    v.currentTime = Math.max(0, Math.min(time, duration))
  }, [duration])

  const togglePlay = useCallback(() => {
    const v = videoEl.current
    if (!v) return
    if (v.paused) { v.play().catch(() => {}) } else { v.pause() }
  }, [])

  const timeFromMouseEvent = useCallback((e: React.MouseEvent<HTMLDivElement>) => {
    const rect = e.currentTarget.getBoundingClientRect()
    const pct = Math.max(0, Math.min(1, (e.clientX - rect.left) / rect.width))
    return pct * durationRef.current
  }, [])

  const handleTimelineMouseDown = useCallback((e: React.MouseEvent<HTMLDivElement>) => {
    const time = timeFromMouseEvent(e)
    const startPct = trimStartRef.current / durationRef.current
    const endPct = trimEndRef.current / durationRef.current
    const rect = e.currentTarget.getBoundingClientRect()
    const clickPct = (e.clientX - rect.left) / rect.width
    const handleZone = 0.03

    if (Math.abs(clickPct - startPct) < handleZone) {
      setDragging('start')
    } else if (Math.abs(clickPct - endPct) < handleZone) {
      setDragging('end')
    } else if (clickPct >= startPct && clickPct <= endPct) {
      seek(time)
    } else {
      seek(time)
    }
  }, [timeFromMouseEvent, seek])

  useEffect(() => {
    if (!dragging) return

    const handleMouseMove = (e: MouseEvent) => {
      if (!timelineRef.current || !durationRef.current) return
      const rect = timelineRef.current.getBoundingClientRect()
      const pct = Math.max(0, Math.min(1, (e.clientX - rect.left) / rect.width))
      const time = pct * durationRef.current

      if (dragging === 'start') {
        const newStart = Math.max(0, Math.min(time, trimEndRef.current - 0.5))
        setTrimStart(newStart)
        onTrimChangeRef.current(newStart, trimEndRef.current)
      } else if (dragging === 'end') {
        const newEnd = Math.min(durationRef.current, Math.max(time, trimStartRef.current + 0.5))
        setTrimEnd(newEnd)
        onTrimChangeRef.current(trimStartRef.current, newEnd)
      } else if (dragging === 'seek') {
        videoEl.current?.fastSeek?.(time) ?? (videoEl.current!.currentTime = time)
      }
    }

    const handleMouseUp = () => setDragging(null)

    window.addEventListener('mousemove', handleMouseMove)
    window.addEventListener('mouseup', handleMouseUp)
    return () => {
      window.removeEventListener('mousemove', handleMouseMove)
      window.removeEventListener('mouseup', handleMouseUp)
    }
  }, [dragging])

  const adjustTrimStart = useCallback((delta: number) => {
    const newStart = Math.max(0, Math.min(trimStart + delta, trimEnd - 0.5))
    setTrimStart(newStart)
    seek(newStart)
    onTrimChange(newStart, trimEnd)
  }, [trimStart, trimEnd, seek])

  const adjustTrimEnd = useCallback((delta: number) => {
    const newEnd = Math.min(duration, Math.max(trimEnd + delta, trimStart + 0.5))
    setTrimEnd(newEnd)
    seek(newEnd)
    onTrimChange(trimStart, newEnd)
  }, [trimStart, trimEnd, duration, seek])

  const confirmStartEdit = useCallback(() => {
    const parts = startInput.split(':')
    let seconds = 0
    if (parts.length === 2) {
      seconds = parseInt(parts[0]) * 60 + parseFloat(parts[1])
    } else {
      seconds = parseFloat(startInput)
    }
    if (!isNaN(seconds)) {
      const newStart = Math.max(0, Math.min(seconds, trimEnd - 0.5))
      setTrimStart(newStart)
      seek(newStart)
      onTrimChange(newStart, trimEnd)
    }
    setEditingStart(false)
  }, [startInput, trimEnd, seek])

  const confirmEndEdit = useCallback(() => {
    const parts = endInput.split(':')
    let seconds = 0
    if (parts.length === 2) {
      seconds = parseInt(parts[0]) * 60 + parseFloat(parts[1])
    } else {
      seconds = parseFloat(endInput)
    }
    if (!isNaN(seconds)) {
      const newEnd = Math.min(duration, Math.max(seconds, trimStart + 0.5))
      setTrimEnd(newEnd)
      seek(newEnd)
      onTrimChange(trimStart, newEnd)
    }
    setEditingEnd(false)
  }, [endInput, duration, trimStart, seek])

  const startPct = duration > 0 ? (trimStart / duration) * 100 : 0
  const endPct = duration > 0 ? (trimEnd / duration) * 100 : 100
  const playheadPct = duration > 0 ? (currentTime / duration) * 100 : 0

  return (
    <div className="space-y-3 outline-none" tabIndex={0} onKeyDown={handleKeyDown}>
      <div className="relative bg-black rounded-xl overflow-hidden" style={{ maxHeight: '480px' }}>
        <video
          ref={videoEl}
          src={src}
          className="w-full cursor-pointer"
          style={{ maxHeight: '480px', display: 'block', backgroundColor: '#000' }}
          playsInline
          preload="auto"
          onClick={togglePlay}
        />
      </div>

      {duration > 0 && (
        <div className="space-y-1.5">
          <div className="flex items-center justify-between px-1">
            <div className="flex items-center space-x-2">
              <button onClick={() => seek(trimStart)} className="btn-ghost p-1.5" title="Go to trim start">
                <SkipBack className="h-4 w-4" />
              </button>
              <button onClick={togglePlay} className="btn-primary px-3.5 py-1.5" title={playing ? 'Pause [Space]' : 'Play [Space]'}>
                {playing ? <Pause className="h-4 w-4" /> : <Play className="h-4 w-4 ml-0.5" />}
              </button>
              <button onClick={() => seek(trimEnd)} className="btn-ghost p-1.5" title="Go to trim end">
                <SkipForward className="h-4 w-4" />
              </button>
            </div>
            <div className="flex items-center space-x-3 text-xs font-mono">
              <span className="text-sand-300">{formatTime(currentTime)}</span>
              <span className="text-sand-600">/</span>
              <span className="text-sand-500">{formatTime(duration)}</span>
            </div>
            <div className="flex items-center space-x-1">
              <button onClick={() => seek(currentTime - 1)} className="btn-ghost p-1.5" title="Back 1s">
                <ChevronLeft className="h-4 w-4" />
              </button>
              <button onClick={() => seek(currentTime + 1)} className="btn-ghost p-1.5" title="Forward 1s">
                <ChevronRight className="h-4 w-4" />
              </button>
            </div>
          </div>

          <div
            ref={timelineRef}
            className="relative h-12 bg-forest-900/80 rounded-lg cursor-pointer select-none border border-forest-800/60 hover:border-forest-700/80 transition-colors"
            onMouseDown={handleTimelineMouseDown}
          >
            <div className="absolute inset-0 rounded-lg overflow-hidden">
              {Array.from({ length: Math.min(Math.floor(duration / 5), 48) + 2 }).map((_, i) => {
                const pct = (i / (Math.min(Math.floor(duration / 5), 48) + 1)) * 100
                return (
                  <div
                    key={i}
                    className="absolute top-0 bottom-0 w-px bg-forest-700/40"
                    style={{ left: `${pct}%` }}
                  />
                )
              })}
            </div>

            <div
              className="absolute top-0 bottom-0 rounded-sm transition-[left,width] duration-75"
              style={{
                left: `${startPct}%`,
                width: `${endPct - startPct}%`,
                background: 'linear-gradient(180deg, rgba(74, 222, 128, 0.08) 0%, rgba(74, 222, 128, 0.15) 100%)',
                borderTop: '2px solid rgba(74, 222, 128, 0.6)',
                borderBottom: '2px solid rgba(74, 222, 128, 0.3)',
              }}
            />

            <div
              className="absolute top-0 bottom-0 w-2.5 cursor-ew-resize z-20 group"
              style={{ left: `calc(${startPct}% - 5px)` }}
              onMouseDown={(e) => { e.stopPropagation(); setDragging('start') }}
            >
              <div className="absolute inset-y-1 left-1/2 -translate-x-1/2 w-1 rounded-full bg-moss-400 group-hover:bg-moss-300 transition-colors" />
              <div className="absolute top-1/2 left-1/2 -translate-x-1/2 -translate-y-1/2 space-y-0.5">
                <div className="w-2 h-px bg-forest-950/60" />
                <div className="w-2 h-px bg-forest-950/60" />
                <div className="w-2 h-px bg-forest-950/60" />
              </div>
            </div>

            <div
              className="absolute top-0 bottom-0 w-2.5 cursor-ew-resize z-20 group"
              style={{ left: `calc(${endPct}% - 5px)` }}
              onMouseDown={(e) => { e.stopPropagation(); setDragging('end') }}
            >
              <div className="absolute inset-y-1 left-1/2 -translate-x-1/2 w-1 rounded-full bg-forest-400 group-hover:bg-forest-300 transition-colors" />
              <div className="absolute top-1/2 left-1/2 -translate-x-1/2 -translate-y-1/2 space-y-0.5">
                <div className="w-2 h-px bg-forest-950/60" />
                <div className="w-2 h-px bg-forest-950/60" />
                <div className="w-2 h-px bg-forest-950/60" />
              </div>
            </div>

            <div
              className="absolute top-0 bottom-0 w-0.5 bg-white/70 z-10 pointer-events-none transition-[left] duration-75"
              style={{ left: `${playheadPct}%` }}
            >
              <div className="absolute -top-1 left-1/2 -translate-x-1/2 w-2 h-2 rounded-full bg-white/80" />
            </div>
          </div>

          <div className="card">
            <div className="section-header flex items-center space-x-2.5 py-2.5">
              <Scissors className="h-3.5 w-3.5 text-moss-400" />
              <span className="text-sm font-semibold text-sand-200">Trim</span>
              <span className="text-xs text-sand-500 font-normal ml-auto">
                {formatTime(trimEnd - trimStart)} selected of {formatTime(duration)}
              </span>
            </div>
            <div className="card-body py-3 space-y-4">
              <div className="flex items-center space-x-3">
                <button onClick={markIn} className="flex-1 px-3 py-2 rounded-lg bg-moss-900/60 hover:bg-moss-800/70 border border-moss-600/50 hover:border-moss-500/70 text-moss-300 hover:text-moss-200 text-sm font-semibold inline-flex items-center justify-center space-x-2 transition-colors" title="Set start to playhead [S]">
                  <ArrowLeftFromLine className="h-4 w-4" />
                  <span>Mark In</span>
                  <kbd className="text-[10px] bg-moss-800/60 px-1 py-0.5 rounded opacity-60">S</kbd>
                </button>
                <button onClick={markOut} className="flex-1 px-3 py-2 rounded-lg bg-forest-900/60 hover:bg-forest-800/70 border border-forest-600/50 hover:border-forest-500/70 text-forest-300 hover:text-forest-200 text-sm font-semibold inline-flex items-center justify-center space-x-2 transition-colors" title="Set end to playhead [E]">
                  <ArrowRightFromLine className="h-4 w-4" />
                  <span>Mark Out</span>
                  <kbd className="text-[10px] bg-forest-800/60 px-1 py-0.5 rounded opacity-60">E</kbd>
                </button>
              </div>
              <div className="grid grid-cols-2 gap-4">
                <div>
                  <div className="flex items-center justify-between mb-1.5">
                    <span className="text-xs font-medium text-moss-400">Start</span>
                    {editingStart ? (
                      <input
                        type="text"
                        value={startInput}
                        onChange={(e) => setStartInput(e.target.value)}
                        onKeyDown={(e) => { if (e.key === 'Enter') confirmStartEdit(); if (e.key === 'Escape') setEditingStart(false) }}
                        onBlur={confirmStartEdit}
                        className="w-20 text-xs font-mono bg-forest-950 border border-forest-700 rounded px-1.5 py-0.5 text-sand-200 focus:outline-none focus:ring-1 focus:ring-moss-500"
                        autoFocus
                      />
                    ) : (
                      <button
                        onClick={() => { setStartInput(formatTimePrecise(trimStart)); setEditingStart(true) }}
                        className="text-xs font-mono text-sand-300 hover:text-sand-100 bg-forest-900/60 hover:bg-forest-800/60 px-1.5 py-0.5 rounded transition-colors"
                      >
                        {formatTime(trimStart)}
                      </button>
                    )}
                  </div>
                  <div className="flex items-center space-x-1">
                    <button onClick={() => adjustTrimStart(-5)} className="btn-ghost px-1.5 py-0.5 text-xs font-mono" title="Back 5s">-5s</button>
                    <button onClick={() => adjustTrimStart(-1)} className="btn-ghost px-1.5 py-0.5 text-xs font-mono" title="Back 1s">-1s</button>
                    <button onClick={() => adjustTrimStart(-0.1)} className="btn-ghost px-1.5 py-0.5 text-xs font-mono">-.1</button>
                    <button onClick={() => { seek(trimStart) }} className="btn-ghost px-1.5 py-0.5 text-xs" title="Seek to start">
                      <SkipBack className="h-3 w-3" />
                    </button>
                  </div>
                </div>
                <div>
                  <div className="flex items-center justify-between mb-1.5">
                    <span className="text-xs font-medium text-forest-400">End</span>
                    {editingEnd ? (
                      <input
                        type="text"
                        value={endInput}
                        onChange={(e) => setEndInput(e.target.value)}
                        onKeyDown={(e) => { if (e.key === 'Enter') confirmEndEdit(); if (e.key === 'Escape') setEditingEnd(false) }}
                        onBlur={confirmEndEdit}
                        className="w-20 text-xs font-mono bg-forest-950 border border-forest-700 rounded px-1.5 py-0.5 text-sand-200 focus:outline-none focus:ring-1 focus:ring-forest-500"
                        autoFocus
                      />
                    ) : (
                      <button
                        onClick={() => { setEndInput(formatTimePrecise(trimEnd)); setEditingEnd(true) }}
                        className="text-xs font-mono text-sand-300 hover:text-sand-100 bg-forest-900/60 hover:bg-forest-800/60 px-1.5 py-0.5 rounded transition-colors"
                      >
                        {formatTime(trimEnd)}
                      </button>
                    )}
                  </div>
                  <div className="flex items-center space-x-1">
                    <button onClick={() => adjustTrimEnd(5)} className="btn-ghost px-1.5 py-0.5 text-xs font-mono" title="Forward 5s">+5s</button>
                    <button onClick={() => adjustTrimEnd(1)} className="btn-ghost px-1.5 py-0.5 text-xs font-mono" title="Forward 1s">+1s</button>
                    <button onClick={() => adjustTrimEnd(0.1)} className="btn-ghost px-1.5 py-0.5 text-xs font-mono">+.1</button>
                    <button onClick={() => { seek(trimEnd) }} className="btn-ghost px-1.5 py-0.5 text-xs" title="Seek to end">
                      <SkipForward className="h-3 w-3" />
                    </button>
                  </div>
                </div>
              </div>
              <div className="flex items-center justify-between">
                <p className="text-xs text-sand-600">
                  <kbd className="px-1 py-0.5 bg-forest-800/60 rounded text-sand-500 text-[10px]">S</kbd> mark in &nbsp;
                  <kbd className="px-1 py-0.5 bg-forest-800/60 rounded text-sand-500 text-[10px]">E</kbd> mark out &nbsp;
                  <kbd className="px-1 py-0.5 bg-forest-800/60 rounded text-sand-500 text-[10px]">Space</kbd> play
                </p>
                <button
                  onClick={() => {
                    setTrimStart(0)
                    setTrimEnd(duration)
                    onTrimChange(0, duration)
                    seek(0)
                  }}
                  className="text-xs text-sand-600 hover:text-sand-400 transition-colors"
                >
                  Reset trim
                </button>
              </div>
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
  const abortRef = useRef(false)

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

    abortRef.current = false
    const isTrimmed = trimStart > 0.5 || trimEnd < videoDuration - 0.5

    if (isTrimmed && ffmpegAvailable) {
      setProcessing(true)
      setProcessingProgress('Trimming video...')
      try {
        const trimmedServeName = await TrimVideo({
          input_path: selectedFilePath,
          start_time: trimStart,
          duration: Math.max(0.1, trimEnd - trimStart),
        })

        if (abortRef.current) { setProcessing(false); return }

        setProcessingProgress('Uploading trimmed clip...')
        const result = await UploadFile(trimmedServeName)
        setProcessing(false)
        if (abortRef.current) return
        await finalizeUpload(result.clip_id, result.file_size_bytes)
      } catch (err: any) {
        if (abortRef.current) { setProcessing(false); return }
        console.error('[EditorPage] Native ffmpeg trim error:', err)
        setProcessing(false)
        const detail = err?.message || String(err)
        const shortDetail = detail.split('\n')[0].substring(0, 120)
        setError(`Trimming failed: ${shortDetail}. Uploading full clip instead.`)
        try {
          const result = await UploadFile(servedName)
          if (abortRef.current) return
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
        if (abortRef.current) { setProcessing(false); return }
        setProcessing(false)
        await finalizeUpload(result.clip_id, result.file_size_bytes)
      } catch (err: any) {
        if (abortRef.current) { setProcessing(false); return }
        setError(err.message || 'Upload failed')
        setStep('editing')
      } finally {
        if (!abortRef.current) setProcessing(false)
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
    abortRef.current = true
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
        <div className="space-y-4">
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
              {ffmpegAvailable && (
                <span className="badge-moss ml-auto">Native ffmpeg</span>
              )}
            </div>
            <div className="card-body space-y-4">
              {!ffmpegAvailable && !ffmpegInstalling && (
                <div className="bg-earth-900/30 border border-earth-700/40 rounded-lg px-3 py-2 flex items-center justify-between">
                  <div>
                    <p className="text-sm text-earth-300">FFmpeg required for trimming</p>
                    {ffmpegInstallError && <p className="text-xs text-earth-400 mt-0.5">{ffmpegInstallError}</p>}
                  </div>
                  <button className="btn-secondary text-xs px-3 py-1.5" onClick={handleInstallFFmpeg}>
                    Install
                  </button>
                </div>
              )}
              {ffmpegInstalling && (
                <div className="bg-forest-900/60 border border-forest-700/40 rounded-lg px-3 py-2 space-y-1">
                  <p className="text-sm text-sand-300">{ffmpegInstallStage}</p>
                  <div className="h-1.5 bg-stone-700 rounded-full overflow-hidden">
                    <div className="h-full bg-forest-500 transition-all duration-300" style={{ width: `${ffmpegInstallPct}%` }} />
                  </div>
                </div>
              )}
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
                <div className="grid grid-cols-2 gap-x-4 gap-y-1 text-sm text-sand-500">
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
            <button onClick={() => { abortRef.current = true; reset() }} className="btn-ghost inline-flex items-center space-x-2 text-earth-500 hover:text-earth-400">
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