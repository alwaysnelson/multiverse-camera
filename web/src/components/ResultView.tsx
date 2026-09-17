import { useState } from 'react'
import { downloadDataUrl, downloadName, formatDuration } from '../lib/image'
import type { TransformResult } from '../lib/types'
import { CompareSlider } from './CompareSlider'
import { CameraIcon, DownloadIcon, SparklesIcon } from './icons'

interface ResultViewProps {
  originalUrl: string
  result: TransformResult
  onAnotherUniverse: () => void
  onNewPhoto: () => void
}

/**
 * Presents the comparison, the story of the chosen universe, and the three
 * follow-up actions. "Another universe" re-rolls the same photo, which is
 * the fastest way to get a better result when the model drifted.
 */
export function ResultView({
  originalUrl,
  result,
  onAnotherUniverse,
  onNewPhoto,
}: ResultViewProps) {
  const { universe, plan, image, timings_ms: timings } = result
  // Aspect ratio of the rendered image; measured on load so the frame is exact.
  const [ratio, setRatio] = useState(1)

  return (
    <section className="result" aria-label="Result">
      {/* Hidden probe to measure the rendered image's natural size. */}
      <img
        src={image.data_url}
        alt=""
        aria-hidden="true"
        className="visually-hidden"
        onLoad={(e) => {
          const img = e.currentTarget
          if (img.naturalWidth && img.naturalHeight) setRatio(img.naturalWidth / img.naturalHeight)
        }}
      />

      <header className="result__header">
        <span className="result__universe">{universe.name}</span>
        <h2 className="result__caption">{plan.caption}</h2>
      </header>

      <CompareSlider
        beforeSrc={originalUrl}
        afterSrc={image.data_url}
        beforeLabel="This universe"
        afterLabel={universe.name}
        ratio={ratio}
      />

      <div className="actions">
        <button type="button" className="btn btn--primary" onClick={onAnotherUniverse}>
          <SparklesIcon /> Another universe
        </button>
        <button
          type="button"
          className="btn btn--secondary"
          onClick={() => downloadDataUrl(image.data_url, downloadName(universe.name))}
        >
          <DownloadIcon /> Download
        </button>
        <button type="button" className="btn btn--secondary" onClick={onNewPhoto}>
          <CameraIcon /> New photo
        </button>
      </div>

      <div className="story">
        <p className="story__lead">{universe.description}</p>
        <dl className="story__grid">
          <div className="story__row">
            <dt>Here</dt>
            <dd>
              {plan.subject}, {plan.action}
              {plan.held_object && plan.held_object !== 'nothing'
                ? `, holding ${plan.held_object}`
                : ''}
              {plan.setting ? `, ${plan.setting}` : ''}.
            </dd>
          </div>
          <div className="story__row">
            <dt>There</dt>
            <dd>{universe.identity}</dd>
          </div>
          {universe.object && universe.object !== 'nothing' && (
            <div className="story__row">
              <dt>In hand</dt>
              <dd>{universe.object}</dd>
            </div>
          )}
          <div className="story__row">
            <dt>Setting</dt>
            <dd>{universe.setting}</dd>
          </div>
          <div className="story__row">
            <dt>Style</dt>
            <dd>{universe.style}</dd>
          </div>
        </dl>
        <p className="story__meta">
          {result.mock ? 'Mock result · ' : ''}
          analysed in {formatDuration(timings.analyze)}, rendered in{' '}
          {formatDuration(timings.render)}
        </p>
      </div>
    </section>
  )
}
