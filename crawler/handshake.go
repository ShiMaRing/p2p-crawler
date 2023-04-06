package crawler

import (
	"crypto/ecdsa"
	"fmt"
	"math/big"
	"net"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/forkid"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/p2p"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/ethereum/go-ethereum/p2p/rlpx"
	"github.com/ethereum/go-ethereum/params"

	"github.com/pkg/errors"
)

var (
	_status          *Status
	lastStatusUpdate time.Time
)

type ClientInfo struct {
	ClientType      string
	SoftwareVersion uint64
	Capabilities    []p2p.Cap
	NetworkID       uint64
	ForkID          forkid.ID
	LastSeenTime    time.Time
	TotalDifficulty *big.Int
	HeadHash        common.Hash
}

func getClientInfo(genesis *core.Genesis, networkID uint64, n *enode.Node) (*ClientInfo, error) {
	var info ClientInfo

	conn, sk, err := dial(n)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	if err = conn.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
		return nil, errors.Wrap(err, "cannot set conn deadline")
	}

	if err = writeHello(conn, sk); err != nil {
		return nil, err
	}
	if err = readHello(conn, &info); err != nil {
		return nil, err
	}

	// If node provides no eth version, we can skip it.
	if conn.negotiatedProtoVersion == 0 {
		return &info, nil
	}

	if err = conn.SetDeadline(time.Now().Add(15 * time.Second)); err != nil {
		return nil, errors.Wrap(err, "cannot set conn deadline")
	}

	s := getStatus(genesis.Config, uint32(conn.negotiatedProtoVersion), genesis.ToBlock().Hash(), networkID)
	if err = conn.Write(s); err != nil {
		return nil, err
	}

	// Regardless of whether we wrote a status message or not, the remote side
	// might still send us one.
	time.Sleep(respTimeout) // wait for response

	if err = readStatus(conn, &info); err != nil {
		return nil, err
	}

	//we also need more info from the node
	// Disconnect from client
	_ = conn.Write(Disconnect{Reason: p2p.DiscQuitting})
	return &info, nil
}

// dial attempts to dial the given node and perform a handshake,
func dial(n *enode.Node) (*Conn, *ecdsa.PrivateKey, error) {
	var conn Conn

	// dial
	fd, err := net.Dial("tcp", fmt.Sprintf("%v:%d", n.IP(), n.TCP()))
	if err != nil {
		return nil, nil, err
	}

	conn.Conn = rlpx.NewConn(fd, n.Pubkey())

	if err = conn.SetDeadline(time.Now().Add(10 * time.Second)); err != nil {
		return nil, nil, errors.Wrap(err, "cannot set conn deadline")
	}

	// do encHandshake
	ourKey, _ := crypto.GenerateKey()

	_, err = conn.Handshake(ourKey)
	if err != nil {
		return nil, nil, err
	}

	return &conn, ourKey, nil
}

func writeHello(conn *Conn, priv *ecdsa.PrivateKey) error {
	pub0 := crypto.FromECDSAPub(&priv.PublicKey)[1:]

	h := &Hello{
		Version: 5,
		Caps: []p2p.Cap{
			{Name: "diff", Version: 1},
			{Name: "eth", Version: 64},
			{Name: "eth", Version: 65},
			{Name: "eth", Version: 66},
			{Name: "eth", Version: 67},
			{Name: "eth", Version: 68},
			{Name: "les", Version: 2},
			{Name: "les", Version: 3},
			{Name: "les", Version: 4},
			{Name: "les", Version: 5},
			{Name: "snap", Version: 1},
			{Name: "opera", Version: 62},
		},
		ID: pub0,
	}

	conn.ourHighestProtoVersion = 66

	return conn.Write(h)
}

func readHello(conn *Conn, info *ClientInfo) error {
	switch msg := conn.Read().(type) {
	case *Hello:
		// set snappy if version is at least 5
		if msg.Version >= 5 {
			conn.SetSnappy(true)
		}
		info.Capabilities = msg.Caps
		info.SoftwareVersion = msg.Version
		info.ClientType = msg.Name
		info.LastSeenTime = time.Now()
	case *Disconnect:
		return fmt.Errorf("bad hello handshake-1: %v", msg.Reason.Error())
	case *Error:
		return fmt.Errorf("bad hello handshake-2: %v", msg.Error())
	default:
		return fmt.Errorf("bad hello handshake-3: %v", msg.Code())
	}

	conn.negotiateEthProtocol(info.Capabilities)

	return nil
}

func getStatus(config *params.ChainConfig, version uint32, genesis common.Hash, network uint64) *Status {
	if _status == nil {
		_status = &Status{
			ProtocolVersion: version,
			NetworkID:       network,
			TD:              big.NewInt(0),
			Head:            genesis,
			Genesis:         genesis,
			ForkID:          forkid.NewID(config, genesis, 0),
		}
	}
	return _status
}

func readStatus(conn *Conn, info *ClientInfo) error {
	switch msg := conn.Read().(type) {
	case *Status:
		info.ForkID = msg.ForkID
		info.HeadHash = msg.Head
		info.NetworkID = msg.NetworkID
		// m.ProtocolVersion
		info.TotalDifficulty = msg.TD

		// Set correct TD if received TD is higher
		if msg.TD.Cmp(_status.TD) > 0 {
			_status.TD = msg.TD
		}
	case *Disconnect:
		return fmt.Errorf("bad status handshake-1: %v", msg.Reason.Error())
	case *Error:
		return fmt.Errorf("bad status handshake-2: %v", msg.Error())
	default:
		return fmt.Errorf("bad status handshake-3: %v", msg.Code())
	}
	return nil
}

func readHeaderInfo(conn *Conn, info *ClientInfo) error {
	switch msg := conn.Read().(type) {
	case *BlockHeaders:
		header := (*msg)[len(*msg)-1] //get latest header
		info.HeadHash = header.Hash()
	case *Disconnect:
		return fmt.Errorf("bad header info-1: %v", msg.Reason.Error())
	case *Error:
		return fmt.Errorf("bad header info-2: %v", msg.Error())
	default:
		return fmt.Errorf("bad header info-3: %v", msg.Code())
	}
	return nil
}
