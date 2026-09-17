import { describe, expect, it } from 'vitest'
import { downloadName, fitWithin, formatDuration } from './image'

describe('fitWithin', () => {
  it('returns the size unchanged when already within the limit', () => {
    expect(fitWithin({ width: 800, height: 600 }, 1536)).toEqual({ width: 800, height: 600 })
  })

  it('scales the longest edge down and preserves aspect ratio', () => {
    expect(fitWithin({ width: 4000, height: 3000 }, 1536)).toEqual({ width: 1536, height: 1152 })
    expect(fitWithin({ width: 1080, height: 1920 }, 1536)).toEqual({ width: 864, height: 1536 })
  })

  it('never produces a zero-sized edge', () => {
    expect(fitWithin({ width: 10000, height: 1 }, 100)).toEqual({ width: 100, height: 1 })
    expect(fitWithin({ width: 0, height: 0 }, 100)).toEqual({ width: 0, height: 0 })
  })
})

describe('formatDuration', () => {
  it('uses milliseconds below one second', () => {
    expect(formatDuration(0)).toBe('0ms')
    expect(formatDuration(849.6)).toBe('850ms')
  })

  it('uses one decimal of seconds otherwise', () => {
    expect(formatDuration(1000)).toBe('1.0s')
    expect(formatDuration(12_345)).toBe('12.3s')
  })
})

describe('downloadName', () => {
  it('slugifies the universe name', () => {
    expect(downloadName('Infernal Court')).toBe('multiverse-infernal-court.jpg')
    expect(downloadName('  Drowned  Pantheon!! ', 'png')).toBe('multiverse-drowned-pantheon.png')
  })

  it('falls back when the name has no usable characters', () => {
    expect(downloadName('✨✨')).toBe('multiverse-photo.jpg')
  })
})
