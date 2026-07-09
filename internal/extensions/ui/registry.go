package ui

import (
	"fmt"
	"sort"
	"sync"
	"sync/atomic"

	coreapi "github.com/aulyc/aulycmail/internal/core/api/v1"
)

// Registry stores built-in rail tab registrations. Safe for concurrent
// Register/Unregister/List from multiple goroutines.
type Registry struct {
	mu       sync.RWMutex
	nextID   atomic.Uint64
	railTabs map[uint64]coreapi.RailTabRequest
}

// NewRegistry constructs an empty Registry.
func NewRegistry() *Registry {
	return &Registry{
		railTabs: make(map[uint64]coreapi.RailTabRequest),
	}
}

// RegisterRailTab adds a rail tab. Returns an Unregister func that removes it.
func (r *Registry) RegisterRailTab(req coreapi.RailTabRequest) (coreapi.Unregister, error) {
	if req.ExtensionID == "" {
		return nil, fmt.Errorf("ui.RegisterRailTab: ExtensionID is required")
	}
	if req.Label == "" || req.Component == "" {
		return nil, fmt.Errorf("ui.RegisterRailTab: Label and Component are required")
	}
	id := r.nextID.Add(1)
	r.mu.Lock()
	r.railTabs[id] = req
	r.mu.Unlock()
	return r.unregisterFunc(id), nil
}

// ListRailTabs returns all registered rail tabs in Order ASC then registration
// order. The returned slice is a copy.
func (r *Registry) ListRailTabs() []coreapi.RailTabRequest {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]coreapi.RailTabRequest, 0, len(r.railTabs))
	for _, t := range r.railTabs {
		out = append(out, t)
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].Order < out[j].Order
	})
	return out
}

func (r *Registry) unregisterFunc(id uint64) coreapi.Unregister {
	return func() {
		r.mu.Lock()
		defer r.mu.Unlock()
		delete(r.railTabs, id)
	}
}

var _ coreapi.UI = (*Registry)(nil)
