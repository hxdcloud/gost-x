package http

import (
	"net/http"

	mdata "github.com/go-gost/core/metadata"
	mdx "github.com/hxdcloud/gost-x/metadata"
)

type metadata struct {
	host   string
	header http.Header
}

func (d *obfsHTTPDialer) parseMetadata(md mdata.Metadata) (err error) {
	const (
		header = "header"
		host   = "host"
	)

	if m := mdx.GetStringMapString(md, header); len(m) > 0 {
		h := http.Header{}
		for k, v := range m {
			h.Add(k, v)
		}
		d.md.header = h
	}
	d.md.host = mdx.GetString(md, host)
	return
}
