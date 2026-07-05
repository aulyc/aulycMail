<script lang="ts">
  import Icon from '@iconify/svelte'
  import { GetInlineAttachments, AddImageAllowlist, OpenURL } from '../../../../wailsjs/go/app/App'
  import { getCached, setCache } from '../../stores/inlineAttachmentCache'
  import { isImageAllowedSync, refreshImageAllowlist } from '$lib/stores/imageAllowlist.svelte'
  import { setFocusedPane, focusPreviousPane, focusNextPane } from '$lib/stores/keyboard.svelte'
  import * as DropdownMenu from '$lib/components/ui/dropdown-menu'
  import { _ } from '$lib/i18n'
  import { toasts } from '$lib/stores/toast'
  import { getAlwaysLoadImages, getThemeMode } from '$lib/stores/settings.svelte'
  import { buildDarkMailFilterStyles, getDarkMailSurfaceBackground } from '$lib/utils/dark-mail'

  interface Props {
    messageId: string
    accountId?: string
    bodyHtml?: string
    bodyText?: string
    fromEmail?: string
    onCompose?: (to: string) => void
    onImagesLoaded?: () => void
    darken?: boolean
  }

  let { messageId, accountId: _accountId, bodyHtml = '', bodyText = '', fromEmail = '', onCompose, onImagesLoaded, darken = false }: Props = $props()

  // State for remote image handling
  let imagesBlocked = $state(true)
  let iframeElement = $state<HTMLIFrameElement | null>(null)
  let iframeReady = $state(false)

  // Inline attachment state
  let inlineAttachments = $state<Record<string, string>>({})
  let lastSentMessageId = $state<string | null>(null)

  // Link tooltip state
  let tooltipVisible = $state(false)
  let tooltipUrl = $state('')
  let tooltipX = $state(0)
  let tooltipY = $state(0)

  // Context menu state (unified for text selection and links)
  let ctxMenuVisible = $state(false)
  let ctxMenuText = $state('')
  let ctxMenuUrl = $state('')
  let ctxMenuX = $state(0)
  let ctxMenuY = $state(0)

  // Derived state
  let hasRemoteImages = $derived(checkForRemoteImages(bodyHtml))

  // Outer iframe element bg: matches the dark-mail surface color (derived from
  // active theme) when darken=true, otherwise white. Reactive to theme changes
  // via getThemeMode() so the iframe outer color updates when user switches.
  let iframeOuterBg = $derived.by(() => {
    void getThemeMode()
    return darken ? getDarkMailSurfaceBackground() : 'white'
  })

  // Loading placeholder SVG
  const loadingPlaceholder = `data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='120' height='80' viewBox='0 0 120 80'%3E%3Crect fill='%23f3f4f6' width='120' height='80' rx='4'/%3E%3Cg transform='translate(60,40)'%3E%3Ccircle cx='0' cy='0' r='12' fill='none' stroke='%239ca3af' stroke-width='2' stroke-dasharray='20 10'%3E%3CanimateTransform attributeName='transform' type='rotate' from='0' to='360' dur='1s' repeatCount='indefinite'/%3E%3C/circle%3E%3C/g%3E%3Ctext x='60' y='65' text-anchor='middle' fill='%239ca3af' font-size='9' font-family='sans-serif'%3ELoading...%3C/text%3E%3C/svg%3E`

  // Regex pattern for CSS url() with remote http(s) URLs.
  // Handles all quote styles: raw ' or ", decimal &#39;/&#34;, hex &#x27;/&#x22;, named &apos;/&quot;
  // Used as a string so we can create fresh RegExp instances (avoids lastIndex issues with /g)
  const CSS_QUOTE = `(?:['"]|&#(?:39|x27|34|x22);|&(?:apos|quot);)?`
  const CSS_REMOTE_URL_PATTERN = `url\\(\\s*${CSS_QUOTE}\\s*https?://[^)]*?${CSS_QUOTE}\\s*\\)`
  const MAX_IFRAME_URL_LENGTH = 4096
  const MAX_IFRAME_TEXT_LENGTH = 10000
  const MAX_IFRAME_COORDINATE = 100000

  type IframeMessage =
    | { type: 'iframe-height'; height: number }
    | { type: 'iframe-ready' }
    | { type: 'open-link'; url: string }
    | { type: 'iframe-keydown'; key: string; code: string; altKey: boolean; ctrlKey: boolean; metaKey: boolean; shiftKey: boolean }
    | { type: 'iframe-focus' }
    | { type: 'link-hover'; url: string; x: number; y: number }
    | { type: 'link-hover-end' }
    | { type: 'contextmenu'; text: string; url: string; x: number; y: number }

  function checkForRemoteImages(html: string): boolean {
    if (!html) return false
    // Check <img> tags with remote src
    if (/<img[^>]+src=["'](https?:\/\/[^"']+)["']/i.test(html)) return true
    // Check CSS url() references with remote URLs (background-image, background, etc.)
    if (new RegExp(CSS_REMOTE_URL_PATTERN, 'i').test(html)) return true
    // Check HTML background attribute with remote URLs
    if (/\bbackground\s*=\s*["'](https?:\/\/[^"']+)["']/i.test(html)) return true
    return false
  }

  function isRecord(value: unknown): value is Record<string, unknown> {
    return typeof value === 'object' && value !== null
  }

  function boundedString(value: unknown, maxLength: number): string | null {
    if (typeof value !== 'string') return null
    if (value.length > maxLength) return value.slice(0, maxLength)
    return value
  }

  function boundedNumber(value: unknown, min: number, max: number): number | null {
    if (typeof value !== 'number' || !Number.isFinite(value)) return null
    if (value < min || value > max) return null
    return value
  }

  function parseIframeMessage(data: unknown): IframeMessage | null {
    if (!isRecord(data) || typeof data.type !== 'string') return null

    switch (data.type) {
      case 'iframe-height': {
        const height = boundedNumber(data.height, 0, MAX_IFRAME_COORDINATE)
        return height === null ? null : { type: 'iframe-height', height }
      }
      case 'iframe-ready':
        return { type: 'iframe-ready' }
      case 'open-link': {
        const url = boundedString(data.url, MAX_IFRAME_URL_LENGTH)
        return url === null ? null : { type: 'open-link', url }
      }
      case 'iframe-keydown': {
        const key = boundedString(data.key, 64)
        const code = boundedString(data.code, 64)
        if (key === null || code === null) return null
        return {
          type: 'iframe-keydown',
          key,
          code,
          altKey: data.altKey === true,
          ctrlKey: data.ctrlKey === true,
          metaKey: data.metaKey === true,
          shiftKey: data.shiftKey === true
        }
      }
      case 'iframe-focus':
        return { type: 'iframe-focus' }
      case 'link-hover': {
        const url = boundedString(data.url, MAX_IFRAME_URL_LENGTH)
        const x = boundedNumber(data.x, -MAX_IFRAME_COORDINATE, MAX_IFRAME_COORDINATE)
        const y = boundedNumber(data.y, -MAX_IFRAME_COORDINATE, MAX_IFRAME_COORDINATE)
        return url === null || x === null || y === null ? null : { type: 'link-hover', url, x, y }
      }
      case 'link-hover-end':
        return { type: 'link-hover-end' }
      case 'contextmenu': {
        const text = boundedString(data.text ?? '', MAX_IFRAME_TEXT_LENGTH)
        const url = boundedString(data.url ?? '', MAX_IFRAME_URL_LENGTH)
        const x = boundedNumber(data.x, -MAX_IFRAME_COORDINATE, MAX_IFRAME_COORDINATE)
        const y = boundedNumber(data.y, -MAX_IFRAME_COORDINATE, MAX_IFRAME_COORDINATE)
        return text === null || url === null || x === null || y === null ? null : { type: 'contextmenu', text, url, x, y }
      }
      default:
        return null
    }
  }

  function parseAllowedEmailURL(rawURL: string): URL | null {
    try {
      const parsed = new URL(rawURL)
      const protocol = parsed.protocol.toLowerCase()
      if (protocol !== 'http:' && protocol !== 'https:' && protocol !== 'mailto:') return null
      if ((protocol === 'http:' || protocol === 'https:') && !parsed.host) return null
      return parsed
    } catch {
      return null
    }
  }

  function redactURLForLog(rawURL: string): string {
    try {
      const parsed = new URL(rawURL)
      const protocol = parsed.protocol.toLowerCase()
      if (protocol !== 'http:' && protocol !== 'https:' && protocol !== 'mailto:') {
        return `${parsed.protocol}[redacted]`
      }
      parsed.search = ''
      parsed.hash = ''
      parsed.username = ''
      parsed.password = ''
      return parsed.toString()
    } catch {
      return `[invalid-url length=${rawURL.length}]`
    }
  }

  function processCidReferences(html: string): string {
    if (!html) return html
    return html.replace(
      /src=["']cid:([^"']+)["']/gi,
      (match, contentId) => `src="${loadingPlaceholder}" data-cid="${contentId}"`
    )
  }

  function processHtml(html: string, blockImages: boolean): string {
    if (!html) return ''
    let processed = processCidReferences(html)
    if (blockImages) {
      // Block <img> tags with remote sources
      processed = processed.replace(
        /(<img[^>]+)src=["'](https?:\/\/[^"']+)["']([^>]*>)/gi,
        (match, before, src, after) => {
          const placeholder = `data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='100' height='60' viewBox='0 0 100 60'%3E%3Crect fill='%23e5e7eb' width='100' height='60'/%3E%3Ctext x='50' y='35' text-anchor='middle' fill='%239ca3af' font-size='10' font-family='sans-serif'%3EImage blocked%3C/text%3E%3C/svg%3E`
          return `${before}src="${placeholder}" data-blocked-src="${encodeURIComponent(src)}"${after}`
        }
      )
      // Block remote URLs in CSS url() references (covers background-image, background, etc.)
      // Handles all quote encodings: raw, decimal entities, hex entities, named entities
      processed = processed.replace(new RegExp(CSS_REMOTE_URL_PATTERN, 'gi'), 'url()')
      // Block HTML background attribute with remote URLs
      processed = processed.replace(
        /\bbackground\s*=\s*["'](https?:\/\/[^"']+)["']/gi,
        'background=""'
      )
    }
    return processed
  }

  function buildIframeContent(html: string, applyDarken: boolean): string {
    const processedHtml = processHtml(html, imagesBlocked)
    const imgSrc = imagesBlocked ? "'self' data:" : '* data:'

    // Double-invert: page-level invert + image-level re-invert keeps photos
    // looking normal while flipping text, backgrounds, and CSS-defined colors.
    // Blocked-image placeholders intentionally skip the re-invert so they end
    // up dark too — they're chrome, not content.
    // color-scheme: dark switches the UA-default iframe viewport bg to dark so
    // the rounded-corner edge has no white sliver bleeding through.
    // Invert amount derived from theme's chrome lightness — pure invert(1)
    // produces stark black against themes whose chrome isn't pure black.
    const darkMailStyles = applyDarken ? buildDarkMailFilterStyles() : null
    const darkenStyles = applyDarken ? `
    html { filter: ${darkMailStyles?.contentFilter}; background: #fff; color-scheme: dark; }
    img:not([data-blocked-src]), video, iframe, [data-no-invert] { filter: ${darkMailStyles?.mediaFilter}; }
` : `
    html { color-scheme: light; }
`

    const iframeScript = `
      function sendHeight() {
        var height = document.body.scrollHeight;
        window.parent.postMessage({ type: 'iframe-height', height: height }, '*');
      }
      
      function attachImageHandlers() {
        document.querySelectorAll('img').forEach(function(img) {
          if (!img.dataset.heightHandlerAttached) {
            img.dataset.heightHandlerAttached = 'true';
            img.onload = sendHeight;
            img.onerror = sendHeight;
          }
        });
      }
      
      window.addEventListener('message', function(e) {
        if (e.data?.type === 'select-all') {
          var range = document.createRange();
          range.selectNodeContents(document.body);
          var selection = window.getSelection();
          if (selection) {
            selection.removeAllRanges();
            selection.addRange(range);
          }
          return;
        }
        if (e.data?.type === 'inline-images' && e.data.images) {
          var images = e.data.images;
          var replaced = 0;
          Object.keys(images).forEach(function(cid) {
            // querySelectorAll (not querySelector): a body can legitimately
            // reference the same cid: from multiple <img> tags — e.g. the
            // composer emits cid:c1 twice when the user pastes the same
            // image twice and the dedup collapses both into one inline
            // attachment. Single-querySelector would leave every <img>
            // except the first stuck on the loading placeholder.
            var imgs = document.querySelectorAll('img[data-cid="' + cid + '"]');
            imgs.forEach(function(img) {
              img.src = images[cid];
              img.removeAttribute('data-cid');
              replaced++;
            });
          });
          if (replaced > 0) {
            attachImageHandlers();
            setTimeout(sendHeight, 50);
            setTimeout(sendHeight, 150);
            setTimeout(sendHeight, 300);
          }
        }
      });

      window.addEventListener('load', function() {
        attachImageHandlers();
        sendHeight();
        window.parent.postMessage({ type: 'iframe-ready' }, '*');
      });
      
      window.addEventListener('resize', sendHeight);
      new ResizeObserver(sendHeight).observe(document.body);
      setTimeout(sendHeight, 50);
      setTimeout(sendHeight, 200);
      
      document.addEventListener('click', function(e) {
        var link = e.target.closest('a');
        if (link && link.href) {
          e.preventDefault();
          window.parent.postMessage({ type: 'open-link', url: link.href }, '*');
        }
      });

      // Handle link hover for tooltip
      document.addEventListener('mouseover', function(e) {
        var link = e.target.closest('a');
        if (link && link.href) {
          var rect = link.getBoundingClientRect();
          window.parent.postMessage({
            type: 'link-hover',
            url: link.href,
            x: rect.left,
            y: rect.bottom
          }, '*');
        }
      });

      document.addEventListener('mouseout', function(e) {
        var link = e.target.closest('a');
        if (link && link.href) {
          window.parent.postMessage({ type: 'link-hover-end' }, '*');
        }
      });

      // Handle right-click context menu — always prevent native menu, show custom one
      document.addEventListener('contextmenu', function(e) {
        e.preventDefault();
        var selection = window.getSelection();
        var selectedText = (selection && selection.toString().trim().length > 0) ? selection.toString() : '';
        var link = e.target.closest('a');
        var linkUrl = (link && link.href) ? link.href : '';

        window.parent.postMessage({
          type: 'contextmenu',
          text: selectedText,
          url: linkUrl,
          x: e.clientX,
          y: e.clientY
        }, '*');
      });

      // Forward keyboard events to parent for global shortcuts (only modifier keys and Escape)
      document.addEventListener('keydown', function(e) {
        // Only forward events that need global handling
        if (e.altKey || e.ctrlKey || e.metaKey || e.key === 'Escape') {
          // For pane navigation, blur inside iframe first
          if (e.altKey && (e.key === 'ArrowLeft' || e.key === 'ArrowRight' || e.key === 'h' || e.key === 'l')) {
            if (document.activeElement) {
              document.activeElement.blur();
            }
            document.body.blur();
            window.blur();
          }
          window.parent.postMessage({
            type: 'iframe-keydown',
            key: e.key,
            code: e.code,
            altKey: e.altKey,
            ctrlKey: e.ctrlKey,
            metaKey: e.metaKey,
            shiftKey: e.shiftKey
          }, '*');
        }
      });

      // Notify parent when iframe receives focus/click (but not for links/buttons)
      function isInteractiveElement(el) {
        if (!el) return false;
        var link = el.closest('a');
        if (link && link.href) return true;
        var button = el.closest('button');
        if (button) return true;
        if (el.tagName === 'INPUT' || el.tagName === 'SELECT' || el.tagName === 'TEXTAREA') return true;
        return false;
      }
      document.addEventListener('click', function(e) {
        if (!isInteractiveElement(e.target)) {
          window.parent.postMessage({ type: 'iframe-focus' }, '*');
        }
      });
      document.addEventListener('focus', function(e) {
        if (!isInteractiveElement(e.target)) {
          window.parent.postMessage({ type: 'iframe-focus' }, '*');
        }
      }, true);
    `

    return `<!DOCTYPE html>
<html>
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <meta http-equiv="Content-Security-Policy" content="default-src 'self' data:; img-src ${imgSrc}; style-src 'unsafe-inline'; script-src 'unsafe-inline';">
  <sty` + `le>${darkenStyles}
    /* Minimal base styles - avoid overriding email's inline styles */
    * { box-sizing: border-box; }
    html, body {
      margin: 0; padding: 0;
      font-family: system-ui, sans-serif;
      font-size: 14px; line-height: 1.5;
      color: #1a1a0a; background-color: white;
      overflow-x: auto; word-wrap: break-word;
      scrollbar-width: none; /* Firefox */
      -ms-overflow-style: none; /* IE/Edge */
    }
    html::-webkit-scrollbar, body::-webkit-scrollbar { display: none; /* Chrome/Safari/WebKit */ }
    body { padding: 16px; }
    img { max-width: 100%; height: auto; }
    /* Ensure empty paragraphs (blank lines) render with visible height */
    p:empty { min-height: 1em; }
    p:has(> br:only-child) { min-height: 1em; }
    img[data-cid] { min-width: 100px; min-height: 60px; }
    a { color: #2563eb; }
    /* Only apply defaults to elements without inline styles */
    blockquote:not([style]) { margin: 0.5em 0; padding-left: 1em; border-left: 3px solid #e5e7eb; color: #6b7280; }
    pre:not([style]) { background: #f3f4f6; padding: 0.5em; border-radius: 4px; overflow-x: auto; }
    /* Remove table/td defaults that conflict with email layouts */
  </sty` + `le>
</head>
<body>
${processedHtml}
<scr` + `ipt>${iframeScript}</scr` + `ipt>
</body>
</html>`
  }

  // Handle a clicked link URL — routes mailto: to composer, others to system browser.
  // Used by both the iframe message handler and the plain text div click handler.
  function handleLinkClick(url: string) {
    const parsed = parseAllowedEmailURL(url)
    if (parsed?.protocol.toLowerCase() === 'mailto:') {
      const emailAddress = parsed.pathname
      if (onCompose) {
        onCompose(emailAddress)
        return
      }
    }
    safeOpenURL(url)
  }

  // Helper function to safely open URLs
  // Uses our custom OpenURL backend function which properly handles shell escaping
  async function safeOpenURL(url: string) {
    const parsed = parseAllowedEmailURL(url)
    if (!parsed) {
      console.warn('[EmailBody] Blocked invalid or disallowed URL:', redactURLForLog(url))
      return
    }

    try {
      await OpenURL(parsed.href)
      toasts.info($_('toast.linkOpened'))
    } catch (err) {
      console.error('[EmailBody] OpenURL failed:', err)
    }
  }

  function handleIframeMessage(event: MessageEvent) {
    // Only handle messages from this component's iframe
    if (event.source !== iframeElement?.contentWindow) return
    const message = parseIframeMessage(event.data)
    if (!message) return

    if (message.type === 'iframe-height' && iframeElement) {
      iframeElement.style.height = `${message.height + 20}px`
    } else if (message.type === 'iframe-ready') {
      iframeReady = true
    } else if (message.type === 'open-link') {
      handleLinkClick(message.url)
    } else if (message.type === 'iframe-keydown') {
      // Handle Alt+arrow/hjkl directly for pane navigation
      if (message.altKey) {
        const key = message.key
        if (key === 'ArrowLeft' || key === 'h') {
          focusPreviousPane()
          // Dispatch event to let App.svelte handle focus
          window.dispatchEvent(new CustomEvent('escape-iframe-focus'))
          return
        } else if (key === 'ArrowRight' || key === 'l') {
          focusNextPane()
          window.dispatchEvent(new CustomEvent('escape-iframe-focus'))
          return
        }
      }
      // For other shortcuts (Ctrl+, Escape), dispatch to window
      const syntheticEvent = new KeyboardEvent('keydown', {
        key: message.key,
        code: message.code,
        altKey: message.altKey,
        ctrlKey: message.ctrlKey,
        metaKey: message.metaKey,
        shiftKey: message.shiftKey,
        bubbles: true,
        cancelable: true
      })
      window.dispatchEvent(syntheticEvent)
    } else if (message.type === 'iframe-focus') {
      // Set focus to viewer pane when iframe is clicked/focused
      setFocusedPane('viewer')
    } else if (message.type === 'link-hover') {
      // Show tooltip with link URL - adjust coordinates relative to iframe position
      if (iframeElement) {
        const iframeRect = iframeElement.getBoundingClientRect()
        tooltipUrl = message.url
        tooltipX = iframeRect.left + message.x
        tooltipY = iframeRect.top + message.y
        tooltipVisible = true
      }
    } else if (message.type === 'link-hover-end') {
      // Hide tooltip
      tooltipVisible = false
    } else if (message.type === 'contextmenu') {
      // Show unified context menu for text selection and/or links
      if (iframeElement) {
        const iframeRect = iframeElement.getBoundingClientRect()
        ctxMenuText = message.text
        ctxMenuUrl = message.url
        ctxMenuX = iframeRect.left + message.x
        ctxMenuY = iframeRect.top + message.y
        ctxMenuVisible = true
      }
    }
  }

  function sendInlineImagesToIframe(images: Record<string, string>) {
    if (iframeElement?.contentWindow && Object.keys(images).length > 0) {
      // Use spread operator to create plain object from Svelte 5 $state proxy
      // This is needed because postMessage uses structured clone which can't handle proxies
      // srcdoc iframes have an opaque origin, so targetOrigin must remain '*'.
      iframeElement.contentWindow.postMessage({
        type: 'inline-images',
        images: { ...images }
      }, '*')
    }
  }

  function loadImages() {
    imagesBlocked = false
    onImagesLoaded?.()
  }

  // Extract domain from email address
  function extractDomain(email: string): string {
    const parts = email.split('@')
    return parts.length === 2 ? parts[1] : ''
  }

  // Handle "Always load for this sender" action
  async function handleAlwaysLoadSender() {
    if (!fromEmail) return
    try {
      await AddImageAllowlist('sender', fromEmail)
      refreshImageAllowlist()
      loadImages()
    } catch (err) {
      console.error('[EmailBody] Failed to add sender to allowlist:', err)
    }
  }

  // Handle "Always load for this domain" action
  async function handleAlwaysLoadDomain() {
    const domain = extractDomain(fromEmail)
    if (!domain) return
    try {
      await AddImageAllowlist('domain', domain)
      refreshImageAllowlist()
      loadImages()
    } catch (err) {
      console.error('[EmailBody] Failed to add domain to allowlist:', err)
    }
  }

  // Check allowlist and reset state on message change (single effect to avoid race conditions)
  // Uses synchronous frontend cache instead of async Wails call to avoid bridge saturation.
  $effect(() => {
    void messageId // dependency only
    const email = fromEmail
    const hasImages = hasRemoteImages

    // Reset state (was in separate effect — merged to avoid race)
    iframeReady = false
    lastSentMessageId = null
    inlineAttachments = {}
    imagesBlocked = true

    if (!hasImages) return

    if (getAlwaysLoadImages()) {
      imagesBlocked = false
      return
    }

    if (email && isImageAllowedSync(email)) {
      imagesBlocked = false
    }
  })

  // Fetch inline attachments when we have cid references
  $effect(() => {
    const id = messageId
    const html = bodyHtml
    const hasCid = html ? /src=["']cid:([^"']+)["']/i.test(html) : false

    if (!id || !hasCid) {
      return
    }

    // Check memory cache first
    const cached = getCached(id)
    if (cached && Object.keys(cached).length > 0) {
      inlineAttachments = cached
      return
    }

    GetInlineAttachments(id)
      .then((result: Record<string, string>) => {
        const data = result || {}
        inlineAttachments = data
        if (Object.keys(data).length > 0) {
          setCache(id, data)
        }
      })
      .catch((err: Error) => {
        console.error('[EmailBody] Fetch error:', err)
      })
  })

  // Build iframe content
  $effect(() => {
    const html = bodyHtml
    void imagesBlocked // dependency only
    void getThemeMode() // rebuild when theme changes so dark-mail invert re-derives
    const applyDarken = darken

    if (iframeElement && html) {
      const content = buildIframeContent(html, applyDarken)
      iframeElement.srcdoc = content
      iframeReady = false
      lastSentMessageId = null
    }
  })

  // Send inline images when ready
  $effect(() => {
    const ready = iframeReady
    const images = inlineAttachments
    const id = messageId
    const alreadySent = lastSentMessageId === id

    if (ready && Object.keys(images).length > 0 && !alreadySent) {
      sendInlineImagesToIframe(images)
      lastSentMessageId = id
    }
  })

  // Message listener
  $effect(() => {
    window.addEventListener('message', handleIframeMessage)
    return () => window.removeEventListener('message', handleIframeMessage)
  })

  // State for controlling the Always Load dropdown
  let alwaysLoadDropdownOpen = $state(false)

  // Listen for Ctrl-L load images event
  $effect(() => {
    function handleLoadImagesEvent() {
      if (hasRemoteImages && imagesBlocked) {
        loadImages()
      }
    }
    window.addEventListener('load-remote-images', handleLoadImagesEvent)
    return () => window.removeEventListener('load-remote-images', handleLoadImagesEvent)
  })

  // Listen for Ctrl-Shift-L always load dropdown event
  $effect(() => {
    function handleAlwaysLoadDropdownEvent() {
      if (hasRemoteImages && imagesBlocked && fromEmail) {
        alwaysLoadDropdownOpen = true
      }
    }
    window.addEventListener('open-always-load-dropdown', handleAlwaysLoadDropdownEvent)
    return () => window.removeEventListener('open-always-load-dropdown', handleAlwaysLoadDropdownEvent)
  })

  function linkifyText(text: string): string {
    if (!text) return ''
    const urlPattern = /(https?:\/\/[^\s<>"{}|\\^`[\]]+)/g
    const emailPattern = /([a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,})/g
    let escaped = text
      .replace(/&/g, '&amp;')
      .replace(/</g, '&lt;')
      .replace(/>/g, '&gt;')
    escaped = escaped.replace(urlPattern, '<a href="$1" target="_blank" rel="noopener noreferrer" class="text-primary hover:underline">$1</a>')
    escaped = escaped.replace(emailPattern, '<a href="mailto:$1" class="text-primary hover:underline">$1</a>')
    return escaped
  }

  // Copy selected text to clipboard
  async function copyTextToClipboard() {
    if (!ctxMenuText) return
    try {
      await navigator.clipboard.writeText(ctxMenuText)
      ctxMenuVisible = false
    } catch (err) {
      console.error('[EmailBody] Failed to copy text:', err)
    }
  }

  // Copy link to clipboard
  async function copyLinkToClipboard() {
    if (!ctxMenuUrl) return
    try {
      await navigator.clipboard.writeText(ctxMenuUrl)
      ctxMenuVisible = false
    } catch (err) {
      console.error('[EmailBody] Failed to copy link:', err)
    }
  }

  // Select all text in iframe
  function selectAllInIframe() {
    // srcdoc iframes have an opaque origin, so targetOrigin must remain '*'.
    iframeElement?.contentWindow?.postMessage({ type: 'select-all' }, '*')
    ctxMenuVisible = false
  }
</script>

<div class="email-body relative">
  {#if bodyHtml}
    {#if hasRemoteImages && imagesBlocked}
      <div class="flex items-center gap-2 px-3 py-2 mb-3 rounded-md bg-yellow-500/10 border border-yellow-500/30 text-sm">
        <Icon icon="mdi:image-off" class="w-4 h-4 text-yellow-600 flex-shrink-0" />
        <span class="text-yellow-700 dark:text-yellow-400">{$_('viewer.remoteImagesBlocked')}</span>

        <div class="ml-auto flex items-center gap-1">
          <!-- Load Images button -->
          <button
            class="px-2 py-1 text-xs font-medium rounded bg-yellow-600 text-white hover:bg-yellow-700 transition-colors"
            onclick={loadImages}
          >
            {$_('viewer.loadImages')}
          </button>

          <!-- Always Load dropdown -->
          {#if fromEmail}
            <DropdownMenu.Root bind:open={alwaysLoadDropdownOpen}>
              <DropdownMenu.Trigger
                class="px-2 py-1 text-xs font-medium rounded bg-yellow-600 text-white hover:bg-yellow-700 transition-colors flex items-center gap-1"
              >
                {$_('viewer.alwaysLoad')}
                <Icon icon="mdi:chevron-down" class="w-3 h-3" />
              </DropdownMenu.Trigger>
              <DropdownMenu.Content align="end">
                <DropdownMenu.Item onSelect={handleAlwaysLoadDomain}>
                  <Icon icon="mdi:domain" class="w-4 h-4 mr-2" />
                  {$_('viewer.forDomain', { values: { domain: extractDomain(fromEmail) || 'this domain' } })}
                </DropdownMenu.Item>
                <DropdownMenu.Item onSelect={handleAlwaysLoadSender}>
                  <Icon icon="mdi:account" class="w-4 h-4 mr-2" />
                  {$_('viewer.forSender', { values: { email: fromEmail } })}
                </DropdownMenu.Item>
              </DropdownMenu.Content>
            </DropdownMenu.Root>
          {/if}
        </div>
      </div>
    {/if}

    <iframe
      bind:this={iframeElement}
      title={$_('aria.emailContent')}
      sandbox="allow-scripts allow-popups allow-popups-to-escape-sandbox"
      class="w-full border-0 rounded-md min-h-[100px]"
      style="height: 200px; background-color: {iframeOuterBg};"
    ></iframe>
  {:else if bodyText}
    <!-- svelte-ignore a11y_no_static_element_interactions -->
    <div class="whitespace-pre-wrap font-sans text-sm text-foreground bg-muted/30 rounded-md p-4"
      onkeydown={(e) => { if (e.key === 'Enter') { const link = (e.target as HTMLElement).closest('a'); if (link?.href) { e.preventDefault(); handleLinkClick(link.href) } } }}
      onclick={(e) => {
        const link = (e.target as HTMLElement).closest('a')
        if (!link?.href) return
        e.preventDefault()
        handleLinkClick(link.href)
      }}
    >
      <!-- eslint-disable-next-line svelte/no-at-html-tags -- linkifyText escapes user content; only injects safe <a> tags around URLs -->
      {@html linkifyText(bodyText)}
    </div>
  {:else}
    <p class="text-muted-foreground italic">{$_('viewer.noContent')}</p>
  {/if}

  <!-- Link hover tooltip -->
  {#if tooltipVisible && tooltipUrl}
    <div
      class="fixed z-50 px-3 py-1.5 text-xs bg-gray-800 dark:bg-gray-200 text-white dark:text-gray-900 rounded shadow-lg max-w-md truncate pointer-events-none border border-gray-700 dark:border-gray-300"
      style="left: {tooltipX}px; top: {tooltipY + 5}px;"
    >
      {tooltipUrl}
    </div>
  {/if}

  <!-- Context menu (text copy and/or link copy) -->
  {#if ctxMenuVisible}
    <div
      class="fixed z-50 bg-white dark:bg-gray-800 rounded-md shadow-lg border border-gray-200 dark:border-gray-700 py-1 min-w-[160px]"
      style="left: {ctxMenuX}px; top: {ctxMenuY}px;"
      role="menu"
    >
      {#if ctxMenuText}
        <button
          class="w-full px-3 py-2 text-left text-sm hover:bg-gray-100 dark:hover:bg-gray-700 flex items-center gap-2"
          onclick={copyTextToClipboard}
        >
          <Icon icon="mdi:content-copy" class="w-4 h-4" />
          {$_('viewer.copy')}
        </button>
      {/if}
      {#if ctxMenuUrl}
        <button
          class="w-full px-3 py-2 text-left text-sm hover:bg-gray-100 dark:hover:bg-gray-700 flex items-center gap-2"
          onclick={copyLinkToClipboard}
        >
          <Icon icon="mdi:link-variant" class="w-4 h-4" />
          {$_('viewer.copyLink')}
        </button>
      {/if}
      <button
        class="w-full px-3 py-2 text-left text-sm hover:bg-gray-100 dark:hover:bg-gray-700 flex items-center gap-2"
        onclick={selectAllInIframe}
      >
        <Icon icon="mdi:select-all" class="w-4 h-4" />
        {$_('viewer.selectAll')}
      </button>
    </div>
  {/if}
</div>

<!-- Click outside to close context menu -->
{#if ctxMenuVisible}
  <button
    type="button"
    class="fixed inset-0 z-40 cursor-default"
    aria-label={$_('aria.closeContextMenu')}
    onclick={() => ctxMenuVisible = false}
    onkeydown={(e) => { if (e.key === 'Escape') ctxMenuVisible = false }}
  ></button>
{/if}
