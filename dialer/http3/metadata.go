package http3

import (
	"strings"
	"time"

	mdata "github.com/go-gost/core/metadata"
	mdx "github.com/hxdcloud/gost-x/metadata"
)

const (
	dialTimeout          = "dialTimeout"
	defaultAuthorizePath = "/authorize"
	defaultPushPath      = "/push"
	defaultPullPath      = "/pull"
)

const (
	defaultDialTimeout = 5 * time.Second
)

type metadata struct {
	dialTimeout   time.Duration
	authorizePath string
	pushPath      string
	pullPath      string
	host          string
}

func (d *http3Dialer) parseMetadata(md mdata.Metadata) (err error) {
	const (
		authorizePath = "authorizePath"
		pushPath      = "pushPath"
		pullPath      = "pullPath"
		host          = "host"
	)

	d.md.authorizePath = mdx.GetString(md, authorizePath)
	if !strings.HasPrefix(d.md.authorizePath, "/") {
		d.md.authorizePath = defaultAuthorizePath
	}
	d.md.pushPath = mdx.GetString(md, pushPath)
	if !strings.HasPrefix(d.md.pushPath, "/") {
		d.md.pushPath = defaultPushPath
	}
	d.md.pullPath = mdx.GetString(md, pullPath)
	if !strings.HasPrefix(d.md.pullPath, "/") {
		d.md.pullPath = defaultPullPath
	}

	d.md.host = mdx.GetString(md, host)
	return
}
