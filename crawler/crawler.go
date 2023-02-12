package crawler

import (
	"context"
	"database/sql"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"sync"
	"time"
)

const (
	Interval = 6 * time.Second //crawl interval
)

type Crawler struct {
	BootNodes    nodeSet // BootNodes is the set of nodes that the crawler will start from.
	CurrentNodes nodeSet // CurrentNodes is the set of nodes that the crawler is currently crawling.
	NewNodes     nodeSet // NewNodes is the set of nodes that the crawler has found during the current crawl.

	ReqCh    chan *enode.Node // ReqCh is the channel that the crawler uses to send requests to the workers.
	FilterCh chan *enode.Node // FilterCh is the channel that the crawler uses to send requests to the filter.
	EndCh    chan struct{}    // EndCh is the channel that the crawler uses to signal the workers to stop.

	leveldb *enode.DB // db is the database that the crawler uses to store the nodes.
	sqldb   *sql.DB   // db is the database that the crawler uses to store the nodes.

	mu     sync.Mutex      // mu is the mutex that protects the crawler.
	ctx    context.Context // ctx is the context that the crawler uses to cancel the crawl.
	Config                 // config is the config that the crawler uses to store the state of the crawler.
}
