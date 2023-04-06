package crawler

import (
	"bufio"
	"encoding/json"
	"fmt"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"os"
	"testing"
)

//enr:-Iu4QNxBzixRZQmhdBq3wfVwh4PsJxfJ7uIyMnFBekszct6New2OmxRFYklXdm8SQ00ihBoWsx_-vxJ8NEculHf0LNEBgmlkgnY0gmlwhJ_E6LuJc2VjcDI1NmsxoQJH3HAzcSWwTYFkfbdQVlY6g39VrwFZ9myvmUvdq-A0GYN0Y3CCdmCDdWRwgnZg
func TestGetClientInfo(t *testing.T) {
	f, err2 := os.Open("../record")
	if err2 != nil {
		panic(err2)
	}
	reader := bufio.NewReader(f)
	for i := 0; i < 100; i++ {
		bytes, err2 := reader.ReadBytes('\n')
		if err2 != nil {
			panic(err2)
		}
		node := getNode(string(bytes))
		info, err := getClientInfo(makeGenesis(), 1, node)
		if err != nil {
			t.Logf("err: %v", err)
			continue
		}
		marshal, err := json.Marshal(info)
		if err != nil {
			continue
		}
		fmt.Println(string(marshal))
	}
}

func getNode(s string) *enode.Node {
	node := new(enode.Node)
	err := node.UnmarshalText([]byte(s))
	if err != nil {
		panic(err)
	}
	return node
}

func TestPeerInfo(t *testing.T) {

}
