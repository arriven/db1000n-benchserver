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

func udp(logger *zap.Logger, network string) {
	udpADDR, err := net.ResolveUDPAddr(network, *listenAddress)
	if err != nil {
		logger.Fatal("cannot resolve listen address", zap.Error(err))
	}
	pc, err := net.ListenUDP(network, udpADDR)
	if err != nil {
		logger.Fatal("cannot listen", zap.Error(err))
	}
	defer pc.Close()

	for {
		buf := make([]byte, 1024)
		n, addr, err := pc.ReadFrom(buf)
		if err != nil {
			continue
		}
		logger.Info("got message", zap.ByteString("buf", buf), zap.Int("size", n), zap.Any("addr", addr))
	}
}

func tcp(logger *zap.Logger, network string) {
	var bytesRead uint64
	addr, err := net.ResolveTCPAddr(network, *listenAddress)
	if err != nil {
		logger.Fatal("cannot resolve listen address", zap.Error(err))
	}
	l, err := net.ListenTCP(network, addr)
	if err != nil {
		logger.Fatal("cannot listen", zap.Error(err))
	}
	defer l.Close()

	start := time.Now()
	response := strings.Repeat("a", int(*responseSize))
	go func() {
		for {
			time.Sleep(time.Second)
			logger.Info("bytes read", zap.Uint64("bytes", bytesRead), zap.Duration("duration", time.Since(start)))
		}
	}()

	for {
		conn, err := l.Accept()
		if err != nil {
			continue
		}

		go func(conn net.Conn) {
			for {
				buf := make([]byte, 1024)
				n, err := conn.Read(buf)
				if err != nil {
					continue
				}
				atomic.AddUint64(&bytesRead, uint64(n))
				_, err = conn.Write([]byte(response))
				if err != nil {
					continue
				}
			}
		}(conn)
	}
}

func ip(logger *zap.Logger, network string) {
	counter := 0
	start := time.Now()
	decodingLayer := layers.LayerTypeUDP
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
	case "udp":
		udp(logger, *serverType)
	case "tcp":
		tcp(logger, *serverType)
	default:
		ip(logger, *serverType)
	}
}
