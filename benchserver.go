package main

import (
	"net"
	"strings"
	"sync/atomic"
	"time"

	"github.com/alecthomas/kingpin"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/valyala/fasthttp"
	"go.uber.org/zap"
)

var listenAddress = kingpin.Flag("listen", "port to use for benchmarks").
	Default(":8080").
	Short('l').
	String()
var serverType = kingpin.Flag("type", "server type to bring up").
	Default("http").
	Short('t').
	String()
var responseSize = kingpin.Flag("size", "size of response in bytes").
	Default("1024").
	Short('s').
	Uint()

func http(logger *zap.Logger) {
	var requests uint64
	start := time.Now()
	response := strings.Repeat("a", int(*responseSize))
	logger.Info("starting HTTP server on", zap.String("addr", *listenAddress))
	go func() {
		for {
			time.Sleep(time.Second)
			logger.Info("requests handled", zap.Uint64("requests", atomic.LoadUint64(&requests)), zap.Duration("runtime", time.Since(start)))
		}
	}()
	err := fasthttp.ListenAndServe(*listenAddress, func(c *fasthttp.RequestCtx) {
		defer atomic.AddUint64(&requests, 1)
		_, werr := c.WriteString(response)
		if werr != nil {
			logger.Error("error writing response", zap.Error(werr))
		}
	})
	if err != nil {
		logger.Error("server error", zap.Error(err))
	}
}

func ip(logger *zap.Logger, network string) {
	counter := 0
	start := time.Now()
	decodingLayer := layers.LayerTypeIPv4
	if strings.HasPrefix(network, "ip6") {
		decodingLayer = layers.LayerTypeIPv6
	}
	pc, err := net.ListenPacket(network, *listenAddress)
	if err != nil {
		logger.Fatal("error opening connection", zap.Error(err))
	}
	defer pc.Close()

	for {
		buf := make([]byte, 1024)
		n, addr, err := pc.ReadFrom(buf)
		if err != nil {
			continue
		}
		counter++
		packetData := gopacket.NewPacket(buf, decodingLayer, gopacket.Default)
		if packetData.ErrorLayer() != nil {
			logger.Error("error decoding packet", zap.Error(packetData.ErrorLayer().Error()))
		}
		logger.Info("ip packet received", zap.Any("packet", packetData), zap.Int("counter", counter), zap.Any("addr", addr), zap.Any("size", n), zap.Duration("runtime", time.Since(start)))
	}
}

func main() {
	kingpin.Parse()
	logger, err := zap.NewProduction()
	if err != nil {
		panic(err)
	}
	logger.Info("starting server", zap.String("type", *serverType), zap.String("listen", *listenAddress))
	switch *serverType {
	case "http":
		http(logger)
	default:
		ip(logger, *serverType)
	}
}
