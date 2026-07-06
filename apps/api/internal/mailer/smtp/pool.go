package smtp

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/smtp"
	"sync"
	"time"
)

// PooledConn wraps an authenticated smtp.Client that can be reused.
type PooledConn struct {
	client     *smtp.Client
	createdAt  time.Time
	lastUsedAt time.Time
	useCount   int
	inUse      bool
}

// Client returns the underlying smtp.Client. The caller must not close it;
// ownership remains with the pool.
func (pc *PooledConn) Client() *smtp.Client {
	return pc.client
}

// Pool manages reusable SMTP connections. It is safe for concurrent use.
//
// Each connection is authenticated when created and remains open until it
// exceeds idleTimeout, maxLifetime, maxUses, or the pool is closed. The
// smtp.Client type is not concurrency-safe, so a connection can only be used
// by one goroutine at a time.
type Pool struct {
	addr        string
	host        string
	auth        smtp.Auth
	tlsConfig   *tls.Config
	maxConns    int
	idleTimeout time.Duration
	maxLifetime time.Duration
	maxUses     int
	dialTimeout time.Duration

	mu     sync.Mutex
	conns  []*PooledConn
	closed bool
}

// PoolOption configures a Pool.
type PoolOption func(*Pool)

// WithDialTimeout sets the timeout for establishing new TCP connections.
func WithDialTimeout(d time.Duration) PoolOption {
	return func(p *Pool) { p.dialTimeout = d }
}

// NewPool creates a new SMTP connection pool.
func NewPool(addr, host string, auth smtp.Auth, tlsConfig *tls.Config, maxConns int, idleTimeout, maxLifetime time.Duration, maxUses int, opts ...PoolOption) *Pool {
	if maxConns <= 0 {
		maxConns = 10
	}
	if idleTimeout <= 0 {
		idleTimeout = 60 * time.Second
	}
	if maxLifetime <= 0 {
		maxLifetime = 5 * time.Minute
	}
	if maxUses <= 0 {
		maxUses = 100
	}
	p := &Pool{
		addr:        addr,
		host:        host,
		auth:        auth,
		tlsConfig:   tlsConfig,
		maxConns:    maxConns,
		idleTimeout: idleTimeout,
		maxLifetime: maxLifetime,
		maxUses:     maxUses,
		dialTimeout: 10 * time.Second,
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

// Get returns an authenticated SMTP client from the pool, creating one if needed.
func (p *Pool) Get(ctx context.Context) (*PooledConn, error) {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return nil, errors.New("smtp pool is closed")
	}
	now := time.Now()
	valid := p.conns[:0]
	for _, c := range p.conns {
		if c.inUse {
			valid = append(valid, c)
			continue
		}
		if now.Sub(c.createdAt) > p.maxLifetime || now.Sub(c.lastUsedAt) > p.idleTimeout || c.useCount >= p.maxUses {
			_ = c.client.Close()
			continue
		}
		c.inUse = true
		c.lastUsedAt = now
		valid = append(valid, c)
		p.conns = valid
		p.mu.Unlock()
		return c, nil
	}
	p.conns = valid
	p.mu.Unlock()

	// No idle connection available; create a new one.
	client, err := p.dialAndAuth(ctx)
	if err != nil {
		return nil, err
	}
	pc := &PooledConn{
		client:     client,
		createdAt:  now,
		lastUsedAt: now,
		useCount:   1,
		inUse:      true,
	}
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		_ = pc.client.Close()
		return nil, errors.New("smtp pool is closed")
	}
	if len(p.conns) < p.maxConns {
		p.conns = append(p.conns, pc)
	} else {
		// Pool is already at capacity; close the connection after this use.
		p.mu.Unlock()
		return pc, nil
	}
	p.mu.Unlock()
	return pc, nil
}

// Put returns a connection to the pool. If the pool is closed the connection
// is closed instead.
func (p *Pool) Put(pc *PooledConn) {
	if pc == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		_ = pc.client.Close()
		return
	}
	pc.inUse = false
	pc.lastUsedAt = time.Now()
	pc.useCount++
}

// Close closes all connections in the pool.
func (p *Pool) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.closed = true
	var firstErr error
	for _, c := range p.conns {
		if err := c.client.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	p.conns = nil
	return firstErr
}

// Stats returns the current number of tracked connections. Used for tests/metrics.
func (p *Pool) Stats() (total, inUse int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	total = len(p.conns)
	for _, c := range p.conns {
		if c.inUse {
			inUse++
		}
	}
	return total, inUse
}

func (p *Pool) dialAndAuth(ctx context.Context) (*smtp.Client, error) {
	dialer := &net.Dialer{Timeout: p.dialTimeout}
	conn, err := dialer.DialContext(ctx, "tcp", p.addr)
	if err != nil {
		return nil, fmt.Errorf("dial smtp server: %w", err)
	}
	client, err := smtp.NewClient(conn, p.host)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("create smtp client: %w", err)
	}
	if ok, _ := client.Extension("STARTTLS"); ok {
		if err := client.StartTLS(p.tlsConfig); err != nil {
			client.Close()
			return nil, fmt.Errorf("starttls: %w", err)
		}
	}
	if p.auth != nil {
		if err := client.Auth(p.auth); err != nil {
			client.Close()
			return nil, fmt.Errorf("smtp auth: %w", err)
		}
	}
	return client, nil
}
