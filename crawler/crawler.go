package crawler

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/ethereum/go-ethereum/params"
	_ "github.com/go-sql-driver/mysql"
	"sync"
	"time"
)

const (
	Interval       = 6 * time.Second  //crawl interval for each node
	RoundInterval  = 10 * time.Second //crawl interval for each round
	DefaultTimeout = 1 * time.Hour    //check interval for all nodes

	DefaultChanelSize = 512

	seedCount  = 30
	seedMaxAge = 5 * 24 * time.Hour
)

type Crawler struct {
	BootNodes    []*enode.Node // BootNodes is the set of nodes that the crawler will start from.
	CurrentNodes nodeSet       // CurrentNodes is the set of nodes that the crawler is currently crawling.
	NewNodes     nodeSet       // NewNodes is the set of nodes that the crawler has found during the current crawl.

	ReqCh    chan *enode.Node // ReqCh is the channel that the crawler uses to send requests to the workers.
	OutputCh chan *enode.Node // OutputCh is the channel that the crawler uses to send requests to the filter.

	leveldb *enode.DB // leveldb is the database that the crawler uses to store the nodes.
	db      *sql.DB   // db is the database that the crawler uses to store the nodes.

	mu     sync.Mutex         // mu is the mutex that protects the crawler.
	ctx    context.Context    // ctx is the context that the crawler uses to cancel all crawl.
	cancel context.CancelFunc // cancel is the function that the crawler uses to cancel all crawl.
	Config                    // config is the config that the crawler uses to store the state of the crawler.
}

func NewCrawler(config Config) (*Crawler, error) {
	var err error
	nodes := make([]*enode.Node, 0)
	//start from the boot nodes
	var ldb *enode.DB

	//start from the MainBootNodes
	s := params.MainnetBootnodes
	for i, record := range s {
		nodes[i], err = parseNode(record)
		if err != nil {
			return nil, fmt.Errorf("invalid bootstrap node: %v", err)
		}
	}

	if config.IsPersistent == false {
		ldb, err = enode.OpenDB("")
		if err != nil {
			return nil, err
		}
	} else {
		//start from the database
		ldb, err = enode.OpenDB(config.DbName)
		if err != nil {
			return nil, err
		}
		//load the nodes from the database
		tmp := ldb.QuerySeeds(seedCount, seedMaxAge)
		if len(tmp) != 0 {
			nodes = tmp
		}
	}
	//create ctx
	reqCh := make(chan *enode.Node, DefaultChanelSize)
	outputCh := make(chan *enode.Node, DefaultChanelSize)
	var db *sql.DB
	if config.IsSql == true {
		db, err = sql.Open("mysql", config.DatabaseUrl)
		if err != nil {
			return nil, err
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), config.TotalTimeout)
	crawler := &Crawler{
		BootNodes:    nodes,
		CurrentNodes: make(nodeSet),
		NewNodes:     make(nodeSet),
		ReqCh:        reqCh,
		OutputCh:     outputCh,
		leveldb:      ldb,
		db:           db,
		ctx:          ctx,
		cancel:       cancel,
		Config:       config,
	}

	return crawler, err
}

// Boot Start starts the crawler.
func (c *Crawler) Boot() {

}
