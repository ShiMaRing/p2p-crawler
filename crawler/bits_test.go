package crawler

import (
	"fmt"
	"testing"
)

//Bit operation test method set

func TestPrint(t *testing.T) {
	var bits = [10]byte{}
	for i := 0; i < 10; i++ {
		bits[i] = byte(i)
	}
	fmt.Println(bits)

}
