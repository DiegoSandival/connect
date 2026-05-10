package network

import "context"

type AccessController interface {
	Name() string
	Prepare(ctx context.Context, options PrepareOptions) error
	EnsureBlocked(ctx context.Context, clientIP string) error
	AllowClient(ctx context.Context, clientIP string) error
	BlockClient(ctx context.Context, clientIP string) error
}

type PrepareOptions struct {
	PortalPort    int
	HotspotSubnet string
}

type NoopController struct{}

func (n *NoopController) Name() string { return "noop" }

func (n *NoopController) Prepare(context.Context, PrepareOptions) error { return nil }

func (n *NoopController) EnsureBlocked(context.Context, string) error { return nil }

func (n *NoopController) AllowClient(context.Context, string) error { return nil }

func (n *NoopController) BlockClient(context.Context, string) error { return nil }
