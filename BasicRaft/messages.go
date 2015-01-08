// Basic Raft as described here: https://ramcloud.stanford.edu/~ongaro/thesis.pdf

package braft

type VoteRequest struct {
	Name string
	Term int
}

type VoteResponse struct {
	Name string
	Term int
}

type Heartbeat struct {
	Name string
	Term int
}

type HeartbeatResponse struct {
	Name string
	Term int
}


func newVoteRequest( name string, term int) *VoteRequest{
	return &VoteRequest{ Name : name, Term : term }
}

func newHeartbeat( name string, term int) *Heartbeat{
	return &Heartbeat{ Name : name, Term : term }
}


func isVoteResponse( vr VoteResponse, term int ) (bool, state) {
	dif := vr.Term - term
	switch {
		case dif > 0:
			return false, FOLLOWER
		case dif == 0:
			return true, SAME_STATE
		case dif < 0:
			return false, SAME_STATE
	}
	return false, SAME_STATE
}

func isHeartbeat( hb *Heartbeat, term int ) bool {
	if dif := hb.Term - term; dif >= 0{
		return true
	}
	return false
}
