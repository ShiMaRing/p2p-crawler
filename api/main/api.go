package main

import (
	"database/sql"
	"fmt"
	"github.com/gin-gonic/gin"
	_ "github.com/go-sql-driver/mysql"
	"time"
)

const (
	driverName     = "mysql"
	dataSourceName = "root:root@tcp(127.0.0.1:32769)/db_p2p_crawler?parseTime=true"
)

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

func main() {

	// Connect to MySQL
	db, err := sql.Open(driverName, dataSourceName)
	if err != nil {
		panic(err.Error())
	}
	defer db.Close()

	// Create a gin router with default middleware:
	r := gin.Default()

	// Get all NodeRecord objects
	r.GET("/nodeRecords", func(c *gin.Context) {

		// Query data
		rows, err := db.Query("SELECT * FROM db_p2p_crawler.nodes")
		if err != nil {
			panic(err.Error())
		}
		defer rows.Close()

		// Iterate over the rows, sending one JSON object per row to the client with Comma-separated values
		var ret = make([]NodeRecord, 0)
		for rows.Next() {
			var nodeRecord NodeRecord
			err := rows.Scan(&nodeRecord.ID, &nodeRecord.Seq, &nodeRecord.AccessTime, &nodeRecord.Address, &nodeRecord.ConnectAble, &nodeRecord.NeighborCount, &nodeRecord.Country, &nodeRecord.City, &nodeRecord.Clients, &nodeRecord.Os, &nodeRecord.ClientsRuntime, &nodeRecord.NetworkID, &nodeRecord.TotalDifficulty, &nodeRecord.HeadHash)
			if err != nil {
				fmt.Println(err.Error())
				panic(err.Error())
			}
			ret = append(ret, nodeRecord)
		}

		c.JSON(200, ret)
	})

	// Get a NodeRecord object by id
	r.GET("/nodeRecords/:id", func(c *gin.Context) {

		// Get id from url
		id := c.Param("id")

		// Query data
		var nodeRecord NodeRecord
		err := db.QueryRow("SELECT * FROM db_p2p_crawler.nodes WHERE id = ?", id).Scan(&nodeRecord.ID, &nodeRecord.Seq, &nodeRecord.AccessTime, &nodeRecord.Address, &nodeRecord.ConnectAble, &nodeRecord.NeighborCount, &nodeRecord.Country, &nodeRecord.City, &nodeRecord.Clients, &nodeRecord.Os, &nodeRecord.ClientsRuntime, &nodeRecord.NetworkID, &nodeRecord.TotalDifficulty, &nodeRecord.HeadHash)
		if err != nil {
			fmt.Println(err.Error())
			panic(err.Error())
		}

		c.JSON(200, nodeRecord)
	})

	// Create a new NodeRecord
	r.POST("/nodeRecords", func(c *gin.Context) {
		var nodeRecord NodeRecord
		err := c.BindJSON(&nodeRecord)
		if err != nil {
			fmt.Println(err.Error())
			panic(err.Error())
		}

		// Insert data
		stmt, err := db.Prepare("INSERT INTO db_p2p_crawler.nodes(id, seq, access_time, address, connect_able, neighbor_count, country, city, clients, os, clients_runtime, network_id, total_difficulty, head_hash) VALUES(?,?,?,?,?, ?, ?, ?, ?, ?, ?, ?, ?, ?)")
		if err != nil {
			panic(err.Error())
		}
		stmt.Exec(nodeRecord.ID, nodeRecord.Seq, nodeRecord.AccessTime, nodeRecord.Address, nodeRecord.ConnectAble, nodeRecord.NeighborCount, nodeRecord.Country, nodeRecord.City, nodeRecord.Clients, nodeRecord.Os, nodeRecord.ClientsRuntime, nodeRecord.NetworkID, nodeRecord.TotalDifficulty, nodeRecord.HeadHash)
		defer stmt.Close()
	})

	// Update a NodeRecord by id
	r.PUT("/nodeRecords/:id", func(c *gin.Context) {
		var nodeRecord NodeRecord
		err := c.BindJSON(&nodeRecord)
		if err != nil {
			fmt.Println(err.Error())
			panic(err.Error())
		}

		// Update data
		stmt, err := db.Prepare("UPDATE db_p2p_crawler.nodes SET seq=?, access_time=?, address=?, connect_able=?, neighbor_count=?, country=?, city=?, clients=?, os=?, clients_runtime=?, network_id=?, total_difficulty=?, head_hash=? WHERE id=?")
		if err != nil {
			panic(err.Error())
		}
		stmt.Exec(nodeRecord.Seq, nodeRecord.AccessTime, nodeRecord.Address, nodeRecord.ConnectAble, nodeRecord.NeighborCount, nodeRecord.Country, nodeRecord.City, nodeRecord.Clients, nodeRecord.Os, nodeRecord.ClientsRuntime, nodeRecord.NetworkID, nodeRecord.TotalDifficulty, nodeRecord.HeadHash, nodeRecord.ID)
		defer stmt.Close()
	})

	// Delete a NodeRecord by id
	r.DELETE("/nodeRecords/:id", func(c *gin.Context) {
		// Get id from url
		id := c.Param("id")

		// Delete data
		stmt, err := db.Prepare("DELETE FROM db_p2p_crawler.nodes WHERE id=?")
		if err != nil {
			panic(err.Error())
		}
		stmt.Exec(id)
		defer stmt.Close()
	})

	// Run server on port 8888
	r.Run(":8888")
}