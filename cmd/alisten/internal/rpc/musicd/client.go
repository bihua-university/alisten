// Package musicd provides the RPC client for alisten-musicd.
package musicd

import (
	"context"
	"time"

	"github.com/RomiChan/protobuf/rpc"
	pb "github.com/bihua-university/alisten/internal/proto/musicd"
)

// URL workers can take three minutes, and loudness analysis downloads audio.
const defaultTimeout = 210 * time.Second

// Client exposes the musicd.v1 RPC methods. Requests and replies are the
// generated protobuf messages; pooling, dialing, and write deadlines are
// handled by rpc.Client.
type Client struct {
	rpc *rpc.Client
}

// NewClient creates a pool that connects lazily to the TCP address.
// A non-positive timeout uses the default timeout of 210 seconds.
func NewClient(address string, timeout time.Duration) *Client {
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	return &Client{rpc.NewClient("tcp", address, rpc.ClientOptions{CallTimeout: timeout})}
}

// Close releases the pooled connections.
func (c *Client) Close() error { return c.rpc.Close() }

func (c *Client) Load(ctx context.Context, req *pb.SongRequest) (*pb.Song, error) {
	song, err := c.rpc.Call[pb.Song](ctx, "musicd.v1.Load", req)
	if err != nil {
		return nil, err
	}
	return &song, nil
}
func (c *Client) Search(ctx context.Context, req *pb.SearchRequest) (pb.MusicResult, error) {
	return c.rpc.Call[pb.MusicResult](ctx, "musicd.v1.Search", req)
}
func (c *Client) Playlists(ctx context.Context, req *pb.SearchRequest) (pb.PlaylistResult, error) {
	return c.rpc.Call[pb.PlaylistResult](ctx, "musicd.v1.Playlists", req)
}
func (c *Client) SongList(ctx context.Context, req *pb.SearchRequest) (pb.MusicResult, error) {
	return c.rpc.Call[pb.MusicResult](ctx, "musicd.v1.SongList", req)
}
func (c *Client) Loudness(ctx context.Context, source, id string) (float64, error) {
	r, err := c.rpc.Call[pb.CoefficientReply](ctx, "musicd.v1.Loudness", &pb.SongRequest{Source: source, Id: id})
	return r.Coefficient, err
}
func (c *Client) Similar(ctx context.Context, id string) ([]string, error) {
	r, err := c.rpc.Call[pb.IDsReply](ctx, "musicd.v1.Similar", &pb.SongRequest{Id: id})
	return r.Ids, err
}
func (c *Client) CookieStatus(ctx context.Context) (bool, error) {
	r, err := c.rpc.Call[pb.BoolReply](ctx, "musicd.v1.CookieStatus", &pb.Empty{})
	return r.Value, err
}
func (c *Client) SetCookie(ctx context.Context, cookie string) (bool, error) {
	r, err := c.rpc.Call[pb.BoolReply](ctx, "musicd.v1.SetCookie", &pb.CookieRequest{Cookie: cookie})
	return r.Value, err
}
