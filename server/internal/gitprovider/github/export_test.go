package github

import "testing/fstest"

// ResetCache empties the SHA cache so tests that reuse repository names stay independent.
func ResetCache() {
	cache.mu.Lock()
	defer cache.mu.Unlock()
	cache.snapshots, cache.blobs = map[string]fstest.MapFS{}, map[string]string{}
	cache.snapOrder, cache.blobOrder = nil, nil
}
