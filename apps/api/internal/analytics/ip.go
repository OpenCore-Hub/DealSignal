package analytics

import "net/netip"

// parseIP parses and anonymises an IP address for privacy compliance.
// IPv4 addresses are truncated to /24 (last octet zeroed).
// IPv6 addresses are truncated to /48.
func parseIP(s string) *netip.Addr {
	if s == "" {
		return nil
	}
	addr, err := netip.ParseAddr(s)
	if err != nil {
		return nil
	}
	anonymized := anonymizeIP(addr)
	return &anonymized
}

// anonymizeIP zeroes the low-order bits of an IP address.
func anonymizeIP(addr netip.Addr) netip.Addr {
	if addr.Is4() {
		as4 := addr.As4()
		as4[3] = 0 // zero last octet (→ /24)
		return netip.AddrFrom4(as4)
	}
	// IPv6: zero last 80 bits (→ /48 prefix preserved)
	as16 := addr.As16()
	for i := 6; i < 16; i++ {
		as16[i] = 0
	}
	return netip.AddrFrom16(as16)
}
