package crawler

import (
	"encoding/json"
	"fmt"
	"github.com/ethereum/go-ethereum/core/forkid"
	"testing"
)

func TestGetClientInfo(t *testing.T) {
	nodeUrl := "enode://901b084f050a1e08c46c670b56f41df00f4f697886b2b4cace59b5f36428e65bb76f16fb3cb44338ce8309df589467c7ad26ba32c9a31abc713b4e5d5698f599@86.48.19.175:30303"
	target, _ := parseNode(nodeUrl)
	fmt.Println(target.TCP())
	info, err := getClientInfo(makeGenesis(), 1, "", target)
	if err != nil {
		t.Fatal(err)
	}
	marshal, err := json.Marshal(info)
	if err != nil {
		t.Fatal(err)
	}
	s := info.TotalDifficulty.String()
	fmt.Println(s)
	fmt.Println("client info: ", string(marshal))

}

var myFilter = func(id forkid.ID) error {
	return nil
}

func TestPeerInfo(t *testing.T) {

}
