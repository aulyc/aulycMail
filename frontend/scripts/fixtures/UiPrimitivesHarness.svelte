<script lang="ts">
  import {
    AlertDialogAction,
    AlertDialogCancel,
    AlertDialogContent,
    AlertDialogDescription,
    AlertDialogFooter,
    AlertDialogHeader,
    AlertDialogTitle,
  } from '$lib/components/ui/alert-dialog'
  import BoolSelect from '$lib/components/ui/bool-select/BoolSelect.svelte'
  import { DialogDescription, DialogFooter } from '$lib/components/ui/dialog'
  import {
    DropdownMenuContent,
    DropdownMenuItem,
    DropdownMenuSeparator,
  } from '$lib/components/ui/dropdown-menu'

  let {
    preventCloseAutoFocus = false,
    checked = $bindable(false),
    disabled = false,
    onAction,
    onCancel,
    onSelect,
    onCheckedChange,
    onOpenAutoFocus,
  } = $props<{
    preventCloseAutoFocus?: boolean
    checked?: boolean
    disabled?: boolean
    onAction?: (event: MouseEvent) => void
    onCancel?: (event: MouseEvent) => void
    onSelect?: () => void
    onCheckedChange?: (checked: boolean) => void
    onOpenAutoFocus?: (event: Event) => void
  }>()
</script>

<section data-alert-wrappers>
  <AlertDialogContent class="alert-content-custom" {preventCloseAutoFocus} {onOpenAutoFocus}>
    <AlertDialogHeader class="alert-header-custom">
      <AlertDialogTitle class="alert-title-custom">Alert title</AlertDialogTitle>
      <AlertDialogDescription class="alert-description-custom">Alert description</AlertDialogDescription>
    </AlertDialogHeader>
    <AlertDialogFooter class="alert-footer-custom">
      <AlertDialogCancel class="alert-cancel-custom" onclick={onCancel} {disabled}>Cancel</AlertDialogCancel>
      <AlertDialogAction class="alert-action-custom" onclick={onAction} {disabled}>Confirm</AlertDialogAction>
    </AlertDialogFooter>
  </AlertDialogContent>
</section>

<section data-dropdown-wrappers>
  <DropdownMenuContent class="dropdown-content-custom" side="top" align="end" sideOffset={9}>
    <DropdownMenuItem class="dropdown-item-custom" inset {disabled} {onSelect}>Dropdown item</DropdownMenuItem>
    <DropdownMenuSeparator class="dropdown-separator-custom" />
  </DropdownMenuContent>
</section>

<section data-dialog-static-wrappers>
  <DialogDescription class="dialog-description-custom">Dialog description</DialogDescription>
  <DialogFooter class="dialog-footer-custom">Dialog footer</DialogFooter>
</section>

<section data-bool-select-wrapper>
  <BoolSelect bind:checked {disabled} {onCheckedChange} class="bool-custom" />
</section>
