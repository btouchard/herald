package tunnel

import (
	"context"
	"net"
)

// Tunnel exposes a local address via a public HTTPS URL.
type Tunnel interface {
	Start(ctx context.Context, localAddr string) (publicURL string, err error)
	Close() error
	PublicURL() string
	Listener() net.Listener
}
