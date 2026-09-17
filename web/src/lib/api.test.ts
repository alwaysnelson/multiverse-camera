import { afterEach, describe, expect, it, vi } from 'vitest'
import { ApiError, fetchHealth, transformPhoto } from './api'

/** Builds a minimal Response-like object for fetch mocks. */
function jsonResponse(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  })
}

afterEach(() => {
  vi.restoreAllMocks()
})

describe('fetchHealth', () => {
  it('returns the parsed health payload', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue(jsonResponse({ status: 'ok', configured: true })),
    )
    await expect(fetchHealth()).resolves.toMatchObject({ status: 'ok', configured: true })
  })

  it('maps a network failure to an "unreachable" ApiError', async () => {
    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new TypeError('Failed to fetch')))
    await expect(fetchHealth()).rejects.toMatchObject({ code: 'unreachable', status: 0 })
  })

  it('re-throws aborts untouched so callers can ignore them', async () => {
    const abort = new DOMException('aborted', 'AbortError')
    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(abort))
    await expect(fetchHealth()).rejects.toBe(abort)
  })
})

describe('transformPhoto', () => {
  it('posts the photo as multipart and returns the result', async () => {
    const fetchMock = vi.fn().mockResolvedValue(jsonResponse({ id: 'x', universe: { name: 'Y' } }))
    vi.stubGlobal('fetch', fetchMock)

    const blob = new Blob(['jpeg'], { type: 'image/jpeg' })
    const result = await transformPhoto(blob)

    expect(result.id).toBe('x')
    const [url, init] = fetchMock.mock.calls[0] as [string, RequestInit]
    expect(url).toBe('/api/transform')
    expect(init.method).toBe('POST')
    const form = init.body as FormData
    const file = form.get('image') as File
    expect(file.name).toBe('capture.jpg')
  })

  it('surfaces the server error envelope as an ApiError', async () => {
    vi.stubGlobal(
      'fetch',
      vi
        .fn()
        .mockResolvedValue(jsonResponse({ error: { code: 'moderated', message: 'nope' } }, 422)),
    )
    const err = await transformPhoto(new Blob()).catch((e: unknown) => e)
    expect(err).toBeInstanceOf(ApiError)
    expect(err).toMatchObject({ code: 'moderated', message: 'nope', status: 422 })
  })

  it('falls back to a generic error when the body is not JSON', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue(new Response('<html>502</html>', { status: 502 })),
    )
    await expect(transformPhoto(new Blob())).rejects.toMatchObject({
      code: 'http_error',
      status: 502,
    })
  })
})
