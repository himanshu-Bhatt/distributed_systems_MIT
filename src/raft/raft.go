package raft

//
// this is an outline of the API that raft must expose to
// the service (or tester). see comments below for
// each of these functions for more details.
//
// rf = Make(...)
//   create a new Raft server.
// rf.Start(command interface{}) (index, term, isleader)
//   start agreement on a new log entry
// rf.GetState() (term, isLeader)
//   ask a Raft for its current term, and whether it thinks it is leader
// ApplyMsg
//   each time a new entry is committed to the log, each Raft peer
//   should send an ApplyMsg to the service (or tester)
//   in the same server.
//

import (
	//	"bytes"

	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	// "6.5840/labgob"
	"6.5840/labrpc"
)

// as each Raft peer becomes aware that successive log entries are
// committed, the peer should send an ApplyMsg to the service (or
// tester) on the same server, via the applyCh passed to Make(). set
// CommandValid to true to indicate that the ApplyMsg contains a newly
// committed log entry.
//
// in part 2D you'll want to send other kinds of messages (e.g.,
// snapshots) on the applyCh, but set CommandValid to false for these
// other uses.
type ApplyMsg struct {
	CommandValid bool
	Command      interface{}
	CommandIndex int

	// For 2D:
	SnapshotValid bool
	Snapshot      []byte
	SnapshotTerm  int
	SnapshotIndex int
}

type State string

const (
	Leader    = "Leader"
	Candidate = "Candidate"
	Follower  = "Follower"
)

type LogEntry struct {
	Term    int
	Command interface{}
}

// A Go object implementing a single Raft peer.
type Raft struct {
	mu        sync.Mutex          // Lock to protect shared access to this peer's state
	peers     []*labrpc.ClientEnd // RPC end points of all peers
	persister *Persister          // Object to hold this peer's persisted state
	me        int                 // this peer's index into peers[]
	dead      int32               // set by Kill()

	// Your data here (2A, 2B, 2C).
	// Look at the paper's Figure 2 for a description of what
	// state a Raft server must maintain.

	// persistent state on all servers
	currentTerm int
	votedFor    int
	logs        []LogEntry

	// volatile state on all servers
	commitIndex int
	lastApplied int

	// volatile state on leaders
	nextIndex  []int
	matchIndex []int

	// other auxiliary states
	state       State
	voteCount   int
	applyCh     chan ApplyMsg
	winElectCh  chan bool
	stepDownCh  chan bool
	grantVoteCh chan bool
	heartbeatCh chan bool
}

// return currentTerm and whether this server
// believes it is the leader.
func (rf *Raft) GetState() (int, bool) {
	// Your code here (2A).
	rf.mu.Lock()
	defer rf.mu.Unlock()

	return rf.currentTerm, rf.state == Leader
}

// get the randomized election timeout.
func (rf *Raft) getElectionTimeout() time.Duration {
	// return time.Duration(200 + rand.Intn(150))
	return time.Duration(360 + rand.Intn(240))
	// return time.Duration(50 + rand.Intn(300))
}

// save Raft's persistent state to stable storage,
// where it can later be retrieved after a crash and restart.
// see paper's Figure 2 for a description of what should be persistent.
// before you've implemented snapshots, you should pass nil as the
// second argument to persister.Save().
// after you've implemented snapshots, pass the current snapshot
// (or nil if there's not yet a snapshot).
func (rf *Raft) persist() {
	// Your code here (2C).
	// Example:
	// w := new(bytes.Buffer)
	// e := labgob.NewEncoder(w)
	// e.Encode(rf.xxx)
	// e.Encode(rf.yyy)
	// raftstate := w.Bytes()
	// rf.persister.Save(raftstate, nil)
}

// restore previously persisted state.
func (rf *Raft) readPersist(data []byte) {
	if data == nil || len(data) < 1 { // bootstrap without any state?
		return
	}
	// Your code here (2C).
	// Example:
	// r := bytes.NewBuffer(data)
	// d := labgob.NewDecoder(r)
	// var xxx
	// var yyy
	// if d.Decode(&xxx) != nil ||
	//    d.Decode(&yyy) != nil {
	//   error...
	// } else {
	//   rf.xxx = xxx
	//   rf.yyy = yyy
	// }
}

// the service says it has created a snapshot that has
// all info up to and including index. this means the
// service no longer needs the log through (and including)
// that index. Raft should now trim its log as much as possible.
func (rf *Raft) Snapshot(index int, snapshot []byte) {
	// Your code here (2D).

}

// example RequestVote RPC arguments structure.
// field names must start with capital letters!
type RequestVoteArgs struct {
	// Your data here (2A, 2B).
	Term         int
	CandidateId  int
	LastLogIndex int
	LastLogTerm  int
}

// example RequestVote RPC reply structure.
// field names must start with capital letters!
type RequestVoteReply struct {
	// Your data here (2A).
	Term        int
	VoteGranted bool
}

func (rf *Raft) sendToChannel(ch chan bool, val bool) {
	select {
	case ch <- val:
	default:
	}
}

func (rf *Raft) stepDownToFollower(term int) {
	state := rf.state
	rf.votedFor = -1
	rf.state = Follower
	rf.currentTerm = term

	if state != Follower {
		rf.sendToChannel(rf.stepDownCh, true)
	}

}

// example RequestVote RPC handler.
func (rf *Raft) RequestVote(args *RequestVoteArgs, reply *RequestVoteReply) {
	// Your code here (2A, 2B).
	rf.mu.Lock()
	defer rf.mu.Unlock()

	if args.Term < rf.currentTerm {
		reply.Term = rf.currentTerm
		reply.VoteGranted = false
		return
	}
	if args.Term > rf.currentTerm {
		rf.stepDownToFollower(args.Term)
	}
	reply.Term = rf.currentTerm
	reply.VoteGranted = false
	if (rf.votedFor < 0 || rf.votedFor == args.CandidateId) &&
		args.LastLogTerm >= rf.logs[len(rf.logs)-1].Term {
		reply.VoteGranted = true
		rf.votedFor = args.CandidateId
		rf.sendToChannel(rf.grantVoteCh, true)
	}
}

type AppendEntriesArgs struct {
	Term         int
	LeaderId     int
	PrevLogIndex int
	PrevLogTerm  int
	Entries      []LogEntry
	LeaderCommit int
}
type AppendEntriesReply struct {
	Term    int
	Success bool
}

func (rf *Raft) AppendEntries(args *AppendEntriesArgs, reply *AppendEntriesReply) {
	rf.mu.Lock()
	defer rf.mu.Unlock()
	if rf.currentTerm > args.Term {
		reply.Success = false
		reply.Term = rf.currentTerm
		return
	} 
	if rf.currentTerm < args.Term {
		rf.stepDownToFollower(args.Term)
	}
	if len(rf.logs) < args.PrevLogIndex+1 {
		reply.Success = false
		reply.Term = args.Term
		return
	}
	// cTerm := rf.logs[args.PrevLogIndex].Term
	// conflictIndex := args.PrevLogIndex
	// if cTerm != args.PrevLogTerm {
	// 	for i := args.PrevLogIndex - 1; i >= 0 && rf.logs[i].Term == rf.logs[args.PrevLogIndex].Term; i-- {
	// 		conflictIndex = i
	// 	}
	// 	return
	// }

	// rf.logs=rf.logs[:conflictIndex]
	i := args.PrevLogIndex + 1
	j := 0
	for ; i < len(rf.logs) && j < len(args.Entries); i, j = i+1, j+1 {
		if rf.logs[i].Term != args.Entries[j].Term {
			break
		}
	}
	// if len(args.Entries) > 0 {
	// 	newIndex := args.PrevLogIndex + 1
	// 	rf.logs = rf.logs[:newIndex]
	// 	rf.logs = append(rf.logs, args.Entries...)
	// }
	rf.logs = rf.logs[:i]
	args.Entries = args.Entries[j:]
	rf.logs = append(rf.logs, args.Entries...)

	rf.sendToChannel(rf.heartbeatCh, true)

}

// example code to send a RequestVote RPC to a server.
// server is the index of the target server in rf.peers[].
// expects RPC arguments in args.
// fills in *reply with RPC reply, so caller should
// pass &reply.
// the types of the args and reply passed to Call() must be
// the same as the types of the arguments declared in the
// handler function (including whether they are pointers).
//
// The labrpc package simulates a lossy network, in which servers
// may be unreachable, and in which requests and replies may be lost.
// Call() sends a request and waits for a reply. If a reply arrives
// within a timeout interval, Call() returns true; otherwise
// Call() returns false. Thus Call() may not return for a while.
// A false return can be caused by a dead server, a live server that
// can't be reached, a lost request, or a lost reply.
//
// Call() is guaranteed to return (perhaps after a delay) *except* if the
// handler function on the server side does not return.  Thus there
// is no need to implement your own timeouts around Call().
//
// look at the comments in ../labrpc/labrpc.go for more details.
//
// if you're having trouble getting RPC to work, check that you've
// capitalized all field names in structs passed over RPC, and
// that the caller passes the address of the reply struct with &, not
// the struct itself.
func (rf *Raft) sendRequestVote(server int, args *RequestVoteArgs, reply *RequestVoteReply) {
	ok := rf.peers[server].Call("Raft.RequestVote", args, reply)
	if !ok {
		return
	}
	rf.mu.Lock()
	defer rf.mu.Unlock()
	if rf.state != Candidate || args.Term != rf.currentTerm || reply.Term < rf.currentTerm {
		return
	}
	if rf.currentTerm < reply.Term {
		rf.stepDownToFollower(reply.Term)
		return
	}
	// DPrintf("%d called sendRequestVote to %d", rf.me, server)
	if reply.VoteGranted {
		rf.voteCount++
		if rf.voteCount == len(rf.peers)/2+1 {
			// DPrintf("majority achieved by %d", rf.me)
			rf.sendToChannel(rf.winElectCh, true)
		}
	}

	// return ok
}
func (rf *Raft) sendAppendEntries(server int, args *AppendEntriesArgs, reply *AppendEntriesReply) bool {
	ok := rf.peers[server].Call("Raft.AppendEntries", args, reply)
	return ok
}

// the service using Raft (e.g. a k/v server) wants to start
// agreement on the next command to be appended to Raft's log. if this
// server isn't the leader, returns false. otherwise start the
// agreement and return immediately. there is no guarantee that this
// command will ever be committed to the Raft log, since the leader
// may fail or lose an election. even if the Raft instance has been killed,
// this function should return gracefully.
//
// the first return value is the index that the command will appear at
// if it's ever committed. the second return value is the current
// term. the third return value is true if this server believes it is
// the leader.
func (rf *Raft) Start(command interface{}) (int, int, bool) {
	index := -1
	term := -1
	isLeader := rf.state == Leader

	// Your code here (2B).
	rf.mu.Lock()
	defer rf.mu.Unlock()
	if isLeader {
		return index, term, false
	}
	rf.logs = append(rf.logs, LogEntry{rf.currentTerm, command})

	return rf.getLastIndex(), term, isLeader
}

// the tester doesn't halt goroutines created by Raft after each test,
// but it does call the Kill() method. your code can use killed() to
// check whether Kill() has been called. the use of atomic avoids the
// need for a lock.
//
// the issue is that long-running goroutines use memory and may chew
// up CPU time, perhaps causing later tests to fail and generating
// confusing debug output. any goroutine with a long-running loop
// should call killed() to check whether it should stop.
func (rf *Raft) Kill() {
	atomic.StoreInt32(&rf.dead, 1)
	// Your code here, if desired.
}

func (rf *Raft) killed() bool {
	z := atomic.LoadInt32(&rf.dead)
	return z == 1
}

func (rf *Raft) getLastIndex() int {
	return len(rf.logs) - 1
}
func (rf *Raft) getLastTerm() int {
	return rf.logs[rf.getLastIndex()].Term
}

func (rf *Raft) resetChannels() {
	rf.grantVoteCh = make(chan bool)
	rf.winElectCh = make(chan bool)
	rf.stepDownCh = make(chan bool)
	rf.heartbeatCh = make(chan bool)
}

func (rf *Raft) convertToCandidate(fromState State) {
	rf.mu.Lock()
	defer rf.mu.Unlock()

	if rf.state != fromState {
		return
	}

	rf.resetChannels()
	rf.votedFor = rf.me
	rf.voteCount = 1
	rf.currentTerm++
	rf.state = Candidate
	// rf.persist()
	// DPrintf("%d becomes candidate", rf.me)
	// DPrintf("%d started election", rf.me)

	rf.broadcastRequestVote()

}
func (rf *Raft) convertToLeader() {
	rf.mu.Lock()
	defer rf.mu.Unlock()
	if rf.state != Candidate {
		return
	}
	rf.resetChannels()
	rf.state = Leader
	// DPrintf("%d is leader", rf.me)
	rf.broadcastAppendEntries()

}
func (rf *Raft) broadcastAppendEntries() {
	if rf.state != Leader {
		return
	}
	en := []LogEntry{}
	for server := range rf.peers {
		if server != rf.me {
			args := AppendEntriesArgs{}
			args.Term = rf.currentTerm
			args.LeaderId = rf.me
			// args.PrevLogIndex = rf.nextIndex[server] - 1
			args.PrevLogIndex = len(rf.logs) - 1
			args.PrevLogTerm = rf.logs[args.PrevLogIndex].Term
			args.LeaderCommit = rf.commitIndex
			// entries := rf.logs[rf.nextIndex[server]:]
			entries := en
			args.Entries = make([]LogEntry, len(entries))
			// make a deep copy of the entries to send
			// copy(args.Entries, entries)
			go rf.sendAppendEntries(server, &args, &AppendEntriesReply{})
		}
	}
}
func (rf *Raft) broadcastRequestVote() {
	if rf.state != Candidate {
		return
	}
	args := RequestVoteArgs{
		Term:         rf.currentTerm,
		CandidateId:  rf.me,
		LastLogIndex: rf.getLastIndex(),
		LastLogTerm:  rf.getLastTerm(),
	}
	// DPrintf("%d broadcasted request vote", rf.me)
	for server := range rf.peers {
		if server != rf.me {
			go rf.sendRequestVote(server, &args, &RequestVoteReply{})
		}
	}
}

func (rf *Raft) ticker() {
	for rf.killed() == false {

		// Your code here (2A)
		// Check if a leader election should be started.

		rf.mu.Lock()
		state := rf.state
		rf.mu.Unlock()

		switch state {
		case Leader:
			select {
			case <-rf.stepDownCh:
			case <-time.After(120 * time.Millisecond):
				rf.mu.Lock()
				rf.broadcastAppendEntries()
				rf.mu.Unlock()
			}
		case Follower:
			select {
			case <-rf.grantVoteCh:
			case <-rf.heartbeatCh:
			case <-time.After(rf.getElectionTimeout() * time.Millisecond):
				rf.convertToCandidate(Follower)
			}
		case Candidate:
			select {
			case <-rf.stepDownCh:
				// state should already be follower
			case <-rf.winElectCh:
				rf.convertToLeader()
			case <-time.After(rf.getElectionTimeout() * time.Millisecond):
				rf.convertToCandidate(Candidate)
			}
		}

		// pause for a random amount of time between 50 and 350
		// milliseconds.
		// ms := 50 + (rand.Int63() % 300)
		// time.Sleep(time.Duration(ms) * time.Millisecond)
	}
}

// the service or tester wants to create a Raft server. the ports
// of all the Raft servers (including this one) are in peers[]. this
// server's port is peers[me]. all the servers' peers[] arrays
// have the same order. persister is a place for this server to
// save its persistent state, and also initially holds the most
// recent saved state, if any. applyCh is a channel on which the
// tester or service expects Raft to send ApplyMsg messages.
// Make() must return quickly, so it should start goroutines
// for any long-running work.
func Make(peers []*labrpc.ClientEnd, me int,
	persister *Persister, applyCh chan ApplyMsg) *Raft {
	rf := &Raft{}
	rf.peers = peers
	rf.persister = persister
	rf.me = me

	// Your initialization code here (2A, 2B, 2C).
	rf.state = Follower
	rf.currentTerm = 0
	rf.votedFor = -1
	rf.voteCount = 0
	rf.commitIndex = 0
	rf.lastApplied = 0
	rf.applyCh = applyCh
	rf.winElectCh = make(chan bool)
	rf.stepDownCh = make(chan bool)
	rf.grantVoteCh = make(chan bool)
	rf.heartbeatCh = make(chan bool)
	rf.logs = append(rf.logs, LogEntry{Term: 0})

	// initialize from state persisted before a crash
	rf.readPersist(persister.ReadRaftState())

	// start ticker goroutine to start elections
	go rf.ticker()

	return rf
}
