package mpipe

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/hashicorp/golang-lru/v2/expirable"

	"github.com/bihua-university/alisten/internal/logx"
	"github.com/bihua-university/alisten/internal/music/netease"
)

const (
	DefaultCacheSize = 64
	DefaultCacheTTL  = 30 * time.Minute
	streamBufSize    = 32 * 1024
)

// outbound 包装一个可读取的流及其元数据，用于统一响应写出。
type outbound struct {
	r             io.Reader
	contentType   string
	contentLength int64
}

// cached 是缓存中的纯数据实体，不持有运行时状态。
// 与 outbound 分离是因为 outbound.r 只能读取一次；
// 每次命中缓存时都从 data 创建新的 bytes.NewReader，保证可重复读取。
type cached struct {
	data          []byte
	contentType   string
	contentLength int64
}

func (c *cached) outbound() *outbound {
	return &outbound{r: bytes.NewReader(c.data), contentType: c.contentType, contentLength: c.contentLength}
}

// downloader 是 Pipe 依赖的上游下载链接提供者，便于测试注入 mock。
type downloader interface {
	GetDownloadURL(songID string) (string, error)
}

type Pipe struct {
	client   downloader
	mu       sync.Mutex
	inflight map[string]*inflighting
	cache    *expirable.LRU[string, *cached]
}

// inflighting 从上游拉取数据并缓存，支持多个 reader 实时共享。
type inflighting struct {
	ready         chan struct{}
	mu            sync.Mutex
	cond          *sync.Cond
	r             io.ReadCloser
	buf           []byte
	done          bool
	err           error
	contentType   string
	contentLength int64
}

func (c *inflighting) outbound() *outbound {
	return &outbound{r: c.shared(), contentType: c.contentType, contentLength: c.contentLength}
}

func (c *inflighting) pump() {
	defer func() {
		_ = c.r.Close()
		c.mu.Lock()
		c.done = true
		c.mu.Unlock()
		c.cond.Broadcast()
	}()

	tmp := make([]byte, streamBufSize)
	for {
		n, err := c.r.Read(tmp)
		if n > 0 {
			c.mu.Lock()
			c.buf = append(c.buf, tmp[:n]...)
			c.mu.Unlock()
			c.cond.Broadcast()
		}
		if err != nil {
			if err != io.EOF {
				c.mu.Lock()
				c.err = err
				c.mu.Unlock()
			}
			return
		}
	}
}

func (c *inflighting) shared() io.Reader {
	return &shared{source: c}
}

type shared struct {
	source *inflighting
	offset int
}

func (r *shared) Read(p []byte) (int, error) {
	r.source.mu.Lock()
	defer r.source.mu.Unlock()

	for r.offset >= len(r.source.buf) && !r.source.done {
		r.source.cond.Wait()
	}

	if r.offset >= len(r.source.buf) {
		if r.source.err != nil {
			return 0, r.source.err
		}
		return 0, io.EOF
	}

	n := copy(p, r.source.buf[r.offset:])
	r.offset += n
	return n, nil
}

func New(cookie string, size int, ttl time.Duration) *Pipe {
	return NewWithClient(netease.New(cookie), size, ttl)
}

func NewWithClient(client downloader, size int, ttl time.Duration) *Pipe {
	if size <= 0 {
		size = DefaultCacheSize
	}
	if ttl <= 0 {
		ttl = DefaultCacheTTL
	}
	return &Pipe{
		client:   client,
		cache:    expirable.NewLRU[string, *cached](size, nil, ttl),
		inflight: make(map[string]*inflighting),
	}
}

func (p *Pipe) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	source := r.URL.Query().Get("source")
	id := r.URL.Query().Get("id")
	if source != "netease" || id == "" {
		http.Error(w, "invalid source or id", http.StatusBadRequest)
		return
	}

	if e, ok := p.cache.Get(id); ok {
		p.serve(w, e.outbound())
		return
	}

	p.mu.Lock()
	cs, ok := p.inflight[id]
	if !ok {
		cs = p.lift(id)
	}
	p.mu.Unlock()

	<-cs.ready

	if cs.err != nil {
		http.Error(w, cs.err.Error(), http.StatusNotFound)
		return
	}

	p.serve(w, cs.outbound())
}

func (p *Pipe) serve(w http.ResponseWriter, o *outbound) {
	w.Header().Set("Content-Type", o.contentType)
	if o.contentLength > 0 {
		w.Header().Set("Content-Length", strconv.FormatInt(o.contentLength, 10))
	}
	if _, err := io.Copy(w, o.r); err != nil {
		logx.Errorf("mpipe stream error: %v", err)
	}
}

func (p *Pipe) lift(id string) *inflighting {
	i := &inflighting{ready: make(chan struct{})}
	i.cond = sync.NewCond(&i.mu)
	p.inflight[id] = i
	go p.download(id, i)
	return i
}

func (p *Pipe) download(id string, cs *inflighting) {
	defer func() {
		var c *cached
		cs.mu.Lock()
		if cs.err == nil && len(cs.buf) > 0 {
			c = &cached{
				data:          cs.buf,
				contentType:   cs.contentType,
				contentLength: cs.contentLength,
			}
		}
		cs.mu.Unlock()
		if c != nil {
			p.cache.Add(id, c)
		}

		p.mu.Lock()
		delete(p.inflight, id)
		p.mu.Unlock()
	}()

	url, err := p.client.GetDownloadURL(id)
	if err != nil {
		cs.err = err
		close(cs.ready)
		return
	}
	if url == "" {
		cs.err = errors.New("empty download url")
		close(cs.ready)
		return
	}

	resp, err := http.Get(url)
	if err != nil {
		cs.err = err
		close(cs.ready)
		return
	}

	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	contentLength, _ := strconv.ParseInt(resp.Header.Get("Content-Length"), 10, 64)

	if resp.StatusCode != http.StatusOK {
		cs.err = errors.New("upstream status: " + resp.Status)
		cs.contentType = contentType
		close(cs.ready)
		_ = resp.Body.Close()
		return
	}

	cs.contentType = contentType
	cs.contentLength = contentLength
	cs.r = resp.Body
	close(cs.ready)

	cs.pump()
}
