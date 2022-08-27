package root_hints

import (
	"fmt"
	"github.com/miekg/dns"
	"go.uber.org/zap"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

func resolveInternic() (string, error) {

	m1 := new(dns.Msg)
	m1.Id = dns.Id()
	m1.RecursionDesired = true
	m1.Question = make([]dns.Question, 1)
	m1.Question[0] = dns.Question{Name: "www.internic.net.", Qtype: dns.TypeA, Qclass: dns.ClassINET}

	c := new(dns.Client)
	in, _, err := c.Exchange(m1, "1.1.1.1:53")
	if err != nil {
		return "", err
	}

	if len(in.Answer) == 0 {
		return "", fmt.Errorf("no answer")
	}

	for _, rr := range in.Answer {
		if rr.Header().Rrtype == dns.TypeA {
			return rr.(*dns.A).A.String(), nil
		}
	}

	return "", fmt.Errorf("no A record")
}

var rootZoneRegex = regexp.MustCompile(`^([.a-z-A-Z0-9-]+)\s+(\d+)\s+(A+)\s+([\d:.a-f]+)$`)

func downloadRootHints() (serverIpv4 []string, serverIpv6 []string, err error) {
	var internicIp string
	internicIp, err = resolveInternic()
	if err != nil {
		return
	}

	url := fmt.Sprintf("http://%s/domain/named.root", internicIp)

	var req *http.Request
	req, err = http.NewRequest("GET", url, nil)
	if err != nil {
		return
	}

	req.Header.Add("Host", "www.internic.net")
	req.Header.Add("Accept", "*/*")
	req.Header.Add("User-Agent", "Test-Resolver by ferdinand@linnenberg.dev")
	req.Host = "internic.net"

	var res *http.Response
	res, err = http.DefaultClient.Do(req)
	if err != nil {
		return
	}

	var body []byte
	body, err = io.ReadAll(res.Body)
	if err != nil {
		return
	}
	err = res.Body.Close()
	if err != nil {
		return
	}

	// Iterate over body, removing lines starting with ;
	for _, s := range strings.Split(string(body), "\n") {
		if !strings.HasPrefix(s, ";") {
			matches := rootZoneRegex.FindStringSubmatch(s)
			if len(matches) == 0 {
				continue
			}
			if matches[3] == "A" {
				serverIpv4 = append(serverIpv4, matches[4])
			} else if matches[3] == "AAAA" {
				serverIpv6 = append(serverIpv6, matches[4])
			}
		}
	}

	return
}

var serversIpv4 []string
var serversIpv6 []string
var lastRootZoneUpdate int64

func updateRootZone() (err error) {
	if serversIpv4 == nil || serversIpv6 == nil {
		zap.S().Infof("Downloading root zone")
		serversIpv4, serversIpv6, err = downloadRootHints()
		if err != nil {
			return err
		}
		lastRootZoneUpdate = time.Now().Unix()
	}

	// Check if we need to update the root zone
	if time.Now().Unix()-lastRootZoneUpdate > 60*60*24 {
		zap.S().Infof("Updating root zone")
		serversIpv4, serversIpv6, err = downloadRootHints()
		if err != nil {
			return err
		}
		lastRootZoneUpdate = time.Now().Unix()
	}
	return nil
}

func GetRootServersCached() (ipv4 []string, ipv6 []string, err error) {
	err = updateRootZone()
	if err != nil {
		return
	}
	return serversIpv4, serversIpv6, err
}
