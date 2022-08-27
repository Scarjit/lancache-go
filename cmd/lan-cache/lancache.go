package lan_cache

import (
	jsoniter "github.com/json-iterator/go"
	"go.uber.org/zap"
	"os"
	"path"
	"strings"
	"time"
)
import "github.com/go-git/go-git/v5"

func getCacheDomainsDir() string {
	dir, err := os.UserCacheDir()
	if err != nil {
		zap.S().Fatal(err)
	}
	p := path.Join(dir, "abs-resolver", "cache-domains")
	err = os.MkdirAll(p, os.ModeDir)
	if err != nil {
		zap.S().Fatal(err)
	}
	return p
}

func downloadCacheDomains() error {
	cachedir := getCacheDomainsDir()

	if _, err := os.Stat(path.Join(cachedir, "cache_domains.json")); err == nil {
		var repository *git.Repository
		repository, err = git.PlainOpen(cachedir)
		if err != nil {
			return err
		}
		err = repository.Fetch(
			&git.FetchOptions{
				RemoteName: "origin",
			})
		if err != nil {
			if err == git.NoErrAlreadyUpToDate {
				return nil
			}
			return err
		}
		var worktree *git.Worktree
		worktree, err = repository.Worktree()
		if err != nil {
			return err
		}
		err = worktree.Pull(&git.PullOptions{RemoteName: "origin", Force: true})
		if err != nil {
			if err == git.NoErrAlreadyUpToDate {
				return nil
			}
			return err
		}

		return nil
	}

	_, err := git.PlainClone(
		cachedir, false, &git.CloneOptions{
			URL:      "https://github.com/uklans/cache-domains.git",
			Progress: os.Stdout,
		})
	if err != nil {
		return err
	}
	return nil
}

func parseCacheDomainsJson() (CacheDomainsJson, error) {
	cachedir := getCacheDomainsDir()
	jsonFile := path.Join(cachedir, "cache_domains.json")

	bytes, err := os.ReadFile(jsonFile)
	if err != nil {
		return CacheDomainsJson{}, err
	}

	var cdjson CacheDomainsJson
	err = jsoniter.Unmarshal(bytes, &cdjson)
	if err != nil {
		return CacheDomainsJson{}, err
	}

	return cdjson, nil
}

var rdl []string
var lastUpdate int64

func GetRedirectList() ([]string, error) {

	// If lastUpdate is more than 24 hours ago, download the cache domains json file
	if rdl != nil && lastUpdate != 0 && lastUpdate > (time.Now().Unix()-(24*60*60)) {
		return rdl, nil
	}

	err := downloadCacheDomains()
	if err != nil {
		return nil, err
	}

	var cjd CacheDomainsJson
	cjd, err = parseCacheDomainsJson()
	if err != nil {
		return nil, err
	}

	var domains []string
	for _, domain := range cjd.CacheDomains {
		if domain.MixedContent {
			// Not going to handle HTTPS mixed content domains yet
			continue
		}
		for _, domainFile := range domain.DomainFiles {
			var f []string
			f, err = readDomainFile(domainFile)
			if err != nil {
				return nil, err
			}
			domains = append(domains, f...)
		}
	}
	rdl = domains
	lastUpdate = time.Now().Unix()

	return domains, nil
}

func readDomainFile(domainfile string) ([]string, error) {
	cachedir := getCacheDomainsDir()
	urlFile := path.Join(cachedir, domainfile)
	bytes, err := os.ReadFile(urlFile)
	if err != nil {
		return nil, err
	}
	var urls []string
	for _, line := range strings.Split(string(bytes), "\n") {
		if line == "" {
			continue
		}
		urls = append(urls, line)
	}
	return urls, nil
}

type CacheDomainsJson struct {
	CacheDomains []struct {
		Name         string   `json:"name"`
		Description  string   `json:"description"`
		DomainFiles  []string `json:"domain_files"`
		Notes        string   `json:"notes,omitempty"`
		MixedContent bool     `json:"mixed_content,omitempty"`
	} `json:"cache_domains"`
}
