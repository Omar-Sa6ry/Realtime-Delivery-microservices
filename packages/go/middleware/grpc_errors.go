package middleware

import "google.golang.org/grpc/codes"

// HTTPStatusToGRPC maps an HTTP status code to the equivalent gRPC status code.
// Mirrors the NestJS GrpcExceptionFilter mapping in the TypeScript common package.
func HTTPStatusToGRPC(statusCode int) codes.Code {
	switch statusCode {
	case 400:
		return codes.InvalidArgument
	case 401:
		return codes.Unauthenticated
	case 403:
		return codes.PermissionDenied
	case 404:
		return codes.NotFound
	case 409:
		return codes.AlreadyExists
	case 429:
		return codes.ResourceExhausted
	default:
		return codes.Internal
	}
}
