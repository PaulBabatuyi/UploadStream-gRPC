package middleware

import (
	"context"

	"google.golang.org/grpc"
)

// ChainUnaryInterceptors chains multiple unary interceptors in order
func ChainUnaryInterceptors(interceptors ...grpc.UnaryServerInterceptor) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		// Build chain from right to left
		chain := handler
		for i := len(interceptors) - 1; i >= 0; i-- {
			interceptor := interceptors[i]
			prevChain := chain

			chain = func(ctx context.Context, req interface{}) (interface{}, error) {
				return interceptor(ctx, req, info, func(ctx context.Context, req interface{}) (interface{}, error) {
					return prevChain(ctx, req)
				})
			}
		}

		return chain(ctx, req)
	}
}

// ChainStreamInterceptors chains multiple stream interceptors in order
func ChainStreamInterceptors(interceptors ...grpc.StreamServerInterceptor) grpc.StreamServerInterceptor {
	return func(
		srv interface{},
		ss grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) error {
		// Build chain from right to left
		chain := handler
		for i := len(interceptors) - 1; i >= 0; i-- {
			interceptor := interceptors[i]
			prevChain := chain

			chain = func(srv interface{}, ss grpc.ServerStream) error {
				return interceptor(srv, ss, info, func(srv interface{}, ss grpc.ServerStream) error {
					return prevChain(srv, ss)
				})
			}
		}

		return chain(srv, ss)
	}
}
