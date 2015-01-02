// Basic Raft as described here: https://ramcloud.stanford.edu/~ongaro/thesis.pdf

package braft

import (
	"io/ioutil"
	"fmt"
	"net"
	"net/http"
	"net/rpc"
	"encoding/json"
	"errors"
	"strconv"
	// "reflect"
)

type state int
const (
	FOLLOWER state = iota
	CANDIDATE state = iota
	LEADER state = iota
)

type ServerLocation struct {
	Addr string `json:"addr"`
	Port int `json:"port"`
}

type Server struct{
	// Persistent state on all serves
	name string
	location ServerLocation
	state state
	currentTerm int
	votedFor int
	// log

}

func NewServer() (*Server, error){
	s := &Server{}
	config, err := s.readConfigFile()
	if config == nil {
		return s, err
	}
	fmt.Println("NewServer: config:", config)

	addr, port, err := s.serve(config)
	if err != nil {
		return nil, err
	}
	s.name = "Server " + strconv.Itoa(port)
	s.location = ServerLocation{addr, port}
	s.state = FOLLOWER

	fmt.Println(s.name, "started")
	fmt.Println("Server", s)
	return s, nil
}

func (s *Server) readConfigFile() (*Config, error) {
	configFile, err := ioutil.ReadFile("BasicRaft/config.json")
	if err != nil {
		return nil, err
	}

	config := &Config{}
	if err = json.Unmarshal(configFile, config); err != nil {
		return nil, err
	}

	return config, nil
}

func (s *Server) serve(config *Config) (string, int, error) {
	var l net.Listener
	p:=config.StartPort
	for ; p<config.StartPort + config.MaxPorts; p++ {
		var e error
		l, e = net.Listen("tcp", config.Addr + ":" + strconv.Itoa(p))
		if( e == nil ){
			break
		} 
	}
	if p == config.StartPort + config.MaxPorts {
		return "", 0, errors.New("No available ports, check config file")
	}

	rpc.HandleHTTP()
	go http.Serve(l, nil)
	return config.Addr, p, nil
}


