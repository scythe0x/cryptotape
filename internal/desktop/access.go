package desktop

import "context"

// interrupt cancels active work immediately, including requests admitted before a license failure.
func (c *Controller) Interrupt() {
	c.blocked.Store(true)
	c.mu.Lock()
	if c.stop != nil {
		c.stop()
	}
	if c.cancel != nil {
		c.cancel()
	}
	if c.actionCancel != nil {
		c.actionCancel()
	}
	c.mu.Unlock()
	c.audio.Block(true)
}

// suspend waits for cancelled work without dispatching queued actions to terminals.
func (c *Controller) Suspend() {
	c.Interrupt()
	c.feedWG.Wait()
	c.stopListings()
	c.actionWG.Wait()
}

func (c *Controller) actionContext() context.Context {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.actions != nil {
		return c.actions
	}
	return context.Background()
}

// set the native access policy before starting feeds or admitting manual actions.
func (c *Controller) SetAccessCheck(check func() bool) {
	c.mu.Lock()
	c.accessCheck = check
	c.mu.Unlock()
}
func (c *Controller) actionsAllowed() bool {
	if c.blocked.Load() {
		return false
	}
	c.mu.RLock()
	check := c.accessCheck
	c.mu.RUnlock()
	return check == nil || check()
}
