package dns

import (
	"net"
	"time"

	mdata "github.com/go-gost/core/metadata"
	mdx "github.com/hxdcloud/gost-x/metadata"
)

const (
	defaultTimeout    = 5 * time.Second
	defaultBufferSize = 1024
)

type metadata struct {
	readTimeout time.Duration
	ttl         time.Duration
	timeout     time.Duration
	clientIP    net.IP
	// nameservers
	dns []string
}

func (h *dnsHandler) parseMetadata(md mdata.Metadata) (err error) {
	const (
		readTimeout = "readTimeout"
		ttl         = "ttl"
		timeout     = "timeout"
		clientIP    = "clientIP"
		dns         = "dns"
	)

	h.md.readTimeout = mdx.GetDuration(md, readTimeout)
	h.md.ttl = mdx.GetDuration(md, ttl)
	h.md.timeout = mdx.GetDuration(md, timeout)
	if h.md.timeout <= 0 {
		h.md.timeout = defaultTimeout
	}
	sip := mdx.GetString(md, clientIP)
	if sip != "" {
		h.md.clientIP = net.ParseIP(sip)
	}
	h.md.dns = mdx.GetStrings(md, dns)

	return
}
