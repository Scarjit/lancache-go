package recursive_dns_resolver

import (
	"go.elastic.co/ecszap"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"os"
	"testing"
)

func TestResolveDomain(t *testing.T) {
	encoderConfig := ecszap.NewDefaultEncoderConfig()
	var core zapcore.Core
	core = ecszap.NewCore(encoderConfig, os.Stdout, zap.DebugLevel)
	logger := zap.New(core, zap.AddCaller())
	zap.ReplaceGlobals(logger)

	domainsRealIpv4 := []string{
		"cache11-fra1.steamcontent.com",
		"camo.githubusercontent.com",
		"steamcdn-a.akamaihd.net",
		"example.com",
		"example.com",
		"google.com",
		"www.gchq.gov.uk",
		"dynts.pro",
	}

	domainsFakeIpv4 := []string{
		"aw9e4taihaw8e7aw3aosawf.com",
	}

	for _, domain := range domainsRealIpv4 {
		ip, err := ResolveDomain(domain, false, false)
		if err != nil {
			t.Fatal(err)
		}
		if len(ip) == 0 {
			t.Fatalf("no ip for %s", domain)
		}
		zap.S().Infof("%s -> %s\n\n", domain, ip)
	}

	for _, domain := range domainsFakeIpv4 {
		ip, err := ResolveDomain(domain, false, false)
		if err == nil {
			t.Fatalf("Fake domain should not resolve (%s -> %s)", domain, ip)
		}
	}

	domainsRealIpv6 := []string{
		"example.com",
		"example.com",
		"google.com",
		"dynts.pro",
	}

	domainsFakeIpv6 := []string{
		"aw9e4taihaw8e7aw3aosawf.com",
	}

	for _, domain := range domainsRealIpv6 {
		ip, err := ResolveDomain(domain, true, false)
		if err != nil {
			t.Fatal(err)
		}
		if len(ip) == 0 {
			t.Fatalf("no ip for %s", domain)
		}
		zap.S().Infof("%s -> %s\n\n", domain, ip)
	}

	for _, domain := range domainsFakeIpv6 {
		ip, err := ResolveDomain(domain, true, false)
		if err == nil {
			t.Fatalf("Fake domain should not resolve (%s -> %s)", domain, ip)
		}
	}
}
