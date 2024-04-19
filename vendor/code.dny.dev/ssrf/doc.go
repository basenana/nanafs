/*
This package aims to help with implementing SSRF protections.

A [Guardian.Safe] method is provided that you can hook into a [net.Dialer] to
prevent it from ever dialing to endpoints using certain protocols, destination
ports or IPs in certain networks.

Once you have the dialer, you can pass it into things like an [net/http.Transport]
to create an [net/http.Client] that won't allow requests to certain destinations.
It's worth pointing out that DNS resolution of the destination will still take
place, so that a name can be translated to an IP first.

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
*/
package ssrf
