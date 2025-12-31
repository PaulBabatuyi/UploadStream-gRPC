package middleware

import (
	"context"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// UnaryLoggingInterceptor logs unary RPC calls with timing and errors
func UnaryLoggingInterceptor(logger *zap.Logger) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		start := time.Now()

		// Call the handler
		resp, err := handler(ctx, req)

		// Extract error details
		code := codes.OK
		if err != nil {
			code = status.Code(err)
		}

		duration := time.Since(start)

		// Extract request metadata
		md, _ := metadata.FromIncomingContext(ctx)
		requestID := ""
		if ids := md.Get("x-request-id"); len(ids) > 0 {
			requestID = ids[0]
		}

		// Log with contextual fields
		logLevel := zapcore.InfoLevel
		if err != nil {
			logLevel = zapcore.ErrorLevel
		}

		logger.Check(logLevel, "unary RPC").Write(
			zap.String("method", info.FullMethod),
			zap.String("request_id", requestID),
			zap.Duration("duration", duration),
			zap.String("code", code.String()),
			zap.Error(err),
		)

		return resp, err
	}
}

// StreamLoggingInterceptor logs streaming RPC calls with timing and errors
func StreamLoggingInterceptor(logger *zap.Logger) grpc.StreamServerInterceptor {
	return func(
		srv interface{},
		ss grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) error {
		start := time.Now()

		// Extract request metadata
		md, _ := metadata.FromIncomingContext(ss.Context())
		requestID := ""
		if ids := md.Get("x-request-id"); len(ids) > 0 {
			requestID = ids[0]
		}

		logger.Info("stream RPC started",
			zap.String("method", info.FullMethod),
			zap.String("request_id", requestID),
			zap.Bool("is_client_stream", info.IsClientStream),
			zap.Bool("is_server_stream", info.IsServerStream),
		)

		// Call handler
		err := handler(srv, ss)

		duration := time.Since(start)

		// Log completion
		code := codes.OK
		if err != nil {
			code = status.Code(err)
		}

		logLevel := zapcore.InfoLevel
		if err != nil {
			logLevel = zapcore.ErrorLevel
		}

		logger.Check(logLevel, "stream RPC").Write(
			zap.String("method", info.FullMethod),
			zap.String("request_id", requestID),
			zap.Duration("duration", duration),
			zap.String("code", code.String()),
			zap.Error(err),
		)

		return err
	}
}
