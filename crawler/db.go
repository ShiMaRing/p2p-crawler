package crawler

import (
	"database/sql"
	_ "github.com/go-sql-driver/mysql"
	"strconv"
	"time"
)

// DB TODO: operate with mysql database
type DB struct {
	url     string //url: the mysql url
	table   string //table: the table name
	base    *sql.DB
	nodesCh chan *Node //receive node from crawler,and write to db
}

type NodeRecord struct {
	ID              string
	Seq             uint64
	AccessTime      time.Time
	Address         string
	ConnectAble     bool
	NeighborCount   int
	Country         string
	City            string
	Clients         string
	Os              string
	ClientsRuntime  string
	NetworkID       int
	TotalDifficulty string
	HeadHash        string
}

func NewDB(url string, table string, nodesCh chan *Node) (*DB, error) {
	db, err := sql.Open("mysql", url)
	if err != nil {
		return nil, err
	}
	return &DB{
		url:     url,
		table:   table,
		base:    db,
		nodesCh: nodesCh,
	}, nil
}

//node2Record: change our node  to record type for insert and select
func node2Record(node *Node) *NodeRecord {
	record := &NodeRecord{
		ID:            node.ID.String(),
		Seq:           node.Seq,
		AccessTime:    node.AccessTime,
		Address:       node.Address.String(),
		ConnectAble:   node.ConnectAble,
		NeighborCount: node.NeighborsCount,
		Country:       node.Country,
		City:          node.City,
	}
	if node.ClientInfo != nil {
		parse(node.ClientInfo, record)
	}
	return record
}

func parse(info *ClientInfo, record *NodeRecord) {
	parsed := ParseVersionString(info.ClientType)
	record.NetworkID = int(info.NetworkID)
	record.TotalDifficulty = info.TotalDifficulty.Text(10)
	record.HeadHash = info.HeadHash.String()
	if parsed != nil {
		record.Clients = parsed.Name
		record.Os = parsed.Os.Os
		record.ClientsRuntime = parsed.Language.Name
	}
}

func (db *DB) Run() error {
	statement, err := db.base.Prepare("REPLACE INTO " + db.table +
		"(id,seq,access_time,address,connect_able,neighbor_count,country,city,clients,os,clients_runtime,network_id,total_difficulty,head_hash) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?)")
	if err != nil {
		panic(err)
	}
	defer db.Close()
	//receive node from chan
	for {
		node := <-db.nodesCh
		//write to db
		record := node2Record(node)
		//exec statement
		_, err = statement.Exec(
			record.ID,
			strconv.Itoa(int(record.Seq)),
			record.AccessTime, record.Address,
			record.ConnectAble,
			record.NeighborCount,
			record.Country,
			record.City,
			record.Clients,
			record.Os,
			record.ClientsRuntime,
			strconv.Itoa(record.NetworkID),
			record.TotalDifficulty,
			record.HeadHash,
		)
		if err != nil {
			return err
		}
	}
}

func (db *DB) Close() {
	db.base.Close()
}
