package tracing

import (
	"context"
	"fmt"
	"log/slog"

	"google.golang.org/grpc/grpclog"
)

// grpcSlogBridge implements grpclog.LoggerV2 and forwards all gRPC-internal
// log output to the application slog.Logger. gRPC "Info" events are mapped to
// slog.LevelDebug because they are very chatty (idle mode transitions, resolver
// updates, etc.) and should not appear at the default "info" log level.
type grpcSlogBridge struct {
	l *slog.Logger
}

func (b *grpcSlogBridge) Info(args ...interface{}) {
	b.l.Debug("[grpc] " + fmt.Sprint(args...))
}

func (b *grpcSlogBridge) Infoln(args ...interface{}) {
	b.l.Debug("[grpc] " + fmt.Sprint(args...))
}

func (b *grpcSlogBridge) Infof(format string, args ...interface{}) {
	b.l.Debug("[grpc] " + fmt.Sprintf(format, args...))
}

func (b *grpcSlogBridge) Warning(args ...interface{}) {
	b.l.Warn("[grpc] " + fmt.Sprint(args...))
}

func (b *grpcSlogBridge) Warningln(args ...interface{}) {
	b.l.Warn("[grpc] " + fmt.Sprint(args...))
}

func (b *grpcSlogBridge) Warningf(format string, args ...interface{}) {
	b.l.Warn("[grpc] " + fmt.Sprintf(format, args...))
}

func (b *grpcSlogBridge) Error(args ...interface{}) {
	b.l.Error("[grpc] " + fmt.Sprint(args...))
}

func (b *grpcSlogBridge) Errorln(args ...interface{}) {
	b.l.Error("[grpc] " + fmt.Sprint(args...))
}

func (b *grpcSlogBridge) Errorf(format string, args ...interface{}) {
	b.l.Error("[grpc] " + fmt.Sprintf(format, args...))
}

// Fatal is intentionally demoted to Error so the gRPC library cannot call
// os.Exit and bypass Sablier's graceful shutdown.
func (b *grpcSlogBridge) Fatal(args ...interface{}) {
	b.l.Error("[grpc] " + fmt.Sprint(args...))
}

func (b *grpcSlogBridge) Fatalln(args ...interface{}) {
	b.l.Error("[grpc] " + fmt.Sprint(args...))
}

func (b *grpcSlogBridge) Fatalf(format string, args ...interface{}) {
	b.l.Error("[grpc] " + fmt.Sprintf(format, args...))
}

// V returns true only when the logger is configured at Debug level, matching
// gRPC's own verbosity model where V(0) is the base level.
func (b *grpcSlogBridge) V(l int) bool {
	return b.l.Enabled(context.Background(), slog.LevelDebug)
}

// setGRPCLogger replaces the global gRPC logger with an slog-backed bridge.
func setGRPCLogger(logger *slog.Logger) {
	grpclog.SetLoggerV2(&grpcSlogBridge{l: logger})
}
