package crawler

import (
	"time"
)

type Version int

const (
	Discv4 Version = iota
	Discv5

	DefaultWorkers  = 32
	DefaultPoolSize = 512
)

// Config is the context that the crawler uses to store the state of the crawler
type Config struct {
	TotalTimeout time.Duration //limit the time of the crawler life
	RoundTimeout time.Duration //limit the time of each round

	Workers  int // Workers is the number of workers that the crawler will use to crawl the network.
	PoolSize int // PoolSize is the size of the pool for tcp connections when we request for node info.

	//config for level db
	IsPersistent bool   // IsPersistent is the flag that the crawler uses to determine if it should store the nodes in the database.
	DbName       string // dbName is the name of the leveldb ,if the crawler uses leveldb to store the nodes persistent.

	//config for sql db
	IsSql       bool   // IsSql is the flag that the crawler uses to determine if it should store the nodes in the outer database.
	DatabaseUrl string // DatabaseUrl is the url of the database that the crawler will use to store the nodes.
	TableName   string // TableName is the name of the table that the crawler will use to store the nodes.
}

func DefaultConfig() Config {
	return Config{
		TotalTimeout: DefaultTimeout,
		RoundTimeout: RoundInterval,
		Workers:      DefaultWorkers,
		PoolSize:     DefaultPoolSize,
		IsPersistent: false,
		IsSql:        false,
	}
}
