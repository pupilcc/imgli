import { buildIntegrationSnippets, normalizeSnippetBaseURL } from './tokenSnippets'

describe('tokenSnippets', () => {
  it('normalizeSnippetBaseURL trims trailing slash', () => {
    expect(normalizeSnippetBaseURL(' https://img.li/ ')).toBe('https://img.li')
  })

  it('builds curl/picgo/sharex/cli with base and token', () => {
    const list = buildIntegrationSnippets('https://img.li/', 'PLAIN')
    const by = Object.fromEntries(list.map((s) => [s.kind, s.text]))
    expect(by.curl).toContain("https://img.li/api/v1/upload")
    expect(by.curl).toContain('Bearer PLAIN')
    expect(by.curl).toContain("file=@shot.png")
    expect(by.picgo).toContain('"paramName": "file"')
    expect(by.picgo).toContain('data.links.url')
    expect(by.picgo).toContain('Bearer PLAIN')
    expect(by.sharex).toContain('{json:data.links.url}')
    expect(by.sharex).toContain('FileFormName')
    expect(by.sharex).toContain('Bearer PLAIN')
    expect(by.cli).toContain("IMGLI_BASE_URL='https://img.li'")
    expect(by.cli).toContain("IMGLI_TOKEN='PLAIN'")
  })

  it('uses placeholder when token empty', () => {
    const list = buildIntegrationSnippets('http://localhost:8686', '')
    expect(list.every((s) => s.text.includes('YOUR_TOKEN') || s.text.includes('Bearer YOUR_TOKEN'))).toBe(
      true,
    )
  })
})
