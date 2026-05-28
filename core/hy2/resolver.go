package hy2

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/apernet/hysteria/extras/v2/outbounds"
)

type ipv4OnlyResolver struct {
	next outbounds.PluggableOutbound
}

func newIPv4OnlyResolver(next outbounds.PluggableOutbound) outbounds.PluggableOutbound {
	return &ipv4OnlyResolver{next: next}
}

func (r *ipv4OnlyResolver) resolve(reqAddr *outbounds.AddrEx) {
	if ip := net.ParseIP(reqAddr.Host); ip != nil {
		reqAddr.ResolveInfo = &outbounds.ResolveInfo{}
		if ip4 := ip.To4(); ip4 != nil {
			reqAddr.ResolveInfo.IPv4 = ip4
		} else {
			reqAddr.ResolveInfo.Err = fmt.Errorf("no IPv4 address available")
		}
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	ips, err := net.DefaultResolver.LookupIP(ctx, "ip4", reqAddr.Host)
	if err != nil {
		reqAddr.ResolveInfo = &outbounds.ResolveInfo{Err: err}
		return
	}
	for _, ip := range ips {
		if ip4 := ip.To4(); ip4 != nil {
			reqAddr.ResolveInfo = &outbounds.ResolveInfo{IPv4: ip4}
			return
		}
	}
	reqAddr.ResolveInfo = &outbounds.ResolveInfo{Err: fmt.Errorf("no IPv4 address available")}
}

func (r *ipv4OnlyResolver) TCP(reqAddr *outbounds.AddrEx) (net.Conn, error) {
	r.resolve(reqAddr)
	return r.next.TCP(reqAddr)
}

func (r *ipv4OnlyResolver) UDP(reqAddr *outbounds.AddrEx) (outbounds.UDPConn, error) {
	r.resolve(reqAddr)
	return r.next.UDP(reqAddr)
}

func (r *ipv4OnlyResolver) CheckUDP(reqAddr *outbounds.AddrEx) error {
	r.resolve(reqAddr)
	return r.next.CheckUDP(reqAddr)
}
