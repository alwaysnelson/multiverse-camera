import type { Health } from '../lib/types'
import { BrandMark } from './icons'

interface AppHeaderProps {
  /** Undefined while the health check is still in flight. */
  health?: Health
  /** True when the health check failed (server down). */
  offline: boolean
}

/**
 * Top bar with the brand and a single status chip. The chip is the quickest
 * answer to "why is nothing happening?" so it is always visible.
 */
export function AppHeader({ health, offline }: AppHeaderProps) {
  const chip = statusChip(health, offline)
  return (
    <header className="header">
      <div className="header__brand">
        <BrandMark className="header__mark" />
        <div>
          <h1 className="header__title">Multiverse Camera</h1>
          <p className="header__tagline">Same moment. Different existence.</p>
        </div>
      </div>
      <span className={`chip chip--${chip.tone}`} title={chip.detail}>
        <span className="chip__dot" aria-hidden="true" />
        {chip.label}
      </span>
    </header>
  )
}

/** Derives the chip label and colour from server state. */
function statusChip(health: Health | undefined, offline: boolean) {
  if (offline)
    return { tone: 'error', label: 'Server offline', detail: 'The Go server is not reachable.' }
  if (!health) return { tone: 'muted', label: 'Connecting', detail: 'Checking the server.' }
  if (health.mock)
    return { tone: 'warning', label: 'Mock mode', detail: 'Results are placeholders.' }
  if (!health.configured) {
    return { tone: 'warning', label: 'Needs API key', detail: 'Set OPENAI_API_KEY in .env.' }
  }
  return {
    tone: 'ok',
    label: 'Ready',
    detail: `${health.vision_model} + ${health.image_model} (${health.image_quality})`,
  }
}
