export interface DarkMailFilterStyles {
  surfaceBackground: string
  contentFilter: string
  mediaFilter: string
}

function rootStyles(): CSSStyleDeclaration {
  return getComputedStyle(document.documentElement)
}

// Reads the active theme's chrome lightness so the dark-mail filter can tune
// its invert amount. Pure invert(1) is too stark against non-black themes.
function getChromeBgLightness(): number {
  const styles = rootStyles()
  const override = styles.getPropertyValue('--dark-mail-bg-l').trim()
  if (override) {
    const n = parseFloat(override)
    if (!Number.isNaN(n)) return Math.max(0, Math.min(1, n / 100))
  }

  const bg = styles.getPropertyValue('--background').trim()
  const lMatch = bg.match(/(\d+(?:\.\d+)?)%\s*$/)
  if (lMatch) {
    const l = parseFloat(lMatch[1])
    if (!Number.isNaN(l)) return Math.max(0, Math.min(1, l / 100))
  }

  return 0
}

export function getDarkMailSurfaceBackground(): string {
  const bg = rootStyles().getPropertyValue('--background').trim()
  return bg ? `hsl(${bg})` : '#000'
}

function getChromeBgSaturate(): number {
  const override = rootStyles().getPropertyValue('--dark-mail-saturate').trim()
  if (override) {
    const n = parseFloat(override)
    if (!Number.isNaN(n) && n > 0) return n
  }
  return 1
}

function getChromeBgHueRotate(): number {
  const override = rootStyles().getPropertyValue('--dark-mail-hue').trim()
  if (override) {
    const n = parseFloat(override)
    if (!Number.isNaN(n)) return n
  }
  return 0
}

export function buildDarkMailFilterStyles(): DarkMailFilterStyles {
  const invertAmount = 1 - getChromeBgLightness()
  const saturate = getChromeBgSaturate()
  const hueRotate = getChromeBgHueRotate()
  const imageSaturate = 1 / saturate

  return {
    surfaceBackground: getDarkMailSurfaceBackground(),
    contentFilter: `invert(${invertAmount}) hue-rotate(180deg) saturate(${saturate}) hue-rotate(${hueRotate}deg)`,
    mediaFilter: `invert(${invertAmount}) hue-rotate(180deg) saturate(${imageSaturate}) hue-rotate(${-hueRotate}deg)`,
  }
}
