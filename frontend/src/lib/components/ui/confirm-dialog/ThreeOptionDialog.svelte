<script lang="ts">
  import * as AlertDialog from '$lib/components/ui/alert-dialog'
  import Icon from '@iconify/svelte'
  import { tick } from 'svelte'
  import { getEnhancedKeyboardNavigation } from '$lib/stores/settings.svelte'

  interface Props {
    open: boolean                    // bindable
    title: string
    description?: string
    option1Label?: string            // default: "Option 1" (destructive action)
    option2Label?: string            // default: "Option 2" (primary action)
    option3Label?: string            // default: "Cancel" (cancel action)
    option1Variant?: 'default' | 'destructive'  // default: 'destructive'
    option2Variant?: 'default' | 'destructive'  // default: 'default'
    loading?: 'option1' | 'option2' | null  // which button is loading
    onOption1: () => void
    onOption2: () => void
    onOption3?: () => void           // cancel/keep editing
  }

  let {
    open = $bindable(false),
    title,
    description = '',
    option1Label = 'Option 1',
    option2Label = 'Option 2',
    option3Label = 'Cancel',
    option1Variant = 'destructive',
    option2Variant = 'default',
    loading = null,
    onOption1,
    onOption2,
    onOption3,
  }: Props = $props()

  // Button refs for keyboard navigation
  let button1Ref = $state<HTMLButtonElement | null>(null)
  let button2Ref = $state<HTMLButtonElement | null>(null)
  let button3Ref = $state<HTMLButtonElement | null>(null)
  let activeIndex = $state(1)
  let focusRequest = 0

  function getButtons() {
    return [button1Ref, button2Ref, button3Ref].filter((button): button is HTMLButtonElement => {
      return Boolean(button && !button.disabled)
    })
  }

  function focusButton(index: number) {
    const buttons = getButtons()
    if (buttons.length === 0) return
    const nextIndex = (index + buttons.length) % buttons.length
    activeIndex = nextIndex
    buttons[nextIndex]?.focus({ preventScroll: true })
  }

  async function focusDefaultButton() {
    const request = ++focusRequest
    await tick()
    requestAnimationFrame(() => {
      if (!open || request !== focusRequest) return
      focusButton(1)
    })
  }

  // Handle auto-focus when dialog opens - focus "Save & Close" (button 2)
  function handleOpenAutoFocus(e: Event) {
    e.preventDefault() // Prevent bits-ui default focus behavior
    activeIndex = 1
    void focusDefaultButton()
  }

  $effect(() => {
    if (open) {
      activeIndex = 1
      void focusDefaultButton()
    } else {
      focusRequest++
    }
  })

  // Register once while the component is mounted. The listener checks `open` at
  // key time, so the first Tab after opening cannot be missed by an open-state
  // effect that has not run yet.
  $effect(() => {
    function onKeydown(e: KeyboardEvent) {
      if (!open || e.altKey || e.ctrlKey || e.metaKey) return
      const isShiftTab = (e.key === 'Tab' || e.key === 'ISO_Left_Tab') && e.shiftKey
      const isTab = isShiftTab || (e.key === 'Tab' && !e.shiftKey)
      if (!getEnhancedKeyboardNavigation() && !isTab) return
      const isPrev = ['ArrowLeft', 'ArrowUp', 'h'].includes(e.key) || isShiftTab
      const isNext = ['ArrowRight', 'ArrowDown', 'l'].includes(e.key) || (e.key === 'Tab' && !e.shiftKey)
      if (!isPrev && !isNext) return

      const buttons = getButtons()
      if (buttons.length === 0) return

      e.preventDefault()
      e.stopImmediatePropagation()
      focusRequest++
      let idx = buttons.findIndex((b) => b === document.activeElement)
      if (idx === -1) idx = Math.min(activeIndex, buttons.length - 1)
      focusButton(isNext ? idx + 1 : idx - 1)
    }
    document.addEventListener('keydown', onKeydown, true)
    return () => document.removeEventListener('keydown', onKeydown, true)
  })

  function handleOpenChange(isOpen: boolean) {
    open = isOpen
    if (!isOpen) {
      onOption3?.()
    }
  }

  function handleOption1() {
    onOption1()
  }

  function handleOption2() {
    onOption2()
  }

  function handleOption3() {
    open = false
    onOption3?.()
  }

  const isLoading = $derived(loading !== null)
  const activeRingClass = 'ring-2 ring-ring ring-offset-2 ring-offset-background'
</script>

<AlertDialog.Root bind:open onOpenChange={handleOpenChange}>
  <AlertDialog.Content onOpenAutoFocus={handleOpenAutoFocus}>
    <AlertDialog.Header>
      <AlertDialog.Title>{title}</AlertDialog.Title>
      {#if description}
        <AlertDialog.Description>{description}</AlertDialog.Description>
      {/if}
    </AlertDialog.Header>

    <AlertDialog.Footer>
      <div class="flex gap-2 sm:flex-row">
        <!-- Option 1 (e.g., Discard) -->
        <button
          bind:this={button1Ref}
          onfocus={() => activeIndex = 0}
          onclick={handleOption1}
          disabled={isLoading}
          class="inline-flex items-center justify-center whitespace-nowrap rounded-md text-sm font-medium ring-offset-background transition-colors focus:outline-none focus:ring-2 focus:ring-ring focus:ring-offset-2 disabled:pointer-events-none disabled:opacity-50 h-10 px-4 py-2 {activeIndex === 0 ? activeRingClass : ''} {option1Variant === 'destructive' ? 'bg-destructive text-destructive-foreground hover:bg-destructive/90' : 'bg-primary text-primary-foreground hover:bg-primary/90'}"
        >
          {#if loading === 'option1'}
            <Icon icon="mdi:loading" class="w-4 h-4 mr-2 animate-spin" />
          {/if}
          {option1Label}
        </button>

        <!-- Option 2 (e.g., Save & Close) -->
        <button
          bind:this={button2Ref}
          onfocus={() => activeIndex = 1}
          onclick={handleOption2}
          disabled={isLoading}
          class="inline-flex items-center justify-center whitespace-nowrap rounded-md text-sm font-medium ring-offset-background transition-colors focus:outline-none focus:ring-2 focus:ring-ring focus:ring-offset-2 disabled:pointer-events-none disabled:opacity-50 h-10 px-4 py-2 {activeIndex === 1 ? activeRingClass : ''} {option2Variant === 'destructive' ? 'bg-destructive text-destructive-foreground hover:bg-destructive/90' : 'bg-primary text-primary-foreground hover:bg-primary/90'}"
        >
          {#if loading === 'option2'}
            <Icon icon="mdi:loading" class="w-4 h-4 mr-2 animate-spin" />
          {/if}
          {option2Label}
        </button>

        <!-- Option 3 (e.g., Keep Editing) - styled as cancel -->
        <button
          bind:this={button3Ref}
          onfocus={() => activeIndex = 2}
          onclick={handleOption3}
          disabled={isLoading}
          class="inline-flex items-center justify-center whitespace-nowrap rounded-md text-sm font-medium ring-offset-background transition-colors focus:outline-none focus:ring-2 focus:ring-ring focus:ring-offset-2 disabled:pointer-events-none disabled:opacity-50 h-10 px-4 py-2 border border-input bg-background hover:bg-accent hover:text-accent-foreground {activeIndex === 2 ? activeRingClass : ''}"
        >
          {option3Label}
        </button>
      </div>
    </AlertDialog.Footer>
  </AlertDialog.Content>
</AlertDialog.Root>
