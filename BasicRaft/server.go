// Basic Raft as described here: https://ramcloud.stanford.edu/~ongaro/thesis.pdf

package braft

import (
	"io/ioutil"
	"net"
	"net/http"
	"net/rpc"
	"encoding/json"
	"errors"
	"strconv"
	"time"
	"math/rand"
	"sync"
	// "os"
	// "reflect"
	"fmt"
)


type state int
const (
	FOLLOWER state = iota
	CANDIDATE state = iota
	LEADER state = iota
)

var (
	randETTime time.Duration = time.Duration(150) + time.Duration(rand.Int() % 150)
	TERM_TIME time.Duration = time.Duration(100) * time.Second
	ELECTION_TIMEOUT_TIME time.Duration  = randETTime * time.Millisecond
	HEARTBEAT_TIME time.Duration = time.Duration(50) * time.Millisecond 
)

type ServerLocation struct {
	Addr string `json:"addr"`
	Port int `json:"port"`
}

type PeerServer struct {
	location ServerLocation
	client *rpc.Client
	addrName string
	connected bool 	// set by peer via rpc
	contacted bool  // set by us after dialHTTP
}

type Server struct{
	// Persistent state on all serves
	name string
	addrName string
	location ServerLocation
	state state
	term int
	votedFor string
	votesReceived int
	// log

	// rpc interface
	rpcFunctions *RpcFunctions

	peerServers []PeerServer
	nPeerServers int 
	nConnected int

	stateChange chan state
	restartElectionTimeout chan int
	stopHeartBeat chan int
	heartbeatChan chan HeartbeatResponse

	bigServerLock sync.Mutex 
}
var GLOBAL_SERVER *Server

type RpcFunctions struct {
}

type VoteRequest struct {
	name string
	term int
}

type VoteResponse struct {
	name string
	term int
}

type Heartbeat struct {
	name string
	term int
}

type HeartbeatResponse struct {
	name string
	term int
}

func (s *Server) newVoteRequest( name string, term int) *VoteRequest{
	return &VoteRequest{ name : name, term : term }
}

func (s *Server) newHeartbeat( name string, term int) *Heartbeat{
	return &Heartbeat{ name : name, term : term }
}

func (s *Server) termTicker() {
	fmt.Println("termTicker")
	termTicker := time.NewTicker(time.Millisecond * TERM_TIME)
	go func() {
		for _ = range termTicker.C {
			s.stateChange <- CANDIDATE
		}
	} ()
}

func (s *Server) electionTimeoutTicker() {
	fmt.Println("electionTimeoutTicker")
	etTicker := time.NewTicker(time.Millisecond * ELECTION_TIMEOUT_TIME)
	go func() {
		for _ = range etTicker.C {
			s.stateChange <- CANDIDATE
		}
	} ()
	// todo: here if a get a heatbeat received confirmation I restart the ticker
	<- s.restartElectionTimeout
	etTicker.Stop()
	s.electionTimeoutTicker()
}

func (s *Server) heartbeatTicker(){
	fmt.Println("heartbeatTicker")
	s.heartbeatChan = make( chan HeartbeatResponse)
	hbTicker := time.NewTicker(time.Millisecond * HEARTBEAT_TIME)
	go func() {
		for _ = range hbTicker.C {
			for _, peerServer := range s.peerServers {
				peerServer.client.Call("Heartbeat", s.newHeartbeat(s.name, s.term), &s.heartbeatChan)
			}
		}
	} ()
	// todo: here if a get a heatbeat received confirmation I restart the ticker
	<- s.stopHeartBeat
	hbTicker.Stop()
	// s.heartbeatChan.Close()
}


func (s *Server) followerLoop(){
	fmt.Println("	followerLoop")
	for {

	}
}


func (s *Server) candidateLoop(){
	fmt.Println("candidateLoop")
	// todo: clean candidate fields
	s.term += 1
	s.votedFor = s.name
	s.votesReceived = 1

	respChan := make( chan VoteResponse, len(s.peerServers))
	var wg sync.WaitGroup
	for _, peerServer := range s.peerServers {
		wg.Add(1)
		go func( peerServer PeerServer) {
			defer wg.Done()
			peerServer.client.Call("RequestVote", s.newVoteRequest(s.name, s.term), respChan)
		} (peerServer)
	}
	s.restartElectionTimeout <- 1

	for {
		if s.votesReceived >= s.getVoteMajority() {
			s.stateChange <- LEADER
			return
		}

		select {
		case resp := <-respChan:
			if s.isVoteResponse(resp){
				fmt.Println("Received response from ", resp.name)
				s.votesReceived += 1
			}
		}
		
	}
	
}

func (s *Server) leaderLoop(){
	fmt.Println("leaderLoop")
	s.heartbeatTicker()
	for {

	}
}

func (s *Server) loop() {
	fmt.Println("	loop ")

	st :=  <-s.stateChange
	fmt.Println("	loop with state =", st)
	for {
		s.state = st
		switch st {
		case FOLLOWER:
			s.followerLoop()
		case CANDIDATE:
			s.candidateLoop()
		case LEADER:
			s.leaderLoop()
		}
	}
}

// start connection with each of the peer servers
func (s *Server) contactServers( serversI []int) {
	fmt.Println("contactServers")
	nServers := len(serversI)
	nextServersI := make([]int, 0)
	for idx:=0; idx<nServers; idx++ {
		i := serversI[idx]

		var client *rpc.Client
		var err error
		if s.peerServers[i].contacted == false {
			client, err = rpc.DialHTTP("tcp", s.peerServers[i].addrName)
		} else {
			client = s.peerServers[i].client
			err = nil
		}

		if err != nil {
			fmt.Println("ERROR: contactServers:", s.peerServers[i].addrName)
			nextServersI = append(nextServersI, i)
		} else{
			fmt.Println("SUCCESS: contactServers:", s.peerServers[i].addrName)
			s.peerServers[i].contacted = true

			var reply int
			client.Call("RpcFunctions.Connect", s.addrName, &reply)
			if reply != 0 {
				fmt.Println("ERROR: connecting,", s.peerServers[i].addrName)
				nextServersI = append(nextServersI, i)
			} else {
				fmt.Println("SUCCESS: connecting,", s.peerServers[i].addrName)
				s.peerServers[i].client = client
			}
		}
	}

	if( len(nextServersI) != 0 ){
		time.Sleep(5 * time.Second )
		s.contactServers(nextServersI)
	}
}
func (s *Server) waitFullContact(){
	for {
		s.bigServerLock.Lock()
		if s.nConnected == s.nPeerServers{
			fmt.Println("waitFullContact", s.nConnected, s.nPeerServers)
			s.bigServerLock.Unlock()
			break
		} else {
			fmt.Println("waitFullContact", s.nConnected, s.nPeerServers)
			s.bigServerLock.Unlock()
			time.Sleep( 3 * time.Second )
		}
	}
}

func NewServer() (*Server, error){
	s := &Server{}
	config, err := s.readConfigFile()
	if config == nil {
		return s, err
	}

	addr, port, err := s.serve(config)
	if err != nil {
		return nil, err
	}
	s.name = "Server " + strconv.Itoa(port)
	s.location = ServerLocation{addr, port}
	s.addrName = addr + ":" + strconv.Itoa(port)
	s.term = 0
	GLOBAL_SERVER = s
	fmt.Println(s.name, "started")

	s.nPeerServers = len(config.Servers)-1
	s.peerServers = make( []PeerServer, 0 )
	for _, peerServer := range config.Servers {
		if peerServer.Addr != s.location.Addr || 
		   peerServer.Port != s.location.Port {
				s.peerServers = append(s.peerServers, 
										PeerServer{location:peerServer, 
											addrName:peerServer.Addr+":"+strconv.Itoa(peerServer.Port)})
		}
	}
	
	serversI := make([]int, s.nPeerServers)
	for i:=0; i<s.nPeerServers; i++ {
		serversI[i] = i
	}
	s.contactServers(serversI)
	s.waitFullContact()

	go s.termTicker()
	go s.electionTimeoutTicker()

	s.stateChange = make(chan state, 1)
	s.stateChange<-FOLLOWER
	s.loop()

	// fmt.Println("Server", s)
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

	s.rpcFunctions = new(RpcFunctions)
	rpc.Register(s.rpcFunctions)
	rpc.HandleHTTP()
	go http.Serve(l, nil)
	return config.Addr, p, nil
}

func (rpcf *RpcFunctions) Heartbeat(hb *Heartbeat, c *chan HeartbeatResponse ) error{
	fmt.Println("Received heartbeat from", hb.name)
	hr := HeartbeatResponse{}
	(*c) <- hr
	return nil
}

func (rpcf *RpcFunctions) RequestVote(rv *VoteRequest, c *chan VoteResponse ) error{
	fmt.Println("Received requestVote from", rv.name)
	(*c) <- VoteResponse{}
	return nil
	
}

func (rpcf *RpcFunctions) Connect(addrName string, reply *int ) error{
	s := GLOBAL_SERVER
	fmt.Println(" !!!!!!!!! Connect from", addrName)
	s.bigServerLock.Lock()
	defer s.bigServerLock.Unlock()

	for i:=0; i<s.nPeerServers; i++{
		if s.peerServers[i].addrName == addrName {
			if s.peerServers[i].connected == true {
				*reply = 1
				return errors.New("already connected to " + s.peerServers[i].addrName)
			} else{
				s.peerServers[i].connected = true
				s.nConnected ++
				*reply = 0
				return nil
			}
		}
	}
	return nil
	
}

func (s *Server) getVoteMajority() int{
	return s.nPeerServers/2+1
}

func (s *Server) isVoteResponse( vr VoteResponse ) bool {
	dif := vr.term - s.term
	switch {
		case dif > 0:
			s.stateChange <- FOLLOWER
			return false
		case dif == 0:
			return true
		case dif < 0:
			return false
	}
	return false
}
