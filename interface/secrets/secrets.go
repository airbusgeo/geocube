package secrets

import (
	"context"
	"fmt"

	"google.golang.org/api/option"

	vkit "cloud.google.com/go/secretmanager/apiv1"
	pb "google.golang.org/genproto/googleapis/cloud/secretmanager/v1"
)

type Client struct {
	pbc *vkit.Client
}

func NewClient(ctx context.Context, opts ...option.ClientOption) (*Client, error) {
	pbc, err := vkit.NewClient(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("secretmanager: %w", err)
	}
	return &Client{pbc}, nil
}

type secretOpt struct {
	version string
}

type SecretOption func(s *secretOpt)

func WithVersion(v string) SecretOption {
	return func(s *secretOpt) {
		s.version = v
	}
}

func (c *Client) Close() error {
	return c.pbc.Close()
}

func (c *Client) GetSecret(ctx context.Context, project, secretName string, opts ...SecretOption) ([]byte, error) {
	so := secretOpt{
		version: "latest",
	}

	for _, o := range opts {
		o(&so)
	}

	req := &pb.AccessSecretVersionRequest{
		Name: fmt.Sprintf("projects/%s/secrets/%s/versions/%s", project, secretName, so.version),
	}
	resp, err := c.pbc.AccessSecretVersion(ctx, req)
	if err != nil {
		return nil, err
	}
	return resp.GetPayload().GetData(), nil
}

func GetSecret(ctx context.Context, project, secretName string, opts ...SecretOption) ([]byte, error) {
	cl, err := NewClient(ctx)
	if err != nil {
		return nil, err
	}
	return cl.GetSecret(ctx, project, secretName, opts...)
}
