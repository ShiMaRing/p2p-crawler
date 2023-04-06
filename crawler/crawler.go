package crawler

import (
	"bufio"
	"context"
	"crypto/ecdsa"
	"database/sql"
	"fmt"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/p2p/discover"
	"github.com/ethereum/go-ethereum/p2p/discover/v4wire"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/ethereum/go-ethereum/p2p/netutil"
	"github.com/ethereum/go-ethereum/params"
	_ "github.com/go-sql-driver/mysql"
	"github.com/oschwald/geoip2-golang"
	"go.uber.org/zap"
	"os"
	"sync"
	"time"
)

const (
	RoundInterval     = 30 * time.Second //crawl interval for each node
	DefaultTimeout    = 1 * time.Hour    //check interval for all nodes
	respTimeout       = 500 * time.Millisecond
	DefaultChanelSize = 2048
	seedCount         = 30
	seedMaxAge        = 5 * 24 * time.Hour
	MaxDHTSize        = 17 * 16
	Threshold         = 1
)

type Crawler struct {
	BootNodes []*enode.Node         // BootNodes is the set of nodes that the crawler will start from.
	Cache     map[enode.ID]struct{} // Cache is the set of nodes that the crawler is currently crawling,as a cache
	tokens    chan struct{}         //tokens store token

	ReqCh    chan *enode.Node // ReqCh is the channel that the crawler uses to send requests to the workers.
	OutputCh chan *Node       // OutputCh is the channel that the crawler uses to send requests to the filter.
	DHTCh    chan *enode.Node // DHTCh is the channel that the crawler uses to send requests to the DHT.

	leveldb   *enode.DB          // leveldb is the database that the crawler uses to store the nodes.
	db        *sql.DB            // db is the database that the crawler uses to store the nodes.
	tableName string             // tableName is the name of the table that the crawler will use to store the nodes.
	mu        sync.Mutex         // mu is the mutex that protects the crawler.
	ctx       context.Context    // ctx is the context that the crawler uses to cancel all crawl.
	cancel    context.CancelFunc // cancel is the function that the crawler uses to cancel all crawl.

	genesis *core.Genesis // genesis is the genesis block that the crawler uses to verify the nodes.

	counter Counter //counter is the counter that the crawler uses to count the nodes.

	logger *zap.Logger // logger is the logger that the crawler uses to log the information.
	Config             // config is the config that the crawler uses to store the state of the crawler.

	writer  *bufio.Writer  //writer is the writer that the crawler uses to write the nodes to the file.
	geoipDB *geoip2.Reader //geoipDB is the database that the crawler uses to get the country and city from ip address.
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
		for i := range nodes {
			ldb.UpdateNode(nodes[i])
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
	dhtCh := make(chan *enode.Node, DefaultChanelSize)
	outputCh := make(chan *Node, DefaultChanelSize)

	var db *sql.DB
	if config.IsSql == true { //we need to store the nodes to the database
		db, err = sql.Open("mysql", config.DatabaseUrl)
		if err != nil {
			return nil, err
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), config.TotalTimeout)
	logger, _ := zap.NewProduction()

	geoipDB, err := geoip2.Open("db/GeoLite2-City.mmdb")

	crawler := &Crawler{
		BootNodes: nodes,
		Cache:     make(map[enode.ID]struct{}),
		ReqCh:     reqCh,
		tokens:    make(chan struct{}, config.Workers),
		OutputCh:  outputCh,
		DHTCh:     dhtCh,
		leveldb:   ldb,
		db:        db,
		ctx:       ctx,
		logger:    logger,
		tableName: config.TableName,
		cancel:    cancel,
		Config:    config,
		geoipDB:   geoipDB,
		genesis:   makeGenesis(),
	}

	if err != nil {
		return nil, err
	}
	//send the tokens to determine the number of workers parallel
	for i := 0; i < config.Workers; i++ {
		crawler.tokens <- struct{}{}
	}

	return crawler, err
}

// Boot starts the crawler.
func (c *Crawler) Boot() error {
	f, err := os.OpenFile("record", os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	if err != nil {
		c.logger.Error("open file error", zap.Error(err))
		return err
	}
	c.writer = bufio.NewWriter(f)
	c.counter.StartTime = time.Now()
	//put start nodes to the reqch ,and start crawling
	for i := range c.BootNodes {
		c.ReqCh <- c.BootNodes[i]
		//add the node to the cache
		c.Cache[c.BootNodes[i].ID()] = struct{}{}
	}

	defer func() {
		c.cancel()
		c.writer.Flush()
		c.geoipDB.Close()
		f.Close()
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
	//create a file ,if exists, truncate it
	buffer := make([]*Node, 0, DefaultChanelSize)
	var err error
	var statement *sql.Stmt
	if c.IsSql { //we need to store the nodes to the sql database
		statement, err = c.db.Prepare(`replace into nodes (id,seq,access_time,address) values (?,?,?,?,?)`)
		if err != nil {
			c.logger.Fatal("prepare sql statement failed", zap.Error(err))
		}
		defer statement.Close()
	}
	ticker := time.NewTicker(2 * time.Second) //every 2 seconds, we output the nodes to the file
	go func() {
		for {
			select {
			case <-c.ctx.Done():
				ticker.Stop()
				return
			case <-ticker.C:
				c.mu.Lock()
				fmt.Println(c.counter.ToString()) //print the counter
				c.mu.Unlock()
			}
		}
	}()
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

			if node.ConnectAble {
				c.writer.WriteString(node.n.String() + "\n")
			}
			//we need to deal with the node info and save it to the mysql database
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
		case node := <-c.DHTCh: //add the node to the reqch back
			c.mu.Lock()
			err = c.leveldb.UpdateNode(node)
			if err != nil {
				c.logger.Error("save nodes to leveldb failed", zap.Error(err))
			}
			//we did not crawl the node,so we should add it to the reqch
			if _, ok := c.Cache[node.ID()]; !ok {
				c.Cache[node.ID()] = struct{}{} //add to the cache
				c.ReqCh <- node                 //add to the reqch
			}
			c.mu.Unlock()
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

		result, err := c.crawl(node) //we also updated the node info

		myNode := &Node{
			n:          node,
			Seq:        node.Seq(),
			Address:    node.IP(),
			ID:         node.ID(),
			AccessTime: time.Now(),
		}
		if err != nil {
			c.logger.Error("crawl node failed", zap.Error(err))
		}
		if result != nil {
			for i := range result {
				c.DHTCh <- result[i] //add the node to the dhtch
			}
			myNode.NeighborsCount = len(result)
			myNode.ConnectAble = true
			c.counter.AddConnectAbleNodes() //add the connectable nodes
			info, err := getClientInfo(makeGenesis(), 1, myNode.n)
			if err != nil {
				c.logger.Error("get client info failed", zap.Error(err))
			}
			myNode.clientInfo = info
		} else {
			myNode.ConnectAble = false
		}
		//feat: get the country and city from ip address
		country, city, err := c.geoSearch(node.IP())
		if err == nil {
			myNode.Country = country
			myNode.City = city
		}
		c.counter.AddNodesNum()
		c.OutputCh <- myNode //add the node to the outputch
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
	nodesChan := make(chan nodes, 32)
	enrChan := make(chan *enode.Node, 1)
	defer func() {
		conn.Close()
		cancel()
		time.Sleep(500 * time.Millisecond)
		close(enrChan)
		c.tokens <- struct{}{} //send token back for next worker
	}()
	go func() {
		c.loop(conn, ctx, ld, prk, nodesChan, enrChan)
	}()
	err := c.Ping(conn, ld, node, prk)
	if err != nil {
		c.logger.Error("ping pong failed", zap.Error(err))
		return nil, err
	}
	err = c.getENR(conn, node, prk)
	if err != nil {
		c.logger.Error("get enr failed", zap.Error(err))
		return nil, err
	}
	newRecord := <-enrChan //get new node info from the enrChan
	if newRecord == nil {
		c.logger.Error("get enr failed", zap.String("node", node.String()))
		return nil, fmt.Errorf("get enr failed: %s", node.String())
	}
	//check the record is correct
	if newRecord.ID() != node.ID() {
		return nil, fmt.Errorf("invalid ID in response record")
	}

	//whether we need to update the node info
	if newRecord.Seq() > node.Seq() {
		//update node
		if err := netutil.CheckRelayIP(node.IP(), newRecord.IP()); err == nil {
			*node = *newRecord
		}
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
			var count = 0
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
					if end { //we get all same nodes we have crawled
						count++
					}
					if count >= Threshold || len(res) >= MaxDHTSize {
						return res, nil
					}
				}
			}
		}
	}
}

//keep read the message from the connection, we will deal with the different message
//
func (c *Crawler) loop(conn UDPConn, ctx context.Context, ld *enode.LocalNode,
	prk *ecdsa.PrivateKey, nodesChan chan nodes, enrChan chan *enode.Node) {
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
			case *v4wire.ENRResponse:
				//we send it to channel
				respN, err := enode.New(enode.ValidSchemes, &packet.(*v4wire.ENRResponse).Record)
				if err != nil {
					c.logger.Error("new enr failed", zap.Error(err))
					enrChan <- nil //send nil to the channel
					return
				}
				//send the enr to the channel
				enrChan <- respN
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
	for i := range buffer {
		node := buffer[i]
		_, err = statement.Exec(node.n.ID().String(), node.n.Seq(), node.AccessTime, node.n.IP().String())
		if err != nil {
			return fmt.Errorf("exec sql statement failed: %v", err)
		}
	}
	return nil
}
