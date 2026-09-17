/**
 * Image helpers for capturing a frame from the camera, downscaling uploads
 * and triggering downloads. All functions are browser-only except the pure
 * helpers at the bottom, which are unit tested.
 */

/** Longest edge sent to the server. Larger inputs add cost, not quality. */
export const MAX_CAPTURE_EDGE = 1536

/** JPEG quality for captures: high enough to keep faces, small enough to upload fast. */
const CAPTURE_QUALITY = 0.92

export interface CaptureOptions {
  /** Flip horizontally so a selfie matches what the preview showed. */
  mirror?: boolean
  /** Maximum longest edge in pixels. Defaults to MAX_CAPTURE_EDGE. */
  maxEdge?: number
}

/**
 * Grabs the current frame of a playing video element as a JPEG blob.
 * Throws if the video has no dimensions yet (stream not ready).
 */
export async function captureFrame(
  video: HTMLVideoElement,
  options: CaptureOptions = {},
): Promise<Blob> {
  const { mirror = false, maxEdge = MAX_CAPTURE_EDGE } = options
  const source = { width: video.videoWidth, height: video.videoHeight }
  if (source.width === 0 || source.height === 0) {
    throw new Error('The camera has not produced a frame yet.')
  }

  const target = fitWithin(source, maxEdge)
  const canvas = document.createElement('canvas')
  canvas.width = target.width
  canvas.height = target.height

  const ctx = canvas.getContext('2d')
  if (!ctx) throw new Error('Canvas 2D is not available in this browser.')

  if (mirror) {
    ctx.translate(target.width, 0)
    ctx.scale(-1, 1)
  }
  ctx.drawImage(video, 0, 0, target.width, target.height)

  return canvasToBlob(canvas, 'image/jpeg', CAPTURE_QUALITY)
}

/**
 * Loads a user-selected file, applies EXIF orientation via the browser's
 * decoder, and downscales it to the capture limit. HEIC and other formats
 * the browser cannot decode are rejected with a readable error.
 */
export async function prepareUpload(file: File, maxEdge = MAX_CAPTURE_EDGE): Promise<Blob> {
  let bitmap: ImageBitmap
  try {
    bitmap = await createImageBitmap(file, { imageOrientation: 'from-image' })
  } catch {
    throw new Error('That file could not be read as an image. Try a JPEG or PNG.')
  }

  try {
    const target = fitWithin({ width: bitmap.width, height: bitmap.height }, maxEdge)
    const canvas = document.createElement('canvas')
    canvas.width = target.width
    canvas.height = target.height
    const ctx = canvas.getContext('2d')
    if (!ctx) throw new Error('Canvas 2D is not available in this browser.')
    ctx.drawImage(bitmap, 0, 0, target.width, target.height)
    return await canvasToBlob(canvas, 'image/jpeg', CAPTURE_QUALITY)
  } finally {
    bitmap.close()
  }
}

/** Promise wrapper around the callback-based canvas.toBlob. */
function canvasToBlob(canvas: HTMLCanvasElement, type: string, quality: number): Promise<Blob> {
  return new Promise((resolve, reject) => {
    canvas.toBlob(
      (blob) => (blob ? resolve(blob) : reject(new Error('Could not encode the photo.'))),
      type,
      quality,
    )
  })
}

/**
 * Triggers a browser download of a data URL. Uses a temporary anchor, which
 * works on desktop and Android; iOS Safari opens the image in a new tab
 * where it can be saved with a long press.
 */
export function downloadDataUrl(dataUrl: string, filename: string): void {
  const link = document.createElement('a')
  link.href = dataUrl
  link.download = filename
  link.rel = 'noopener'
  document.body.appendChild(link)
  link.click()
  link.remove()
}

/* ---------- Pure helpers (unit tested) ---------- */

export interface Size {
  width: number
  height: number
}

/**
 * Scales a size down so its longest edge is at most `maxEdge`, preserving
 * aspect ratio. Sizes already within the limit are returned unchanged.
 */
export function fitWithin(size: Size, maxEdge: number): Size {
  const longest = Math.max(size.width, size.height)
  if (longest <= maxEdge || longest === 0) return { ...size }
  const scale = maxEdge / longest
  return {
    width: Math.max(1, Math.round(size.width * scale)),
    height: Math.max(1, Math.round(size.height * scale)),
  }
}

/** Formats a millisecond duration for display, e.g. "12.4s" or "850ms". */
export function formatDuration(ms: number): string {
  if (ms < 1000) return `${Math.round(ms)}ms`
  return `${(ms / 1000).toFixed(1)}s`
}

/** Builds a filesystem-friendly download name from a universe title. */
export function downloadName(universe: string, ext = 'jpg'): string {
  const slug = universe
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/(^-|-$)/g, '')
  return `multiverse-${slug || 'photo'}.${ext}`
}
