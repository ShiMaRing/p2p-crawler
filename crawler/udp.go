package crawler

import "net"

//udp with counter

type CounterUDP struct {
	conn    UDPConn
	counter *Counter
}

func NewCounterUDP(conn UDPConn, counter *Counter) *CounterUDP {
	return &CounterUDP{conn: conn, counter: counter}
}

func (c *CounterUDP) ReadFromUDP(b []byte) (n int, addr *net.UDPAddr, err error) {
	udp, udpAddr, err := c.conn.ReadFromUDP(b)
	if err == nil {
		c.counter.AddRecvNum()
		c.counter.AddDataSizeReceived(uint64(udp))
	}
	return udp, udpAddr, err
}

func (c *CounterUDP) WriteToUDP(b []byte, addr *net.UDPAddr) (n int, err error) {
	udp, err := c.conn.WriteToUDP(b, addr)
	if err == nil {
		c.counter.AddSendNum()
		c.counter.AddDataSizeSent(uint64(udp))
	}
	return udp, err
}

func (c *CounterUDP) Close() error {
	return c.conn.Close()
}

func (c *CounterUDP) LocalAddr() net.Addr {
	return c.conn.LocalAddr()
}
