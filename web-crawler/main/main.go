package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

const (
	driverName     = "mysql"
	dataSourceName = "root:root@tcp(127.0.0.1:32769)/db_p2p_crawler?parseTime=true"
)

type NodeRecord struct {
	ID              string    `json:"id,omitempty"`
	Seq             uint64    `json:"seq,omitempty"`
	AccessTime      time.Time `json:"lastUpdate,omitempty"`
	Address         string    `json:"host,omitempty"`
	ConnectAble     bool      `json:"connect_able,omitempty"`
	NeighborCount   int       `json:"neighbor_count,omitempty"`
	Country         string    `json:"country,omitempty"`
	City            string    `json:"city,omitempty"`
	Clients         string    `json:"client,omitempty"`
	Os              string    `json:"os,omitempty"`
	ClientsRuntime  string    `json:"clients_runtime,omitempty"`
	NetworkID       int       `json:"network_id,omitempty"`
	TotalDifficulty string    `json:"total_difficulty,omitempty"`
	HeadHash        string    `json:"head_hash,omitempty"`
}

type RespData struct {
	Draw            int
	RecordsTotal    int
	RecordsFiltered int
	Data            []NodeRecord
}

func getUrl(start, len int) string {
	url := fmt.Sprintf("https://ethernodes.org/data?draw=4&columns[0][data]=id&columns[0][name]=&columns[0][searchable]=true&columns[0][orderable]=true&columns[0][search][value]=&columns[0][search][regex]=false&columns[1][data]=host&columns[1][name]=&columns[1][searchable]=true&columns[1][orderable]=true&columns[1][search][value]=&columns[1][search][regex]=false&columns[2][data]=isp&columns[2][name]=&columns[2][searchable]=true&columns[2][orderable]=true&columns[2][search][value]=&columns[2][search][regex]=false&columns[3][data]=country&columns[3][name]=&columns[3][searchable]=true&columns[3][orderable]=true&columns[3][search][value]=&columns[3][search][regex]=false&columns[4][data]=client&columns[4][name]=&columns[4][searchable]=true&columns[4][orderable]=true&columns[4][search][value]=&columns[4][search][regex]=false&columns[5][data]=clientVersion&columns[5][name]=&columns[5][searchable]=true&columns[5][orderable]=true&columns[5][search][value]=&columns[5][search][regex]=false&columns[6][data]=os&columns[6][name]=&columns[6][searchable]=true&columns[6][orderable]=true&columns[6][search][value]=&columns[6][search][regex]=false&columns[7][data]=lastUpdate&columns[7][name]=&columns[7][searchable]=true&columns[7][orderable]=true&columns[7][search][value]=&columns[7][search][regex]=false&columns[8][data]=inSync&columns[8][name]=&columns[8][searchable]=true&columns[8][orderable]=true&columns[8][search][value]=&columns[8][search][regex]=false&order[0][column]=0&order[0][dir]=asc&start=%v&length=%v&search[value]=&search[regex]=false&_=1680864802468", start, len)
	return url
}

func getData(start int) {

	response, err := http.Get(getUrl(start, 100))
	if err != nil {
		fmt.Println(err)
		return
	}
	defer response.Body.Close()

	var respData RespData
	err = json.NewDecoder(response.Body).Decode(&respData)
	if err != nil {
		fmt.Println(err)
		return
	}

	db, err := sql.Open(driverName, dataSourceName)
	if err != nil {
		fmt.Println(err.Error())
		panic(err.Error())
	}
	defer db.Close()

	// Insert data
	stmt, err := db.Prepare("INSERT INTO db_p2p_crawler.nodes(id, seq, access_time, address, connect_able, neighbor_count, country, city, clients, os, clients_runtime, network_id, total_difficulty, head_hash) VALUES(?,?,?,?,?, ?, ?, ?, ?, ?, ?, ?, ?, ?)")
	if err != nil {
		panic(err.Error())
	}
	defer stmt.Close()

	for _, nodeRecord := range respData.Data {
		stmt.Exec(nodeRecord.ID, nodeRecord.Seq, nodeRecord.AccessTime, nodeRecord.Address, nodeRecord.ConnectAble, nodeRecord.NeighborCount, nodeRecord.Country, nodeRecord.City, nodeRecord.Clients, nodeRecord.Os, nodeRecord.ClientsRuntime, nodeRecord.NetworkID, nodeRecord.TotalDifficulty, nodeRecord.HeadHash)
	}

	fmt.Printf("OK: from: %v, to: %v\n", start, start+100)

}

func main() {
	var waitGroup sync.WaitGroup

	//getData(0)
	//time.Sleep(1000 * time.Millisecond)
	//getData(100)

	for i := 0; i < 100000; i += 100 {
		//waitGroup.Add(1)
		//go func(i int) {
		//	defer waitGroup.Done()
		//	getData(i)
		//}(i)
		getData(i)
		time.Sleep(1000)
	}
	waitGroup.Wait()

}
