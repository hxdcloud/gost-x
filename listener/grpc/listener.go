package grpc

import (
	"net"

	admission "github.com/go-gost/core/admission/wrapper"
	"github.com/go-gost/core/listener"
	"github.com/go-gost/core/logger"
	md "github.com/go-gost/core/metadata"
	metrics "github.com/go-gost/core/metrics/wrapper"
	pb "github.com/hxdcloud/gost-x/internal/util/grpc/proto"
	"github.com/hxdcloud/gost-x/registry"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

func init() {
	registry.ListenerRegistry().Register("grpc", NewListener)
}

type grpcListener struct {
	addr    net.Addr
	server  *grpc.Server
	cqueue  chan net.Conn
	errChan chan error
	md      metadata
	logger  logger.Logger
	options listener.Options
}

func NewListener(opts ...listener.Option) listener.Listener {
	options := listener.Options{}
	for _, opt := range opts {
		opt(&options)
	}
	return &grpcListener{
		logger:  options.Logger,
		options: options,
	}
}

func (l *grpcListener) Init(md md.Metadata) (err error) {
	if err = l.parseMetadata(md); err != nil {
		return
	}

	ln, err := net.Listen("tcp", l.options.Addr)
	if err != nil {
		return
	}
	ln = metrics.WrapListener(l.options.Service, ln)
	ln = admission.WrapListener(l.options.Admission, ln)

	var opts []grpc.ServerOption
	if !l.md.insecure {
		opts = append(opts, grpc.Creds(credentials.NewTLS(l.options.TLSConfig)))
	}

	l.server = grpc.NewServer(opts...)
	l.addr = ln.Addr()
	l.cqueue = make(chan net.Conn, l.md.backlog)
	l.errChan = make(chan error, 1)

	pb.RegisterGostTunelServer(l.server, &server{
		cqueue:    l.cqueue,
		localAddr: l.addr,
		logger:    l.options.Logger,
	})

	go func() {
		err := l.server.Serve(ln)
		if err != nil {
			l.errChan <- err
		}
		close(l.errChan)
	}()

	return
}

func (l *grpcListener) Accept() (conn net.Conn, err error) {
	var ok bool
	select {
	case conn = <-l.cqueue:
	case err, ok = <-l.errChan:
		if !ok {
			err = listener.ErrClosed
		}
	}
	return
}

func (l *grpcListener) Close() error {
	l.server.Stop()
	return nil
}

func (l *grpcListener) Addr() net.Addr {
	return l.addr
}
