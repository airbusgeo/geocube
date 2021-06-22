package client

import (
	"context"
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
