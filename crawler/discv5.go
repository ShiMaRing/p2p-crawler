package crawler

import (
	"context"
	"crypto/ecdsa"
	"errors"
	gethlog "github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/p2p/discover"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"net"
)

type Version int

const (
	Discv4 Version = iota
	Discv5
)

type Resolver interface {
	RequestENR(*enode.Node) (*enode.Node, error)
	RandomNodes() enode.Iterator
}

type DiscvService struct {
	Resolver
	ctx        context.Context
	ethNode    *enode.LocalNode
	iterator   enode.Iterator
	enrHandler func(*enode.Node)
}

func NewvDiscService(
	ctx context.Context,
	port int,
	privkey *ecdsa.PrivateKey,
	ethNode *enode.LocalNode,
	bootnodes []*enode.Node,
	enrHandler func(*enode.Node), version Version) (Resolver, error) {
	if len(bootnodes) == 0 {
		return nil, errors.New("unable to start peer discovery, no bootnodes provided")
	}
	// udp address to listen
	udpAddr := &net.UDPAddr{
		IP:   net.IPv4zero,
		Port: port,
	}
	// start listening and create a connection object
	conn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		return nil, err
	}
	// Set custom logger for the discovery service (Debug)
	gethLogger := gethlog.New()
	gethLogger.SetHandler(gethlog.FuncHandler(func(r *gethlog.Record) error {
		return nil
	}))

	var resolver Resolver
	// configuration of the discovery5
	cfg := discover.Config{
		PrivateKey:   privkey,
		NetRestrict:  nil,
		Bootnodes:    bootnodes,
		Unhandled:    nil, // Not used in dv5
		Log:          gethLogger,
		ValidSchemes: enode.ValidSchemes,
	}

	switch version {
	case Discv4:
		resolver, err = discover.ListenV4(conn, ethNode, cfg)
		if err != nil {
			return nil, err
		}
	case Discv5:
		resolver, err = discover.ListenV5(conn, ethNode, cfg)
		if err != nil {
			return nil, err
		}
	}

	iterator := resolver.RandomNodes()

	return &DiscvService{
		Resolver:   resolver,
		ctx:        ctx,
		ethNode:    ethNode,
		iterator:   iterator,
		enrHandler: enrHandler,
	}, nil
}

func (dv *DiscvService) Run() {
	for {
		// check if the context is still up
		if err := dv.ctx.Err(); err != nil {
			break
		}
		if dv.iterator.Next() {
			newNode := dv.iterator.Node()
			dv.enrHandler(newNode)
		}

	}
}
