import type { ApiErrorBody, Health, TransformResult } from './types'

/**
 * Error thrown by the API client. `code` is the stable machine-readable
 * identifier from the server; `message` is safe to show to people.
 */
export class ApiError extends Error {
  readonly code: string
  readonly status: number

  constructor(code: string, message: string, status: number) {
    super(message)
    this.name = 'ApiError'
    this.code = code
    this.status = status
  }
}

/**
 * Reads the JSON error envelope from a failed response, falling back to a
 * generic message when the body is not JSON (e.g. a proxy error page).
 */
async function toApiError(response: Response): Promise<ApiError> {
  try {
    const body = (await response.json()) as ApiErrorBody
    if (body?.error?.message) {
      return new ApiError(body.error.code, body.error.message, response.status)
    }
  } catch {
    // Not JSON; fall through to the generic error below.
  }
  return new ApiError(
    'http_error',
    `The server responded with ${response.status}.`,
    response.status,
  )
}

/**
 * Fetches server readiness. Throws ApiError when the server is unreachable
 * so the UI can explain that the Go process is not running.
 */
export async function fetchHealth(signal?: AbortSignal): Promise<Health> {
  let response: Response
  try {
    response = await fetch('/api/health', { signal })
  } catch (err) {
    if (isAbort(err)) throw err
    throw new ApiError(
      'unreachable',
      'Cannot reach the server. Start it with `make dev` and reload.',
      0,
    )
  }
  if (!response.ok) throw await toApiError(response)
  return (await response.json()) as Health
}

/**
 * Uploads a captured photo and returns its parallel-universe rendering.
 * The request can be cancelled with the AbortSignal; the server stops the
 * upstream model calls when the connection drops.
 */
export async function transformPhoto(photo: Blob, signal?: AbortSignal): Promise<TransformResult> {
  const form = new FormData()
  form.append('image', photo, photo.type === 'image/png' ? 'capture.png' : 'capture.jpg')

  let response: Response
  try {
    response = await fetch('/api/transform', { method: 'POST', body: form, signal })
  } catch (err) {
    if (isAbort(err)) throw err
    throw new ApiError('unreachable', 'Lost contact with the server while uploading.', 0)
  }
  if (!response.ok) throw await toApiError(response)
  return (await response.json()) as TransformResult
}

/** True when the error came from an aborted fetch. */
export function isAbort(err: unknown): boolean {
  return err instanceof DOMException && err.name === 'AbortError'
}
