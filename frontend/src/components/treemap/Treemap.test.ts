import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render } from '@testing-library/svelte'
import { fireEvent } from '@testing-library/dom'
import { tick } from 'svelte'
import Treemap from './Treemap.svelte'
import { node } from '../../test/fixtures'

const ctx = {
  setTransform: vi.fn(), clearRect: vi.fn(), fillRect: vi.fn(), strokeRect: vi.fn(),
  fillText: vi.fn(), measureText: vi.fn(() => ({ width: 10 })),
} as unknown as CanvasRenderingContext2D
const disconnect = vi.fn()
let clientWidth: PropertyDescriptor | undefined
let clientHeight: PropertyDescriptor | undefined

beforeEach(() => {
  vi.clearAllMocks()
  HTMLCanvasElement.prototype.getContext = (() => ctx) as unknown as HTMLCanvasElement['getContext']
  HTMLCanvasElement.prototype.getBoundingClientRect = () => ({ left: 0, top: 0 }) as DOMRect
  clientWidth = Object.getOwnPropertyDescriptor(HTMLElement.prototype, 'clientWidth')
  clientHeight = Object.getOwnPropertyDescriptor(HTMLElement.prototype, 'clientHeight')
  Object.defineProperty(HTMLElement.prototype, 'clientWidth', { configurable: true, get: () => 200 })
  Object.defineProperty(HTMLElement.prototype, 'clientHeight', { configurable: true, get: () => 100 })
  vi.stubGlobal('ResizeObserver', class { observe() {} disconnect = disconnect })
})
afterEach(() => {
  vi.unstubAllGlobals()
  if (clientWidth) Object.defineProperty(HTMLElement.prototype, 'clientWidth', clientWidth)
  if (clientHeight) Object.defineProperty(HTMLElement.prototype, 'clientHeight', clientHeight)
})

// Folder (id 1) is 75% of the area: it is laid out first, on the left. Leaf (id 2) is on the right.
const nodes = [node({ id: 1, size: 750, hasChildren: true }), node({ id: 2, size: 250, hasChildren: false })]

function setup() {
  const onOpen = vi.fn(), onSelect = vi.fn(), onHover = vi.fn()
  const r = render(Treemap, { nodes, onOpen, onSelect, onHover })
  const canvas = r.container.querySelector('canvas')!
  return { ...r, canvas, onOpen, onSelect, onHover }
}

describe('Treemap', () => {
  it('draws and is hidden from assistive tech', async () => {
    const { canvas } = setup()
    await tick()
    expect(canvas).toHaveAttribute('aria-hidden', 'true')
    expect(ctx.fillRect).toHaveBeenCalledTimes(2)
  })

  it('opens a folder on click', async () => {
    const { canvas, onOpen, onSelect } = setup()
    await tick()
    await fireEvent.click(canvas, { clientX: 50, clientY: 50 })
    expect(onOpen).toHaveBeenCalledWith(1)
    expect(onSelect).not.toHaveBeenCalled()
  })

  it('selects a leaf on click without opening', async () => {
    const { canvas, onOpen, onSelect } = setup()
    await tick()
    await fireEvent.click(canvas, { clientX: 190, clientY: 50 })
    expect(onSelect).toHaveBeenCalledWith(2)
    expect(onOpen).not.toHaveBeenCalled()
  })

  it('reports hover only when the id changes', async () => {
    const { canvas, onHover } = setup()
    await tick()
    await fireEvent.pointerMove(canvas, { clientX: 50, clientY: 50 })
    await fireEvent.pointerMove(canvas, { clientX: 60, clientY: 55 })
    expect(onHover).toHaveBeenCalledTimes(1)
    expect(onHover).toHaveBeenLastCalledWith(1)
    await fireEvent.pointerMove(canvas, { clientX: 190, clientY: 50 })
    expect(onHover).toHaveBeenLastCalledWith(2)
    await fireEvent.pointerLeave(canvas)
    expect(onHover).toHaveBeenLastCalledWith(null)
  })

  it('disconnects the ResizeObserver on unmount', async () => {
    const { unmount } = setup()
    await tick()
    unmount()
    expect(disconnect).toHaveBeenCalled()
  })
})
