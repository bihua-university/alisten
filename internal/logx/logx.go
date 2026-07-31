// Package logx 提供统一的全局日志输出，基于标准库 log/slog。
// 所有日志带级别与时间戳；Debug 级别由 SetDebug 开关控制。
package logx

import (
	"fmt"
	"log/slog"
	"os"
	"strings"
)

var level = new(slog.LevelVar) // 默认 Info

func init() {
	h := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})
	slog.SetDefault(slog.New(h))
}

// SetDebug 开启或关闭 debug 级别日志。
func SetDebug(debug bool) {
	if debug {
		level.Set(slog.LevelDebug)
	} else {
		level.Set(slog.LevelInfo)
	}
}

// sprint 保持与 fmt.Println 一致的空格分隔语义。
func sprint(args ...any) string {
	return strings.TrimSuffix(fmt.Sprintln(args...), "\n")
}

func Debug(args ...any)                 { slog.Debug(sprint(args...)) }
func Debugf(format string, args ...any) { slog.Debug(fmt.Sprintf(format, args...)) }
func Info(args ...any)                  { slog.Info(sprint(args...)) }
func Infof(format string, args ...any)  { slog.Info(fmt.Sprintf(format, args...)) }
func Warn(args ...any)                  { slog.Warn(sprint(args...)) }
func Warnf(format string, args ...any)  { slog.Warn(fmt.Sprintf(format, args...)) }
func Error(args ...any)                 { slog.Error(sprint(args...)) }
func Errorf(format string, args ...any) { slog.Error(fmt.Sprintf(format, args...)) }

// Fatal 输出错误日志后以状态码 1 退出进程。
func Fatal(args ...any) {
	slog.Error(sprint(args...))
	os.Exit(1)
}

// Fatalf 输出格式化错误日志后以状态码 1 退出进程。
func Fatalf(format string, args ...any) {
	slog.Error(fmt.Sprintf(format, args...))
	os.Exit(1)
}
