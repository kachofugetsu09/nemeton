package meeting

import (
	"context"
	"sync"
)

type Coordinator struct {
	service *Service
	wake    chan struct{}
	mu      sync.Mutex
	queued  map[string]bool
	pending []string
}

func NewCoordinator(service *Service) *Coordinator {
	return &Coordinator{service: service, wake: make(chan struct{}, 1), queued: make(map[string]bool)}
}

func (c *Coordinator) Enqueue(meetingID string) {
	c.mu.Lock()
	if c.queued[meetingID] {
		c.mu.Unlock()
		return
	}
	c.queued[meetingID] = true
	c.pending = append(c.pending, meetingID)
	c.mu.Unlock()
	select {
	case c.wake <- struct{}{}:
	default:
	}
}

func (c *Coordinator) Run(ctx context.Context) {
	for {
		meetingID, ok := c.next()
		if ok {
			_ = c.service.Run(ctx, meetingID)
			c.mu.Lock()
			delete(c.queued, meetingID)
			c.mu.Unlock()
			continue
		}
		select {
		case <-ctx.Done():
			return
		case <-c.wake:
		}
	}
}

func (c *Coordinator) next() (string, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.pending) == 0 {
		return "", false
	}
	meetingID := c.pending[0]
	if len(c.pending) == 1 {
		c.pending = nil
	} else {
		c.pending = c.pending[1:]
	}
	return meetingID, true
}

func (s *Service) Resumable(ctx context.Context) ([]string, error) {
	return s.store.ResumableMeetings(ctx)
}
