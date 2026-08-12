package workspace

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"golang.org/x/net/dns/dnsmessage"
)

// errNoCNAME means the resolver answered but no CNAME RR exists (A/AAAA-only or NX).
var errNoCNAME = errors.New("no cname record")

// Public resolvers avoid Docker's embedded DNS, which follows A records and hides CNAME RRs.
var publicDNSResolvers = []string{"1.1.1.1:53", "8.8.8.8:53"}

func lookupFirstHopCNAME(ctx context.Context, hostname string) (string, error) {
	host := strings.ToLower(strings.TrimSpace(strings.TrimSuffix(hostname, ".")))
	if host == "" {
		return "", errNoCNAME
	}
	fqdn := host + "."

	var lastErr error
	for _, resolver := range publicDNSResolvers {
		rctx, cancel := context.WithTimeout(ctx, 2*time.Second)
		target, err := queryCNAMERR(rctx, resolver, fqdn)
		cancel()
		if err == nil && strings.TrimSpace(target) != "" {
			return target, nil
		}
		if errors.Is(err, errNoCNAME) {
			return "", errNoCNAME
		}
		if err != nil {
			lastErr = err
		}
	}
	if lastErr != nil {
		return "", lastErr
	}
	return "", errNoCNAME
}

func queryCNAMERR(ctx context.Context, resolver, fqdn string) (string, error) {
	name, err := dnsmessage.NewName(fqdn)
	if err != nil {
		return "", fmt.Errorf("dns name: %w", err)
	}
	id := uint16(time.Now().UnixNano())
	msg := dnsmessage.Message{
		Header: dnsmessage.Header{ID: id, RecursionDesired: true},
		Questions: []dnsmessage.Question{{
			Name:  name,
			Type:  dnsmessage.TypeCNAME,
			Class: dnsmessage.ClassINET,
		}},
	}
	packed, err := msg.Pack()
	if err != nil {
		return "", fmt.Errorf("pack dns query: %w", err)
	}

	d := net.Dialer{}
	conn, err := d.DialContext(ctx, "udp", resolver)
	if err != nil {
		return "", err
	}
	defer conn.Close()

	deadline, ok := ctx.Deadline()
	if !ok {
		deadline = time.Now().Add(2 * time.Second)
	}
	if err := conn.SetDeadline(deadline); err != nil {
		return "", err
	}
	if _, err := conn.Write(packed); err != nil {
		return "", err
	}

	buf := make([]byte, 1232)
	n, err := conn.Read(buf)
	if err != nil {
		return "", err
	}

	var parsed dnsmessage.Message
	if err := parsed.Unpack(buf[:n]); err != nil {
		return "", fmt.Errorf("unpack dns: %w", err)
	}
	if parsed.ID != id {
		return "", errors.New("dns id mismatch")
	}
	if parsed.RCode != dnsmessage.RCodeSuccess && parsed.RCode != dnsmessage.RCodeNameError {
		return "", fmt.Errorf("dns rcode %v", parsed.RCode)
	}

	for _, a := range parsed.Answers {
		if a.Header.Type != dnsmessage.TypeCNAME {
			continue
		}
		body, ok := a.Body.(*dnsmessage.CNAMEResource)
		if !ok {
			continue
		}
		target := strings.ToLower(strings.TrimSuffix(body.CNAME.String(), "."))
		if target != "" {
			return target, nil
		}
	}
	return "", errNoCNAME
}
