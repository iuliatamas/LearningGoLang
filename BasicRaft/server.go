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
	"os"
	// "reflect"
	"fmt"
)


type state int
const (
	FOLLOWER state   = iota
	CANDIDATE state  = iota
	LEADER state     = iota
	SAME_STATE state = iota // neutral value, indicates maintaining state
	NO_STATE state   = iota // before looping on any state
)

func initRand() (seed int64)  {
	seed = int64(time.Now().Nanosecond())
	rand.Seed(seed)
	return
}

var (
	seed = initRand()
	randETTime time.Duration = time.Duration(150) + time.Duration(rand.Intn(150))
	TERM_TIME time.Duration = time.Duration(60) * time.Second
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

	leader string

	peerServers []PeerServer
	nPeerServers int 
	nConnected int

	stateChange chan state
	restartElectionTimeout chan int
	stopElectionTimeout chan int
	stopHeartbeat chan int
	heartbeatChan chan HeartbeatResponse
	respChan chan VoteResponse
	stopLoopChan []chan int
	doneLoopChan []chan int
	heartbeatDone chan int

	bigServerLock sync.Mutex 
	stateLock sync.Mutex
}
var GLOBAL_SERVER *Server

func (s *Server) termTicker() {
	termTicker := time.NewTicker(TERM_TIME)
	go func() {
		for _ = range termTicker.C {
			s.stateLock.Lock()
			fmt.Println("term tick")
			s.stateChange <- CANDIDATE
			s.stateLock.Unlock()
		}
	} ()
}

func (s *Server) electionTimeoutTicker() {
	ELECTION_TIMEOUT_TIME = (time.Duration(150) + time.Duration(rand.Intn(150))) * time.Millisecond
	fmt.Println("electionTimeoutTicker, ", ELECTION_TIMEOUT_TIME)
	etTicker := time.NewTicker(ELECTION_TIMEOUT_TIME*100)

	go func() {
		for _ = range etTicker.C {
			s.stateLock.Lock()
			fmt.Println("election tick")
			s.stateChange <- CANDIDATE
			s.stateLock.Unlock()
		}
	} ()

	for {
		select{
		case <- s.restartElectionTimeout:
			etTicker.Stop()
			s.electionTimeoutTicker()
		// case <- s.stopElectionTimeout:
			// fmt.Println("stop election timeout ticker")
			// etTicker.Stop()
		}
	}
	
}

func (s *Server) heartbeatTicker(){
	defer func() { s.heartbeatDone<-1 } ()
	fmt.Println("heartbeatTicker")
	s.heartbeatChan = make( chan HeartbeatResponse)
	hbTicker := time.NewTicker(HEARTBEAT_TIME)
	go func() {
		for _ = range hbTicker.C {
			for _, peerServer := range s.peerServers {
				s.stateLock.Lock()
				if s.state != LEADER {
					continue
				}
				s.stateLock.Unlock()
				s.bigServerLock.Lock()
				term := s.term
				s.bigServerLock.Unlock()
				go peerServer.client.Call("RpcFunctions.Heartbeat", newHeartbeat(s.name, term), &s.heartbeatChan)
			}
		}
	} ()
	<- s.stopHeartbeat
	fmt.Println("stopping heartbeatTicker")
	hbTicker.Stop()
	// s.heartbeatChan.Close()
}


func (s *Server) followerLoop(){
	s.restartElectionTimeout <- 1
	defer func() {s.doneLoopChan[int(FOLLOWER)] <- 1 } ()
	fmt.Println("	followerLoop for leader", s.leader)
	time.Sleep(time.Second * 2)
	for {
		select{
		case <-s.stopLoopChan[int(FOLLOWER)]:
			return
		}
	}
}

func (s *Server) candidateLoop(){
	defer func() { s.doneLoopChan[int(CANDIDATE)] <- 1 } ()
	s.bigServerLock.Lock()
	s.stateLock.Lock()
	s.term += 1
	s.votedFor = s.name
	s.votesReceived = 1
	s.leader = ""
	s.bigServerLock.Unlock()
	s.stateLock.Unlock()

	var wg sync.WaitGroup
	for _, peerServer := range s.peerServers {
		wg.Add(1)
		go func( peerServer PeerServer) {
			defer wg.Done()
			var voteResponse VoteResponse
			s.bigServerLock.Lock()
			term := s.term
			s.bigServerLock.Unlock()

			err := peerServer.client.Call("RpcFunctions.RequestVote", newVoteRequest(s.name, term), &voteResponse)
			if err != nil {
				fmt.Println(err, peerServer.addrName)
				os.Exit(1)
			}
			s.respChan <- voteResponse
		} (peerServer)
	}
	// wg.Wait()
	// s.bigServerLock.Unlock()
	// s.restartElectionTimeout <- 1

	for {
		if s.votesReceived >= s.getVoteMajority() {
			fmt.Println("LEADER")
			
			s.bigServerLock.Lock()
			s.leader = s.name
			s.bigServerLock.Unlock()

			s.stateLock.Lock()
			s.stateChange <- LEADER
			s.stateLock.Unlock()
			return
		}

		select {
		case resp := <-s.respChan:
			s.bigServerLock.Lock()
			term := s.term
			s.bigServerLock.Unlock()

			if is, stateChange := isVoteResponse(resp, term); is {
				fmt.Println("Received response from ", resp.Name)
				s.votesReceived += 1

			} else if stateChange != SAME_STATE {
				s.stateLock.Lock()
				s.stateChange <- stateChange
				s.stateLock.Unlock()
			}
		case <-s.stopLoopChan[int(CANDIDATE)]:
			return
		}
	}
	
}

func (s *Server) leaderLoop(){
	defer func(){ s.doneLoopChan[int(LEADER)] <- 1 } ()
	fmt.Println("leaderLoop")
	s.stopHeartbeat = make(chan int, 1)
	s.heartbeatDone = make(chan int, 1)
	go s.heartbeatTicker()
	for {
		select{
		case <-s.stopLoopChan[int(LEADER)]:
			s.stopHeartbeat <- 1
			<-s.heartbeatDone
			return
		}

	}
}

func (s *Server) loop() {
	fmt.Println("	loop ")

	for {
		select{
		case st :=  <-s.stateChange:
			s.stateLock.Lock()			
			fmt.Println("change state from", s.state, " to", st)
			if s.state != NO_STATE {
				s.stopLoopChan[int(s.state)] <- 1
				<-s.doneLoopChan[int(s.state)]
			}
			s.state = st
			s.stateLock.Unlock()			
			switch st {
			case FOLLOWER:
				go s.followerLoop()
			case CANDIDATE:
				go s.candidateLoop()
			case LEADER:
				go s.leaderLoop()
			}
		}
	}
}

// start connection with each of the peer servers
func (s *Server) contactServers( serversI []int) {
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

// waiting for peer servers to complete their connection to us
func (s *Server) waitFullContact(){
	for {
		s.bigServerLock.Lock()
		if s.nConnected == s.nPeerServers{
			fmt.Println("waitFullContact", s.nConnected, "connected of", s.nPeerServers)
			s.bigServerLock.Unlock()
			break
		} else {
			fmt.Println("waitFullContact", s.nConnected, "connected of", s.nPeerServers)
			s.bigServerLock.Unlock()
			time.Sleep( 3 * time.Second )
		}
	}
}

func NewServer() (*Server, error){
	s := &Server{}
	GLOBAL_SERVER = s
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
	s.state = NO_STATE
	s.term = 0
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

	s.restartElectionTimeout = make(chan int, 1)
	s.stopElectionTimeout = make(chan int, 2)
	s.respChan = make( chan VoteResponse, len(s.peerServers)*10)
	s.stateChange = make(chan state, 1)
	s.stopLoopChan = make([]chan int, 3)
	s.stopLoopChan[int(FOLLOWER)] = make(chan int, 1)
	s.stopLoopChan[int(CANDIDATE)] = make(chan int, 1)
	s.stopLoopChan[int(LEADER)] = make(chan int, 1)
	s.doneLoopChan = make([]chan int, 3)
	s.doneLoopChan[int(FOLLOWER)] = make(chan int, 1)
	s.doneLoopChan[int(CANDIDATE)] = make(chan int, 1)
	s.doneLoopChan[int(LEADER)] = make(chan int, 1)
	
	s.stateChange<-FOLLOWER
	go s.termTicker()
	go s.electionTimeoutTicker()

	s.loop()

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

func (s *Server) getVoteMajority() int{
	return s.nPeerServers/2+1
}
