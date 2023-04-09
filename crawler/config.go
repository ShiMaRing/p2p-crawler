package crawler

import (
	"time"
)

const (
	DefaultWorkers = 32
	MAX_WORKERS    = 64
)

// Config is the context that the crawler uses to store the state of the crawler
type Config struct {
	TotalTimeout time.Duration //limit the time of the crawler life
	RoundTimeout time.Duration //limit the time of each round

	Workers int // Workers is the number of workers that the crawler will use to crawl_bfs the network.

	//config for level db
	IsPersistent bool   // IsPersistent is the flag that the crawler uses to determine if it should store the nodes in the database.
	DbName       string // dbName is the name of the leveldb ,if the crawler uses leveldb to store the nodes persistent.

	//config for sql db
	IsSql       bool   // IsSql is the flag that the crawler uses to determine if it should store the nodes in the outer database.
	DatabaseUrl string // DatabaseUrl is the url of the database that the crawler will use to store the nodes.
	TableName   string // TableName is the name of the table that the crawler will use to store the nodes.

	Zeus bool // Zeus is the flag that the crawler uses to determine if it should store the nodes in the outer database.
}

func DefaultConfig() Config {
	return Config{
		TotalTimeout: DefaultTimeout,
		RoundTimeout: RoundInterval,
		Workers:      DefaultWorkers,
		IsPersistent: false,

		IsSql:       true,
		DatabaseUrl: "root:xgs1150840779@tcp(localhost:3306)/ethernodes?charset=utf8",
		TableName:   "nodes",
		Zeus:        false,
	}
}
