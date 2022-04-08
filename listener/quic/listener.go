package quic

import (
	"context"
	"net"

	"github.com/go-gost/core/listener"
	"github.com/go-gost/core/logger"
	md "github.com/go-gost/core/metadata"
	metrics "github.com/go-gost/core/metrics/wrapper"
	quic_util "github.com/hxdcloud/gost-x/internal/util/quic"
	"github.com/hxdcloud/gost-x/registry"
	"github.com/lucas-clemente/quic-go"
)

func init() {
	registry.ListenerRegistry().Register("quic", NewListener)
}

type quicListener struct {
	ln      quic.Listener
	cqueue  chan net.Conn
	errChan chan error
	logger  logger.Logger
	md      metadata
	options listener.Options
}

func NewListener(opts ...listener.Option) listener.Listener {
	options := listener.Options{}
	for _, opt := range opts {
		opt(&options)
	}
	return &quicListener{
		logger:  options.Logger,
		options: options,
	}
}

func (l *quicListener) Init(md md.Metadata) (err error) {
	if err = l.parseMetadata(md); err != nil {
		return
	}

	addr := l.options.Addr
	if _, _, err := net.SplitHostPort(addr); err != nil {
		addr = net.JoinHostPort(addr, "0")
	}

	var laddr *net.UDPAddr
	laddr, err = net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return
	}

	var conn net.PacketConn
	conn, err = net.ListenUDP("udp", laddr)
	if err != nil {
		return
	}
	if l.md.cipherKey != nil {
		conn = quic_util.CipherPacketConn(conn, l.md.cipherKey)
	}

	config := &quic.Config{
		KeepAlive:            l.md.keepAlive,
		HandshakeIdleTimeout: l.md.handshakeTimeout,
		MaxIdleTimeout:       l.md.maxIdleTimeout,
		Versions: []quic.VersionNumber{
			quic.Version1,
			quic.VersionDraft29,
		},
	}

	tlsCfg := l.options.TLSConfig
	tlsCfg.NextProtos = []string{"http/3", "quic/v1"}

	ln, err := quic.Listen(conn, tlsCfg, config)
	if err != nil {
		return
	}

	l.ln = ln
	l.cqueue = make(chan net.Conn, l.md.backlog)
	l.errChan = make(chan error, 1)

	go l.listenLoop()

	return
}

func (l *quicListener) Accept() (conn net.Conn, err error) {
	var ok bool
	select {
	case conn = <-l.cqueue:
		conn = metrics.WrapConn(l.options.Service, conn)
	case err, ok = <-l.errChan:
		if !ok {
			err = listener.ErrClosed
		}
	}
	return
}

func (l *quicListener) Close() error {
	return l.ln.Close()
}

func (l *quicListener) Addr() net.Addr {
	return l.ln.Addr()
}

func (l *quicListener) listenLoop() {
	for {
		ctx := context.Background()
		session, err := l.ln.Accept(ctx)
		if err != nil {
			l.logger.Error("accept:", err)
			l.errChan <- err
			close(l.errChan)
			return
		}
		go l.mux(ctx, session)
	}
}

func (l *quicListener) mux(ctx context.Context, session quic.Session) {
	defer session.CloseWithError(0, "closed")

	for {
		stream, err := session.AcceptStream(ctx)
		if err != nil {
			l.logger.Error("accept stream:", err)
			return
		}

		conn := &quicConn{
			Stream: stream,
			laddr:  session.LocalAddr(),
			raddr:  session.RemoteAddr(),
		}
		select {
		case l.cqueue <- conn:
		case <-stream.Context().Done():
			stream.Close()
		default:
			stream.Close()
			l.logger.Warnf("connection queue is full, client %s discarded", session.RemoteAddr())
		}
	}
}
