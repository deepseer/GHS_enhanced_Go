package main

import (
	"errors"
	"fmt"
	"math/rand"
	"os"
	"time"
)

func (G *graphType) AddEdge(node1, node2, weight int) (err error) {
	if weight <= 0 || weight >= INFINITY ||
		node1 < 0 || node1 >= G.Size ||
		node2 < 0 || node2 >= G.Size {
		return errors.New("Invalid AddEdge() parameter")
	}
	if node1 >= node2 {
		node1, node2 = node2, node1
	}
	for i := 1; i <= G.Node[node1].PortCount; i++ {
		if G.Node[node1].Port[i].NodeV == node2 {
			return fmt.Errorf("Duplicated Edge: %d - %d", node1, node2)
		}
	}
	for j := 1; j <= G.Node[node2].PortCount; j++ {
		if G.Node[node2].Port[j].NodeV == node1 {
			return fmt.Errorf("Duplicated Edge %d - %d", node2, node1)
		}
	}
	G.Node[node1].PortCount++
	G.Node[node2].PortCount++
	G.Node[node1].Port[G.Node[node1].PortCount] = edgeType{NodeU: node1, NodeV: node2, Weight: weight, MSTStatus: basic, Activated: false}
	G.Node[node2].Port[G.Node[node2].PortCount] = edgeType{NodeU: node2, NodeV: node1, Weight: weight, MSTStatus: basic, Activated: false}
	logger.Printf("Edge added: %d - %d, weight = %d", node1, node2, weight)
	return nil
}

func (G *graphType) SendMessage(msg messageType) error {
	if msg.Target < 0 || msg.Target >= G.Size {
		logger.Printf("SendMessage error: %v\n", msg)
		return errors.New("SendMessage Error: invalid target")
	}

	var latency int
	if msg.Source != msg.Target { // simulate network latency
		latency = rand.Intn(G.Option.Latency-1) + 1 // 1~5 ms
	}

	var str string
	if msg.Source == msg.Target {
		str = fmt.Sprintf("[%d]->self {%s}", msg.Source, messageBodyTypeString[msg.MessageBodyType])
	} else {
		str = fmt.Sprintf("[%d]->[%d] {%s}", msg.Source, msg.Target, messageBodyTypeString[msg.MessageBodyType])
		G.Statistics.MessageCount++
	}
	switch msg.MessageBodyType {
	case report:
		str += fmt.Sprintf(" || MWOE: [%d]-[%d] Weight=%d", msg.MWOE.NodeU, msg.MWOE.NodeV, msg.MWOE.Weight)
	case changecore:
		str += fmt.Sprintf(" || MWOE: [%d]-[%d] Weight=%d, former CoreEdge=%d-%d (lv %d)", msg.MWOE.NodeU, msg.MWOE.NodeV, msg.MWOE.Weight,
			msg.Fragment.CoreEdge.NodeU, msg.Fragment.CoreEdge.NodeV, msg.Fragment.Level)
	case initiate:
		str += fmt.Sprintf(" || Fragment: CoreEdge=%d-%d (lv %d), status = %s, Sender's FindCount = %d",
			msg.Fragment.CoreEdge.NodeU, msg.Fragment.CoreEdge.NodeV, msg.Fragment.Level, nodeStatusTypeString[msg.CoreStatus],
			G.Node[msg.Source].FindCount)
	case connect:
		str += fmt.Sprintf(" || From {CoreEdge:%d-%d, lv:%d} to {CoreEdge:%d-%d, lv:%d}",
			G.Node[msg.Source].Fragment.CoreEdge.NodeU, G.Node[msg.Source].Fragment.CoreEdge.NodeV, G.Node[msg.Source].Fragment.Level,
			G.Node[msg.Target].Fragment.CoreEdge.NodeU, G.Node[msg.Target].Fragment.CoreEdge.NodeV, G.Node[msg.Target].Fragment.Level)
	case test:
		str += fmt.Sprintf(" || From {CoreEdge:%d-%d, lv:%d} to {CoreEdge:%d-%d, lv:%d}",
			G.Node[msg.Source].Fragment.CoreEdge.NodeU, G.Node[msg.Source].Fragment.CoreEdge.NodeV, G.Node[msg.Source].Fragment.Level,
			G.Node[msg.Target].Fragment.CoreEdge.NodeU, G.Node[msg.Target].Fragment.CoreEdge.NodeV, G.Node[msg.Target].Fragment.Level)
	}
	if msg.Source != msg.Target && msg.MessageBodyType != wakeup {
		str += fmt.Sprintf(" || %d ms", latency)
	}
	logger.Printf(str)

	if msg.Source != msg.Target && msg.MessageBodyType != wakeup {
		time.Sleep(time.Duration(latency) * time.Millisecond)
	}
	G.Node[msg.Target].IncomingMessage <- msg
	G.Statistics.TotalNetworkLatency += latency
	G.LastUpdate = time.Now()

	return nil
}

func (G *graphType) InitializeGraph(filename string) (err error) {
	if filename == "" {
		filename = "input.txt"
	}
	inputFile, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer inputFile.Close()
	var n int
	fmt.Fscanf(inputFile, "%d", &n)
	fmt.Println(n)

	INFINITYEDGE = edgeType{NodeU: -1, NodeV: -1, Weight: INFINITY, MSTStatus: rejected, Activated: true}

	G.Size = n
	G.Node = make([]nodeType, n)
	for i := 0; i < n; i++ {
		G.Node[i] = nodeType{ID: i,
			Port:                      make([]edgeType, n+1),
			PortCount:                 0,
			Status:                    sleeping,
			Fragment:                  fragmentType{CoreEdge: INFINITYEDGE, Level: 0, CoreDirectionPort: 0},
			MWOEDirectionPort:         0,
			MWOE:                      INFINITYEDGE,
			FindCount:                 0,
			TestEdge:                  -1,
			IncomingMessage:           make(chan messageType, MaxMessageSize),
			PendingTestMessage:        make([]messageType, G.Size),
			PendingTestMessageSize:    0,
			PendingConnectMessage:     make([]messageType, G.Size),
			PendingConnectMessageSize: 0,
			HasPendingReportMessage:   false,
			ParentGraph:               G,
			UniversalLastUpdatePtr:    &(G.LastUpdate),
			TerminateSignal:           make(chan int),
		}
		G.Node[i].Port[0] = edgeType{NodeU: i, NodeV: i, Weight: INFINITY, MSTStatus: loop, Activated: true}
	}

	var p, q, w int
	fmt.Fscanf(inputFile, "%d %d %d", &p, &q, &w)
	for p >= 0 && q >= 0 && w > 0 {
		err = G.AddEdge(p, q, w)
		if err != nil {
			fmt.Println("Error:", err.Error())
			return err
		}
		fmt.Fscanf(inputFile, "%d %d %d", &p, &q, &w)
	}

	// Sort each node's outbound edges by weight (primary) and neighbor's ID (secondary)
	for k := 0; k < n; k++ {
		p := G.Node[k].PortCount
		for i := 1; i < p; i++ {
			MinLabel := i
			for j := i + 1; j <= p; j++ {
				if G.Node[k].Port[j].CompareTo(G.Node[k].Port[MinLabel]) < 0 {
					MinLabel = j
				}
			}
			if i != MinLabel {
				G.Node[k].Port[i], G.Node[k].Port[MinLabel] = G.Node[k].Port[MinLabel], G.Node[k].Port[i]
			}
		}
	}

	return nil
}

// SlowStart : Start from a single node
func (G *graphType) SlowStart(StartNode int) {
	fmt.Println("Activating node", StartNode)
	G.SendMessage(messageType{Source: StartNode, Target: StartNode, MessageBodyType: wakeup})

}

// FastStart : All nodes start simultaneously
func (G *graphType) FastStart() {
	fmt.Println("Activing all nodes")
	for i := 0; i < G.Size; i++ {
		G.SendMessage(messageType{Source: i, Target: i, MessageBodyType: wakeup})
	}
}

func (G *graphType) PrintGraph() {
	fmt.Println("Print Graph:")
	fmt.Println("G.Size =", G.Size)
	for i := 0; i < G.Size; i++ {
		fmt.Printf("Node %d: PortCount = %d\n", i, G.Node[i].PortCount)
		for j := 0; j <= G.Node[i].PortCount; j++ {
			fmt.Printf("-[%d](%d)<%v> ", G.Node[i].Port[j].NodeV, G.Node[i].Port[j].Weight, edgeStatusTypeString[G.Node[i].Port[j].MSTStatus])
		}
		fmt.Println()
	}
}

func (G *graphType) PrintNodeStatus() {
	fmt.Println("Print Node Status:")
	for i := 0; i < G.Size; i++ {
		G.Node[i].PrintOneNodeStatus()
	}

}

func (G *graphType) PrintResult() {
	fmt.Println("Result:")
	s := 0
	for i := 0; i < G.Size; i++ {
		for j := 1; j <= G.Node[i].PortCount; j++ {
			if i < G.Node[i].Port[j].NodeV && G.Node[i].Port[j].MSTStatus == branch {
				s += G.Node[i].Port[j].Weight
				fmt.Printf("[%d]-[%d] : %d\n", i, G.Node[i].Port[j].NodeV, G.Node[i].Port[j].Weight)
			}
		}
	}
	fmt.Printf("Minimal weight = %d\n", s)
	fmt.Printf("Total message = %d\n", G.Statistics.MessageCount)
	fmt.Printf("Total network latency = %d (ms)\n", G.Statistics.TotalNetworkLatency)
}

func (G *graphType) LogResult() {
	logger.Println("*** Log Result:")
	s := 0
	for i := 0; i < G.Size; i++ {
		for j := 1; j <= G.Node[i].PortCount; j++ {
			if i < G.Node[i].Port[j].NodeV && G.Node[i].Port[j].MSTStatus == branch {
				s += G.Node[i].Port[j].Weight
				logger.Printf("*** [%d]-[%d] : %d", i, G.Node[i].Port[j].NodeV, G.Node[i].Port[j].Weight)
			}
		}
	}
	//fmt.Printf("Minimal weight = %d\n", s)
}

func (G *graphType) StartAlgorithm(Option optionType) (err error) {
	G.Option = Option
	G.Statistics = statisticsType{MessageCount: 0, TotalNetworkLatency: 0}
	G.PrintGraph()
	for i := 0; i < G.Size; i++ {
		go G.Node[i].ProcessNode()
	}

	if Option.StartNode == -1 {
		G.FastStart()
	} else {
		G.SlowStart(Option.StartNode)
	}
	G.UniversalWatcher()
	G.PrintNodeStatus()
	G.PrintResult()
	return nil
}
