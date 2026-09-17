/**
 * Types shared between the API client and the UI. They mirror the JSON
 * shapes produced by the Go server in `server/internal/httpapi`.
 */

/** Server readiness and active configuration, from `GET /api/health`. */
export interface Health {
  status: 'ok'
  /** True when an OpenAI key is present or mock mode is on. */
  configured: boolean
  /** True when the server returns placeholder results without calling OpenAI. */
  mock: boolean
  vision_model: string
  image_model: string
  image_quality: 'low' | 'medium' | 'high'
  max_upload_mb: number
}

/** What the vision model observed and designed. */
export interface Plan {
  subject: string
  action: string
  held_object: string
  setting: string
  universe_name: string
  universe_description: string
  new_identity: string
  mirrored_object: string
  new_setting: string
  visual_style: string
  caption: string
  edit_prompt: string
}

/** Human-facing summary of the chosen universe. */
export interface UniverseSummary {
  name: string
  description: string
  identity: string
  object: string
  setting: string
  style: string
}

/** Per-stage latency in milliseconds. */
export interface Timings {
  analyze: number
  render: number
  total: number
}

/** Successful response from `POST /api/transform`. */
export interface TransformResult {
  id: string
  plan: Plan
  image: {
    data_url: string
    mime_type: string
  }
  timings_ms: Timings
  mock: boolean
  universe: UniverseSummary
}

/** Error envelope returned by every failing API call. */
export interface ApiErrorBody {
  error: {
    code: string
    message: string
  }
}
