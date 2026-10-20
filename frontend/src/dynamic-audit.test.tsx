// @vitest-environment jsdom
import { Profiler } from 'react'
import { cleanup, render } from '@testing-library/react'
import { expect, it } from 'vitest'

type Peak = { id: number; change: number }

function PresentationProbe({ peaks }: { peaks: Peak[] }) {
  // model the visible virtualized rows and the three notification cards.
  return <><section>{peaks.slice(0, 30).map((peak) => <output key={peak.id}>{peak.change.toFixed(2)}</output>)}</section>
    <aside>{peaks.slice(-3).map((peak) => <output key={peak.id}>{peak.change.toFixed(2)}</output>)}</aside></>
}

it.skipIf(import.meta.env.VITE_DYNAMIC_AUDIT !== '1')('profiles batched continuation presentation for an entire market crash', () => {
  for (const rules of [2, 10]) {
    const count = 750 * rules
    let peaks: Peak[] = Array.from({ length: count }, (_, id) => ({ id, change: -10 }))
    let commits = 0
    let reactMs = 0
    const durations: number[] = []
    const view = render(<Profiler id="dynamic-model" onRender={(_, __, duration) => { commits++; reactMs += duration }}>
      <PresentationProbe peaks={peaks} />
    </Profiler>)
    for (let step = 1; step <= 40; step++) {
      const started = performance.now()
      // every active record improves, including records outside the viewport.
      peaks = peaks.map((peak) => ({ ...peak, change: -10 - step / 2 }))
      view.rerender(<Profiler id="dynamic-model" onRender={(_, __, duration) => { commits++; reactMs += duration }}>
        <PresentationProbe peaks={peaks} />
      </Profiler>)
      durations.push(performance.now() - started)
    }
    durations.sort((a, b) => a - b)
    expect(commits).toBe(41)
    expect(view.container.querySelectorAll('output')).toHaveLength(33)
    expect(view.container.textContent).toContain('-30.00')
    console.info(JSON.stringify({ active: count, commits, visible: 33, reactMs, batchP99Ms: durations.at(-1), environment: 'jsdom model, not native Wails' }))
    cleanup()
  }
})
