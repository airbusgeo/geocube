package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/airbusgeo/geocube/interface/database/pg/secrets"
	"github.com/airbusgeo/geocube/internal/utils"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

var userTokenKey = "user"
var adminTokenKey = "admin"
var eventTokenKey = "event"
var bearerAuths map[string]utils.TokenAuth

func loadBearerAuths(ctx context.Context, project, secretName string) (map[string]utils.TokenAuth, error) {
	secret, err := secrets.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("loadBearerAuths: failed to create secret client (%w)", err)
	}
	ba, err := secret.GetSecret(ctx, project, secretName)
	if err != nil {
		return nil, fmt.Errorf("loadBearerAuths: failed to load basic auth secret (%w)", err)
	}

	ma := make(map[string]string)
	if err = json.Unmarshal(ba, &ma); err != nil {
		return nil, fmt.Errorf("loadBearerAuths: failed to unmarshal basic auth secret (%w)", err)
	}

	ret := make(map[string]utils.TokenAuth)
	for k, v := range ma {
		ret[k] = utils.TokenAuth{Token: v}
	}
	return ret, nil
}

func tokenKeysFromPath(urlpath string) []string {
	if strings.HasPrefix(urlpath, "/geocube.Admin/") {
		return []string{adminTokenKey}
	}
	return []string{userTokenKey, adminTokenKey}
}

func authStreamInterceptor(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	meta, ok := metadata.FromIncomingContext(ss.Context())
	if !ok {
		return status.Errorf(codes.Unauthenticated, "missing context metadata")
	}
	if err := authenticate(tokenKeysFromPath(info.FullMethod), meta[utils.AuthorizationHeader]); err != nil {
		return err
	}
	return handler(srv, ss)
}

func authUnaryInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	meta, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, status.Errorf(codes.Unauthenticated, "missing context metadata")
	}
	if err := authenticate(tokenKeysFromPath(info.FullMethod), meta[utils.AuthorizationHeader]); err != nil {
		return nil, err
	}
	return handler(ctx, req)
}

// authenticate returns nil or Error(codes.Unauthenticated)
// return grpc error
func authenticate(tokenKeys []string, tokens []string) error {
	if bearerAuths == nil {
		return status.Errorf(codes.Unauthenticated, "fatal error: no auth info found")
	}
	var err error
	for _, tokenKey := range tokenKeys {
		if bearerAuths[tokenKey].Token == "" {
			return nil
		}

		// First valid token
		for _, token := range tokens {
			if token != "" {
				if err = bearerAuths[tokenKey].Authenticate(token); err == nil {
					return nil // Authentication successful
				}
			}
		}
	}
	if err != nil {
		return err
	}
	return status.Errorf(codes.Unauthenticated, "invalid token")
}
