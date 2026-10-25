package desktop

import "context"

func (c *Controller) actionContext() context.Context {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.actions != nil {
		return c.actions
	}
	return context.Background()
}

func (c *Controller) actionsActive() bool {
	return c.actionContext().Err() == nil
}
