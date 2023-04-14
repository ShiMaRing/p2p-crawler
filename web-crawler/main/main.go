package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"p2p-crawler/crawler"
	"sync"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

const (
	driverName     = "mysql"
	dbName         = "ethernodes"
	dataSourceName = crawler.DataSourceURl
)

// Wrapper the query
func Wrapper(s string) string {
	return fmt.Sprintf(s, dbName)
}

type NodeRecord struct {
	ID              string    `json:"id,omitempty"`
	Seq             uint64    `json:"seq,omitempty"`
	AccessTime      time.Time `json:"accessTime,omitempty"`
	Address         string    `json:"host,omitempty"`
	ConnectAble     bool      `json:"connectAble,omitempty"`
	NeighborCount   int       `json:"neighborCount,omitempty"`
	Country         string    `json:"country,omitempty"`
	City            string    `json:"city,omitempty"`
	Clients         string    `json:"client,omitempty"`
	Os              string    `json:"os,omitempty"`
	ClientsRuntime  string    `json:"clientsRuntime,omitempty"`
	NetworkID       int       `json:"networkId,omitempty"`
	TotalDifficulty string    `json:"totalDifficulty,omitempty"`
	HeadHash        string    `json:"headHash,omitempty"`
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
	stmt, err := db.Prepare(Wrapper("REPLACE INTO %s.nodes(id, seq, access_time, address, connect_able, neighbor_count, country, city, clients, os, clients_runtime, network_id, total_difficulty, head_hash) VALUES(?,?,?,?,?, ?, ?, ?, ?, ?, ?, ?, ?, ?)"))
	if err != nil {
		panic(err.Error())
	}
	defer stmt.Close()

	var count int
	for _, nodeRecord := range respData.Data {
		c, err := stmt.Exec(nodeRecord.ID, nodeRecord.Seq, RandomTime(), nodeRecord.Address, true, nodeRecord.NeighborCount, nodeRecord.Country, nodeRecord.City, nodeRecord.Clients, nodeRecord.Os, nodeRecord.ClientsRuntime, nodeRecord.NetworkID, nodeRecord.TotalDifficulty, nodeRecord.HeadHash)
		if err != nil {
			fmt.Println(err)
		}
		if c != nil {
			count++
		}
	}

	fmt.Printf("OK: from: %v, to: %v, updated %v\n", start, start+100, count)

}
func main() {
	var waitGroup sync.WaitGroup

	for i := 0; i < 100000; i += 100 {
		getData(i)
		time.Sleep(1000)
	}
	waitGroup.Wait()

}

// 随机生成时间，要在之前五天内,时间也要随机
func RandomTime() time.Time {
	date := time.Now().AddDate(0, 0, -rand.Intn(5))
	hour := rand.Intn(24)
	minute := rand.Intn(60)
	second := rand.Intn(60)
	return time.Date(date.Year(), date.Month(), date.Day(), hour, minute, second, 0, date.Location())
}
