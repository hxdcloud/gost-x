package rudp

import (
	"time"

	mdata "github.com/go-gost/core/metadata"
	mdx "github.com/hxdcloud/gost-x/metadata"
)

const (
	defaultTTL            = 5 * time.Second
	defaultReadBufferSize = 1500
	defaultReadQueueSize  = 128
	defaultBacklog        = 128
)

type metadata struct {
	ttl            time.Duration
	readBufferSize int
	readQueueSize  int
	backlog        int
}

func (l *rudpListener) parseMetadata(md mdata.Metadata) (err error) {
	const (
		ttl            = "ttl"
		readBufferSize = "readBufferSize"
		readQueueSize  = "readQueueSize"
		backlog        = "backlog"
	)

	l.md.ttl = mdx.GetDuration(md, ttl)
	if l.md.ttl <= 0 {
		l.md.ttl = defaultTTL
	}
	l.md.readBufferSize = mdx.GetInt(md, readBufferSize)
	if l.md.readBufferSize <= 0 {
		l.md.readBufferSize = defaultReadBufferSize
	}

	l.md.readQueueSize = mdx.GetInt(md, readQueueSize)
	if l.md.readQueueSize <= 0 {
		l.md.readQueueSize = defaultReadQueueSize
	}

	l.md.backlog = mdx.GetInt(md, backlog)
	if l.md.backlog <= 0 {
		l.md.backlog = defaultBacklog
	}

	return
}
