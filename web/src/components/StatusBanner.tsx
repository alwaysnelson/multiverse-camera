import type { ReactNode } from 'react'
import { AlertIcon } from './icons'

export type BannerTone = 'info' | 'warning' | 'error'

interface StatusBannerProps {
  tone: BannerTone
  title: string
  children?: ReactNode
  /** Optional action rendered on the right, e.g. a retry button. */
  action?: ReactNode
}

/**
 * Persistent message strip under the header. Used for "server not
 * running", "no API key" and "mock mode" so people always know why a
 * result looks the way it does.
 */
export function StatusBanner({ tone, title, children, action }: StatusBannerProps) {
  return (
    <div className={`banner banner--${tone}`} role={tone === 'error' ? 'alert' : 'status'}>
      <AlertIcon className="banner__icon" />
      <div className="banner__body">
        <strong className="banner__title">{title}</strong>
        {children && <div className="banner__text">{children}</div>}
      </div>
      {action && <div className="banner__action">{action}</div>}
    </div>
  )
}
