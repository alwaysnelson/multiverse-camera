import { useEffect, useState } from 'react'
import { formatDuration } from '../lib/image'
import { CloseIcon } from './icons'

interface ProcessingViewProps {
  /** Object URL of the captured photo, shown dimmed behind the progress. */
  photoUrl: string
  onCancel: () => void
}

/**
 * Stage messages shown while the request is in flight. The pipeline is a
 * single HTTP call, so these advance on a timer that roughly matches real
 * latency: analysis is fast, rendering is slow.
 */
const STAGES: { at: number; label: string }[] = [
  { at: 0, label: 'Reading the scene' },
  { at: 6_000, label: 'Choosing a universe' },
  { at: 12_000, label: 'Rendering the parallel world' },
  { at: 45_000, label: 'Still rendering. Detailed worlds take a minute' },
]

/**
 * Shown between shutter and result. Keeps the photo visible so the wait
 * feels anchored, shows elapsed time honestly, and always offers cancel.
 */
export function ProcessingView({ photoUrl, onCancel }: ProcessingViewProps) {
  const [elapsed, setElapsed] = useState(0)

  useEffect(() => {
    const started = Date.now()
    const id = window.setInterval(() => setElapsed(Date.now() - started), 250)
    return () => window.clearInterval(id)
  }, [])

  const stage = STAGES.reduce((current, s) => (elapsed >= s.at ? s : current), STAGES[0])

  return (
    <section className="processing" aria-live="polite" aria-busy="true">
      <div className="processing__frame">
        <img src={photoUrl} alt="Your photo, being transformed" className="processing__photo" />
        <div className="processing__scan" aria-hidden="true" />
      </div>
      <div className="processing__status">
        <span className="spinner" aria-hidden="true" />
        <div>
          <p className="processing__stage">{stage.label}…</p>
          <p className="processing__elapsed">{formatDuration(elapsed)}</p>
        </div>
        <button type="button" className="btn btn--ghost" onClick={onCancel}>
          <CloseIcon /> Cancel
        </button>
      </div>
    </section>
  )
}
