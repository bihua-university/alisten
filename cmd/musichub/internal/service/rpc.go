package service

import (
	"context"
	"strings"
	"time"

	"github.com/RomiChan/protobuf/rpc"
	pb "github.com/bihua-university/alisten/internal/proto/musichub"
)

// RPCBackend is the musichub method set served over RPC.
type RPCBackend interface {
	Load(context.Context, *pb.SongRequest) (*pb.Song, error)
	Search(context.Context, *pb.SearchRequest) (*pb.MusicResult, error)
	Playlists(context.Context, *pb.SearchRequest) (*pb.PlaylistResult, error)
	SongList(context.Context, *pb.SearchRequest) (*pb.MusicResult, error)
	Loudness(context.Context, *pb.SongRequest) (*pb.CoefficientReply, error)
	Similar(context.Context, *pb.SongRequest) (*pb.IDsReply, error)
	CookieStatus(context.Context, *pb.Empty) (*pb.BoolReply, error)
	SetCookie(context.Context, *pb.CookieRequest) (*pb.BoolReply, error)
}

// Server implements rpc.Handler for the musichub RPC methods.
type Server struct {
	handlers map[string]rpc.Handler
}

// NewServer adapts backend methods to RPC handlers once.
func NewServer(backend RPCBackend) *Server {
	return &Server{
		handlers: map[string]rpc.Handler{
			"Load":         rpc.HandlerOf(backend.Load),
			"Search":       rpc.HandlerOf(backend.Search),
			"Playlists":    rpc.HandlerOf(backend.Playlists),
			"SongList":     rpc.HandlerOf(backend.SongList),
			"Loudness":     rpc.HandlerOf(backend.Loudness),
			"Similar":      rpc.HandlerOf(backend.Similar),
			"CookieStatus": rpc.HandlerOf(backend.CookieStatus),
			"SetCookie":    rpc.HandlerOf(backend.SetCookie),
		},
	}
}

// Handle implements rpc.Handler.
func (s *Server) Handle(ctx context.Context, req *rpc.Request) (any, error) {
	method, ok := strings.CutPrefix(req.Method, "musichub.v1.")
	if !ok {
		return nil, &rpc.Error{Code: rpc.MethodNotFound, Message: "unknown music RPC method"}
	}
	h, ok := s.handlers[method]
	if !ok {
		return nil, &rpc.Error{Code: rpc.MethodNotFound, Message: "unknown music RPC method"}
	}
	ctx, cancel := context.WithTimeout(ctx, defaultRequestTimeout)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return h.Handle(ctx, req)
}

const defaultRequestTimeout = 210 * time.Second
