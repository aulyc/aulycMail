// Responsive layout store for tiled/narrow windows
// Three modes: full (>1024px), medium (768-1024px), narrow (<768px)

export type LayoutMode = 'full' | 'medium' | 'narrow'
export type ResponsiveView = 'default' | 'viewer' | 'sidebar'

let layoutMode = $state<LayoutMode>('full')
let responsiveView = $state<ResponsiveView>('default')

let narrowMql: MediaQueryList | null = null
let mediumMql: MediaQueryList | null = null

function updateMode() {
  // Always use the full three-column layout — the responsive medium/narrow
  // collapse modes are disabled by request. All three panes stay visible at
  // every window width.
  layoutMode = 'full'
  responsiveView = 'default'
}

export function initLayout() {
  narrowMql = window.matchMedia('(max-width: 767px)')
  mediumMql = window.matchMedia('(max-width: 1024px)')

  updateMode()

  narrowMql.addEventListener('change', updateMode)
  mediumMql.addEventListener('change', updateMode)
}

export function getLayoutMode(): LayoutMode {
  return layoutMode
}

export function getResponsiveView(): ResponsiveView {
  return responsiveView
}

export function isResponsive(): boolean {
  return layoutMode !== 'full'
}

export function showViewer() {
  if (layoutMode === 'full') return
  responsiveView = 'viewer'
}

export function hideViewer() {
  if (layoutMode === 'full') return
  responsiveView = 'default'
}

export function showSidebar() {
  if (layoutMode !== 'narrow') return
  responsiveView = 'sidebar'
}

export function hideSidebar() {
  if (layoutMode !== 'narrow') return
  if (responsiveView === 'sidebar') {
    responsiveView = 'default'
  }
}
