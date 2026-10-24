package main

import (
	"context"
	"flag"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/RomiChan/protobuf/rpc"
	backend "github.com/bihua-university/alisten/cmd/alisten-musicd/internal"
	"github.com/bihua-university/alisten/internal/base"
	"github.com/bihua-university/alisten/internal/logx"
	"github.com/bihua-university/alisten/internal/task"
)

func main() {
	config := flag.String("config", "musicd-config.json", "music provider configuration")
	listen := flag.String("listen", "127.0.0.1:9090", "music RPC listen address")
	httpListen := flag.String("http", "127.0.0.1:8081", "worker HTTP listen address")
	flag.Parse()
	if err := base.LoadConfig(*config); err != nil {
		logx.Fatal("load musicd config:", err)
	}
	logx.SetDebug(base.Config.Debug)
	backend.Init()
	task.Scheduler = task.NewServer(base.Config.Token)
	service := backend.NewBackend()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	listener, err := net.Listen("tcp", *listen)
	if err != nil {
		logx.Fatal("listen music RPC:", err)
	}
	defer listener.Close()
	mux := newHandler()
	httpServer := &http.Server{Addr: *httpListen, Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	errors := make(chan error, 2)
	go func() { errors <- rpc.NewServer(backend.NewServer(service)).Serve(ctx, listener) }()
	go func() { errors <- httpServer.ListenAndServe() }()
	logx.Info("alisten-musicd RPC ", listener.Addr(), " HTTP ", *httpListen)
	select {
	case <-ctx.Done():
	case err := <-errors:
		if err != nil && err != http.ErrServerClosed {
			logx.Error("musicd serve:", err)
		}
		stop()
	}
	shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = httpServer.Shutdown(shutdown)
}

func newHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /tasks/poll", task.Scheduler.PollTaskHandler)
	mux.HandleFunc("POST /tasks/result", task.Scheduler.SubmitResultHandler)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
	return mux
}
