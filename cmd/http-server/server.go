package http_server

import (
	"go.uber.org/zap"
	"net/http"
	recursive_dns_resolver "resolver/cmd/recursive-dns-resolver"
)

func Start() {
	http.HandleFunc("/", httpHandler)
	err := http.ListenAndServe(":80", nil)
	if err != nil {
		zap.S().Fatal(err)
	}
}

func httpHandler(responseWriter http.ResponseWriter, request *http.Request) {
	origin := request.Host
	origin += request.URL.Path
	zap.S().Infof("Origin: %s", origin)
	zap.S().Infof("Header: %s", request.Header)
	zap.S().Infof("Body: %s", request.Body)
	zap.S().Infof("Method: %s", request.Method)
	/*
		{"log.level":"info","@timestamp":"2022-08-26T15:57:43.312+0200","log.origin":{"file.name":"http-server/server.go","file.line":19},"message":"Origin: cache8-fra1.steamcontent.com/depot/431961/manifest/2239709941380521483/5/6181091970455626236","ecs.version":"1.6.0"}
		{"log.level":"info","@timestamp":"2022-08-26T15:57:43.312+0200","log.origin":{"file.name":"http-server/server.go","file.line":20},"message":"Header: map[Accept:[text/html,**;q=0.9] Accept-Charset:[ISO-8859-1,utf-8,*;q=0.7] Accept-Encoding:[gzip,identity,*;q=0] User-Agent:[Valve/Steam HTTP Client 1.0] X-Steam-Proxy:[LANCache]]","ecs.version":"1.6.0"}
		{"log.level":"info","@timestamp":"2022-08-26T15:57:43.313+0200","log.origin":{"file.name":"http-server/server.go","file.line":21},"message":"Body: {}","ecs.version":"1.6.0"}
		{"log.level":"info","@timestamp":"2022-08-26T15:57:43.313+0200","log.origin":{"file.name":"http-server/server.go","file.line":22},"message":"Method: GET","ecs.version":"1.6.0"}
	*/

	domain, err := recursive_dns_resolver.ResolveDomain(origin, false, true)
	if err != nil {
		zap.S().Error(err)
		return
	}
	zap.S().Infof("Actual domain: %s", domain)
}
