// source for practice and inspiration: http://golang.org/pkg/net/rpc/
package rpcserver

import (
	"errors"
	"time"
)

type Arith int

type Args struct{
	A1,A2 int
}

type Quotient struct {
	Quot, Rem int
} 


func (t *Arith) Multiply( args *Args, reply *int) error {
	time.Sleep(2000000000)
	*reply = args.A1 * args.A2
	return nil
}

func (t *Arith) Divide( args *Args, reply *Quotient) error {
	time.Sleep(2000000000)
	if( args.A2 == 0 ){
		return errors.New("cannot divide by 0")
	}
	reply.Quot = args.A1 / args.A2
	reply.Rem = args.A1 % args.A2
	return nil
}