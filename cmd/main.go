package main

import (
	"go.elastic.co/ecszap"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"net"
	"os"
	dns_server "resolver/cmd/dns-server"
	http_server "resolver/cmd/http-server"
	lan_cache "resolver/cmd/lan-cache"
	root_hints "resolver/cmd/root-hints"
	"time"
)

func configureLogger() *zap.Logger {
	loglevel, found := os.LookupEnv("LOGGING_LEVEL")
	encoderConfig := ecszap.NewDefaultEncoderConfig()
	var core zapcore.Core

	ll := zap.InfoLevel
	if found {
		switch loglevel {
		case "debug":
			ll = zap.DebugLevel
		case "info":
			ll = zap.InfoLevel
		case "warn":
			ll = zap.WarnLevel
		case "error":
			ll = zap.ErrorLevel
		}
	}

	core = ecszap.NewCore(encoderConfig, os.Stdout, ll)
	logger := zap.New(core, zap.AddCaller())
	zap.ReplaceGlobals(logger)
	return logger
}

func main() {
	logger := configureLogger()
	defer logger.Sync()
	_, _, err := root_hints.GetRootServersCached()
	if err != nil {
		panic(err)
	}

	bindIpDns, ok := os.LookupEnv("BIND_IP_DNS")
	if !ok {
		bindIpDns = "127.0.0.1"
	}
	bidns := net.ParseIP(bindIpDns)
	if bidns == nil {
		panic("Invalid DNS bind ip")
	}

	go dns_server.Start(bidns)
	go http_server.Start()

	for {
		_, err := lan_cache.GetRedirectList()
		if err == nil {
			break
		}
		time.Sleep(time.Second)
	}

	select {}
}
