package braft

import (
	"testing"
	// "fmt"
)

func TestNewServer( t *testing.T ){
	// nServers := 3
	// s := make([]*Server, 3, 3)
	// var e error
	// for i:=0; i<nServers; i++ {
	// 	fmt.Println("Server #", i)
	// 	s[i], e = NewServer()
	// 	if e != nil {
	// 		t.Error("err: ", e)
	// 	}
	// }

	_, e := NewServer()
	if e != nil {
		t.Error("err: ", e)
	}
}