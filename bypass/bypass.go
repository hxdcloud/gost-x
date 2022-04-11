package bypass

import (
	"bufio"
	"context"
	"io"
	"net"
	"strings"
	"sync"
	"time"

	bypass_pkg "github.com/go-gost/core/bypass"
	"github.com/go-gost/core/logger"
	"github.com/hxdcloud/gost-x/internal/loader"
	"github.com/hxdcloud/gost-x/internal/matcher"
)

type options struct {
	reverse     bool
	matchers    []string
	fileLoader  loader.Loader
	redisLoader loader.Loader
	period      time.Duration
	logger      logger.Logger
}

type Option func(opts *options)

func ReverseOption(reverse bool) Option {
	return func(opts *options) {
		opts.reverse = reverse
	}
}

func MatchersOption(matchers []string) Option {
	return func(opts *options) {
		opts.matchers = matchers
	}
}

func ReloadPeriodOption(period time.Duration) Option {
	return func(opts *options) {
		opts.period = period
	}
}

func FileLoaderOption(fileLoader loader.Loader) Option {
	return func(opts *options) {
		opts.fileLoader = fileLoader
	}
}

func RedisLoaderOption(redisLoader loader.Loader) Option {
	return func(opts *options) {
		opts.redisLoader = redisLoader
	}
}

func LoggerOption(logger logger.Logger) Option {
	return func(opts *options) {
		opts.logger = logger
	}
}

type bypass struct {
	ipMatcher       matcher.Matcher
	cidrMatcher     matcher.Matcher
	domainMatcher   matcher.Matcher
	wildcardMatcher matcher.Matcher
	mu              sync.RWMutex
	cancelFunc      context.CancelFunc
	options         options
}

// NewBypass creates and initializes a new Bypass.
// The rules will be reversed if the reverse option is true.
func NewBypass(opts ...Option) bypass_pkg.Bypass {
	var options options
	for _, opt := range opts {
		opt(&options)
	}

	ctx, cancel := context.WithCancel(context.TODO())

	bp := &bypass{
		cancelFunc: cancel,
		options:    options,
	}

	if err := bp.reload(ctx); err != nil {
		options.logger.Warnf("reload: %v", err)
	}
	if bp.options.period > 0 {
		go bp.periodReload(ctx)
	}

	return bp
}

func (bp *bypass) periodReload(ctx context.Context) error {
	period := bp.options.period
	if period < time.Second {
		period = time.Second
	}
	ticker := time.NewTicker(period)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := bp.reload(ctx); err != nil {
				bp.options.logger.Warnf("reload: %v", err)
				// return err
			}
			bp.options.logger.Debugf("bypass reload done")
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (bp *bypass) reload(ctx context.Context) error {
	v, err := bp.load(ctx)
	if err != nil {
		return err
	}
	patterns := append(bp.options.matchers, v...)

	var ips []net.IP
	var inets []*net.IPNet
	var domains []string
	var wildcards []string
	for _, pattern := range patterns {
		if ip := net.ParseIP(pattern); ip != nil {
			ips = append(ips, ip)
			continue
		}
		if _, inet, err := net.ParseCIDR(pattern); err == nil {
			inets = append(inets, inet)
			continue
		}
		if strings.ContainsAny(pattern, "*?") {
			wildcards = append(wildcards, pattern)
			continue
		}
		domains = append(domains, pattern)
	}

	bp.mu.Lock()
	defer bp.mu.Unlock()

	bp.ipMatcher = matcher.IPMatcher(ips)
	bp.cidrMatcher = matcher.CIDRMatcher(inets)
	bp.domainMatcher = matcher.DomainMatcher(domains)
	bp.wildcardMatcher = matcher.WildcardMatcher(wildcards)

	return nil
}

func (bp *bypass) load(ctx context.Context) (patterns []string, err error) {
	if bp.options.fileLoader != nil {
		r, er := bp.options.fileLoader.Load(ctx)
		if er != nil {
			bp.options.logger.Warnf("file loader: %v", er)
		}
		if v, _ := bp.parsePatterns(r); v != nil {
			patterns = append(patterns, v...)
		}
	}
	if bp.options.redisLoader != nil {
		r, er := bp.options.redisLoader.Load(ctx)
		if er != nil {
			bp.options.logger.Warnf("redis loader: %v", er)
		}
		if v, _ := bp.parsePatterns(r); v != nil {
			patterns = append(patterns, v...)
		}
	}

	return
}

func (bp *bypass) parsePatterns(r io.Reader) (patterns []string, err error) {
	if r == nil {
		return
	}

	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Text()
		if n := strings.IndexByte(line, '#'); n >= 0 {
			line = line[:n]
		}
		line = strings.TrimSpace(line)
		if line != "" {
			patterns = append(patterns, line)
		}
	}

	err = scanner.Err()
	return
}

func (bp *bypass) Contains(addr string) bool {
	if addr == "" || bp == nil {
		return false
	}

	// try to strip the port
	if host, _, _ := net.SplitHostPort(addr); host != "" {
		addr = host
	}

	matched := bp.matched(addr)

	b := !bp.options.reverse && matched ||
		bp.options.reverse && !matched
	if b {
		bp.options.logger.Debugf("bypass: %s", addr)
	}
	return b
}

func (bp *bypass) matched(addr string) bool {
	bp.mu.RLock()
	defer bp.mu.RUnlock()

	if ip := net.ParseIP(addr); ip != nil {
		return bp.ipMatcher.Match(addr) ||
			bp.cidrMatcher.Match(addr)
	}

	return bp.domainMatcher.Match(addr) ||
		bp.wildcardMatcher.Match(addr)
}

func (bp *bypass) Close() error {
	bp.cancelFunc()
	if bp.options.fileLoader != nil {
		bp.options.fileLoader.Close()
	}
	if bp.options.redisLoader != nil {
		bp.options.redisLoader.Close()
	}
	return nil
}
