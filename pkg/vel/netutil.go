package vel

import "net"

// Reserved CIDR ranges that are not covered by Go's net.IP methods.
var reservedCIDRs []*net.IPNet

func init() {
	for _, cidr := range []string{
		"100.64.0.0/10",  // Shared address space (RFC 6598)
		"192.0.0.0/24",   // IETF protocol assignments (RFC 6890)
		"192.0.2.0/24",   // TEST-NET-1 (RFC 5737)
		"198.18.0.0/15",  // Benchmarking (RFC 2544)
		"198.51.100.0/24", // TEST-NET-2 (RFC 5737)
		"203.0.113.0/24", // TEST-NET-3 (RFC 5737)
		"240.0.0.0/4",    // Reserved for future use
	} {
		_, ipNet, _ := net.ParseCIDR(cidr)
		reservedCIDRs = append(reservedCIDRs, ipNet)
	}
}

// IsPrivateOrReservedIP checks if an IP is in a private, loopback, link-local,
// or other reserved range. Use this to prevent SSRF when making outbound HTTP
// requests.
func IsPrivateOrReservedIP(ip net.IP) bool {
	if ip == nil {
		return true // treat nil as blocked
	}
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() || ip.IsMulticast() || ip.IsUnspecified() {
		return true
	}
	// IPv4 link-local 169.254.0.0/16 (already covered by IsLinkLocalUnicast,
	// but explicit for clarity)
	if ip4 := ip.To4(); ip4 != nil && ip4[0] == 169 && ip4[1] == 254 {
		return true
	}
	// Additional reserved ranges not covered by stdlib
	for _, ipNet := range reservedCIDRs {
		if ipNet.Contains(ip) {
			return true
		}
	}
	return false
}
