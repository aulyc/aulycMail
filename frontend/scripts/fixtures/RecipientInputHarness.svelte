<script lang="ts">
  import RecipientInput from '../../src/lib/components/composer/RecipientInput.svelte'

  interface Recipient {
    name?: string
    address?: string
    email?: string
  }

  interface Props {
    primary?: Recipient[]
    secondary?: Recipient[]
    searchContactsFn?: (query: string, limit: number) => Promise<unknown[]>
  }

  let {
    primary: initialPrimary = [],
    secondary: initialSecondary = [],
    searchContactsFn,
  }: Props = $props()

  let primary = $state<Recipient[]>([])
  let secondary = $state<Recipient[]>([])
  let initialized = false

  $effect.pre(() => {
    if (initialized) return
    primary = [...initialPrimary]
    secondary = [...initialSecondary]
    initialized = true
  })

  export function getPrimary() {
    return primary
  }

  export function getSecondary() {
    return secondary
  }
</script>

<section data-field="primary">
  <RecipientInput bind:recipients={primary} placeholder="Primary recipients" {searchContactsFn} />
</section>

<section data-field="secondary">
  <RecipientInput bind:recipients={secondary} placeholder="Secondary recipients" {searchContactsFn} />
</section>
