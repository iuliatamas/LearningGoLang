// source for practice and inspiration: http://golang.org/pkg/net/rpc/
package rpcserver

import (
	"testing"
	"github.com/iuliatamas/BasicRaft/Practice/Impl"
	"net"
	"net/rpc"
	"log"
	"net/http"
	"fmt"
	"time"
	// "reflect"
)

func TestMain(t *testing.T) { 
	arith := new(rpcserver.Arith)
	rpc.Register(arith)
	rpc.HandleHTTP()
	l,e := net.Listen("tcp", "127.0.0.100:1234")
	if( e != nil ){
		log.Fatal("listen error:", e)
	}
	
	go http.Serve(l, nil)

	// 	os.Exit(m.Run()) 
}

func TestSync(t *testing.T) {
	client, err := rpc.DialHTTP("tcp", "127.0.0.100:1234")
	if err != nil {
		t.Error("dialing:", err)
	}
	defer client.Close()

	args := &rpcserver.Args{7,8}
	var reply int
	err = client.Call("Arith.Multiply", args, &reply)
	fmt.Println("After call")
	if( err != nil ){
		t.Error("call:", err)
	} 
	if reply != 7*8 {
		t.Error("For 7*8 expected 56, got ", reply)
	}

	time.Sleep(2000000000)


}

func TestAsync(t *testing.T) {
	client, err := rpc.DialHTTP("tcp", "127.0.0.100:1234")
	if err != nil {
		t.Error("dialing:", err)
	}
	defer client.Close()

	args := &rpcserver.Args{11,5}
	quotient := new(rpcserver.Quotient)
	divCall := client.Go("Arith.Divide", args, quotient, nil)
	fmt.Println("After client go")
	<- divCall.Done

	if quotient.Quot != 2 || quotient.Rem != 1 {
		t.Error("For 11/5 expected 2, 1 got ", quotient.Quot, quotient.Rem )
	}	
}



