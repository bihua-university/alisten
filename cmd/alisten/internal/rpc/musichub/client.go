// Package musichub provides the RPC client for musichub.
package musichub

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/RomiChan/protobuf/rpc"
	"github.com/bihua-university/alisten/internal/logx"
	pb "github.com/bihua-university/alisten/internal/proto/musichub"
)

const defaultTimeout = 210 * time.Second

type DiscoverFunc func(context.Context) (string, error)

// Client exposes the musichub.v1 RPC methods.
type Client struct {
	rpc *rpc.Client

	mu       sync.RWMutex
	discover DiscoverFunc
	dial     func(string, time.Duration) *rpc.Client
	address  string
	timeout  time.Duration
	closed   bool
}

// NewClient creates a pool that connects lazily to the TCP address.
// A non-positive timeout uses the default timeout of 210 seconds.
func NewClient(address string, timeout time.Duration) *Client {
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	return &Client{
		rpc:     rpc.NewClient("tcp", address, rpc.ClientOptions{CallTimeout: timeout}),
		dial:    newRPCClient,
		address: address,
		timeout: timeout,
	}
}

// NewDiscoveryClient creates a client that rediscovers its instance after RPC failures.
func NewDiscoveryClient(address string, discover DiscoverFunc, timeout time.Duration) *Client {
	client := NewClient(address, timeout)
	client.discover = discover
	return client
}

func newRPCClient(address string, timeout time.Duration) *rpc.Client {
	return rpc.NewClient("tcp", address, rpc.ClientOptions{CallTimeout: timeout})
}

// Close releases the pooled connections and prevents further calls.
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil
	}
	c.closed = true
	if c.rpc == nil {
		return nil
	}
	return c.rpc.Close()
}

func (c *Client) acquire() (*rpc.Client, func(), error) {
	c.mu.RLock()
	if c.closed {
		c.mu.RUnlock()
		return nil, nil, rpc.ErrClientClosed
	}
	return c.rpc, c.mu.RUnlock, nil
}

func (c *Client) refreshLocked(ctx context.Context) {
	address, err := c.discover(ctx)
	if err != nil {
		logx.Errorf("rediscover musichub: %v", err)
		return
	}
	next := c.dial(address, c.timeout)
	previous := c.rpc
	c.rpc = next
	c.address = address
	if previous != nil {
		_ = previous.Close()
	}
}

func (c *Client) afterCall(err error) {
	if err == nil || errors.Is(err, context.Canceled) {
		return
	}
	var remote *rpc.Error
	if errors.As(err, &remote) {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed || c.discover == nil {
		return
	}
	c.refreshLocked(context.Background())
}

func (c *Client) call[Req any, Resp any](ctx context.Context, method string, req *Req) (Resp, error) {
	client, release, err := c.acquire()
	if err != nil {
		var zero Resp
		return zero, err
	}
	response, err := client.Call[Resp](ctx, method, req)
	release()
	c.afterCall(err)
	return response, err
}

func (c *Client) Load(ctx context.Context, req *pb.SongRequest) (*pb.Song, error) {
	song, err := c.call[pb.SongRequest, pb.Song](ctx, "musichub.v1.Load", req)
	if err != nil {
		return nil, err
	}
	return &song, nil
}
func (c *Client) Search(ctx context.Context, req *pb.SearchRequest) (pb.MusicResult, error) {
	return c.call[pb.SearchRequest, pb.MusicResult](ctx, "musichub.v1.Search", req)
}
func (c *Client) Playlists(ctx context.Context, req *pb.SearchRequest) (pb.PlaylistResult, error) {
	return c.call[pb.SearchRequest, pb.PlaylistResult](ctx, "musichub.v1.Playlists", req)
}
func (c *Client) SongList(ctx context.Context, req *pb.SearchRequest) (pb.MusicResult, error) {
	return c.call[pb.SearchRequest, pb.MusicResult](ctx, "musichub.v1.SongList", req)
}
func (c *Client) Loudness(ctx context.Context, source, id string) (float64, error) {
	r, err := c.call[pb.SongRequest, pb.CoefficientReply](ctx, "musichub.v1.Loudness", &pb.SongRequest{Source: source, Id: id})
	return r.Coefficient, err
}
func (c *Client) Similar(ctx context.Context, id string) ([]string, error) {
	r, err := c.call[pb.SongRequest, pb.IDsReply](ctx, "musichub.v1.Similar", &pb.SongRequest{Id: id})
	return r.Ids, err
}
func (c *Client) CookieStatus(ctx context.Context) (bool, error) {
	r, err := c.call[pb.Empty, pb.BoolReply](ctx, "musichub.v1.CookieStatus", &pb.Empty{})
	return r.Value, err
}
func (c *Client) SetCookie(ctx context.Context, cookie string) (bool, error) {
	r, err := c.call[pb.CookieRequest, pb.BoolReply](ctx, "musichub.v1.SetCookie", &pb.CookieRequest{Cookie: cookie})
	return r.Value, err
}
