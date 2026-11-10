package github

import (
	"sync"
	"testing/fstest"
)

// A commit's tree and its blobs never change, so what is read for a commit SHA can be
// kept without any risk of showing stale files. This is the one place a process keeps
// state in memory: it is a bounded copy of immutable data, and losing it (a restart,
// another pod) only costs extra GitHub requests. Nothing here is a credential.
const (
	maxCachedSnapshots = 64
	maxCachedBlobs     = 4096
)

type shaCache struct {
	mu        sync.Mutex
	snapshots map[string]fstest.MapFS // owner/repo/folder@treeSHA
	snapOrder []string
	blobs     map[string]string // owner/repo@blobSHA
	blobOrder []string
}

var cache = &shaCache{snapshots: map[string]fstest.MapFS{}, blobs: map[string]string{}}

func (c *shaCache) snapshot(key string) (fstest.MapFS, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	m, ok := c.snapshots[key]
	return m, ok
}

func (c *shaCache) putSnapshot(key string, m fstest.MapFS) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, ok := c.snapshots[key]; !ok {
		c.snapOrder = append(c.snapOrder, key)
		if len(c.snapOrder) > maxCachedSnapshots {
			delete(c.snapshots, c.snapOrder[0])
			c.snapOrder = c.snapOrder[1:]
		}
	}
	c.snapshots[key] = m
}

func (c *shaCache) blob(key string) (string, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	s, ok := c.blobs[key]
	return s, ok
}

func (c *shaCache) putBlob(key, content string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, ok := c.blobs[key]; !ok {
		c.blobOrder = append(c.blobOrder, key)
		if len(c.blobOrder) > maxCachedBlobs {
			delete(c.blobs, c.blobOrder[0])
			c.blobOrder = c.blobOrder[1:]
		}
	}
	c.blobs[key] = content
}
