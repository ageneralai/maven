//go:build android

// Package dnsfix overrides net.DefaultResolver on Android builds.
// Go's pure-Go DNS resolver reads Android userland resolv.conf which points to [::1]:53.
// Android's netd DNS proxy only listens on 127.0.0.1:53 (IPv4), so IPv6 UDP
// queries fail with connection refused. This init wires a resolver that dials
// Google's public DNS over IPv4 instead.
package dnsfix

import (
	"context"
	"net"
)

func init() {
	net.DefaultResolver = &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, "udp4", "8.8.8.8:53")
		},
	}
}
