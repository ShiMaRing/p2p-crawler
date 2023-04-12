package main

import (
	"database/sql"
	"fmt"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	_ "github.com/go-sql-driver/mysql"
	"p2p-crawler/crawler"
	"time"
)

const (
	driverName     = "mysql"
	dataSourceName = crawler.DataSourceURl
)

type NodeRecord struct {
	ID              string    `json:"id,omitempty"`
	Seq             uint64    `json:"seq,omitempty"`
	AccessTime      time.Time `json:"accessTime,omitempty"`
	Address         string    `json:"address,omitempty"`
	ConnectAble     bool      `json:"connectAble,omitempty"`
	NeighborCount   int       `json:"neighborCount,omitempty"`
	Country         string    `json:"country,omitempty"`
	City            string    `json:"city,omitempty"`
	Clients         string    `json:"clients,omitempty"`
	Os              string    `json:"os,omitempty"`
	ClientsRuntime  string    `json:"clientsRuntime,omitempty"`
	NetworkID       int       `json:"networkId,omitempty"`
	TotalDifficulty string    `json:"totalDifficulty,omitempty"`
	HeadHash        string    `json:"headHash,omitempty"`
}

const dbName = "ethernodes"

//wrapper the query
func wrapper(s string) string {
	return fmt.Sprintf(s, dbName)
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

	// Enable CORS
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"http://localhost", "http://127.0.0.1:5173"},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowCredentials: true,
	}))

	// Get all NodeRecord objects
	r.GET("/nodeRecords", func(c *gin.Context) {

		// Query data
		rows, err := db.Query(wrapper("SELECT * FROM %s.nodes"))
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
		err := db.QueryRow(wrapper("SELECT * FROM %s.nodes WHERE id = ?"), id).Scan(&nodeRecord.ID, &nodeRecord.Seq, &nodeRecord.AccessTime, &nodeRecord.Address, &nodeRecord.ConnectAble, &nodeRecord.NeighborCount, &nodeRecord.Country, &nodeRecord.City, &nodeRecord.Clients, &nodeRecord.Os, &nodeRecord.ClientsRuntime, &nodeRecord.NetworkID, &nodeRecord.TotalDifficulty, &nodeRecord.HeadHash)
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
		stmt, err := db.Prepare(wrapper("INSERT INTO %s.nodes(id, seq, access_time, address, connect_able, neighbor_count, country, city, clients, os, clients_runtime, network_id, total_difficulty, head_hash) VALUES(?,?,?,?,?, ?, ?, ?, ?, ?, ?, ?, ?, ?)"))
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
		stmt, err := db.Prepare(wrapper("UPDATE %s.nodes SET seq=?, access_time=?, address=?, connect_able=?, neighbor_count=?, country=?, city=?, clients=?, os=?, clients_runtime=?, network_id=?, total_difficulty=?, head_hash=? WHERE id=?"))
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
		stmt, err := db.Prepare(wrapper("DELETE FROM %s.nodes WHERE id=?"))
		if err != nil {
			panic(err.Error())
		}
		stmt.Exec(id)
		defer stmt.Close()
	})

	// Run server on port 8888
	r.Run(":8888")
}
