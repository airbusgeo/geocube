package utils

import (
	"context"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Source: https://jbrandhorst.com/post/grpc-auth/

const (
	// AuthorizationHeader is the header key to get the authorization token
	AuthorizationHeader = "authorization"
	// ESRIAuthorizationHeader is the header key to get the authorization token
	ESRIAuthorizationHeader = "X-Esri-Authorization"
	tokenPrefix             = "Bearer "
)

// TokenAuth to use with grpc.WithPerRPCCredentials
type TokenAuth struct {
	Token string
}

// GetRequestMetadata implements grpc.PerRPCCredentials
func (t TokenAuth) GetRequestMetadata(ctx context.Context, uri ...string) (map[string]string, error) {
	return map[string]string{
		AuthorizationHeader: tokenPrefix + t.Token,
	}, nil
}

// RequireTransportSecurity implements grpc.PerRPCCredentials
func (TokenAuth) RequireTransportSecurity() bool {
	return false
	//return true
}

// Authenticate returns nil if token is compliant with TokenAuth else GRPC Error Unauthenticated.
func (t TokenAuth) Authenticate(token string) error {
	if !strings.HasPrefix(token, tokenPrefix) {
		return status.Error(codes.Unauthenticated, `missing "`+tokenPrefix+`" prefix`)
	}

	if strings.TrimPrefix(token, tokenPrefix) != t.Token {
		return status.Error(codes.Unauthenticated, "invalid token")
	}
	return nil
}
