package tun

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"sync"
	"time"

	"github.com/go-gost/core/chain"
	"github.com/go-gost/core/common/bufpool"
	"github.com/go-gost/core/handler"
	"github.com/go-gost/core/logger"
	md "github.com/go-gost/core/metadata"
	"github.com/hxdcloud/gost-x/internal/util/ss"
	tun_util "github.com/hxdcloud/gost-x/internal/util/tun"
	"github.com/hxdcloud/gost-x/registry"
	"github.com/shadowsocks/go-shadowsocks2/core"
	"github.com/shadowsocks/go-shadowsocks2/shadowaead"
	"github.com/songgao/water/waterutil"
	"golang.org/x/net/ipv4"
	"golang.org/x/net/ipv6"
)

func init() {
	registry.HandlerRegistry().Register("tun", NewHandler)
}

type tunHandler struct {
	group   *chain.NodeGroup
	routes  sync.Map
	exit    chan struct{}
	cipher  core.Cipher
	router  *chain.Router
	md      metadata
	options handler.Options
}

func NewHandler(opts ...handler.Option) handler.Handler {
	options := handler.Options{}
	for _, opt := range opts {
		opt(&options)
	}

	return &tunHandler{
		exit:    make(chan struct{}, 1),
		options: options,
	}
}

func (h *tunHandler) Init(md md.Metadata) (err error) {
	if err = h.parseMetadata(md); err != nil {
		return
	}

	if h.options.Auth != nil {
		method := h.options.Auth.Username()
		password, _ := h.options.Auth.Password()
		h.cipher, err = ss.ShadowCipher(method, password, h.md.key)
		if err != nil {
			return
		}
	}

	h.router = h.options.Router
	if h.router == nil {
		h.router = (&chain.Router{}).WithLogger(h.options.Logger)
	}

	return
}

// Forward implements handler.Forwarder.
func (h *tunHandler) Forward(group *chain.NodeGroup) {
	h.group = group
}

func (h *tunHandler) Handle(ctx context.Context, conn net.Conn, opts ...handler.HandleOption) error {
	defer os.Exit(0)
	defer conn.Close()

	log := h.options.Logger

	v, _ := conn.(md.Metadatable)
	if v == nil {
		err := errors.New("tun: wrong connection type")
		log.Error(err)
		return err
	}

	start := time.Now()
	log = log.WithFields(map[string]any{
		"remote": conn.RemoteAddr().String(),
		"local":  conn.LocalAddr().String(),
	})

	log.Infof("%s <> %s", conn.RemoteAddr(), conn.LocalAddr())
	defer func() {
		log.WithFields(map[string]any{
			"duration": time.Since(start),
		}).Infof("%s >< %s", conn.RemoteAddr(), conn.LocalAddr())
	}()

	network := "udp"
	var raddr net.Addr
	var err error

	target := h.group.Next()
	if target != nil {
		raddr, err = net.ResolveUDPAddr(network, target.Addr)
		if err != nil {
			log.Error(err)
			return err
		}
		log = log.WithFields(map[string]any{
			"dst": fmt.Sprintf("%s/%s", raddr.String(), raddr.Network()),
		})
		log.Infof("%s >> %s", conn.RemoteAddr(), target.Addr)
	}

	config := v.GetMetadata().Get("config").(*tun_util.Config)
	h.handleLoop(ctx, conn, raddr, config, log)
	return nil
}

func (h *tunHandler) handleLoop(ctx context.Context, conn net.Conn, addr net.Addr, config *tun_util.Config, log logger.Logger) {
	var tempDelay time.Duration
	for {
		err := func() error {
			var err error
			var pc net.PacketConn
			if addr != nil {
				cc, err := h.router.Dial(ctx, addr.Network(), "")
				if err != nil {
					return err
				}

				var ok bool
				pc, ok = cc.(net.PacketConn)
				if !ok {
					cc.Close()
					return errors.New("wrong connection type")
				}
			} else {
				laddr, _ := net.ResolveUDPAddr("udp", conn.LocalAddr().String())
				pc, err = net.ListenUDP("udp", laddr)
			}
			if err != nil {
				return err
			}

			if h.cipher != nil {
				pc = h.cipher.PacketConn(pc)
			}
			defer pc.Close()

			return h.transport(conn, pc, addr, config, log)
		}()
		if err != nil {
			log.Error(err)
		}

		select {
		case <-h.exit:
			return
		default:
		}

		if err != nil {
			if tempDelay == 0 {
				tempDelay = 1000 * time.Millisecond
			} else {
				tempDelay *= 2
			}
			if max := 6 * time.Second; tempDelay > max {
				tempDelay = max
			}
			time.Sleep(tempDelay)
			continue
		}
		tempDelay = 0
	}

}

func (h *tunHandler) transport(tun net.Conn, conn net.PacketConn, raddr net.Addr, config *tun_util.Config, log logger.Logger) error {
	errc := make(chan error, 1)

	go func() {
		for {
			err := func() error {
				b := bufpool.Get(h.md.bufferSize)
				defer bufpool.Put(b)

				n, err := tun.Read(*b)
				if err != nil {
					select {
					case h.exit <- struct{}{}:
					default:
					}
					return err
				}

				var src, dst net.IP
				if waterutil.IsIPv4((*b)[:n]) {
					header, err := ipv4.ParseHeader((*b)[:n])
					if err != nil {
						log.Error(err)
						return nil
					}
					log.Debugf("%s >> %s %-4s %d/%-4d %-4x %d",
						header.Src, header.Dst, ipProtocol(waterutil.IPv4Protocol((*b)[:n])),
						header.Len, header.TotalLen, header.ID, header.Flags)

					src, dst = header.Src, header.Dst
				} else if waterutil.IsIPv6((*b)[:n]) {
					header, err := ipv6.ParseHeader((*b)[:n])
					if err != nil {
						log.Warn(err)
						return nil
					}
					log.Debugf("%s >> %s %s %d %d",
						header.Src, header.Dst,
						ipProtocol(waterutil.IPProtocol(header.NextHeader)),
						header.PayloadLen, header.TrafficClass)

					src, dst = header.Src, header.Dst
				} else {
					log.Warn("unknown packet, discarded")
					return nil
				}

				// client side, deliver packet directly.
				if raddr != nil {
					_, err := conn.WriteTo((*b)[:n], raddr)
					return err
				}

				addr := h.findRouteFor(dst, config.Routes...)
				if addr == nil {
					log.Warnf("no route for %s -> %s", src, dst)
					return nil
				}

				log.Debugf("find route: %s -> %s", dst, addr)

				if _, err := conn.WriteTo((*b)[:n], addr); err != nil {
					return err
				}
				return nil
			}()

			if err != nil {
				errc <- err
				return
			}
		}
	}()

	go func() {
		for {
			err := func() error {
				b := bufpool.Get(h.md.bufferSize)
				defer bufpool.Put(b)

				n, addr, err := conn.ReadFrom(*b)
				if err != nil &&
					err != shadowaead.ErrShortPacket {
					return err
				}

				var src, dst net.IP
				if waterutil.IsIPv4((*b)[:n]) {
					header, err := ipv4.ParseHeader((*b)[:n])
					if err != nil {
						log.Warn(err)
						return nil
					}

					log.Debugf("%s >> %s %-4s %d/%-4d %-4x %d",
						header.Src, header.Dst, ipProtocol(waterutil.IPv4Protocol((*b)[:n])),
						header.Len, header.TotalLen, header.ID, header.Flags)

					src, dst = header.Src, header.Dst
				} else if waterutil.IsIPv6((*b)[:n]) {
					header, err := ipv6.ParseHeader((*b)[:n])
					if err != nil {
						log.Warn(err)
						return nil
					}

					log.Debugf("%s > %s %s %d %d",
						header.Src, header.Dst,
						ipProtocol(waterutil.IPProtocol(header.NextHeader)),
						header.PayloadLen, header.TrafficClass)

					src, dst = header.Src, header.Dst
				} else {
					log.Warn("unknown packet, discarded")
					return nil
				}

				// client side, deliver packet to tun device.
				if raddr != nil {
					_, err := tun.Write((*b)[:n])
					return err
				}

				rkey := ipToTunRouteKey(src)
				if actual, loaded := h.routes.LoadOrStore(rkey, addr); loaded {
					if actual.(net.Addr).String() != addr.String() {
						log.Debugf("update route: %s -> %s (old %s)",
							src, addr, actual.(net.Addr))
						h.routes.Store(rkey, addr)
					}
				} else {
					log.Warnf("no route for %s -> %s", src, addr)
				}

				if addr := h.findRouteFor(dst, config.Routes...); addr != nil {
					log.Debugf("find route: %s -> %s", dst, addr)

					_, err := conn.WriteTo((*b)[:n], addr)
					return err
				}

				if _, err := tun.Write((*b)[:n]); err != nil {
					select {
					case h.exit <- struct{}{}:
					default:
					}
					return err
				}
				return nil
			}()

			if err != nil {
				errc <- err
				return
			}
		}
	}()

	err := <-errc
	if err != nil && err == io.EOF {
		err = nil
	}
	return err
}

func (h *tunHandler) findRouteFor(dst net.IP, routes ...tun_util.Route) net.Addr {
	if v, ok := h.routes.Load(ipToTunRouteKey(dst)); ok {
		return v.(net.Addr)
	}
	for _, route := range routes {
		if route.Net.Contains(dst) && route.Gateway != nil {
			if v, ok := h.routes.Load(ipToTunRouteKey(route.Gateway)); ok {
				return v.(net.Addr)
			}
		}
	}
	return nil
}

var mIPProts = map[waterutil.IPProtocol]string{
	waterutil.HOPOPT:     "HOPOPT",
	waterutil.ICMP:       "ICMP",
	waterutil.IGMP:       "IGMP",
	waterutil.GGP:        "GGP",
	waterutil.TCP:        "TCP",
	waterutil.UDP:        "UDP",
	waterutil.IPv6_Route: "IPv6-Route",
	waterutil.IPv6_Frag:  "IPv6-Frag",
	waterutil.IPv6_ICMP:  "IPv6-ICMP",
}

func ipProtocol(p waterutil.IPProtocol) string {
	if v, ok := mIPProts[p]; ok {
		return v
	}
	return fmt.Sprintf("unknown(%d)", p)
}

type tunRouteKey [16]byte

func ipToTunRouteKey(ip net.IP) (key tunRouteKey) {
	copy(key[:], ip.To16())
	return
}
