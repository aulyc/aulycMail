package v1

// RailTabRequest registers a built-in pane in the left activity rail.
type RailTabRequest struct {
	ExtensionID string `json:"extensionId"`
	Label       string `json:"label"`
	Icon        string `json:"icon"`      // iconify identifier or asset path
	Component   string `json:"component"` // Svelte component identifier
	Order       int    `json:"order,omitempty"`
}

// UI is the minimal host surface needed by built-in rail panes.
type UI interface {
	RegisterRailTab(req RailTabRequest) (Unregister, error)
}
