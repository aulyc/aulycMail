export interface RailPane {
  id: string
  labelKey: string
  icon: string
}

const CONTACTS_PANE_ID = 'contacts'

export const BUILT_IN_RAIL_PANES: RailPane[] = [
  {
    id: CONTACTS_PANE_ID,
    labelKey: 'contacts.sidebar.title',
    icon: 'mdi:account-multiple',
  },
]

export const RAIL_PANE_ORDER = ['mail', ...BUILT_IN_RAIL_PANES.map(pane => pane.id)]
