package crawler

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/ethereum/go-ethereum/params"
	_ "github.com/go-sql-driver/mysql"
	"go.uber.org/zap"
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

	ReqCh    chan *enode.Node // ReqCh is the channel that the crawler uses to send requests to the workers.
	OutputCh chan *Node       // OutputCh is the channel that the crawler uses to send requests to the filter.

	leveldb   *enode.DB          // leveldb is the database that the crawler uses to store the nodes.
	db        *sql.DB            // db is the database that the crawler uses to store the nodes.
	tableName string             // tableName is the name of the table that the crawler will use to store the nodes.
	mu        sync.Mutex         // mu is the mutex that protects the crawler.
	ctx       context.Context    // ctx is the context that the crawler uses to cancel all crawl.
	cancel    context.CancelFunc // cancel is the function that the crawler uses to cancel all crawl.

	logger *zap.Logger // logger is the logger that the crawler uses to log the information.
	Config             // config is the config that the crawler uses to store the state of the crawler.
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
	outputCh := make(chan *Node, DefaultChanelSize)
	var db *sql.DB
	if config.IsSql == true {
		db, err = sql.Open("mysql", config.DatabaseUrl)
		if err != nil {
			return nil, err
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), config.TotalTimeout)
	logger, _ := zap.NewProduction()
	crawler := &Crawler{
		BootNodes:    nodes,
		CurrentNodes: make(nodeSet),
		ReqCh:        reqCh,
		OutputCh:     outputCh,
		leveldb:      ldb,
		db:           db,
		ctx:          ctx,
		logger:       logger,
		tableName:    config.TableName,
		cancel:       cancel,
		Config:       config,
	}

	return crawler, err
}

// Boot Start starts the crawler.
func (c *Crawler) Boot() {

}

// Persistent persists the nodes to the database,which run in the background.
func (c *Crawler) Persistent() {
	//save the nodes to the database
	buffer := make([]*Node, 0, DefaultChanelSize)
	statement, err := c.db.Prepare(`replace into nodes (id,seq,access_time,address,connected) values (?,?,?,?,?)`)
	if err != nil {
		c.logger.Fatal("prepare sql statement failed", zap.Error(err))
	}
	defer statement.Close()
	for {
		select {
		case <-c.ctx.Done():
			//save the nodes in outputCh to the database
			for n := range c.OutputCh {
				c.leveldb.UpdateNode(n.n)
				buffer = append(buffer, n)
			}
			if c.IsSql {
				err := c.saveNodes(buffer, statement)
				if err != nil {
					c.logger.Error("save nodes to sql db failed", zap.Error(err))
				}
			}
			return
		case node := <-c.OutputCh:
			err := c.leveldb.UpdateNode(node.n)
			if err != nil {
				c.logger.Error("save nodes to leveldb failed", zap.Error(err))
			}
			if !c.IsSql {
				continue
			}
			buffer = append(buffer, node)
			if len(buffer) == DefaultChanelSize {
				err := c.saveNodes(buffer, statement)
				if err != nil {
					c.logger.Error("save nodes to sql db failed", zap.Error(err))
				}
				buffer = buffer[:0]
			}
		}
	}
}

// saveNodes saves the nodes to the database.

func (c *Crawler) saveNodes(buffer []*Node, statement *sql.Stmt) error {
	//batch insert
	if c.db == nil {
		return fmt.Errorf("invalid database")
	}
	var err error
	tx, err := c.db.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction failed: %v", err)
	}
	for i := range buffer {
		node := buffer[i]
		_, err = tx.Stmt(statement).Exec(node.n.ID().String(), node.n.Seq(), node.AccessTime, node.n.IP().String(), node.ConnectAble)
		if err != nil {
			defer tx.Rollback()
			return fmt.Errorf("exec sql statement failed: %v", err)
		}
	}
	return tx.Commit()
}
