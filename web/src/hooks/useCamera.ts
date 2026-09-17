import { useCallback, useEffect, useRef, useState } from 'react'

/** Which camera to use: `user` is front-facing, `environment` is rear. */
export type Facing = 'user' | 'environment'

/** Lifecycle of the camera stream as seen by the UI. */
export type CameraStatus =
  | 'idle' // not requested yet
  | 'starting' // permission prompt or stream negotiation in progress
  | 'ready' // frames are flowing
  | 'denied' // the person refused permission
  | 'unavailable' // no camera, or insecure context
  | 'error' // anything else

export interface CameraState {
  status: CameraStatus
  facing: Facing
  /** True when more than one camera exists, so a flip button makes sense. */
  canFlip: boolean
  /** Human-readable explanation for denied/unavailable/error states. */
  message?: string
}

export interface UseCameraOptions {
  /** Start the stream only while true; the stream stops when it turns false. */
  enabled: boolean
  /** Initial camera. Defaults to the rear camera on touch devices. */
  initialFacing?: Facing
}

/**
 * Picks a sensible default camera: phones and tablets usually want the rear
 * camera to photograph a scene, laptops only have a front camera.
 */
export function defaultFacing(): Facing {
  if (typeof window === 'undefined') return 'user'
  const coarse = window.matchMedia?.('(pointer: coarse)').matches ?? false
  const uaMobile = (navigator as Navigator & { userAgentData?: { mobile?: boolean } }).userAgentData
    ?.mobile
  return coarse || uaMobile ? 'environment' : 'user'
}

/**
 * Manages a getUserMedia stream bound to a video element.
 *
 * Returns the video ref to attach, the current state, and controls. The hook
 * owns the stream lifecycle: it stops tracks when disabled, when the facing
 * mode changes and on unmount, so the camera light never stays on.
 */
export function useCamera({ enabled, initialFacing }: UseCameraOptions) {
  const videoRef = useRef<HTMLVideoElement | null>(null)
  const streamRef = useRef<MediaStream | null>(null)
  const [facing, setFacing] = useState<Facing>(() => initialFacing ?? defaultFacing())
  // The active effect's start routine, so retry() can re-run it in place.
  const startRef = useRef<(() => Promise<void>) | null>(null)
  const [state, setState] = useState<CameraState>({
    status: 'idle',
    facing: initialFacing ?? defaultFacing(),
    canFlip: false,
  })

  /** Stops every track and detaches the stream from the video element. */
  const stop = useCallback(() => {
    streamRef.current?.getTracks().forEach((track) => track.stop())
    streamRef.current = null
    if (videoRef.current) videoRef.current.srcObject = null
  }, [])

  useEffect(() => {
    if (!enabled) {
      stop()
      return
    }

    let cancelled = false

    /** Requests the stream, retrying without facingMode when it is rejected. */
    async function start() {
      // Synchronising with the camera hardware is exactly what effects are
      // for; this setState only reflects that a request is now in flight.
      // oxlint-disable-next-line react/set-state-in-effect
      setState((s) => ({ ...s, status: 'starting', facing, message: undefined }))

      if (!navigator.mediaDevices?.getUserMedia) {
        setState({
          status: 'unavailable',
          facing,
          canFlip: false,
          message: window.isSecureContext
            ? 'This browser does not support camera access.'
            : 'Camera access needs HTTPS. Open the app on localhost or run `make dev-https`.',
        })
        return
      }

      try {
        let stream: MediaStream
        try {
          stream = await navigator.mediaDevices.getUserMedia({
            video: { facingMode: { ideal: facing }, ...idealResolution() },
            audio: false,
          })
        } catch (err) {
          // Some devices reject facingMode entirely; fall back to any camera.
          if (isConstraintError(err)) {
            stream = await navigator.mediaDevices.getUserMedia({ video: true, audio: false })
          } else {
            throw err
          }
        }

        if (cancelled) {
          stream.getTracks().forEach((t) => t.stop())
          return
        }

        stop()
        streamRef.current = stream
        const video = videoRef.current
        if (video) {
          video.srcObject = stream
          // play() can reject on autoplay-restricted browsers; muted+playsInline
          // avoids that, but guard anyway so a rejection never surfaces as a crash.
          await video.play().catch(() => undefined)
        }

        const canFlip = await countVideoInputs()
          .then((n) => n > 1)
          .catch(() => false)
        if (!cancelled) setState({ status: 'ready', facing, canFlip })
      } catch (err) {
        if (cancelled) return
        setState({
          status: classifyError(err),
          facing,
          canFlip: false,
          message: describeError(err),
        })
      }
    }

    startRef.current = start
    void start()

    return () => {
      cancelled = true
      startRef.current = null
      stop()
    }
  }, [enabled, facing, stop])

  /** Switches between front and rear cameras. */
  const flip = useCallback(() => {
    setFacing((f) => (f === 'user' ? 'environment' : 'user'))
  }, [])

  /** Re-runs the permission flow, e.g. after the person changes settings. */
  const retry = useCallback(() => {
    void startRef.current?.()
  }, [])

  // While disabled the stream is stopped, so report idle without touching state.
  const visibleState: CameraState = enabled ? state : { ...state, status: 'idle' }

  return { videoRef, state: visibleState, flip, retry }
}

/**
 * Asks for a full-HD stream oriented like the screen, so a phone held
 * upright gets a portrait feed instead of a letterboxed landscape one.
 */
function idealResolution(): MediaTrackConstraints {
  const portrait = window.innerHeight > window.innerWidth
  return portrait
    ? { width: { ideal: 1080 }, height: { ideal: 1920 } }
    : { width: { ideal: 1920 }, height: { ideal: 1080 } }
}

/** Counts attached cameras. Labels are empty before permission, but counts are reliable. */
async function countVideoInputs(): Promise<number> {
  const devices = await navigator.mediaDevices.enumerateDevices()
  return devices.filter((d) => d.kind === 'videoinput').length
}

/** True for errors that mean "these constraints cannot be satisfied". */
function isConstraintError(err: unknown): boolean {
  return (
    err instanceof DOMException &&
    (err.name === 'OverconstrainedError' || err.name === 'NotFoundError')
  )
}

/** Maps a getUserMedia error onto a CameraStatus. */
function classifyError(err: unknown): CameraStatus {
  if (!(err instanceof DOMException)) return 'error'
  switch (err.name) {
    case 'NotAllowedError':
    case 'SecurityError':
      return 'denied'
    case 'NotFoundError':
    case 'OverconstrainedError':
      return 'unavailable'
    default:
      return 'error'
  }
}

/** Produces a sentence a person can act on for each error class. */
function describeError(err: unknown): string {
  if (err instanceof DOMException) {
    switch (err.name) {
      case 'NotAllowedError':
      case 'SecurityError':
        return 'Camera permission was denied. Allow it in your browser settings, or upload a photo instead.'
      case 'NotFoundError':
      case 'OverconstrainedError':
        return 'No camera was found on this device. You can upload a photo instead.'
      case 'NotReadableError':
        return 'The camera is busy in another app. Close it and try again.'
      default:
        return `The camera could not start (${err.name}). You can upload a photo instead.`
    }
  }
  return 'The camera could not start. You can upload a photo instead.'
}
