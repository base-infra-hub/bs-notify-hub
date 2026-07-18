package logger

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/cloudwego/hertz/pkg/common/hlog"
	"gopkg.in/natefinch/lumberjack.v2"
)

const (
	defaultLogDir     = "logs"
	defaultLogFile    = "app.log"
	defaultMaxSize    = 1 // MB
	defaultMaxBackups = 30
	defaultMaxAge     = 7 // days
)

// filteredSystemLogger 包装 Hertz 系统 logger，用于过滤扫描类噪音日志。
type filteredSystemLogger struct {
	logger hlog.FullLogger
}

func (l *filteredSystemLogger) Trace(v ...interface{}) { l.logger.Trace(v...) }
func (l *filteredSystemLogger) Debug(v ...interface{}) { l.logger.Debug(v...) }
func (l *filteredSystemLogger) Info(v ...interface{})  { l.logger.Info(v...) }
func (l *filteredSystemLogger) Notice(v ...interface{}) {
	l.logger.Notice(v...)
}
func (l *filteredSystemLogger) Warn(v ...interface{}) { l.logger.Warn(v...) }
func (l *filteredSystemLogger) Error(v ...interface{}) {
	msg := fmt.Sprint(v...)
	if isNoise(msg) {
		return
	}
	l.logger.Error(v...)
}
func (l *filteredSystemLogger) Fatal(v ...interface{}) { l.logger.Fatal(v...) }

func (l *filteredSystemLogger) Tracef(format string, v ...interface{}) {
	l.logger.Tracef(format, v...)
}
func (l *filteredSystemLogger) Debugf(format string, v ...interface{}) {
	l.logger.Debugf(format, v...)
}
func (l *filteredSystemLogger) Infof(format string, v ...interface{}) {
	l.logger.Infof(format, v...)
}
func (l *filteredSystemLogger) Noticef(format string, v ...interface{}) {
	l.logger.Noticef(format, v...)
}
func (l *filteredSystemLogger) Warnf(format string, v ...interface{}) {
	l.logger.Warnf(format, v...)
}
func (l *filteredSystemLogger) Errorf(format string, v ...interface{}) {
	msg := fmt.Sprintf(format, v...)
	if isNoise(msg) {
		return
	}
	l.logger.Errorf(format, v...)
}
func (l *filteredSystemLogger) Fatalf(format string, v ...interface{}) {
	l.logger.Fatalf(format, v...)
}

func (l *filteredSystemLogger) CtxTracef(ctx context.Context, format string, v ...interface{}) {
	l.logger.CtxTracef(ctx, format, v...)
}
func (l *filteredSystemLogger) CtxDebugf(ctx context.Context, format string, v ...interface{}) {
	l.logger.CtxDebugf(ctx, format, v...)
}
func (l *filteredSystemLogger) CtxInfof(ctx context.Context, format string, v ...interface{}) {
	l.logger.CtxInfof(ctx, format, v...)
}
func (l *filteredSystemLogger) CtxNoticef(ctx context.Context, format string, v ...interface{}) {
	l.logger.CtxNoticef(ctx, format, v...)
}
func (l *filteredSystemLogger) CtxWarnf(ctx context.Context, format string, v ...interface{}) {
	l.logger.CtxWarnf(ctx, format, v...)
}
func (l *filteredSystemLogger) CtxErrorf(ctx context.Context, format string, v ...interface{}) {
	msg := fmt.Sprintf(format, v...)
	if isNoise(msg) {
		return
	}
	l.logger.CtxErrorf(ctx, format, v...)
}
func (l *filteredSystemLogger) CtxFatalf(ctx context.Context, format string, v ...interface{}) {
	l.logger.CtxFatalf(ctx, format, v...)
}

func (l *filteredSystemLogger) SetLevel(level hlog.Level) { l.logger.SetLevel(level) }
func (l *filteredSystemLogger) SetOutput(writer io.Writer) {
	l.logger.SetOutput(writer)
}

// isNoise 判断是否为扫描/畸形请求等噪音日志。
func isNoise(msg string) bool {
	return strings.Contains(msg, "malformed HTTP request") ||
		(strings.Contains(msg, "HERTZ: Error=read tcp") && strings.Contains(msg, "wsarecv"))
}

// Init 初始化日志输出：统一输出到 lumberjack 轮转文件，并过滤 Hertz 系统噪音日志。
func Init() error {
	if err := os.MkdirAll(defaultLogDir, 0755); err != nil {
		return fmt.Errorf("创建日志目录失败: %w", err)
	}

	writer := &lumberjack.Logger{
		Filename:   filepath.Join(defaultLogDir, defaultLogFile),
		MaxSize:    defaultMaxSize,
		MaxBackups: defaultMaxBackups,
		MaxAge:     defaultMaxAge,
		Compress:   true,
		LocalTime:  true,
	}

	// 业务日志输出到文件
	hlog.SetOutput(writer)

	// 系统日志（HERTZ 框架错误）同样输出到文件，并过滤扫描噪音
	// 注意：使用 DefaultLogger 作为底层，避免 systemLogger 嵌套导致 HERTZ 前缀重复。
	hlog.SetSystemLogger(&filteredSystemLogger{logger: hlog.DefaultLogger()})

	// 标准库 log 同时输出到控制台和文件，保证启动信息、配置、端口等仍在终端可见
	log.SetOutput(io.MultiWriter(os.Stdout, writer))
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	return nil
}
