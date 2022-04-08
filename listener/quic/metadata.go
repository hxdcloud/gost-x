package quic

import (
	"time"

	mdata "github.com/go-gost/core/metadata"
	mdx "github.com/hxdcloud/gost-x/metadata"
)

const (
	defaultBacklog = 128
)

type metadata struct {
	keepAlive        bool
	handshakeTimeout time.Duration
	maxIdleTimeout   time.Duration

	cipherKey []byte
	backlog   int
}

func (l *quicListener) parseMetadata(md mdata.Metadata) (err error) {
	const (
		keepAlive        = "keepAlive"
		handshakeTimeout = "handshakeTimeout"
		maxIdleTimeout   = "maxIdleTimeout"

		backlog   = "backlog"
		cipherKey = "cipherKey"
	)

	l.md.backlog = mdx.GetInt(md, backlog)
	if l.md.backlog <= 0 {
		l.md.backlog = defaultBacklog
	}

	if key := mdx.GetString(md, cipherKey); key != "" {
		l.md.cipherKey = []byte(key)
	}

	l.md.keepAlive = mdx.GetBool(md, keepAlive)
	l.md.handshakeTimeout = mdx.GetDuration(md, handshakeTimeout)
	l.md.maxIdleTimeout = mdx.GetDuration(md, maxIdleTimeout)

	return
}
