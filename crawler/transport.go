package crawler

import (
	"bytes"
	"github.com/ethereum/go-ethereum/p2p"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"io"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/p2p/rlpx"
	"github.com/ethereum/go-ethereum/rlp"
)

type Transport interface {
	Self() *enode.Node
	RequestENR(*enode.Node) (*enode.Node, error)
	lookupRandom() []*enode.Node
	lookupSelf() []*enode.Node
	ping(*enode.Node) (seq uint64, err error)
}

const (
	handshakeTimeout = 5 * time.Second

	discWriteTimeout = 1 * time.Second

	frameReadTimeout time.Duration = 30 * time.Second
)
const frameWriteTimeout time.Duration = 20 * time.Second

const DiscNetworkError p2p.DiscReason = 1

const discMsg uint64 = 0x01

type rlpxTransport struct {
	rmu, wmu sync.Mutex
	wbuf     bytes.Buffer
	conn     *rlpx.Conn
}

func newRLPX(conn *rlpx.Conn) *rlpxTransport {
	return &rlpxTransport{conn: conn}
}

func (t *rlpxTransport) ReadMsg() (p2p.Msg, error) {
	t.rmu.Lock()
	defer t.rmu.Unlock()

	var msg p2p.Msg
	t.conn.SetReadDeadline(time.Now().Add(frameReadTimeout))
	code, data, _, err := t.conn.Read()
	if err == nil {
		// Protocol messages are dispatched to subprotocol handlers asynchronously,
		// but package rlpx may reuse the returned 'data' buffer on the next call
		// to Read. Copy the message data to avoid this being an issue.
		data = common.CopyBytes(data)
		msg = p2p.Msg{
			ReceivedAt: time.Now(),
			Code:       code,
			Size:       uint32(len(data)),
			Payload:    bytes.NewReader(data),
		}
	}
	return msg, err
}

func (t *rlpxTransport) WriteMsg(msg p2p.Msg) error {
	t.wmu.Lock()
	defer t.wmu.Unlock()

	// Copy message data to write buffer.
	t.wbuf.Reset()
	if _, err := io.CopyN(&t.wbuf, msg.Payload, int64(msg.Size)); err != nil {
		return err
	}

	// Write the message.
	t.conn.SetWriteDeadline(time.Now().Add(frameWriteTimeout))
	_, err := t.conn.Write(msg.Code, t.wbuf.Bytes())
	if err != nil {
		return err
	}
	return nil
}

func (t *rlpxTransport) close(err error) {
	t.wmu.Lock()
	defer t.wmu.Unlock()

	// Tell the remote end why we're disconnecting if possible.
	// We only bother doing this if the underlying connection supports
	// setting a timeout tough.
	if t.conn != nil {
		if r, ok := err.(p2p.DiscReason); ok && r != DiscNetworkError {
			deadline := time.Now().Add(discWriteTimeout)
			if err := t.conn.SetWriteDeadline(deadline); err == nil {
				// Connection supports write deadline.
				t.wbuf.Reset()
				rlp.Encode(&t.wbuf, []p2p.DiscReason{r})
				t.conn.Write(discMsg, t.wbuf.Bytes())
			}
		}
	}
	t.conn.Close()
}
