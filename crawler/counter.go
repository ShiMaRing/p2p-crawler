package crawler

import (
	"strconv"
	"sync"
	"time"
)

const (
	KB = 1024
	MB = KB * KB
)

// Counter records the package numbers we send and received
// also records nodes number and time cost
type Counter struct {
	SendNum          int
	RecvNum          int
	NodesNum         int
	ConnectAbleNodes int //connectable nodes
	dataSizeSended   int //bytes
	dataSizeReceived int //data size received
	Rwlock           sync.RWMutex
	StartTime        time.Time //start time we send the first package
	ClientInfoCount  int
	EthNodesCount    int
}

// AddSendNum add send number
func (c *Counter) AddSendNum() {
	c.Rwlock.Lock()
	defer c.Rwlock.Unlock()
	c.SendNum++
}

// AddEthNodesCount add eth nodes count
func (c *Counter) AddEthNodesCount() {
	c.Rwlock.Lock()
	defer c.Rwlock.Unlock()
	c.EthNodesCount++
}

// AddClientInfoCount add client info count
func (c *Counter) AddClientInfoCount() {
	c.Rwlock.Lock()
	defer c.Rwlock.Unlock()
	c.ClientInfoCount++
}

// AddRecvNum add recv number
func (c *Counter) AddRecvNum() {
	c.Rwlock.Lock()
	defer c.Rwlock.Unlock()
	c.RecvNum++
}

// AddNodesNum add nodes number
func (c *Counter) AddNodesNum() {
	c.Rwlock.Lock()
	defer c.Rwlock.Unlock()
	c.NodesNum++
}

// AddDataSizeSent  add data size sended
func (c *Counter) AddDataSizeSent(size uint64) {
	c.Rwlock.Lock()
	defer c.Rwlock.Unlock()
	c.dataSizeSended += int(size)
}

// AddDataSizeReceived add data size received
func (c *Counter) AddDataSizeReceived(size uint64) {
	c.Rwlock.Lock()
	defer c.Rwlock.Unlock()
	c.dataSizeReceived += int(size)
}

//get functions for Counter
func (c *Counter) GetSendNum() int {
	c.Rwlock.RLock()
	defer c.Rwlock.RUnlock()
	return c.SendNum
}

func (c *Counter) GetRecvNum() int {
	c.Rwlock.RLock()
	defer c.Rwlock.RUnlock()
	return c.RecvNum
}

func (c *Counter) GetNodesNum() int {
	c.Rwlock.RLock()
	defer c.Rwlock.RUnlock()
	return c.NodesNum
}
func (c *Counter) GetDataSizeSended() int {
	c.Rwlock.RLock()
	defer c.Rwlock.RUnlock()
	return c.dataSizeSended
}

func (c *Counter) GetDataSizeReceived() int {
	c.Rwlock.RLock()
	defer c.Rwlock.RUnlock()
	return c.dataSizeReceived
}

func (c *Counter) ToString() string {
	c.Rwlock.RLock()
	defer c.Rwlock.RUnlock()
	return "sendNum:" + strconv.Itoa(c.SendNum) + "  recvNum:" + strconv.Itoa(c.RecvNum) + "  nodesNum:" + strconv.Itoa(c.NodesNum) +
		"  connectable:" + strconv.Itoa(c.ConnectAbleNodes) + "  dataSizeSended:" + dataSize2String(c.dataSizeSended) +
		"  dataSizeReceived:" + dataSize2String(c.dataSizeReceived) + "  ClientInfoCount:" + strconv.Itoa(c.ClientInfoCount) +
		"  EthNodesCount:" + strconv.Itoa(c.EthNodesCount)

}

func (c *Counter) GetStartTime() time.Time {
	c.Rwlock.RLock()
	defer c.Rwlock.RUnlock()
	return c.StartTime
}

func (c *Counter) GetCostTime() time.Duration {
	c.Rwlock.RLock()
	defer c.Rwlock.RUnlock()
	return time.Now().Sub(c.StartTime)
}

// AddConnectAbleNodes add connectable nodes
func (c *Counter) AddConnectAbleNodes() {
	c.Rwlock.Lock()
	defer c.Rwlock.Unlock()
	c.ConnectAbleNodes++
}

//GetConnectedCount get counter
func (c *Counter) GetConnectedCount() int {
	c.Rwlock.RLock()
	defer c.Rwlock.RUnlock()
	return c.ConnectAbleNodes
}

func dataSize2String(size int) string {
	var res = float64(size) / MB
	return strconv.FormatFloat(res, 'f', 2, 64) + "MB"
}
