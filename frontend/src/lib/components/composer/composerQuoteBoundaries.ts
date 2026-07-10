const FORWARDED_SEPARATOR = '---------- Forwarded message ----------'

export function findPlainQuoteBoundary(text: string): number {
  const wroteIndex = text.search(/^.*wrote:\s*$/m)
  const fwdIndex = text.indexOf(FORWARDED_SEPARATOR)
  const cutoffs = [wroteIndex, fwdIndex].filter(i => i > -1)
  return cutoffs.length > 0 ? Math.min(...cutoffs) : -1
}

export function findRichQuoteBoundary(html: string): number {
  const blockquoteIndex = html.indexOf('<blockquote')
  const wroteIndex = html.search(/wrote:\s*(<br[^>]*>)?\s*<\/p>/i)
  const fwdIndex = html.indexOf(FORWARDED_SEPARATOR)
  const cutoffs = [blockquoteIndex, wroteIndex, fwdIndex].filter(i => i > -1)
  return cutoffs.length > 0 ? Math.min(...cutoffs) : -1
}

export function hasPlainQuoteBoundary(text: string): boolean {
  return findPlainQuoteBoundary(text) >= 0
}
