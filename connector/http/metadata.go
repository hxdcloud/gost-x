package http

import (
	"net/http"
	"time"

	mdata "github.com/go-gost/core/metadata"
	mdx "github.com/hxdcloud/gost-x/metadata"
)

type metadata struct {
	connectTimeout time.Duration
	header         http.Header
}

func (c *httpConnector) parseMetadata(md mdata.Metadata) (err error) {
	const (
		connectTimeout = "timeout"
		header         = "header"
	)

	c.md.connectTimeout = mdx.GetDuration(md, connectTimeout)

	if mm := mdx.GetStringMapString(md, header); len(mm) > 0 {
		hd := http.Header{}
		for k, v := range mm {
			hd.Add(k, v)
		}
		c.md.header = hd
	}

	return
}
