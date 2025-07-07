package task

// Task 表示一个任务
type Task struct {
	ID   string            `json:"id"`
	Type string            `json:"type"`
	Data map[string]string `json:"payload"`
}

// Result 表示任务执行结果
type Result struct {
	ID      string            `json:"id"`
	Success bool              `json:"success"`
	Result  map[string]string `json:"result,omitempty"`
	Error   string            `json:"error,omitempty"`
}

// NewResult 创建一个新的任务结果
func NewResult(id string, success bool) *Result {
	return &Result{
		ID:      id,
		Success: success,
		Result:  make(map[string]string),
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

// NewResultWithData 创建一个包含数据的成功任务结果
func NewResultWithData(id string, data map[string]string) *Result {
	return &Result{
		ID:      id,
		Success: true,
		Result:  data,
	}
}
