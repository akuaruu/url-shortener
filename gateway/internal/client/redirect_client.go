package client

import (
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	redirectpb "github.com/akuaruu/url-shortener/proto/gen/redirect"
)

// NewRedirectClient dials the Redirect Service and returns a ready-to-use
// gRPC client. The caller is responsible for closing the connection
// (defer conn.Close() in main).
func NewRedirectClient(addr string) (redirectpb.RedirectServiceClient, *grpc.ClientConn, error) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, nil, fmt.Errorf("client: dial redirect at %s: %w", addr, err)
	}
	return redirectpb.NewRedirectServiceClient(conn), conn, nil
}
