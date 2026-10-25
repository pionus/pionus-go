package content

import (
	"hash/fnv"
	"sync"
	"sync/atomic"

	"github.com/pionus/pionus-go/internal/model"
)

type renderKey struct {
	id     int64
	format string
	hash   uint64
	math   bool
}

var (
	renderCache sync.Map
	renderCount atomic.Int64
)

// RenderPostCached is RenderPost with a content-keyed cache. It keeps the
// parsed/normalized body and TOC in memory; edits change the hash and miss.
func RenderPostCached(p *model.Post) RenderedPost {
	if p.ID == 0 {
		return RenderPost(p)
	}

	h := fnv.New64a()
	h.Write([]byte(p.Content))
	key := renderKey{id: p.ID, format: p.Format, hash: h.Sum64(), math: p.Math}

	if value, ok := renderCache.Load(key); ok {
		return value.(RenderedPost)
	}

	rendered := RenderPost(p)
	if renderCount.Add(1) > 512 {
		renderCache.Range(func(k, _ any) bool {
			renderCache.Delete(k)
			return true
		})
		renderCount.Store(0)
	}
	renderCache.Store(key, rendered)
	return rendered
}
