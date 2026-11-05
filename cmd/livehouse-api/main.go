package main

import (
	"context"
	"errors"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/bihua-university/alisten/cmd/livehouse-api/internal/config"
	"github.com/bihua-university/alisten/cmd/livehouse-api/internal/rpc/musichub"
	"github.com/bihua-university/alisten/cmd/livehouse-api/internal/service"
	"github.com/bihua-university/alisten/internal/logx"
	"github.com/bihua-university/alisten/internal/nacos"
	livehouseRPC "github.com/bihua-university/alisten/internal/rpc/livehouse"
)

func discoverMusichubOnce(ctx context.Context) (string, error) {
	if namespace := strings.TrimSpace(os.Getenv("MUSICHUB_NACOS_NAMESPACE")); namespace != "" {
		return nacos.DiscoverInNamespace(ctx, "musichub", namespace)
	}
	return nacos.Discover(ctx, "musichub")
}

func discoverMusichub(ctx context.Context) (string, error) {
	for {
		address, err := discoverMusichubOnce(ctx)
		if err == nil {
			return address, nil
		}
		timer := time.NewTimer(500 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return "", ctx.Err()
		case <-timer.C:
		}
	}
}

func main() {
	if err := nacos.Init("livehouse-api"); err != nil {
		logx.Fatal("initialize Nacos:", err)
	}
	debug := debugMode()
	logx.SetDebug(debug)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	discoveryCtx, cancelDiscovery := context.WithTimeout(ctx, 30*time.Second)
	address, err := discoverMusichub(discoveryCtx)
	cancelDiscovery()
	if err != nil {
		logx.Fatal("discover musichub:", err)
	}
	musicClient := musichub.NewDiscoveryClient(address, discoverMusichubOnce, 0)
	defer musicClient.Close()

	livehouse := livehouseRPC.New(
		envOr("LIVEHOUSE_API", "http://127.0.0.1:8082"),
		&http.Client{Timeout: 3*time.Minute + 5*time.Second},
	)
	if err := ensurePersistentRooms(ctx, livehouse, config.ServerConfig.Get().Persist); err != nil {
		logx.Fatal("synchronize persistent rooms:", err)
	}

	listener, err := net.Listen("tcp", ":"+port("8081"))
	if err != nil {
		logx.Fatal("listen livehouse-api:", err)
	}
	if err := nacos.Register(ctx, "", listener.Addr().String()); err != nil {
		logx.Fatal("register livehouse-api:", err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := nacos.Deregister(ctx, "", listener.Addr().String()); err != nil {
			logx.Error("deregister livehouse-api:", err)
		}
	}()

	server := &http.Server{
		Handler:           service.NewHandler(musicClient, livehouse, debug),
		ReadHeaderTimeout: 10 * time.Second,
	}
	serveErrors := make(chan error, 1)
	go func() { serveErrors <- server.Serve(listener) }()
	logx.Info("livehouse-api HTTP ", listener.Addr())
	select {
	case <-ctx.Done():
	case err := <-serveErrors:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			logx.Error("livehouse-api serve:", err)
		}
		stop()
	}
	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelShutdown()
	if err := server.Shutdown(shutdownCtx); err != nil {
		logx.Error("livehouse-api shutdown:", err)
	}
}

func envOr(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}

func port(fallback string) string { return envOr("PORT", fallback) }

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

func ensurePersistentRooms(ctx context.Context, client *livehouseRPC.Client, rooms []config.PersistHouse) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	for _, room := range rooms {
		for {
			err := client.Call(ctx, "room.ensure", "", "", room, nil)
			if err == nil {
				break
			}
			logx.Error("ensure livehouse persistent room:", err)
			timer := time.NewTimer(500 * time.Millisecond)
			select {
			case <-ctx.Done():
				timer.Stop()
				return ctx.Err()
			case <-timer.C:
			}
		}
	}
	return nil
}
