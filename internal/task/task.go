package task

import "encoding/json"

// Task 表示一个任务
type Task struct {
	ID   string            `json:"id"`
	Type string            `json:"type"`
	Data map[string]string `json:"payload"`
}

// Result 表示任务执行结果
type Result struct {
	ID      string          `json:"id"`
	Success bool            `json:"success"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   string          `json:"error,omitempty"`
}

// NewResult 创建一个新的任务结果
func NewResult(id string, success bool) *Result {
	return &Result{
		ID:      id,
		Success: success,
	}
}

// NewResultWithError 创建一个包含错误的任务结果
func NewResultWithError(id string, err string) *Result {
	return &Result{
		ID:      id,
		Success: false,
		Error:   err,
	}
}
