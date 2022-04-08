package kcp

import (
	"encoding/json"

	mdata "github.com/go-gost/core/metadata"
	kcp_util "github.com/hxdcloud/gost-x/internal/util/kcp"
	mdx "github.com/hxdcloud/gost-x/metadata"
)

const (
	defaultBacklog = 128
)

type metadata struct {
	config  *kcp_util.Config
	backlog int
}

func (l *kcpListener) parseMetadata(md mdata.Metadata) (err error) {
	const (
		backlog = "backlog"
		config  = "config"
	)

	if m := mdx.GetStringMap(md, config); len(m) > 0 {
		b, err := json.Marshal(m)
		if err != nil {
			return err
		}
		cfg := &kcp_util.Config{}
		if err := json.Unmarshal(b, cfg); err != nil {
			return err
		}
		l.md.config = cfg
	}

	if l.md.config == nil {
		l.md.config = kcp_util.DefaultConfig
	}

	l.md.backlog = mdx.GetInt(md, backlog)
	if l.md.backlog <= 0 {
		l.md.backlog = defaultBacklog
	}

	return
}
