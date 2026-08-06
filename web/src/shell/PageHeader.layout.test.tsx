/**
 * Layout contract for PageHeader — guards the padding flip-flop.
 * Assert class tokens (stable under Tailwind) rather than fragile computed px.
 */
import { render } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { PageHeader } from './PageHeader'

describe('PageHeader layout contract', () => {
  it('keeps internal padding classes and does not expand with -mx by default', () => {
    const { getByTestId } = render(
      <main>
        <PageHeader kicker="TEST" title="Hello" extra={<span>extra</span>} />
      </main>,
    )
    const cls = getByTestId('page-header').className
    // Horizontal + vertical padding must remain on the header itself.
    expect(cls.split(/\s+/)).toEqual(expect.arrayContaining(['px-5', 'py-5']))
    // Never re-introduce default negative margins (makes bar wider than siblings).
    expect(cls).not.toMatch(/(?:^|\s)-mx-/)
    expect(cls).toMatch(/page-header/)
    expect(cls).toMatch(/sticky/)
  })
})
