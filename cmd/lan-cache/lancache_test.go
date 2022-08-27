package lan_cache

import (
	"fmt"
	"testing"
)

func TestGetCacheDir(t *testing.T) {
	fmt.Printf("%s", getCacheDomainsDir())
}

func TestDownloadCacheDomains(t *testing.T) {
	err := downloadCacheDomains()
	if err != nil {
		t.Error(err)
	}
}

func TestParseCacheDomainsJson(t *testing.T) {
	cdj, err := parseCacheDomainsJson()
	if err != nil {
		t.Error(err)
	}
	if len(cdj.CacheDomains) == 0 {
		t.Error("empty cdj")
	}
}

func TestCreateRedirectList(t *testing.T) {
	domains, err := GetRedirectList()
	if err != nil {
		t.Error(err)
	}
	if len(domains) == 0 {
		t.Error("empty domains")
	}
	fmt.Printf("%d domains\n", len(domains))
}
