// 本文件提供全局结构化日志器（启动期注入，未注入时静默）。

package pkg

import "go.uber.org/zap"

// appLogger 全局应用日志器（启动期 SetLogger 注入；未注入时为 Nop，测试静默）。
var appLogger = zap.NewNop()

// SetLogger 设置全局应用日志器（仅启动期调用一次）。
func SetLogger(logger *zap.Logger) {
	appLogger = logger
}

// Logger 返回全局应用日志器。
func Logger() *zap.Logger {
	return appLogger
}

// SLogger 返回全局应用日志器的键值对风格（sugared）形态。
func SLogger() *zap.SugaredLogger {
	return appLogger.Sugar()
}
