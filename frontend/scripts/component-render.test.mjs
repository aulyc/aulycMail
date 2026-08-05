import assert from 'node:assert/strict'
import { render } from 'svelte/server'
import { test } from 'vitest'

import RailButton from '../src/lib/components/rail/RailButton.svelte'
import SearchScopeCarousel from '../src/lib/components/search/SearchScopeCarousel.svelte'

test('rail button renders its accessible active state through Svelte', () => {
  const { body } = render(RailButton, {
    props: {
      icon: 'mdi:email-outline',
      label: 'Mail',
      active: true,
      onclick: () => {},
    },
  })

  assert.match(body, /<button[^>]*aria-label="Mail"[^>]*aria-pressed="true"/)
  assert.match(body, /border-l-primary/)
  assert.doesNotMatch(body, /border-l-transparent/)
})

test('search scope carousel renders one selected scope and wrapped neighbours', () => {
  const { body } = render(SearchScopeCarousel, {
    props: {
      scopes: [
        { id: 'all', label: 'All mail' },
        { id: 'inbox', label: 'Inbox' },
        { id: 'sent', label: 'Sent' },
      ],
      selectedId: 'inbox',
      onSelect: () => {},
    },
  })

  assert.match(body, /aria-pressed="true"[^>]*>Inbox<\/button>/)
  assert.match(body, /aria-pressed="false"[^>]*>All mail<\/button>/)
  assert.match(body, /aria-pressed="false"[^>]*>Sent<\/button>/)
  assert.equal((body.match(/aria-pressed="true"/g) || []).length, 1)
})

test('search scope carousel falls back to the first scope and handles an empty list', () => {
  const unknownSelection = render(SearchScopeCarousel, {
    props: {
      scopes: [
        { id: 'all', label: 'All mail' },
        { id: 'inbox', label: 'Inbox' },
      ],
      selectedId: 'missing',
    },
  }).body
  assert.match(unknownSelection, /aria-pressed="true"[^>]*>All mail<\/button>/)

  const empty = render(SearchScopeCarousel, { props: { scopes: [] } }).body
  assert.doesNotMatch(empty, /aria-pressed="true"/)
})
