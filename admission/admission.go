package admission

import (
	"net"
	"strconv"

	admission_pkg "github.com/go-gost/core/admission"
	"github.com/go-gost/core/logger"
	"github.com/hxdcloud/gost-x/internal/util/matcher"
)

type options struct {
	logger logger.Logger
}

type Option func(opts *options)

func LoggerOption(logger logger.Logger) Option {
	return func(opts *options) {
		opts.logger = logger
	}
}

type admission struct {
	matchers []matcher.Matcher
	reversed bool
	options  options
}

// NewAdmission creates and initializes a new Admission using matchers as its match rules.
// The rules will be reversed if the reversed is true.
func NewAdmission(reversed bool, matchers []matcher.Matcher, opts ...Option) admission_pkg.Admission {
	options := options{}
	for _, opt := range opts {
		opt(&options)
	}
	return &admission{
		matchers: matchers,
		reversed: reversed,
		options:  options,
	}
}

// NewAdmissionPatterns creates and initializes a new Admission using matcher patterns as its match rules.
// The rules will be reversed if the reverse is true.
func NewAdmissionPatterns(reversed bool, patterns []string, opts ...Option) admission_pkg.Admission {
	var matchers []matcher.Matcher
	for _, pattern := range patterns {
		if m := matcher.NewMatcher(pattern); m != nil {
			matchers = append(matchers, m)
		}
	}
	return NewAdmission(reversed, matchers, opts...)
}

func (p *admission) Admit(addr string) bool {
	if addr == "" || p == nil || len(p.matchers) == 0 {
		p.options.logger.Debugf("admission: %v is denied", addr)
		return false
	}

	// try to strip the port
	if host, port, _ := net.SplitHostPort(addr); host != "" && port != "" {
		if p, _ := strconv.Atoi(port); p > 0 { // port is valid
			addr = host
		}
	}

	var matched bool
	for _, matcher := range p.matchers {
		if matcher == nil {
			continue
		}
		if matcher.Match(addr) {
			matched = true
			break
		}
	}

	b := !p.reversed && matched ||
		p.reversed && !matched
	if !b {
		p.options.logger.Debugf("admission: %v is denied", addr)
	}
	return b
}
