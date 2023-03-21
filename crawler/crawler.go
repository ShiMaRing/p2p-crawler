package crawler

import (
	"context"
	"crypto/ecdsa"
	"database/sql"
	"fmt"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/p2p/discover"
	"github.com/ethereum/go-ethereum/p2p/discover/v4wire"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/ethereum/go-ethereum/params"
	_ "github.com/go-sql-driver/mysql"
	"go.uber.org/zap"
	"sync"
	"time"
)

const (
	RoundInterval = 5 * time.Second //crawl interval for each node

	DefaultTimeout    = 1 * time.Hour //check interval for all nodes
	respTimeout       = 500 * time.Millisecond
	DefaultChanelSize = 512
	bondExpiration    = 24 * time.Hour
	seedCount         = 30
	seedMaxAge        = 5 * 24 * time.Hour
	seedsCount        = 32
	MaxDHTSize        = 17 * 16
	Debug             = true
	Threshold         = 5
)

type Crawler struct {
	BootNodes []*enode.Node         // BootNodes is the set of nodes that the crawler will start from.
	Cache     map[enode.ID]struct{} // Cache is the set of nodes that the crawler is currently crawling,as a cache

	ReqCh    chan *enode.Node // ReqCh is the channel that the crawler uses to send requests to the workers.
	tokens   chan struct{}    //tokens store token
	OutputCh chan *Node       // OutputCh is the channel that the crawler uses to send requests to the filter.

	leveldb   *enode.DB          // leveldb is the database that the crawler uses to store the nodes.
	db        *sql.DB            // db is the database that the crawler uses to store the nodes.
	tableName string             // tableName is the name of the table that the crawler will use to store the nodes.
	mu        sync.Mutex         // mu is the mutex that protects the crawler.
	ctx       context.Context    // ctx is the context that the crawler uses to cancel all crawl.
	cancel    context.CancelFunc // cancel is the function that the crawler uses to cancel all crawl.

	counter Counter

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
	for _, record := range s {
		n, err := parseNode(record)
		nodes = append(nodes, n) //add the node to the nodes
		if err != nil {
			return nil, fmt.Errorf("invalid bootstrap node: %v", err)
		}
	}

	if config.IsPersistent == false {
		ldb, err = enode.OpenDB("")
		if err != nil {
			return nil, err
		}
		//add the nodes to the leveldb
		if !Debug {
			ld, cfg := makeDiscoveryConfig(ldb, nodes)
			conn := listen(ld, "")
			defer func() {
				conn.Close()
			}()
			v4, err := discover.ListenV4(conn, ld, cfg)
			if err != nil {
				return nil, err
			}
			key, _ := crypto.GenerateKey()
			pbkey := &key.PublicKey
			nodes = v4.LookupPubkey(pbkey)
		} else {
			node, _ := parseNode("enode://dd6c825d6a07ceaacaf236673bb66c386e3cb33a00fcbb0d8ec633892fdcf7079376bfd7188a775471edd7a4259f3925fa4989ff9d889269e0b3addad552b742@38.242.129.221:30300")
			nodes = append(nodes[:0], node)
		}
		for i := range nodes {
			ldb.UpdateNode(nodes[i]) //update the node to the db
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
			nodes = tmp //change the start nodes  from the database
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
		BootNodes: nodes,
		Cache:     make(map[enode.ID]struct{}),
		ReqCh:     reqCh,
		tokens:    make(chan struct{}, DefaultWorkers),
		OutputCh:  outputCh,
		leveldb:   ldb,
		db:        db,
		ctx:       ctx,
		logger:    logger,
		tableName: config.TableName,
		cancel:    cancel,
		Config:    config,
	}
	for i := 0; i < DefaultWorkers; i++ {
		crawler.tokens <- struct{}{}
	}
	return crawler, err
}

// Boot starts the crawler.
func (c *Crawler) Boot() error {
	//put start nodes to the reqch ,and start crawling
	for i := range c.BootNodes {
		c.ReqCh <- c.BootNodes[i]
		//add the node to the cache
		c.Cache[c.BootNodes[i].ID()] = struct{}{}
	}
	defer func() {
		c.cancel()
	}()
	go func() {
		//read output chan, and persistent the nodes or add to the reqch
		c.daemon()
	}()
	for {
		select {
		case <-c.ctx.Done(): //time out ,break it
			return nil
		case <-c.tokens: //wait for token
			go c.Crawl()
		}
	}

}

// Persistent persists the nodes to the database,which run in the background.
func (c *Crawler) daemon() {
	//save the nodes to the database
	buffer := make([]*Node, 0, DefaultChanelSize)
	var err error
	var statement *sql.Stmt
	if c.IsSql {
		statement, err = c.db.Prepare(`replace into nodes (id,seq,access_time,address) values (?,?,?,?,?)`)
		if err != nil {
			c.logger.Fatal("prepare sql statement failed", zap.Error(err))
		}
		defer statement.Close()
	}

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
			c.mu.Lock()
			//we did not crawl the node,so we should add it to the reqch
			if _, ok := c.Cache[node.n.ID()]; !ok {
				c.Cache[node.n.ID()] = struct{}{} //add to the cache
				c.ReqCh <- node.n                 //add to the reqch
			}
			fmt.Println("get node from the output", node.n.String())
			fmt.Println("the length of the cache is ", len(c.Cache))
			fmt.Println("the length of the tokens is ", len(c.tokens))
			fmt.Println(c.counter.ToString())
			c.mu.Unlock()
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

func (c *Crawler) Crawl() {
	select {
	case <-c.ctx.Done():
		return
	case node := <-c.ReqCh:
		c.mu.Lock()
		c.Cache[node.ID()] = struct{}{}
		c.mu.Unlock()
		//get node for crawl,the node never crawled
		result, err := c.crawl(node)
		if err != nil {
			c.logger.Error("crawl node failed", zap.Error(err))
			return
		}
		if result != nil {
			for i := range result {
				node := result[i]
				n := &Node{
					ID:         node.ID(),
					Seq:        node.Seq(),
					AccessTime: time.Now(),
					Address:    node.IP(),
					n:          node,
				}
				c.OutputCh <- n
			}
		}
	}

}

type nodes []*enode.Node //we will get nodes arr from chan and deal with it

//crawl the node
func (c *Crawler) crawl(node *enode.Node) ([]*enode.Node, error) {
	var ctx, cancel = context.WithTimeout(context.Background(), RoundInterval)
	var cache = make(map[enode.ID]*enode.Node) //cache the nodes
	var res []*enode.Node
	prk, _ := crypto.GenerateKey()
	c.mu.Lock()
	ld := enode.NewLocalNode(c.leveldb, prk)
	c.mu.Unlock()
	conn := listen(ld, "") //bind the local node to the port
	nodesChan := make(chan nodes, 16)
	defer func() {
		conn.Close()
		cancel()
		time.Sleep(time.Second)
		c.tokens <- struct{}{} //send token back for next worker
		fmt.Println("finish work ,and the tokens length is", len(c.tokens))
	}()

	go func() {
		c.loop(conn, ctx, ld, prk, nodesChan)
	}()
	err := c.Ping(conn, ld, node, prk)
	if err != nil {
		c.logger.Error("ping pong failed", zap.Error(err))
		return nil, err
	}
	//we try to crawl the DHT table
	for {
		select {
		case <-ctx.Done():
			close(nodesChan)
			return res, nil
		default:
			//generate the random node
			randomNodes := c.generateRandomNode()
			for i := range randomNodes {
				//send the findnode request and get the response
				targetNode := randomNodes[i]
				err = c.findNode(conn, node, prk, targetNode)
				if err != nil {
					c.logger.Error("find node failed", zap.Error(err))
					continue
				}
			}
			var findNodes nodes
			select {
			case <-ctx.Done():
				return res, nil
			case findNodes = <-nodesChan:
				if findNodes != nil {
					//send to the output channel
					var end = true
					for _, n := range findNodes {
						//check the node is in the cache
						if _, ok := cache[n.ID()]; !ok {
							end = false
							cache[n.ID()] = n
							res = append(res, n)
						}
					}
					if end || len(res) >= MaxDHTSize {
						return res, nil
					}
				}
			}
		}
	}
}

//keep read the message from the connection, we will deal with the different message
//
func (c *Crawler) loop(conn UDPConn, ctx context.Context, ld *enode.LocalNode, prk *ecdsa.PrivateKey, nodesChan chan nodes) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			from, packet, _, hash, err := c.handleResponse(conn)
			if err != nil {
				return
			}
			switch packet.(type) {
			case *v4wire.Ping:
				//send pong
				err = c.Pong(conn, ld, prk, packet, hash, from)
				if err != nil {
					c.logger.Error("pong failed", zap.Error(err))
					continue
				}
			case *v4wire.Pong:
				//we discard the pong message
				continue
			case *v4wire.ENRRequest:
				//we discard the enr request message
				continue
			case *v4wire.Neighbors:
				//we will read the neighbors message,and send to the channel
				nodes := packet.(*v4wire.Neighbors).Nodes
				res := make([]*enode.Node, 0)
				for _, n := range nodes {
					key, err := v4wire.DecodePubkey(crypto.S256(), n.ID)
					if err != nil {
						continue
					}
					n := enode.NewV4(key, n.IP, int(n.TCP), int(n.UDP))
					res = append(res, n)
				}
				nodesChan <- res //send to the channel
			}
		}
	}
}

func (c *Crawler) generateRandomNode() []*enode.Node {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.leveldb.QuerySeeds(seedCount, 1<<63-1)
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
		_, err = tx.Stmt(statement).Exec(node.n.ID().String(), node.n.Seq(), node.AccessTime, node.n.IP().String())
		if err != nil {
			defer tx.Rollback()
			return fmt.Errorf("exec sql statement failed: %v", err)
		}
	}
	return tx.Commit()
}
