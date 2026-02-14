package tunnel

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"strings"

	ngroklib "golang.ngrok.com/ngrok"
	ngrokconfig "golang.ngrok.com/ngrok/config"
)

// NgrokTunnel implements the Tunnel interface using ngrok.
type NgrokTunnel struct {
	authToken string
	domain    string
	listener  net.Listener
	url       string
}

// NewNgrok creates a new ngrok tunnel with the given auth token and optional domain.
func NewNgrok(authToken, domain string) *NgrokTunnel {
	return &NgrokTunnel{
		authToken: authToken,
		domain:    domain,
	}
}

// Start creates an ngrok tunnel and returns the public URL.
// The localAddr parameter is used for logging purposes only - ngrok creates its own listener.
func (n *NgrokTunnel) Start(ctx context.Context, localAddr string) (string, error) {
	if n.authToken == "" {
		return "", fmt.Errorf("ngrok auth token is required (set tunnel.authtoken in config or HERALD_NGROK_AUTHTOKEN env var)")
	}

	slog.Info("starting ngrok tunnel", "local_addr", localAddr, "domain", n.domain)

	// Build tunnel configuration
	var tunnelConfig ngrokconfig.Tunnel
	if n.domain != "" {
		// Fixed domain (paid plans)
		tunnelConfig = ngrokconfig.HTTPEndpoint(
			ngrokconfig.WithDomain(n.domain),
		)
		slog.Debug("using fixed ngrok domain", "domain", n.domain)
	} else {
		// Random domain (free plans)
		tunnelConfig = ngrokconfig.HTTPEndpoint()
		slog.Debug("using random ngrok domain")
	}

	// Create ngrok listener
	listener, err := ngroklib.Listen(
		ctx,
		tunnelConfig,
		ngroklib.WithAuthtoken(n.authToken),
	)
	if err != nil {
		return "", fmt.Errorf("failed to create ngrok tunnel: %w", err)
	}

	n.listener = listener

	// Extract public URL from listener
	// The ngrok listener's Addr() returns the public URL
	addr := listener.Addr().String()

	// Ensure the URL has the https:// prefix
	if !strings.HasPrefix(addr, "http://") && !strings.HasPrefix(addr, "https://") {
		n.url = "https://" + addr
	} else {
		n.url = addr
	}

	slog.Info("ngrok tunnel established", "public_url", n.url)

	return n.url, nil
}

// Close closes the ngrok tunnel.
func (n *NgrokTunnel) Close() error {
	if n.listener == nil {
		return nil
	}

	slog.Info("closing ngrok tunnel", "public_url", n.url)

	if err := n.listener.Close(); err != nil {
		return fmt.Errorf("failed to close ngrok tunnel: %w", err)
	}

	n.listener = nil
	n.url = ""

	return nil
}

// PublicURL returns the public URL of the tunnel.
func (n *NgrokTunnel) PublicURL() string {
	return n.url
}

// Listener returns the underlying net.Listener for serving HTTP requests.
func (n *NgrokTunnel) Listener() net.Listener {
	return n.listener
}
