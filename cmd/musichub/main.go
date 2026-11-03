package main

import (
	"context"
	"net"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/RomiChan/protobuf/rpc"
	"github.com/bihua-university/alisten/cmd/musichub/internal/service"
	"github.com/bihua-university/alisten/internal/logx"
	"github.com/bihua-university/alisten/internal/nacos"
)

func main() {
	if err := nacos.Init("musichub"); err != nil {
		logx.Fatal("initialize Nacos:", err)
	}
	logx.SetDebug(debugMode())
	service.Init()
	backend := service.NewBackend()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	listener, err := net.Listen("tcp", ":"+port("9090"))
	if err != nil {
		logx.Fatal("listen music RPC:", err)
	}
	defer listener.Close()
	if err := nacos.Register(ctx, "", listener.Addr().String()); err != nil {
		logx.Fatal("register musichub:", err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := nacos.Deregister(ctx, "", listener.Addr().String()); err != nil {
			logx.Error("deregister musichub:", err)
		}
	}()

	errors := make(chan error, 1)
	go func() { errors <- rpc.NewServer(service.NewServer(backend)).Serve(ctx, listener) }()
	logx.Info("musichub RPC ", listener.Addr())
	select {
	case <-ctx.Done():
	case err := <-errors:
		if err != nil {
			logx.Error("musichub serve:", err)
		}
		stop()
	}
}

func port(fallback string) string {
	if value := strings.TrimSpace(os.Getenv("PORT")); value != "" {
		return value
	}
	return fallback
}

func debugMode() bool {
	value := strings.TrimSpace(os.Getenv("DEBUG"))
	if value == "" {
		return false
	}
	debug, err := strconv.ParseBool(value)
	if err != nil {
		logx.Fatal("parse DEBUG:", err)
	}
	return debug
}
