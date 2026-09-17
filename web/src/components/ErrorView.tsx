import { AlertIcon, CameraIcon, RetryIcon } from './icons'

interface ErrorViewProps {
  title: string
  message: string
  /** Object URL of the photo that failed, so people see what they are retrying. */
  photoUrl?: string
  onRetry: () => void
  onNewPhoto: () => void
}

/**
 * Full-width failure state. Every error offers the same two exits, so the
 * person never has to guess what to do next.
 */
export function ErrorView({ title, message, photoUrl, onRetry, onNewPhoto }: ErrorViewProps) {
  return (
    <section className="error" role="alert">
      {photoUrl && <img src={photoUrl} alt="" className="error__thumb" />}
      <AlertIcon className="error__icon" />
      <h2 className="error__title">{title}</h2>
      <p className="error__message">{message}</p>
      <div className="actions">
        <button type="button" className="btn btn--primary" onClick={onRetry}>
          <RetryIcon /> Try again
        </button>
        <button type="button" className="btn btn--secondary" onClick={onNewPhoto}>
          <CameraIcon /> New photo
        </button>
      </div>
    </section>
  )
}
