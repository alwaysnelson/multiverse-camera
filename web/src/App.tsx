import { useCallback, useEffect, useRef, useState } from 'react'
import { AppHeader } from './components/AppHeader'
import { CameraStage } from './components/CameraStage'
import { ErrorView } from './components/ErrorView'
import { ProcessingView } from './components/ProcessingView'
import { ResultView } from './components/ResultView'
import { StatusBanner } from './components/StatusBanner'
import { ApiError, fetchHealth, isAbort, transformPhoto } from './lib/api'
import type { Health, TransformResult } from './lib/types'

/** A captured photo plus the object URL used to display it. */
interface Photo {
  blob: Blob
  url: string
}

/**
 * The app is a small state machine. Every phase carries exactly the data it
 * needs, which keeps impossible states (a result without a photo) out.
 */
type Phase =
  | { kind: 'camera' }
  | { kind: 'processing'; photo: Photo; controller: AbortController }
  | { kind: 'result'; photo: Photo; result: TransformResult }
  | { kind: 'error'; photo?: Photo; title: string; message: string }

/** Health check state, separate from the phase so it can update independently. */
type ServerState = { status: 'loading' } | { status: 'ok'; health: Health } | { status: 'down' }

/** Root component: wires the health check, the phase machine and the views. */
export default function App() {
  const [phase, setPhase] = useState<Phase>({ kind: 'camera' })
  const [server, setServer] = useState<ServerState>({ status: 'loading' })
  // Tracks the latest photo URL so it can be revoked exactly once.
  const lastUrlRef = useRef<string | null>(null)

  /** Polls health once on load and again whenever the server was unreachable. */
  const checkHealth = useCallback(async (signal?: AbortSignal) => {
    try {
      const health = await fetchHealth(signal)
      setServer({ status: 'ok', health })
    } catch (err) {
      if (!isAbort(err)) setServer({ status: 'down' })
    }
  }, [])

  useEffect(() => {
    const controller = new AbortController()
    // State is only set after the server answers, never synchronously.
    // oxlint-disable-next-line react/set-state-in-effect
    void checkHealth(controller.signal)
    return () => controller.abort()
  }, [checkHealth])

  /** Releases the object URL of a photo that is no longer displayed. */
  const releasePhoto = useCallback(() => {
    if (lastUrlRef.current) {
      URL.revokeObjectURL(lastUrlRef.current)
      lastUrlRef.current = null
    }
  }, [])

  /** Sends a photo to the server and moves through processing → result/error. */
  const runTransform = useCallback(async (photo: Photo) => {
    const controller = new AbortController()
    setPhase({ kind: 'processing', photo, controller })
    try {
      const result = await transformPhoto(photo.blob, controller.signal)
      setPhase({ kind: 'result', photo, result })
    } catch (err) {
      if (isAbort(err)) return // cancel already moved the phase
      const { title, message } = describeError(err)
      setPhase({ kind: 'error', photo, title, message })
    }
  }, [])

  /** Receives a fresh capture or upload from the camera stage. */
  const handlePhoto = useCallback(
    (blob: Blob) => {
      releasePhoto()
      const url = URL.createObjectURL(blob)
      lastUrlRef.current = url
      void runTransform({ blob, url })
    },
    [releasePhoto, runTransform],
  )

  /** Aborts an in-flight transform and returns to the camera. */
  const handleCancel = useCallback(() => {
    if (phase.kind === 'processing') phase.controller.abort()
    releasePhoto()
    setPhase({ kind: 'camera' })
  }, [phase, releasePhoto])

  /** Re-runs the same photo; a new universe is drawn server-side each time. */
  const handleRetry = useCallback(() => {
    if (phase.kind === 'result' || (phase.kind === 'error' && phase.photo)) {
      void runTransform(phase.photo as Photo)
    } else {
      setPhase({ kind: 'camera' })
    }
  }, [phase, runTransform])

  /** Discards the current photo and returns to the camera. */
  const handleNewPhoto = useCallback(() => {
    releasePhoto()
    setPhase({ kind: 'camera' })
  }, [releasePhoto])

  /** Shows capture-side failures (canvas, unreadable file) in the error view. */
  const handleCaptureError = useCallback((message: string) => {
    setPhase({ kind: 'error', title: 'Could not use that photo', message })
  }, [])

  // Revoke the last object URL when the app unmounts.
  useEffect(() => releasePhoto, [releasePhoto])

  const shutterDisabledReason = disabledReason(server)

  return (
    <div className="app">
      <AppHeader
        health={server.status === 'ok' ? server.health : undefined}
        offline={server.status === 'down'}
      />

      <main className="app__main">
        <ServerBanner server={server} onRetry={() => void checkHealth()} />

        {phase.kind === 'camera' && (
          <CameraStage
            onPhoto={handlePhoto}
            disabledReason={shutterDisabledReason}
            onError={handleCaptureError}
          />
        )}
        {phase.kind === 'processing' && (
          <ProcessingView photoUrl={phase.photo.url} onCancel={handleCancel} />
        )}
        {phase.kind === 'result' && (
          <ResultView
            originalUrl={phase.photo.url}
            result={phase.result}
            onAnotherUniverse={handleRetry}
            onNewPhoto={handleNewPhoto}
          />
        )}
        {phase.kind === 'error' && (
          <ErrorView
            title={phase.title}
            message={phase.message}
            photoUrl={phase.photo?.url}
            onRetry={handleRetry}
            onNewPhoto={handleNewPhoto}
          />
        )}
      </main>

      <footer className="app__footer">
        <p>Runs locally. Photos go to OpenAI for rendering and are never stored by this app.</p>
      </footer>
    </div>
  )
}

/** Banner explaining server state; returns null when everything is normal. */
function ServerBanner({ server, onRetry }: { server: ServerState; onRetry: () => void }) {
  if (server.status === 'down') {
    return (
      <StatusBanner
        tone="error"
        title="The server is not running"
        action={
          <button type="button" className="btn btn--ghost" onClick={onRetry}>
            Retry
          </button>
        }
      >
        Start it with <code>make dev</code> in the project folder, then retry.
      </StatusBanner>
    )
  }
  if (server.status === 'ok' && server.health.mock) {
    return (
      <StatusBanner tone="warning" title="Mock mode">
        The server returns placeholder results. Unset <code>MOCK_MODE</code> and add{' '}
        <code>OPENAI_API_KEY</code> to travel for real.
      </StatusBanner>
    )
  }
  if (server.status === 'ok' && !server.health.configured) {
    return (
      <StatusBanner tone="warning" title="No OpenAI API key">
        Add <code>OPENAI_API_KEY=sk-…</code> to the <code>.env</code> file and restart the server.
      </StatusBanner>
    )
  }
  return null
}

/** Why the shutter is disabled, or undefined when it should work. */
function disabledReason(server: ServerState): string | undefined {
  if (server.status === 'down') return 'The server is not running.'
  if (server.status === 'ok' && !server.health.configured) return 'Add an OpenAI API key first.'
  return undefined
}

/** Turns any thrown value into a title and message people can act on. */
function describeError(err: unknown): { title: string; message: string } {
  if (err instanceof ApiError) {
    const titles: Record<string, string> = {
      not_configured: 'No OpenAI API key',
      invalid_api_key: 'OpenAI rejected the API key',
      rate_limited: 'Slow down a little',
      moderated: 'This photo was declined',
      timeout: 'That took too long',
      too_large: 'Photo too large',
      unsupported_type: 'Unsupported image',
      unreachable: 'Lost the server',
    }
    return { title: titles[err.code] ?? 'Something went wrong', message: err.message }
  }
  if (err instanceof Error) return { title: 'Something went wrong', message: err.message }
  return { title: 'Something went wrong', message: 'An unknown error occurred.' }
}
