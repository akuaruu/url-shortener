package client

import (
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	shortenerpb "github.com/akuaruu/url-shortener/proto/gen/shortener"
)

// NewShortenerClient dials the Shortener Service and returns a ready-to-use
// gRPC client. The caller is responsible for closing the connection
// (defer conn.Close() in main).
func NewShortenerClient(addr string) (shortenerpb.ShortenerServiceClient, *grpc.ClientConn, error) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, nil, fmt.Errorf("client: dial shortener at %s: %w", addr, err)
	}
	return shortenerpb.NewShortenerServiceClient(conn), conn, nil
}
