// Basic Raft as described here: https://ramcloud.stanford.edu/~ongaro/thesis.pdf

package braft

import (
	"fmt"
	"errors"
)

type RpcFunctions struct {
}

func (rpcf *RpcFunctions) Heartbeat(hb *Heartbeat, c *chan HeartbeatResponse ) error{
	s := GLOBAL_SERVER
	s.bigServerLock.Lock()
	defer s.bigServerLock.Unlock()
	
	if is := isHeartbeat(hb, s.term); is {
		fmt.Println("Received good heartbeat from ",  hb.Name, hb.Term)
		s.leader = hb.Name
		s.stateLock.Lock()
		if s.state != FOLLOWER {
			s.stateChange <- FOLLOWER
		}
		s.stateLock.Unlock()
		s.term = hb.Term
		s.restartElectionTimeout <- 1
	
		// hr := HeartbeatResponse{Name: s.name, Term: s.term}
		// (*c) <- hr
		return nil
	}

	return errors.New("Denied outdated heartbeat")
}

func (rpcf *RpcFunctions) RequestVote(rv *VoteRequest, voteResponse *VoteResponse ) error{
	s := GLOBAL_SERVER
	s.bigServerLock.Lock()
	term := s.term
	votedFor := s.votedFor
	s.bigServerLock.Unlock()

	switch {
	case rv.Term < term:
		*voteResponse = VoteResponse{Name: s.name, Term: 0}
		return errors.New("Request outdated")
	case rv.Term == term:
		*voteResponse = VoteResponse{Name: s.name, Term: 0}
		if votedFor != "" {
			return errors.New("Already voted for this term")		
		}
	}

	s.bigServerLock.Lock()
	s.term = rv.Term		
	s.votedFor = rv.Name
	*voteResponse = VoteResponse{Name: s.name, Term: s.term}
	s.bigServerLock.Unlock()
	return nil
}

func (rpcf *RpcFunctions) Connect(addrName string, reply *int ) error{
	s := GLOBAL_SERVER
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