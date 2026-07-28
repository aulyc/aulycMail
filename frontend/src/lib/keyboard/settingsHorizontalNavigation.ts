export interface HorizontalActionCandidate {
  action: string
  available: boolean
}

export function resolveHorizontalActionIndex(
  candidates: HorizontalActionCandidate[],
  preferredAction: string,
): number {
  const preferredIndex = candidates.findIndex(({ action }) => action === preferredAction)
  if (preferredIndex >= 0 && candidates[preferredIndex].available) return preferredIndex

  const pairedAction = preferredAction === 'move-up'
    ? 'move-down'
    : preferredAction === 'move-down'
      ? 'move-up'
      : ''
  const pairedIndex = candidates.findIndex(({ action, available }) => (
    available && action === pairedAction
  ))
  if (pairedIndex >= 0) return pairedIndex

  let closestIndex = -1
  let closestDistance = Number.POSITIVE_INFINITY
  candidates.forEach(({ available }, index) => {
    if (!available) return
    const distance = preferredIndex >= 0 ? Math.abs(index - preferredIndex) : index
    if (distance < closestDistance) {
      closestIndex = index
      closestDistance = distance
    }
  })
  return closestIndex
}
