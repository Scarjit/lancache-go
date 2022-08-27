package dns_server

import (
	"github.com/miekg/dns"
	"go.uber.org/zap"
	"golang.org/x/net/dns/dnsmessage"
	"net"
	recursive_dns_resolver "resolver/cmd/recursive-dns-resolver"
	"strconv"
	"strings"
	"time"
)

func Start(bindIp net.IP) {
	addr := net.UDPAddr{
		Port: 53,
		IP:   bindIp,
	}
	conn, err := net.ListenUDP("udp", &addr) // code does not block here
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	var buf [1024]byte
	for {
		var rlen int
		var remote *net.UDPAddr
		rlen, remote, err = conn.ReadFromUDP(buf[:])
		if err != nil {
			panic(err)
		}
		go dnsHandler(buf[:rlen], remote, conn)
	}

}

func dnsHandler(buf []byte, remote *net.UDPAddr, conn *net.UDPConn) {
	response := parseAndQuery(buf, remote)
	if response != nil {
		var packed []byte
		var err error
		packed, err = response.Pack()
		if err != nil {
			zap.S().Errorf("Failed to pack DNS response (%s)", err)
			return
		}
		_, err = conn.WriteToUDP(packed, remote)
		if err != nil {
			zap.S().Errorf("Failed to send response to %s (%s)", remote.String(), err)
		}
	}
}

func parseAndQuery(buf []byte, remote *net.UDPAddr) *dnsmessage.Message {
	now := time.Now()
	err := dns.IsMsg(buf)
	if err != nil {
		zap.S().Errorf("Received invalid DNS message from %s (%s)", remote.String(), err)
		return nil
	}

	var m dnsmessage.Message
	err = m.Unpack(buf)
	if err != nil {
		zap.S().Errorf("Failed to unpack DNS message from %s (%s)", remote.String(), err)
		return nil
	}

	var r dnsmessage.Message
	r.Response = true
	r.ID = m.ID
	r.Questions = m.Questions
	r.Authoritative = true
	r.RCode = dnsmessage.RCodeServerFailure

	if m.Response {
		zap.S().Errorf("Received response from %s", remote.String())
		r.RCode = dnsmessage.RCodeRefused
		return &r
	}
	if m.OpCode != 0 {
		zap.S().Errorf("Received non-query from %s", remote.String())
		r.RCode = dnsmessage.RCodeRefused
		return &r
	}
	if len(m.Questions) != 1 {
		zap.S().Errorf("Received non-question from %s", remote.String())
		r.RCode = dnsmessage.RCodeRefused
		return &r
	}
	q := m.Questions[0]

	if strings.Contains(q.Name.String(), "in-addr.arpa") {
		zap.S().Errorf("Received reverse DNS query from %s", remote.String())
		r.RCode = dnsmessage.RCodeNotImplemented
		return &r
	}
	if strings.Contains(q.Name.String(), ".fritz.box") {
		zap.S().Errorf("Received query for wierd frizt.box subdomain %s", remote.String())
		r.RCode = dnsmessage.RCodeRefused
		return &r
	}

	zap.S().Debugf("Received query for %s from %s", q.Name, remote.String())

	var domainV4 []string
	var domainV6 []string
	if q.Type == dnsmessage.TypeA {
		domainV4, err = recursive_dns_resolver.ResolveDomain(q.Name.String(), false, false)
		if err != nil {
			zap.S().Warnf("Failed to resolve domain %s (%s)", q.Name.String(), err)
			r.RCode = dnsmessage.RCodeNameError
			return &r
		}
	} else if q.Type == dnsmessage.TypeAAAA {
		domainV6, err = recursive_dns_resolver.ResolveDomain(q.Name.String(), true, false)
		if err != nil {
			zap.S().Warnf("Failed to resolve domain %s (%s)", q.Name.String(), err)
			r.RCode = dnsmessage.RCodeNameError
			return &r
		}
	} else {
		zap.S().Warnf("Received query for unknown type %d from %s", q.Type, remote.String())
		r.RCode = dnsmessage.RCodeNotImplemented
		return &r
	}

	zap.S().Infof("Resolved domain %s to %s & %s in %v", q.Name.String(), domainV4, domainV6, time.Since(now))

	var res []dnsmessage.Resource
	for _, s := range domainV4 {
		ip := net.ParseIP(s)
		if ip == nil {
			continue
		}
		splits := strings.Split(s, ".")
		if len(splits) != 4 {
			continue
		}

		a, _ := strconv.Atoi(splits[0])
		b, _ := strconv.Atoi(splits[1])
		c, _ := strconv.Atoi(splits[2])
		d, _ := strconv.Atoi(splits[3])

		res = append(
			res, dnsmessage.Resource{
				Header: dnsmessage.ResourceHeader{
					Name:  q.Name,
					Type:  q.Type,
					Class: q.Class,
				},
				Body: &dnsmessage.AResource{
					A: [4]byte{byte(a), byte(b), byte(c), byte(d)},
				},
			})
	}

	for _, s := range domainV6 {
		ip := net.ParseIP(s)
		if ip == nil {
			continue
		}
		// Normalize IPv6, adding 0000, until 8 segments
		//2a00:1450:400e:811::200e

		if strings.Contains(s, "::") {
			s = strings.Replace(s, "::", ":0000:", 1)
			for {
				if len(strings.Split(s, ":")) == 8 {
					break
				}
				s = strings.Replace(s, ":0000:", ":0000:0000:", 1)
			}
		}

		splits := strings.Split(s, ":")
		if len(splits) != 8 {
			zap.S().Errorf("Invalid IPv6 address %s (got %d splits)", s, len(splits))
			continue
		}

		var bx [16]byte
		var n int
		for _, split := range splits {
			if len(split) != 4 {
				split = strings.Repeat("0", 4-len(split)) + split
			}
			a := split[:2]
			b := split[2:]
			ix, _ := strconv.ParseUint(a, 16, 8)
			jx, _ := strconv.ParseUint(b, 16, 8)
			bx[n] = byte(ix)
			n++
			bx[n] = byte(jx)
			n++
		}

		res = append(
			res, dnsmessage.Resource{
				Header: dnsmessage.ResourceHeader{
					Name:  q.Name,
					Type:  q.Type,
					Class: q.Class,
				},
				Body: &dnsmessage.AAAAResource{
					AAAA: bx,
				},
			})
	}

	r.Answers = res
	r.RCode = dnsmessage.RCodeSuccess
	return &r
}
