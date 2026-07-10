// Shared types for the contacts field sub-components.
//
// EmailRow mirrors ContactEditDialog's prior inline shape — extracted so
// AddContactDialog can drive the same field components with the same data
// shape.
//
// SlotConstraint encodes the per-source UI gating ContactFieldsForm applies
// to each repeater: max-items, max-of-a-type, info banner, or none. See
// docs/plan §"Part 1U" for the per-source table.

export type EmailRow = {
  email: string
  type: string
  isPrimary: boolean
}

// SlotConstraint encodes a per-field-component constraint. ContactFieldsForm
// derives one per repeater based on the active source type.
//
//   max          → hard cap on row count; Add button disables at length === max.
//   maxByType    → cap on rows matching `type` (case-insensitive); UI surfaces
//                  a warning + save guard rather than disabling Add (the user
//                  may add a row then change its type).
//   info         → informational note above the repeater; no gating.
//   none         → no UI changes.
export type SlotConstraint =
  | { kind: 'max'; max: number; reason: string }
  | { kind: 'maxByType'; type: string; max: number; reason: string }
  | { kind: 'info'; message: string }
  | { kind: 'none' }

// Source type strings used by field constraint helpers. Empty string means
// "no source picked yet" or "no constraints".
export type SourceTypeID = '' | 'local'

// FieldConstraints bundles the per-repeater constraints ContactFieldsForm
// passes down. Same keys as the repeater components.
export type FieldConstraints = {
  emails: SlotConstraint
  phones: SlotConstraint
  addresses: SlotConstraint
  urls: SlotConstraint
  impps: SlotConstraint
}
