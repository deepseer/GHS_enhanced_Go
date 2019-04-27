package main

import ()

// Edge Status

func (N *nodeType) SetStatus(s nodeStatusType) {
	if N.Status != s {
		logger.Printf("[%d] => <%s>", N.ID, nodeStatusTypeString[s])
		N.Status = s
	}
}

func (N *nodeType) GetStatus() nodeStatusType {
	return N.Status
}

func (N *nodeType) SetEdgeStatus(p int, s edgeStatusType) {
	if p <= 0 || p > N.PortCount {
		logger.Printf("[%d] Illegal SetEdgeStatus: p = %d, s = %s", N.ID, p, edgeStatusTypeString[s])
	}
	if N.Port[p].MSTStatus != s {
		logger.Printf("[%d]-[%d] => <%s>", N.ID, N.Port[p].NodeV, edgeStatusTypeString[s])
		N.Port[p].MSTStatus = s
	}
}

// Compare Edge by Weight

func (E1 *edgeType) CompareTo(E2 edgeType) int {
	if E1.Weight < E2.Weight {
		return -1
	}
	if E1.Weight > E2.Weight {
		return 1
	}
	if E1.NodeU < E2.NodeU {
		return -1
	}
	if E1.NodeU < E2.NodeU {
		return 1
	}
	if E1.NodeV < E2.NodeV {
		return -1
	}
	if E1.NodeV > E2.NodeV {
		return 1
	}
	return 0
}

func (N *nodeType) FindPort(nodeV int) int {
	if nodeV < 0 {
		return -1
	}
	for i := 0; i <= N.PortCount; i++ {
		if nodeV == N.Port[i].NodeV {
			return i
		}
	}
	return -1
}

func (F *fragmentType) Fix() {
	if F.CoreEdge.NodeU != -1 && F.CoreEdge.NodeV != -1 && F.CoreEdge.NodeV > F.CoreEdge.NodeU {
		F.CoreEdge.NodeU, F.CoreEdge.NodeV = F.CoreEdge.NodeV, F.CoreEdge.NodeU
	}
}

func (N *nodeType) IsCore() bool {
	if N.Fragment.Level == 0 {
		return true
	}
	if N.Fragment.CoreEdge.NodeU == N.ID || N.Fragment.CoreEdge.NodeV == N.ID {
		return true
	}
	return false
}

func (N *nodeType) TheOtherCore() int {
	if !N.IsCore() {
		return -1
	}
	if N.Fragment.CoreEdge.NodeU == N.ID {
		return N.Fragment.CoreEdge.NodeV
	}
	return N.Fragment.CoreEdge.NodeU
}

func (N *nodeType) BroadcastExcept(msg messageType, exception int) {
	for i := 1; i <= N.PortCount; i++ {
		if N.Port[i].NodeV != exception {
			msg.Target = N.Port[i].NodeV
			N.ParentGraph.SendMessage(msg)
		}
	}
}

func (N *nodeType) PushPendingTestMessage(msg messageType) {
	var pos, i int
	for pos = 0; pos < N.PendingTestMessageSize; pos++ {
		if N.PendingTestMessage[pos].Fragment.Level > msg.Fragment.Level ||
			(N.PendingTestMessage[pos].Fragment.Level == msg.Fragment.Level) {
			break
		}
	}
	for i = N.PendingTestMessageSize; i > pos; i-- {
		N.PendingTestMessage[i] = N.PendingTestMessage[i-1]
	}
	N.PendingTestMessage[i] = msg
	N.PendingTestMessageSize++
}

func (N *nodeType) PeekPendingTestMessageLevel() int {
	if N.PendingTestMessageSize > 0 {
		return N.PendingTestMessage[0].Fragment.Level
	}
	return -1
}

func (N *nodeType) PopPendingTestMessage() (msg messageType) {
	msg = N.PendingTestMessage[0]
	N.PendingTestMessageSize--
	return
}

func (N *nodeType) FindPossibleMWOE() int {
	node := N.ID
	MWOE := N.LastMWOETestPort + 1
	for MWOE <= N.PortCount && N.Port[MWOE].MSTStatus != basic {
		MWOE++
	}
	N.LastMWOETestPort = MWOE

	if MWOE <= N.PortCount {
		logger.Printf("[%d] found possible MWOE: [%d] (weight %d)", node, N.Port[MWOE].NodeV, N.Port[MWOE].Weight)
	} else {
		logger.Printf("[%d] now MWOE found: all adjacent edges are labeled as [branch] or [reject]", node)
	}
	return MWOE
}
