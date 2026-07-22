package sync

import (
	"context"
	"errors"
	gosync "sync"
)

var ErrCoalesced = errors.New("sync trigger coalesced")

type Trigger int

const (
	TriggerScheduled Trigger = iota + 1
	TriggerIdle
	TriggerWake
	TriggerManual
)

type coordinatorRequest struct {
	trigger Trigger
	ctx     context.Context
	cancel  context.CancelFunc
	work    func(context.Context) error
	done    chan error
}

type coordinatorAccount struct {
	active      *coordinatorRequest
	pending     *coordinatorRequest
	manualQueue []*coordinatorRequest
}

// Coordinator serializes synchronization by account, keeps at most one
// follow-up request, and lets explicit user work preempt background work.
type Coordinator struct {
	mu       gosync.Mutex
	accounts map[string]*coordinatorAccount
}

func NewCoordinator() *Coordinator {
	return &Coordinator{accounts: make(map[string]*coordinatorAccount)}
}

func (c *Coordinator) Do(parent context.Context, accountID string, trigger Trigger, work func(context.Context) error) error {
	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel := context.WithCancel(parent)
	request := &coordinatorRequest{
		trigger: trigger,
		ctx:     ctx,
		cancel:  cancel,
		work:    work,
		done:    make(chan error, 1),
	}

	c.mu.Lock()
	state := c.accounts[accountID]
	if state == nil {
		state = &coordinatorAccount{}
		c.accounts[accountID] = state
	}
	if state.active == nil {
		state.active = request
		c.mu.Unlock()
		go c.execute(accountID, request)
	} else if trigger == TriggerManual {
		if trigger > state.active.trigger {
			state.active.cancel()
		}
		// Explicit user actions are never discarded. Automatic triggers still
		// retain only one pending follow-up behind this manual queue.
		state.manualQueue = append(state.manualQueue, request)
		c.mu.Unlock()
	} else {
		if trigger > state.active.trigger {
			state.active.cancel()
		}
		if state.pending == nil {
			state.pending = request
			c.mu.Unlock()
		} else if trigger > state.pending.trigger {
			replaced := state.pending
			state.pending = request
			c.mu.Unlock()
			replaced.cancel()
			replaced.done <- ErrCoalesced
		} else {
			c.mu.Unlock()
			cancel()
			return ErrCoalesced
		}
	}

	select {
	case err := <-request.done:
		return err
	case <-parent.Done():
		cancel()
		return parent.Err()
	}
}

func (c *Coordinator) execute(accountID string, request *coordinatorRequest) {
	err := request.ctx.Err()
	if err == nil {
		err = request.work(request.ctx)
	}
	request.cancel()
	request.done <- err

	c.mu.Lock()
	state := c.accounts[accountID]
	if state == nil || state.active != request {
		c.mu.Unlock()
		return
	}
	var next *coordinatorRequest
	if len(state.manualQueue) > 0 {
		next = state.manualQueue[0]
		state.manualQueue = state.manualQueue[1:]
	} else {
		next = state.pending
		state.pending = nil
	}
	if next == nil {
		delete(c.accounts, accountID)
		c.mu.Unlock()
		return
	}
	state.active = next
	c.mu.Unlock()

	go c.execute(accountID, next)
}
