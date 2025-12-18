package middleware

import (
	"context"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// i will replace this later with my jwt api key
var validAPIKeys = map[string]bool{
	"dev-key-123":  true,
	"test-key-456": true,
}

// AuthInterceptor validates API keys from metadata
func AuthInterceptor(
	ctx context.Context,
	req interface{},
	info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler,
) (interface{}, error) {
	// Extract API key from metadata
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing metadata")
	}

	apiKeys := md.Get("api-key")
	if len(apiKeys) == 0 {
		return nil, status.Error(codes.Unauthenticated, "missing api-key")
	}

	apiKey := apiKeys[0]
	if !validAPIKeys[apiKey] {
		return nil, status.Error(codes.Unauthenticated, "invalid api-key")
	}

	return handler(ctx, req)
}

// StreamAuthInterceptor for streaming RPCs
func StreamAuthInterceptor(
	srv interface{},
	ss grpc.ServerStream,
	info *grpc.StreamServerInfo,
	handler grpc.StreamHandler,
) error {
	md, ok := metadata.FromIncomingContext(ss.Context())
	if !ok {
		return status.Error(codes.Unauthenticated, "missing metadata")
	}

	apiKeys := md.Get("api-key")
	if len(apiKeys) == 0 {
		return status.Error(codes.Unauthenticated, "missing api-key")
	}

	apiKey := apiKeys[0]
	if !validAPIKeys[apiKey] {
		return status.Error(codes.Unauthenticated, "invalid api-key")
	}

	return handler(srv, ss)
}

// ExtractUserID gets user_id from context (set by auth interceptor)
func ExtractUserID(ctx context.Context) (string, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return "", status.Error(codes.Internal, "missing metadata")
	}

	userIDs := md.Get("user-id")
	if len(userIDs) == 0 {
		return "", status.Error(codes.Unauthenticated, "missing user-id")
	}

	userID := strings.TrimSpace(userIDs[0])
	if userID == "" {
		return "", status.Error(codes.InvalidArgument, "user-id cannot be empty")
	}

	return userID, nil
}
