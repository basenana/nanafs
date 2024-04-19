package ssrf

import (
	"errors"
	"fmt"
	"net/netip"
	"strings"
	"syscall"

	"golang.org/x/exp/slices"
)

var (
	// ErrProhibitedNetwork is returned when trying to dial a destination whose
	// network type is not in our allow list
	ErrProhibitedNetwork = errors.New("prohibited network type")
	// ErrProhibitedPort is returned when trying to dial a destination on a port
	// number that's not in our allow list
	ErrProhibitedPort = errors.New("prohibited port number")
	// ErrProhibitedIP is returned when trying to dial a destionation whose IP
	// is on our deny list
	ErrProhibitedIP = errors.New("prohibited IP address")
	// ErrInvalidHostPort is returned when [netip.ParseAddrPort] is unable to
	// parse our destination into its host and port constituents
	ErrInvalidHostPort = errors.New("invalid host:port pair")
)

// Option sets an option on a Guardian
type Option = func(g *Guardian)

// WithAllowedV4Prefixes adds explicitly allowed IPv4 prefixes.
// Any prefixes added here will be checked before the built-in
// deny list and as such can be used to allow connections to
// otherwise denied prefixes.
//
// This function overrides the allowed IPv4 prefixes, it does not accumulate.
func WithAllowedV4Prefixes(prefixes ...netip.Prefix) Option {
	return func(g *Guardian) {
		g.allowedv4Prefixes = prefixes
	}
}

// WithAllowedV6Prefixes adds explicitly allowed IPv6 prefixes.
// Any prefixes added here will be checked before the checks on
// global unicast range membership, the denied prefixes from
// [WithDeniedV6Prefixes] and the built-in deny list within the
// global unicast space. This can be used to allow connections to
// ranges outside of the global unicast range or connections to
// otherwise denied prefixes within the global unicast range.
//
// This function should be called with [IPv6NAT64Prefix] as one of the
// prefixes, if you run in an IPv6-only environment but provide IPv4
// connectivity through a combination of DNS64+NAT64. The NAT64 prefix
// is outside of the IPv6 global unicast range and as such blocked by
// default. Allowing it is typically harmless in dual-stack setups as
// your clients need an explicit route for 64:ff9b::/96 configured which
// won't be the case by default. Beware that allowing this prefix may
// allow for an address like 64:ff9b::7f00:1, i.e 127.0.0.1 mapped to
// NAT64. A NAT64 gateway should drop this. Ideally a DNS64 server
// would never generate an address for an RFC1918 IP in an A-record.
//
// This function overrides the allowed IPv6 prefixes, it does not accumulate.
func WithAllowedV6Prefixes(prefixes ...netip.Prefix) Option {
	return func(g *Guardian) {
		g.allowedv6Prefixes = prefixes
	}
}

// WithDeniedV4Prefixes allows denying IPv4 prefixes in case you want to deny
// more than the built-in set of denied prefixes. These prefixes are checked
// before checking the prefixes listed in [IPv4DeniedPrefixes].
//
// This function overrides the denied IPv4 prefixes, it does not accumulate.
func WithDeniedV4Prefixes(prefixes ...netip.Prefix) Option {
	return func(g *Guardian) {
		g.deniedv4Prefixes = prefixes
	}
}

// WithDeniedV6Prefixes allows denying IPv6 prefixes in case you want to deny
// more than the built-in set of denied prefixes within the global unicast
// range. These prefixes are checked before checking the prefixes listed in
// [IPv6DeniedPrefixes] but after the [IPv6GlobalUnicast] membership check.
//
// This function overrides the denied IPv6 prefixes, it does not accumulate.
func WithDeniedV6Prefixes(prefixes ...netip.Prefix) Option {
	return func(g *Guardian) {
		g.deniedv6Prefixes = prefixes
	}
}

// WithPorts allows overriding which destination ports are considered valid. By
// default only requests to 80 and 443 are permitted.
//
// This function overrides the allowed ports, it does not accumulate.
func WithPorts(ports ...uint16) Option {
	return func(g *Guardian) {
		g.ports = ports
	}
}

// WithAnyPort allows requests to any port number. It is equivalent to calling
// [WithPorts] without any arguments.
func WithAnyPort() Option {
	return func(g *Guardian) {
		g.ports = nil
	}
}

// WithNetworks allows overriding which network types/protocols are considered
// valid. By default only tcp4 and tcp6 are permitted.
//
// This function overrides the allowed networks, it does not accumulate.
func WithNetworks(networks ...string) Option {
	return func(g *Guardian) {
		g.networks = networks
	}
}

// WithAnyNetwork allows requests to any network. It is equivalent to calling
// [WithNetworks] without any arguments.
func WithAnyNetwork() Option {
	return func(g *Guardian) {
		g.networks = nil
	}
}

// Guardian will help ensure your network service isn't able to connect to
// certain network/protocols, ports or IP addresses. Once a Guardian has been
// created it is safe for concurrent use, but must not be modified.
//
// The Guardian returned by [New] should be set as the [net.Dialer.Control]
// function.
type Guardian struct {
	networks []string
	ports    []uint16

	allowedv4Prefixes []netip.Prefix
	allowedv6Prefixes []netip.Prefix
	deniedv4Prefixes  []netip.Prefix
	deniedv6Prefixes  []netip.Prefix

	strNetworks string
	strPorts    string
}

// New returns a Guardian initialised and ready to keep you safe
//
// It is initialised with some defaults:
//   - tcp4 and tcp6 are considered the only valid networks/protocols
//   - 80 and 443 are considered the only valid ports
//   - For IPv4, any prefix in the IANA Special Purpose Registry for IPv4 is
//     denied
//   - For IPv6, any prefix outside of the IPv6 Global Unicast range is denied,
//     as well as any prefix in the IANA Special Purpose Registry for IPv6
//     that falls within the IPv6 Global Unicast range
//
// Networks and ports can be overridden with [WithNetworks] and [WithPorts] to
// specify different ones, or [WithAnyNetwork] and [WithAnyPort] to
// disable checking for those entirely.
//
// For prefixes [WithAllowedV4Prefixes], [WithAllowedV6Prefixes]
// [WithDeniedV4Prefixes] and [WithDeniedV6Prefixes] can be used to customise
// which prefixes are additionally allowed or denied.
//
// [Guardian.Safe] details the order in which things are checked.
func New(opts ...Option) *Guardian {
	g := &Guardian{
		networks: []string{"tcp4", "tcp6"},
		ports:    []uint16{80, 443},
	}

	for _, opt := range opts {
		opt(g)
	}

	g.deniedv4Prefixes = append(g.deniedv4Prefixes, IPv4DeniedPrefixes...)
	g.deniedv6Prefixes = append(g.deniedv6Prefixes, IPv6DeniedPrefixes...)

	if len(g.networks) < 1 {
		g.strNetworks = "any"
	} else {
		g.strNetworks = strings.Join(g.networks, ", ")
	}
	if len(g.ports) < 1 {
		g.strPorts = "any"
	} else {
		// There are more efficient and less hideous ways of doing this, but we only do it once
		g.strPorts = strings.Trim(strings.Replace(fmt.Sprint(g.ports), " ", ", ", -1), "[]")
	}

	return g
}

// Safe is the function that should be passed in the [net.Dialer]'s Control field
//
// This function checks a number of things, in sequence:
//   - Does the network string match the permitted protocols? If not, deny the request
//   - Does the port match one of our permitted ports? If not, deny the request
//
// Moving on, we then check if the IP we're given is an IPv6 address. IPv4-mapped-IPv6
// addresses are considered as being an IPv6 address.
//
// For IPv6 we then check:
//   - Is the IP part of one of the prefixes passed in through [WithAllowedV6Prefixes]? If
//     so, allow the request
//   - Is the IP part of the IPv6 Global Unicast range? If not, deny the request
//   - Is the IP part of one of the prefixes passed in through [WithDeniedV6Prefixes] or
//     within the denied prefixes in the IPv6 Global Unicast range? If so deny the request
//
// If the IP is not IPv6, the it's IPv4 and so we check:
//   - Is the IP part of one of the prefixes passed in through [WithAllowedV4Prefixes]? If
//     so, allow the request
//   - Is the IP part of one of the prefixes passed in through [WithDeniedV4Prefixes] or
//     within the built-in denied IPv4 prefixes? If so, deny the request
//
// If nothing matched, the request is permitted.
func (g *Guardian) Safe(network string, address string, _ syscall.RawConn) error {
	if g.networks != nil {
		if !slices.Contains(g.networks, network) {
			return fmt.Errorf("%w: %s is not a permitted network type (%s)", ErrProhibitedNetwork, network, g.strNetworks)
		}
	}

	ipport, err := netip.ParseAddrPort(address)
	if err != nil {
		return fmt.Errorf("%w: could not parse %s: %s", ErrInvalidHostPort, address, err)
	}

	if g.ports != nil {
		port := ipport.Port()
		if !slices.Contains(g.ports, port) {
			return fmt.Errorf("%w: %d is not a permitted port (%s)", ErrProhibitedPort, port, g.strPorts)
		}
	}

	ip := ipport.Addr()

	if ip.Is6() {
		for _, net := range g.allowedv6Prefixes {
			if net.Contains(ip) {
				return nil
			}
		}

		// "fast path", in that anything outside of the IPv6 Global Unicast
		// range is something we should never connect to
		if !IPv6GlobalUnicast.Contains(ip) {
			return fmt.Errorf("%w: %s is not a permitted destination as it's outside of the IPv6 Global Unicast range", ErrProhibitedIP, ip)
		}

		for _, net := range g.deniedv6Prefixes {
			if net.Contains(ip) {
				return fmt.Errorf("%w: %s is not a permitted destination (denied by: %s)", ErrProhibitedIP, ip, net.String())
			}
		}
		return nil
	}

	// Since it's not IPv6, it's IPv4. Is6 catches IPv4-mapped IPv6 addresses
	for _, net := range g.allowedv4Prefixes {
		if net.Contains(ip) {
			return nil
		}
	}
	for _, net := range g.deniedv4Prefixes {
		if net.Contains(ip) {
			return fmt.Errorf("%w: %s is not a permitted destination (denied by: %s)", ErrProhibitedIP, ip, net.String())
		}
	}

	return nil
}
