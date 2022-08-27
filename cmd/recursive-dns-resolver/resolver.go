package recursive_dns_resolver

import (
	"crypto/sha512"
	"fmt"
	"github.com/miekg/dns"
	"github.com/patrickmn/go-cache"
	"go.uber.org/zap"
	"log"
	"math/rand"
	"net"
	lan_cache "resolver/cmd/lan-cache"
	root_hints "resolver/cmd/root-hints"
	"strings"
	"time"
)

var domainCacheIpv4 = cache.New(time.Minute*10, time.Minute*10)
var domainCacheIpv6 = cache.New(time.Minute*10, time.Minute*10)

var outboundIp net.IP

func GetOutboundIP() net.IP {
	if outboundIp != nil {
		return outboundIp
	}
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)

	outboundIp = localAddr.IP
	return localAddr.IP
}

func ResolveDomain(domain string, useIpv6 bool, skipRedirect bool) (ip []string, err error) {
	cacheKeyHasher := sha512.New()
	cacheKeyHasher.Write([]byte(domain))
	cacheKey := fmt.Sprintf("%x", cacheKeyHasher.Sum(nil))

	if useIpv6 {
		if ip, found := domainCacheIpv6.Get(cacheKey); found {
			zap.S().Debugf("Cached")
			return ip.([]string), nil
		}
	} else {
		if ip, found := domainCacheIpv4.Get(cacheKey); found {
			zap.S().Debugf("Cached")
			return ip.([]string), nil
		}
	}

	if !skipRedirect {
		redirectList, err := lan_cache.GetRedirectList()
		if err == nil {
			for _, s := range redirectList {
				xdomain := strings.TrimSuffix(domain, ".")
				if strings.Contains(s, "*") {
					sx := strings.Replace(s, "*", "", -1)
					if strings.HasSuffix(domain, sx) {
						zap.S().Debugf("Domain %s matches redirect (wildcard) %s\n", domain, sx)

						return []string{GetOutboundIP().String()}, nil
					}
				} else {
					if strings.EqualFold(xdomain, s) {
						zap.S().Debugf("Domain %s matches redirect %s\n", domain, s)
						return []string{GetOutboundIP().String()}, nil
					}
				}
			}
		} else {
			zap.S().Warnf("Failed to get redirect list: %s", err)
		}
	}

	rootIpv4, rootIpv6, err := root_hints.GetRootServersCached()
	if err != nil {
		return nil, err
	}
	if useIpv6 {
		ip, err = resolveRecursive(rootIpv6, domain, useIpv6, skipRedirect)
	} else {
		ip, err = resolveRecursive(rootIpv4, domain, useIpv6, skipRedirect)
	}
	if err != nil {
		return nil, err
	}
	if len(ip) > 0 {
		if useIpv6 {
			domainCacheIpv6.Set(cacheKey, ip, cache.DefaultExpiration)
		} else {
			domainCacheIpv4.Set(cacheKey, ip, cache.DefaultExpiration)
		}
	}

	return
}

func resolveRecursive(dnsServers []string, domain string, ipv6 bool, skipRedirect bool) (ip []string, err error) {
	if len(dnsServers) == 0 {
		return nil, fmt.Errorf("no dns servers")
	}
	zap.S().Debugf("Resolving %s\n", domain)
	zap.S().Debugf("Using DNS servers: %s\n", dnsServers)
	// Pick random server
	rand.Seed(time.Now().UnixNano())
	server := dnsServers[rand.Intn(len(dnsServers))]

	m1 := new(dns.Msg)
	m1.Id = dns.Id()
	m1.RecursionDesired = false
	m1.Question = make([]dns.Question, 1)
	if ipv6 {
		m1.Question[0] = dns.Question{Name: dns.Fqdn(domain), Qtype: dns.TypeAAAA, Qclass: dns.ClassINET}
	} else {
		m1.Question[0] = dns.Question{Name: dns.Fqdn(domain), Qtype: dns.TypeA, Qclass: dns.ClassINET}
	}

	c := new(dns.Client)
	var in *dns.Msg
	if ipv6 {
		in, _, err = c.Exchange(m1, fmt.Sprintf("[%s]:53", server))
	} else {
		in, _, err = c.Exchange(m1, fmt.Sprintf("%s:53", server))
	}
	if err != nil {
		return nil, err
	}

	//	fmt.Printf("%v\n", in)

	if len(in.Answer) > 0 {
		answers := make([]string, 0)
		for _, rr := range in.Answer {
			if ipv6 {
				if rr.Header().Rrtype == dns.TypeAAAA {
					answers = append(answers, rr.(*dns.AAAA).AAAA.String())
				}
			} else {
				if rr.Header().Rrtype == dns.TypeA {
					answers = append(answers, rr.(*dns.A).A.String())
				}
			}
			if rr.Header().Rrtype == dns.TypeCNAME {
				cname := rr.(*dns.CNAME).Target
				zap.S().Debugf("CNAME %s -> %s\n", domain, cname)
				return ResolveDomain(cname, ipv6, skipRedirect)
			}
		}
		return answers, nil
	} else {
		subServers := make([]string, 0)
		for _, rr := range in.Extra {
			if ipv6 {
				if rr.Header().Rrtype == dns.TypeAAAA {
					subServers = append(subServers, rr.(*dns.AAAA).AAAA.String())
				}
			} else {
				if rr.Header().Rrtype == dns.TypeA {
					subServers = append(subServers, rr.(*dns.A).A.String())
				}
			}
		}

		if len(subServers) == 0 {
			for _, rr := range in.Ns {
				if rr.Header().Rrtype == dns.TypeNS {
					nsDomain := rr.(*dns.NS).Ns
					var ips []string
					ips, err = ResolveDomain(nsDomain, false, skipRedirect)
					if err != nil {
						zap.S().Debugf("Failed to resolve %s: %s\n", nsDomain, err)
						continue
					}
					subServers = append(subServers, ips...)
				}
			}
		}

		ip, err = resolveRecursive(subServers, domain, ipv6, false)
		if err != nil {
			return nil, err
		}
		return ip, nil
	}
}
