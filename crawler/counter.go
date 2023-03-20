package crawler

import (
	"sync"
	"time"
)

// Counter records the package numbers we send and received
// also records nodes number and time cost
type Counter struct {
	SendNum          int
	RecvNum          int
	NodesNum         int
	dataSizeSended   uint64 //bytes
	dataSizeReceived uint64 //data size received
	TimeCost         time.Duration
	Rwlock           sync.RWMutex
	StartTime        time.Time //start time we send the first package
}

// AddSendNum add send number
func (c *Counter) AddSendNum() {
	c.Rwlock.Lock()
	defer c.Rwlock.Unlock()
	c.SendNum++
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
	c.dataSizeSended += size
}

// AddDataSizeReceived add data size received
func (c *Counter) AddDataSizeReceived(size uint64) {
	c.Rwlock.Lock()
	defer c.Rwlock.Unlock()
	c.dataSizeReceived += size
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
func (c *Counter) GetDataSizeSended() uint64 {
	c.Rwlock.RLock()
	defer c.Rwlock.RUnlock()
	return c.dataSizeSended
}

func (c *Counter) GetDataSizeReceived() uint64 {
	c.Rwlock.RLock()
	defer c.Rwlock.RUnlock()
	return c.dataSizeReceived
}
