<h1 align="center">
🌐 ssrf 🔐
</h1>
<h4 align="center">A Go library for implementing SSRF protections</h4>
<p align="center">
    <a href="https://github.com/daenney/ssrf/actions/workflows/test.yaml)"><img src="https://github.com/daenney/ssrf/actions/workflows/test.yaml/badge.svg?branch=main" alt="Build Status"></a>
	<a href="https://github.com/daenney/ssrf/releases"><img src="https://img.shields.io/github/release/daenney/ssrf.svg" alt="Release"></a>
    <a href="https://goreportcard.com/report/code.dny.dev/ssrf"><img src="https://goreportcard.com/badge/code.dny.dev/ssrf" alt="Go report card"></a>
    <a href="https://pkg.go.dev/code.dny.dev/ssrf"><img src="https://pkg.go.dev/badge/code.dny.dev/ssrf.svg" alt="GoDoc"></a>
    <a href="LICENSE"><img src="https://img.shields.io/github/license/daenney/ssrf" alt="License: MIT"></a>
</p>

This package aims to help with implementing SSRF protections. It differs from
other packages in that it is kept automatically in sync with the IANA Special
Purpose Registries for both [IPv4][ipv4] and [IPv6][ipv6] with some additions.

The generation is done by [ssrfgen](cmd/ssrfgen).

A `Safe()` method is provided that you can hook into a `net.Dialer` to prevent
it from ever dialing to endpoints using certain protocols, destination ports
or IPs in certain networks.

Once you have the dialer, you can pass it into things like an `http.Transport`
to create an `http.Client` that won't allow requests to certain destinations.
It's worth pointing out that DNS resolution of the destination will still take
place, so that a name can be translated to an IP first.

## Usage

You can retrieve this package with:

```
go get code.dny.dev/ssrf
```

You can then call the `New()` method to get a Guardian and pass it on to your
`net.Dialer` of choice.

```go
s := ssrf.New()

dialer := &net.Dialer{
	Control: s.Safe,
}

transport := &http.Transport{
	DialContext: dialer.DialContext,
}

client := &http.Client{
	Transport: transport,
}
```

[ipv4]: https://www.iana.org/assignments/iana-ipv4-special-registry/iana-ipv4-special-registry.xhtml
[ipv6]: https://www.iana.org/assignments/iana-ipv6-special-registry/iana-ipv6-special-registry.xhtml
