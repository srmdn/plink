package server

import (
	"sync"

	"github.com/srmdn/plink/internal/db"
)

// settingsCache holds the effective site settings for this process. Plink runs
// as a single process, so one guarded copy is enough: without it a single
// public page issues three to five identical settings queries per render.
type settingsCache struct {
	mu       sync.RWMutex
	loaded   bool
	settings db.SiteSettings
}

func (c *settingsCache) get() (db.SiteSettings, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.settings, c.loaded
}

func (c *settingsCache) store(settings db.SiteSettings) {
	c.mu.Lock()
	c.settings, c.loaded = settings, true
	c.mu.Unlock()
}

func (c *settingsCache) invalidate() {
	c.mu.Lock()
	c.settings, c.loaded = db.SiteSettings{}, false
	c.mu.Unlock()
}
