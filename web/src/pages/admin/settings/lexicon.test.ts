import { formatLexiconExport, mergeLexiconText, parseLexiconText } from './lexicon'

describe('parseLexiconText', () => {
  it('trim、去空行、注释、去重保序', () => {
    expect(
      parseLexiconText(`
# comment
  Foo  
foo
Bar

baz
`),
    ).toEqual(['Foo', 'Bar', 'baz'])
  })

  it('空串 → []', () => {
    expect(parseLexiconText('')).toEqual([])
    expect(parseLexiconText('  \n  #x\n')).toEqual([])
  })
})

describe('mergeLexiconText', () => {
  it('追加去重', () => {
    expect(mergeLexiconText('a\nb', 'B\nc')).toEqual('a\nb\nc')
  })
})

describe('formatLexiconExport', () => {
  it('带头注释与词条', () => {
    const s = formatLexiconExport(['a', 'b'], 'img.li')
    expect(s).toContain('# site: img.li')
    expect(s).toContain('# count: 2')
    expect(s).toContain('\na\nb\n')
  })
})
