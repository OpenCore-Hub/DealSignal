package workspace

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"golang.org/x/net/dns/dnsmessage"
)

func TestQueryCNAMERRFirstHop(t *testing.T) {
	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer pc.Close()

	go func() {
		buf := make([]byte, 2048)
		n, addr, err := pc.ReadFrom(buf)
		if err != nil {
			return
		}
		var req dnsmessage.Message
		if err := req.Unpack(buf[:n]); err != nil {
			return
		}
		name, err := dnsmessage.NewName("cname.dealsignal.com.")
		if err != nil {
			return
		}
		resp := dnsmessage.Message{
			Header: dnsmessage.Header{
				ID:                 req.ID,
				Response:           true,
				RecursionDesired:   true,
				RecursionAvailable: true,
			},
			Questions: req.Questions,
			Answers: []dnsmessage.Resource{{
				Header: dnsmessage.ResourceHeader{
					Name:  req.Questions[0].Name,
					Type:  dnsmessage.TypeCNAME,
					Class: dnsmessage.ClassINET,
					TTL:   60,
				},
				Body: &dnsmessage.CNAMEResource{CNAME: name},
			}},
		}
		packed, err := resp.Pack()
		if err != nil {
			return
		}
		_, _ = pc.WriteTo(packed, addr)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	got, err := queryCNAMERR(ctx, pc.LocalAddr().String(), "www.m3u.vip.")
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if got != "cname.dealsignal.com" {
		t.Fatalf("got %q", got)
	}
}

func TestQueryCNAMERRMissing(t *testing.T) {
	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer pc.Close()

	go func() {
		buf := make([]byte, 2048)
		n, addr, err := pc.ReadFrom(buf)
		if err != nil {
			return
		}
		var req dnsmessage.Message
		if err := req.Unpack(buf[:n]); err != nil {
			return
		}
		resp := dnsmessage.Message{
			Header: dnsmessage.Header{
				ID:               req.ID,
				Response:         true,
				RecursionDesired: true,
			},
			Questions: req.Questions,
		}
		packed, err := resp.Pack()
		if err != nil {
			return
		}
		_, _ = pc.WriteTo(packed, addr)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, err = queryCNAMERR(ctx, pc.LocalAddr().String(), "www.m3u.vip.")
	if !errors.Is(err, errNoCNAME) {
		t.Fatalf("got %v", err)
	}
}
