import { useCallback, useId, useState } from 'react'

interface CompareSliderProps {
  beforeSrc: string
  afterSrc: string
  beforeLabel: string
  afterLabel: string
  /** Aspect ratio of the frame; both images are cover-fitted into it. */
  ratio: number
}

type Mode = 'slider' | 'side'

/**
 * Before/after comparison. The slider mode uses a full-size transparent
 * range input as the drag surface, which gives mouse, touch and keyboard
 * control for free and keeps the widget accessible without custom pointer
 * maths. Side-by-side mode is offered for wide screens and screenshots.
 */
export function CompareSlider({
  beforeSrc,
  afterSrc,
  beforeLabel,
  afterLabel,
  ratio,
}: CompareSliderProps) {
  const [position, setPosition] = useState(50)
  const [mode, setMode] = useState<Mode>('slider')
  const sliderId = useId()

  const handleInput = useCallback((event: React.ChangeEvent<HTMLInputElement>) => {
    setPosition(Number(event.target.value))
  }, [])

  return (
    <div className="compare">
      <div className="compare__toolbar" role="group" aria-label="Comparison mode">
        <button
          type="button"
          className={`seg${mode === 'slider' ? ' seg--active' : ''}`}
          onClick={() => setMode('slider')}
          aria-pressed={mode === 'slider'}
        >
          Slider
        </button>
        <button
          type="button"
          className={`seg${mode === 'side' ? ' seg--active' : ''}`}
          onClick={() => setMode('side')}
          aria-pressed={mode === 'side'}
        >
          Side by side
        </button>
      </div>

      {mode === 'slider' ? (
        <div className="compare__frame" style={{ aspectRatio: String(ratio) }}>
          <img src={afterSrc} alt={afterLabel} className="compare__img" draggable={false} />
          <img
            src={beforeSrc}
            alt={beforeLabel}
            className="compare__img compare__img--before"
            style={{ clipPath: `inset(0 ${100 - position}% 0 0)` }}
            draggable={false}
          />
          <div className="compare__divider" style={{ left: `${position}%` }} aria-hidden="true">
            <span className="compare__handle" />
          </div>
          <span className="compare__label compare__label--left" aria-hidden="true">
            {beforeLabel}
          </span>
          <span className="compare__label compare__label--right" aria-hidden="true">
            {afterLabel}
          </span>
          <label htmlFor={sliderId} className="visually-hidden">
            Reveal original photo
          </label>
          <input
            id={sliderId}
            type="range"
            min={0}
            max={100}
            step={0.5}
            value={position}
            onChange={handleInput}
            className="compare__range"
            aria-valuetext={`${Math.round(position)}% original`}
          />
        </div>
      ) : (
        <div className="compare__grid">
          <figure className="compare__cell" style={{ aspectRatio: String(ratio) }}>
            <img src={beforeSrc} alt={beforeLabel} className="compare__img" />
            <figcaption className="compare__caption">{beforeLabel}</figcaption>
          </figure>
          <figure className="compare__cell" style={{ aspectRatio: String(ratio) }}>
            <img src={afterSrc} alt={afterLabel} className="compare__img" />
            <figcaption className="compare__caption">{afterLabel}</figcaption>
          </figure>
        </div>
      )}
    </div>
  )
}
