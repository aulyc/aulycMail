function normalizedContactText(value: string | null | undefined): string {
  return (value || '').trim().toLocaleLowerCase()
}

/**
 * The primary line already falls back to the email when the display name is
 * empty. Only show an email subtitle when it adds distinct information.
 */
export function shouldShowContactEmail(
  displayName: string | null | undefined,
  email: string | null | undefined,
): boolean {
  const normalizedName = normalizedContactText(displayName)
  const normalizedEmail = normalizedContactText(email)
  return normalizedName !== '' && normalizedEmail !== '' && normalizedName !== normalizedEmail
}
