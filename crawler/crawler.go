package crawler

import (
	"bufio"
	"context"
	"crypto/ecdsa"
	"fmt"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/crypto"
	gethlog "github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/p2p/discover"
	"github.com/ethereum/go-ethereum/p2p/discover/v4wire"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/ethereum/go-ethereum/p2p/netutil"
	"github.com/ethereum/go-ethereum/params"
	_ "github.com/go-sql-driver/mysql"
	"github.com/oschwald/geoip2-golang"
	"go.uber.org/zap"
	"net"
	"os"
	"sync"
	"time"
)

const (
	RoundInterval     = 30 * time.Second //crawl_bfs interval for each node
	DefaultTimeout    = 60 * time.Minute //check interval for all nodes
	respTimeout       = 500 * time.Millisecond
	DefaultChanelSize = 10248
	seedCount         = 64
	seedMaxAge        = 5 * 24 * time.Hour
	MaxDHTSize        = 17 * 16
	Threshold         = 2
	StartRound        = 2
)

type Crawler struct {
	BootNodes []*enode.Node         // BootNodes is the set of nodes that the crawler will start from.
	Cache     map[enode.ID]struct{} // Cache is the set of nodes that the crawler is currently crawling,as a cache
	tokens    chan struct{}         //tokens store token

	ReqCh      chan *enode.Node // ReqCh is the channel that the crawler uses to send requests to the workers.
	OutputCh   chan *Node       // OutputCh is the channel that the crawler uses to send requests to the filter.
	DHTCh      chan *enode.Node // DHTCh is the channel that the crawler uses to send requests to the DHT.
	databaseCh chan *Node       //databaseCh is the channel that the crawler uses to send requests to the database.

	leveldb   *enode.DB          // leveldb is the database that the crawler uses to store the nodes.
	db        *DB                // db is the database that the crawler uses to store the nodes.
	tableName string             // tableName is the name of the table that the crawler will use to store the nodes.
	mu        sync.Mutex         // mu is the mutex that protects the crawler.
	ctx       context.Context    // ctx is the context that the crawler uses to cancel all crawlBFS.
	cancel    context.CancelFunc // cancel is the function that the crawler uses to cancel all crawlBFS.

	prk *ecdsa.PrivateKey // prk is the private key that the crawler uses to sign the requests.
	ld  *enode.LocalNode  // ld is the local node that the crawler uses to sign the requests.

	discv5Pool Pool // discv5 for zeus algorithm ,discv4 not appropriate to apply zeus algorithm.

	genesis *core.Genesis // genesis is the genesis block that the crawler uses to verify the nodes.

	counter *Counter //counter is the counter that the crawler uses to count the nodes.

	logger *zap.Logger // logger is the logger that the crawler uses to log the information.
	Config             // config is the config that the crawler uses to store the state of the crawler.

	writer  *bufio.Writer  //writer is the writer that the crawler uses to write the nodes to the file.
	geoipDB *geoip2.Reader //geoipDB is the database that the crawler uses to get the country and city from ip address.
}

func NewCrawler(config Config) (*Crawler, error) {
	var err error
	nodes := make([]*enode.Node, 0)
	//start from the boot nodes

	//start from the MainBootNodes
	s := params.MainnetBootnodes
	for _, record := range s {
		n, err := parseNode(record)
		nodes = append(nodes, n) //add the node to the nodes
		if err != nil {
			return nil, fmt.Errorf("invalid bootstrap node: %v", err)
		}
	}
	var ldb *enode.DB
	var prk *ecdsa.PrivateKey
	var ld *enode.LocalNode
	var cfg discover.Config

	if config.IsPersistent == false {
		ldb, err = enode.OpenDB("")
		if err != nil {
			return nil, err
		}
		//add the nodes to the leveldb
	} else {
		if config.DbName == "" {
			return nil, fmt.Errorf("invalid database name")
		}
		//start from the database
		ldb, err = enode.OpenDB(config.DbName)
		if err != nil {
			return nil, err
		}
	}
	counter := &Counter{}
	ld, cfg = makeDiscoveryConfig(ldb, nodes)
	prk = cfg.PrivateKey
	conn := listen(ld, "")
	defer func() {
		conn.Close()
	}()
	v4, err := discover.ListenV4(conn, ld, cfg)
	if err != nil {
		return nil, err
	}
	group := &sync.WaitGroup{}
	lock := sync.Mutex{}
	nodes = nodes[:0] //reset the nodes
	for i := 0; i < StartRound; i++ {
		group.Add(1)
		go func() {
			defer group.Done()
			key, _ := crypto.GenerateKey()
			pbkey := &key.PublicKey
			lock.Lock()
			nodes = append(nodes, v4.LookupPubkey(pbkey)...)
			lock.Unlock()
		}()
	}
	group.Wait()
	for i := range nodes {
		ldb.UpdateNode(nodes[i])
	}
	//create a discv5 pool,we do discv5 and v4 parallel
	var discv5pool Pool
	// Set custom logger for the discovery5 service (Debug)
	gethLogger := gethlog.New()
	gethLogger.SetHandler(gethlog.FuncHandler(func(r *gethlog.Record) error {
		return nil
	}))
	tmp, err := enode.OpenDB("")

	if config.Zeus {
		var nodesTmps = make([]*enode.Node, len(nodes))
		copy(nodesTmps, nodes)
		discv5pool, _ = NewChannelPool(DefaultWorkers, MAX_WORKERS, func() (*discover.UDPv5, error) {
			if err != nil {
				return nil, err
			}
			var tmps = make([]*enode.Node, len(nodesTmps)) //we need to copy the nodesTmps
			copy(tmps, nodesTmps)
			prk, err := crypto.GenerateKey()
			// configuration of the discovery5
			ld := enode.NewLocalNode(tmp, prk)
			cfg := discover.Config{
				PrivateKey:   prk,
				NetRestrict:  nil,
				Bootnodes:    nodes,
				Unhandled:    nil, // Not used in dv5
				Log:          gethLogger,
				ValidSchemes: enode.ValidSchemes,
			}
			// udp address to listen
			udpAddr := &net.UDPAddr{
				IP: net.IPv4(0, 0, 0, 0),
			}
			conn, err := net.ListenUDP("udp", udpAddr)
			if err != nil {
				return nil, err
			}
			counterUDP := &CounterUDP{
				conn:    conn,
				counter: counter,
			}
			discv5, err := discover.ListenV5(counterUDP, ld, cfg)
			if err != nil {
				return nil, err
			}
			return discv5, nil
		})
	}

	//create ctx
	reqCh := make(chan *enode.Node, DefaultChanelSize)
	dhtCh := make(chan *enode.Node, DefaultChanelSize)
	outputCh := make(chan *Node, DefaultChanelSize)
	databaseCh := make(chan *Node, DefaultChanelSize)

	var base *DB
	if config.IsSql == true { //we need to store the nodes to the database
		base, err = NewDB(config.DatabaseUrl, config.TableName, databaseCh)
		if err != nil {
			return nil, err
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), config.TotalTimeout)
	logger, _ := zap.NewProduction()

	geoipDB, err := geoip2.Open("db/GeoLite2-City.mmdb")

	crawler := &Crawler{
		BootNodes:  nodes,
		Cache:      make(map[enode.ID]struct{}),
		ReqCh:      reqCh,
		tokens:     make(chan struct{}, config.Workers),
		OutputCh:   outputCh,
		databaseCh: databaseCh,
		DHTCh:      dhtCh,
		leveldb:    ldb,
		db:         base,
		ctx:        ctx,
		logger:     logger,
		tableName:  config.TableName,
		cancel:     cancel,
		Config:     config,
		geoipDB:    geoipDB,
		counter:    counter,
		genesis:    makeGenesis(),
		ld:         ld,
		prk:        prk,
		discv5Pool: discv5pool, //for zeus algorithm
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
	//create the disc service
	c.RunDiscService()

	defer func() {
		c.cancel()
		c.writer.Flush()
		c.geoipDB.Close()
		c.Close()
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
			//fmt.Println("get token, and left:", len(c.tokens))
			go c.Crawl()
		}
	}

}

// Persistent persists the nodes to the database,which run in the background.
func (c *Crawler) daemon() {
	//save the nodes to the database
	//create a file ,if exists, truncate it

	go func() {
		if c.db != nil {
			if err := c.db.Run(); err != nil {
				panic(err)
			}
		}
	}()

	var err error
	ticker := time.NewTicker(2 * time.Second) //every 2 seconds, we output the nodes to the file
	go func() {
		for {
			select {
			case <-ticker.C:
				fmt.Println(c.counter.ToString()) //print the counter
				fmt.Println("worker num:", len(c.tokens), "  req num:", len(c.ReqCh), "  output num:", len(c.OutputCh), "  DHT num:", len(c.DHTCh))
			}
		}
	}()
	for {
		select {
		case <-c.ctx.Done():
			//save the nodes in outputCh to the database
			time.Sleep(RoundInterval)
			c.Close()
			for n := range c.OutputCh {
				c.leveldb.UpdateNode(n.n)
				if c.IsSql {
					c.databaseCh <- n
				}
			}
			return
		case node := <-c.OutputCh:
			if node.ConnectAble {
				c.writer.WriteString(node.n.String() + "\n")
			}
			//we need to deal with the node info and save it to the mysql database
			if !c.IsSql || node.ClientInfo == nil {
				continue
			}
			c.databaseCh <- node //send the node to the database chan
		case node := <-c.DHTCh: //add the node to the reqch back
			c.mu.Lock()
			err = c.leveldb.UpdateNode(node)
			if err != nil {
				c.logger.Error("save nodes to leveldb failed", zap.Error(err))
			}
			//we did not crawl_bfs the node,so we should add it to the reqch
			if _, ok := c.Cache[node.ID()]; !ok {
				c.Cache[node.ID()] = struct{}{} //add to the cache
				c.ReqCh <- node                 //add to the reqch
			}
			c.mu.Unlock()
		}
	}
}

// Crawl bfs crawl_bfs method
func (c *Crawler) Crawl() {
	select {
	case <-c.ctx.Done():
		return
	case node := <-c.ReqCh:
		c.mu.Lock()
		c.Cache[node.ID()] = struct{}{}
		c.mu.Unlock()
		//get node for crawl_bfs,the node never crawled
		// in this wat ,choose the crawl method
		//we can choose the crawl_bfs algorithm here
		var result []*enode.Node
		var err error
		//we try to get the nodes from the dht use zeus first
		result, err = c.crawlZeus(node) //we also updated the node info
		if err != nil || result == nil {
			//c.logger.Error("crawlZeus node failed", zap.Error(err))
			result, err = c.crawlBFS(node) //we also try to get the nodes from the random nodes
		}

		myNode := &Node{
			n:          node,
			Seq:        node.Seq(),
			Address:    node.IP(),
			ID:         node.ID(),
			AccessTime: time.Now(),
		}

		if result != nil {
			for i := range result {
				c.DHTCh <- result[i] //add the node to the dhtch
			}
			myNode.NeighborsCount = len(result)
			myNode.ConnectAble = true
			c.counter.AddConnectAbleNodes() //add the connectable nodes
			info, err := getClientInfo(c.genesis, 1, myNode.n, c.prk)
			if err == nil && info != nil {
				myNode.ClientInfo = info
				c.counter.AddClientInfoCount() //add the client info count
			}
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

// crawlBFS the node with bfs and random method
func (c *Crawler) crawlBFS(node *enode.Node) ([]*enode.Node, error) {
	var ctx, cancel = context.WithTimeout(context.Background(), RoundInterval)
	var cache = make(map[enode.ID]*enode.Node) //cache the nodes
	var res []*enode.Node
	var prk = c.prk
	var ld = c.ld

	conn := listen(ld, "") //bind the local node to the port
	nodesChan := make(chan nodes, 32)
	enrChan := make(chan *enode.Node, 32)

	defer func() {
		c.tokens <- struct{}{} //send token back for next worker
		conn.Close()
		cancel()
		time.Sleep(500 * time.Millisecond)
		close(enrChan)
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

	go func() {
		var newRecord *enode.Node
		select {
		case <-ctx.Done():
			return
		case newRecord = <-enrChan: //get new node info from the enrChan
			if newRecord == nil {
				return
			}
		}
		//check the record is correct
		if newRecord.ID() != node.ID() {
			return
		}
		//whether we need to update the node info
		if newRecord.Seq() > node.Seq() {
			//update node
			if err := netutil.CheckRelayIP(node.IP(), newRecord.IP()); err == nil {
				c.mu.Lock()
				*node = *newRecord
				c.mu.Unlock()
			}
		}
	}()

	//we try to crawl_bfs the DHT table
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

// keep read the message from the connection, we will deal with the different message
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

func (c *Crawler) Close() {
	close(c.DHTCh)
	close(c.OutputCh)
	close(c.ReqCh)
	c.discv5Pool.Close()
}
