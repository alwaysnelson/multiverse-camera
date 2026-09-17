import { useCallback, useRef, useState, type ChangeEvent } from 'react'
import { useCamera } from '../hooks/useCamera'
import { captureFrame, prepareUpload } from '../lib/image'
import { AlertIcon, FlipIcon, RetryIcon, UploadIcon } from './icons'

interface CameraStageProps {
  /** Called with the captured or uploaded photo, ready to upload. */
  onPhoto: (photo: Blob) => void
  /** Disables the shutter (e.g. server unconfigured) with a reason shown as a tooltip. */
  disabledReason?: string
  /** Surfaces capture/upload failures to the parent's error handling. */
  onError: (message: string) => void
}

/**
 * Live camera preview with shutter, flip and upload controls. Owns the
 * camera stream through useCamera and falls back to file upload whenever
 * the camera cannot be used, so nobody hits a dead end.
 */
export function CameraStage({ onPhoto, disabledReason, onError }: CameraStageProps) {
  const { videoRef, state, flip, retry } = useCamera({ enabled: true })
  const fileInputRef = useRef<HTMLInputElement | null>(null)
  const [busy, setBusy] = useState(false)
  // Aspect ratio of the live stream so the frame never letterboxes oddly.
  const [ratio, setRatio] = useState<number | null>(null)

  const mirrored = state.facing === 'user'
  const cameraReady = state.status === 'ready'
  const cameraFailed =
    state.status === 'denied' || state.status === 'unavailable' || state.status === 'error'

  /** Reads the stream's native size once metadata arrives. */
  const handleLoadedMetadata = useCallback(() => {
    const v = videoRef.current
    if (v && v.videoWidth && v.videoHeight) setRatio(v.videoWidth / v.videoHeight)
  }, [videoRef])

  /** Captures the current frame and hands it to the parent. */
  const handleShutter = useCallback(async () => {
    const video = videoRef.current
    if (!video || busy) return
    setBusy(true)
    try {
      const photo = await captureFrame(video, { mirror: mirrored })
      onPhoto(photo)
    } catch (err) {
      onError(err instanceof Error ? err.message : 'Could not capture the photo.')
    } finally {
      setBusy(false)
    }
  }, [videoRef, busy, mirrored, onPhoto, onError])

  /** Handles a file chosen from the upload fallback. */
  const handleFile = useCallback(
    async (event: ChangeEvent<HTMLInputElement>) => {
      const file = event.target.files?.[0]
      // Reset so choosing the same file twice still fires change.
      event.target.value = ''
      if (!file) return
      setBusy(true)
      try {
        onPhoto(await prepareUpload(file))
      } catch (err) {
        onError(err instanceof Error ? err.message : 'Could not read that file.')
      } finally {
        setBusy(false)
      }
    },
    [onPhoto, onError],
  )

  const shutterDisabled = !cameraReady || busy || Boolean(disabledReason)

  return (
    <section className="stage" aria-label="Camera">
      <div
        className={`stage__frame${cameraFailed ? ' stage__frame--fallback' : ''}`}
        style={ratio ? { aspectRatio: String(ratio) } : undefined}
      >
        <video
          ref={videoRef}
          className={`stage__video${mirrored ? ' stage__video--mirrored' : ''}`}
          autoPlay
          muted
          playsInline
          onLoadedMetadata={handleLoadedMetadata}
        />

        {state.status === 'starting' && (
          <div className="stage__overlay">
            <span className="spinner" aria-hidden="true" />
            <p>Starting camera…</p>
          </div>
        )}

        {cameraFailed && (
          <div className="stage__overlay stage__overlay--solid">
            <AlertIcon className="stage__overlay-icon" />
            <p className="stage__overlay-text">{state.message}</p>
            <div className="stage__overlay-actions">
              <button type="button" className="btn btn--secondary" onClick={retry}>
                <RetryIcon /> Try camera again
              </button>
              <button
                type="button"
                className="btn btn--primary"
                onClick={() => fileInputRef.current?.click()}
              >
                <UploadIcon /> Upload a photo
              </button>
            </div>
          </div>
        )}

        {cameraReady && !disabledReason && (
          <p className="stage__hint">Frame one subject. Pose and framing carry over.</p>
        )}
      </div>

      <div className="controls">
        <button
          type="button"
          className="btn btn--icon"
          onClick={() => fileInputRef.current?.click()}
          disabled={busy}
          aria-label="Upload a photo"
          title="Upload a photo"
        >
          <UploadIcon />
        </button>

        <button
          type="button"
          className="shutter"
          onClick={handleShutter}
          disabled={shutterDisabled}
          aria-label="Take photo"
          title={disabledReason ?? 'Take photo'}
        >
          <span className="shutter__inner" />
        </button>

        <button
          type="button"
          className="btn btn--icon"
          onClick={flip}
          disabled={!state.canFlip || busy}
          aria-label="Switch camera"
          title={state.canFlip ? 'Switch camera' : 'Only one camera detected'}
        >
          <FlipIcon />
        </button>
      </div>

      <input
        ref={fileInputRef}
        type="file"
        accept="image/jpeg,image/png,image/webp"
        className="visually-hidden"
        onChange={handleFile}
        tabIndex={-1}
        aria-hidden="true"
      />
    </section>
  )
}
