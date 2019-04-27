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
	if E1.Weight == INFINITYEDGE.Weight && E2.Weight == INFINITYEDGE.Weight {
		return 0
	}
	if (E1.NodeU == E2.NodeU && E1.NodeV == E2.NodeV) || (E1.NodeV == E2.NodeU && E1.NodeU == E2.NodeV) {
		return 0
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

func (E1 *edgeType) IsInfinity() bool {
	if E1.Weight == INFINITYEDGE.Weight {
		return true
	}
	return false
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
	if F.CoreEdge.NodeU != -1 && F.CoreEdge.NodeV != -1 && F.CoreEdge.NodeV < F.CoreEdge.NodeU {
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

func (N *nodeType) StorePendingTestMessage(msg messageType) {
	N.PendingTestMessageSize++
	N.PendingTestMessage[N.PendingTestMessageSize] = msg
}

func (N *nodeType) DeletePendingTestMessage(pos int) {
	for i := pos; i < N.PendingTestMessageSize; i++ {
		N.PendingTestMessage[i] = N.PendingTestMessage[i+1]
	}
	N.PendingTestMessageSize--
}

func (N *nodeType) StorePendingConnectMessage(msg messageType) {
	N.PendingConnectMessageSize++
	N.PendingConnectMessage[N.PendingConnectMessageSize] = msg
}

func (N *nodeType) DeletePendingConnectMessage(pos int) {
	for i := pos; i < N.PendingConnectMessageSize; i++ {
		N.PendingConnectMessage[i] = N.PendingConnectMessage[i+1]
	}
	N.PendingConnectMessageSize--
}

func (N *nodeType) FindOutboundEdge() int {
	node := N.ID
	MWOE := 1
	for MWOE <= N.PortCount && N.Port[MWOE].MSTStatus != basic {
		MWOE++
	}

	if MWOE <= N.PortCount {
		logger.Printf("[%d] found [basic] edge: [%d] (weight %d)", node, N.Port[MWOE].NodeV, N.Port[MWOE].Weight)
	} else {
		logger.Printf("[%d] found no [basic] edge: all adjacent edges are labeled [branch] or [reject]", node)
	}
	return MWOE
}

func (N *nodeType) PrintOneNodeStatus() {
	logger.Printf("-- [%d]: Status=%s, FindCount=%d, {CoreEdge:%d-%d, lv:%d}, CoreDirection=%d, MWOEDirection=%d, MWOEWeight=%d",
		N.ID, nodeStatusTypeString[N.Status], N.FindCount,
		N.Fragment.CoreEdge.NodeU, N.Fragment.CoreEdge.NodeV, N.Fragment.Level,
		N.Port[N.Fragment.CoreDirectionPort].NodeV,
		N.MWOE.NodeV, N.MWOE.Weight)
}
