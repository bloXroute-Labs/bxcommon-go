package auth

import (
	"context"
	"encoding/base64"
	"fmt"

	"google.golang.org/grpc"
)

type blxrCredentials struct {
	authorization string
}

func (bc blxrCredentials) GetRequestMetadata(context.Context, ...string) (map[string]string, error) {
	return map[string]string{
		"authorization": bc.authorization,
	}, nil
}

func (bc blxrCredentials) RequireTransportSecurity() bool {
	return false
}

// NewBLXRCredentials constructs a new bloxroute GRPC auth scheme from a raw auth header
func NewBLXRCredentials(authorization string) grpc.DialOption {
	return grpc.WithPerRPCCredentials(blxrCredentials{authorization: authorization})
}

// NewBLXRCredentialsFromUserPassword constructs a new bloxroute GRPC auth scheme from an RPC user and secret
func NewBLXRCredentialsFromUserPassword(user string, secret string) grpc.DialOption {
	return grpc.WithPerRPCCredentials(blxrCredentials{authorization: EncodeUserSecret(user, secret)})
}

// EncodeUserSecret produces a base64 encoded auth header of a user and secret
func EncodeUserSecret(user string, secret string) string {
	data := fmt.Sprintf("%v:%v", user, secret)
	return base64.StdEncoding.EncodeToString([]byte(data))
}
