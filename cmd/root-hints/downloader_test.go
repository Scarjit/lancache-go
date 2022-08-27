package root_hints

import (
	"testing"
)

func TestResolveInternic(t *testing.T) {
	_, err := resolveInternic()
	if err != nil {
		t.Fatal(err)
	}
}

func TestDownloadRootHints(t *testing.T) {
	sipv4, sipv6, err := downloadRootHints()
	if err != nil {
		t.Fatal(err)
	}
	if len(sipv4) < 10 || len(sipv6) < 10 {
		t.Fatal("too few servers")
	}
}

func TestGetRootServersCached(t *testing.T) {
	sipv4, sipv6, err := GetRootServersCached()
	if err != nil {
		t.Fatal(err)
	}
	if len(sipv4) < 10 || len(sipv6) < 10 {
		t.Fatal("too few servers")
	}
}
