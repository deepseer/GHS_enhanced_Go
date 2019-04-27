package main

import "time"

type nodeStatusType int
type messageBodyType int
type edgeStatusType int

const (
	sleeping nodeStatusType = iota
	find
	found
	illegal
)

var nodeStatusTypeString = []string{"sleeping", "find", "found", "illegal"}

const (
	initiate messageBodyType = iota
	test
	accept
	reject
	report
	changecore
	connect
	wakeup
)

var messageBodyTypeString = []string{"initiate", "test", "accept", "reject", "report", "changecore", "connect", "wakeup"}

const (
	basic edgeStatusType = iota
	branch
	rejected
	loop
)

var edgeStatusTypeString = []string{"basic", "branch", "rejected", "loop"}

type edgeType struct {
	NodeU     int
	NodeV     int
	Weight    int
	MSTStatus edgeStatusType
	Activated bool
	// Latency int
	// PDV int
}

// INFINITY weight for nodes not connected
const INFINITY int = 9999

// INFINITYEDGE for sentinel edges
var INFINITYEDGE edgeType

type fragmentType struct {
	CoreEdge          edgeType
	Level             int
	CoreDirectionPort int
}

type nodeType struct {
	// Unique Node ID
	ID int
	// Node only knows the weight for each edge incident to that node
	// This slice's index ranges from 1 to PortCount. Port[0] is the sentry (local loop) to simplify some process
	Port      []edgeType
	PortCount int
	// Node's current status
	Status nodeStatusType
	// The fragment which contains this node
	Fragment fragmentType
	// For non-core nodes to help core find the node that reported MWOE
	MWOEDirectionPort int // a.k.a. "in-brach"
	MWOE              edgeType
	// Counter of {Initiate} messages (status=find) sent from this node. Decreases when receiving {report}.
	FindCount int
	// Most recent {test} edge (not replied): used in report
	TestEdge int
	// Message receiver buffer channel
	IncomingMessage chan messageType
	// Pending test request: Will reply {accept} or {reject} after this node's level increases.
	PendingTestMessage     []messageType
	PendingTestMessageSize int
	// Pending connect request: Will process by sending {connect} after this node actively connects the other node.
	// The other node will receive a {connect} from an edge that has already been marked as branch, and will start an {initiate} message.
	PendingConnectMessage     []messageType
	PendingConnectMessageSize int
	// Pending report message: Only happens if one core node has not collected the MWOE info from its branch,
	//     while the other node has.
	// No more than ONE message.
	HasPendingReportMessage bool
	PendingReportMessage    messageType

	// To send a message via centralized demostration manager || Not exist in real systems
	ParentGraph *graphType
	// To terminate the demostration algorithm if there is no change over some time || Not exist in real systems
	UniversalLastUpdatePtr *time.Time
	TerminateSignal        chan int
}

type messageType struct {
	Source, Target  int
	MessageBodyType messageBodyType // initiate, test, accept, reject, report, wakeup
	// Fragment core ID: used in test, join and ChangeCoreMsg messages
	Fragment fragmentType
	// MWOE: used in report messages
	MWOE edgeType
	// CoreStatus: find / found
	CoreStatus nodeStatusType
}

type optionType struct {
	StartNode int
	Latency   int
}

type statisticsType struct {
	MessageCount        int
	TotalNetworkLatency int
}

type graphType struct {
	Node []nodeType
	Size int

	LastUpdate time.Time
	Option     optionType
	Statistics statisticsType
}

// MaxMessageSize : Max message queue size (for incoming messages and pending messages)
const MaxMessageSize int = 10
