package crawler

import (
	"net"
	"sync"
)

var mutex sync.Mutex

func (c *Crawler) geoSearch(ip net.IP) (country string, city string, err error) {
	mutex.Lock()
	defer mutex.Unlock()
	res, err := c.geoipDB.City(ip)
	if err != nil {
		return "", "", err
	}
	return res.Country.Names["en"], res.City.Names["en"], nil
}
