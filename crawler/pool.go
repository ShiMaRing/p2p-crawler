package crawler

import (
	"errors"
	"fmt"
	"net"
	"sync"
)

// channelPool implements the Pool interface based on buffered channels.
type channelPool struct {
	// storage for our net.Conn connections
	mu    sync.RWMutex
	conns chan net.Conn

	// net.Conn generator
	factory Factory
}

// Pool interface describes a pool implementation. A pool should have maximum
// capacity.
type Pool interface {
	// Get returns a new connection from the pool. Closing the connections puts
	// it back to the Pool. Closing it when the pool is destroyed or full will
	// be counted as an error.
	Get() (net.Conn, error)

	// Close closes the pool and all its connections. After Close() the pool is
	// no longer usable.
	Close()

	// Len returns the current number of connections of the pool.
	Len() int
}

// Factory is a function to create new connections.
type Factory func() (net.Conn, error)

var (
	// ErrClosed is the error resulting if the pool is closed via pool.Close().
	ErrClosed = errors.New("pool is closed")
)

// NewChannelPool returns a new pool based on buffered channels with an initial
// capacity and maximum capacity. Factory is used when initial capacity is
// greater than zero to fill the pool. A zero initialCap doesn't fill the Pool
// until a new Get() is called. During a Get(), If there is no new connection
// available in the pool, a new connection will be created via the Factory()
// method.
func NewChannelPool(initialCap, maxCap int, factory Factory) (Pool, error) {
	if initialCap < 0 || maxCap <= 0 || initialCap > maxCap {
		return nil, errors.New("invalid capacity settings")
	}

	c := &channelPool{
		conns:   make(chan net.Conn, maxCap),
		factory: factory,
	}

	// create initial connections, if something goes wrong,
	// just close the pool error out.
	for i := 0; i < initialCap; i++ {
		conn, err := factory()
		if err != nil {
			c.Close()
			return nil, fmt.Errorf("factory is not able to fill the pool: %s", err)
		}
		c.conns <- conn
	}

	return c, nil
}

func (c *channelPool) getConnsAndFactory() (chan net.Conn, Factory) {
	c.mu.RLock()
	conns := c.conns
	factory := c.factory
	c.mu.RUnlock()
	return conns, factory
}

// Get implements the Pool interfaces Get() method. If there is no new
// connection available in the pool, a new connection will be created via the
// Factory() method.
func (c *channelPool) Get() (net.Conn, error) {
	conns, factory := c.getConnsAndFactory()
	if conns == nil {
		return nil, ErrClosed
	}

	// wrap our connections with out custom net.Conn implementation (wrapConn
	// method) that puts the connection back to the pool if it's closed.
	select {
	case conn := <-conns:
		if conn == nil {
			return nil, ErrClosed
		}

		return c.wrapConn(conn), nil
	default:
		conn, err := factory()
		if err != nil {
			return nil, err
		}

		return c.wrapConn(conn), nil
	}
}

// put puts the connection back to the pool. If the pool is full or closed,
// conn is simply closed. A nil conn will be rejected.
func (c *channelPool) put(conn net.Conn) error {
	if conn == nil {
		return errors.New("connection is nil. rejecting")
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.conns == nil {
		// pool is closed, close passed connection
		return conn.Close()
	}

	// put the resource back into the pool. If the pool is full, this will
	// block and the default case will be executed.
	select {
	case c.conns <- conn:
		return nil
	default:
		// pool is full, close passed connection
		return conn.Close()
	}
}

func (c *channelPool) Close() {
	c.mu.Lock()
	conns := c.conns
	c.conns = nil
	c.factory = nil
	c.mu.Unlock()

	if conns == nil {
		return
	}

	close(conns)
	for conn := range conns {
		conn.Close()
	}
}

func (c *channelPool) Len() int {
	conns, _ := c.getConnsAndFactory()
	return len(conns)
}

// PoolConn is a wrapper around net.Conn to modify the the behavior of
// net.Conn's Close() method.
type PoolConn struct {
	net.Conn
	mu       sync.RWMutex
	c        *channelPool
	unusable bool
}

// Close() puts the given connects back to the pool instead of closing it.
func (p *PoolConn) Close() error {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.unusable {
		if p.Conn != nil {
			return p.Conn.Close()
		}
		return nil
	}
	return p.c.put(p.Conn)
}

// MarkUnusable() marks the connection not usable any more, to let the pool close it instead of returning it to pool.
func (p *PoolConn) MarkUnusable() {
	p.mu.Lock()
	p.unusable = true
	p.mu.Unlock()
}

// newConn wraps a standard net.Conn to a poolConn net.Conn.
func (c *channelPool) wrapConn(conn net.Conn) net.Conn {
	p := &PoolConn{c: c}
	p.Conn = conn
	return p
}
