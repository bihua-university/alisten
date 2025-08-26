package task

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/bihua-university/alisten/internal/semver"
	"github.com/bihua-university/alisten/internal/syncx"
)

// Server 长轮询任务服务器
type Server struct {
	token   string
	tasks   syncx.UnboundedChan[*Task]
	results sync.Map      // map[string]chan *Result
	idGen   atomic.Uint64 // 原子计数器，用于生成唯一ID
}

var minAllowedVersion = semver.Parse("v0.0.1")

// NewServer 创建新的任务服务器
func NewServer(token string) *Server {
	return &Server{
		tasks: syncx.NewUnboundedChan[*Task](32),
		token: token,
	}
}

// NewTask 创建一个新的任务，自动生成ID
func (s *Server) NewTask(taskType string, data map[string]string) *Task {
	id := s.idGen.Add(1)
	return &Task{
		ID:   strconv.FormatUint(id, 10),
		Type: taskType,
		Data: data,
	}
}

// Call 同步调用任务，添加任务并等待结果
func (s *Server) Call(task *Task, timeout time.Duration) *Result {
	resultChan := make(chan *Result, 1)
	s.results.Store(task.ID, resultChan)

	s.tasks.In() <- task

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer func() {
		cancel()
		s.results.Delete(task.ID)
	}()

	select {
	case result := <-resultChan:
		return result
	case <-ctx.Done():
		return nil
	}
}

func (s *Server) precheck(r *http.Request, w http.ResponseWriter) bool {
	// 验证Token
	if !s.validateToken(r) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"error": "未授权"})
		return false
	}

	version := semver.Parse(r.Header.Get("Music-Let-Version"))
	// 检查版本是否支持
	if !version.GreaterEqual(minAllowedVersion) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUpgradeRequired) // 426 状态码表示需要升级
		json.NewEncoder(w).Encode(map[string]string{
			"error":       "客户端版本过低",
			"min_version": minAllowedVersion.String(),
		})
		return false
	}

	return true
}

func (s *Server) validateToken(r *http.Request) bool {
	if s.token == "" {
		return true // 如果没有设置token，则不验证
	}

	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return false
	}

	const bearerPrefix = "Bearer "
	if len(authHeader) <= len(bearerPrefix) || authHeader[:len(bearerPrefix)] != bearerPrefix {
		return false
	}

	token := authHeader[len(bearerPrefix):]
	return token == s.token
}

// PollTaskHandler 长轮询获取任务的处理器
func (s *Server) PollTaskHandler(w http.ResponseWriter, r *http.Request) {
	if !s.precheck(r, w) {
		return
	}

	// 获取超时参数
	timeoutStr := r.URL.Query().Get("timeout")
	timeout := 30 * time.Second // 默认30秒
	if timeoutStr != "" {
		if t, err := strconv.Atoi(timeoutStr); err == nil {
			timeout = time.Duration(t) * time.Second
		}
	}

	// 创建带超时的上下文
	ctx, cancel := context.WithTimeout(r.Context(), timeout)
	defer cancel()

	select {
	case task := <-s.tasks.Out():
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(task)
	case <-ctx.Done():
		// 超时，返回空内容
		w.WriteHeader(http.StatusNoContent)
	}
}

// SubmitResultHandler 提交任务结果的处理器
func (s *Server) SubmitResultHandler(w http.ResponseWriter, r *http.Request) {
	if !s.precheck(r, w) {
		return
	}

	var result Result
	if err := json.NewDecoder(r.Body).Decode(&result); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "无效的JSON格式"})
		return
	}

	// 通过任务ID查找对应的结果通道
	if chanInterface, ok := s.results.Load(result.ID); ok {
		if resultChan, ok := chanInterface.(chan *Result); ok {
			resultChan <- &result
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"message": "结果已接收"})
			return
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotFound)
	json.NewEncoder(w).Encode(map[string]string{"error": "未找到对应的任务"})
}
