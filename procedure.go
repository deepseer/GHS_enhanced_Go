package main

func (N *nodeType) Test() {
	node := N.ID
	MWOE := N.FindOutboundEdge()

	if MWOE <= N.PortCount {
		logger.Printf("[%d] {test} possible outbound edge: [%d]-[%d] (weight %d)", node, node, N.Port[MWOE].NodeV, N.Port[MWOE].Weight)
		N.TestEdge = N.Port[MWOE].NodeV
		TestMsg := messageType{
			Source:          node,
			Target:          N.Port[MWOE].NodeV,
			MessageBodyType: test,
			Fragment:        N.Fragment,
		}
		N.ParentGraph.SendMessage(TestMsg)
	} else {
		N.MWOE = INFINITYEDGE
		N.TestEdge = -1
		logger.Printf("[%d] found no possible outbound edge, FindCount=%d", node, N.FindCount)
		N.Report()
	}
}

func (N *nodeType) Report() {
	node := N.ID
	if N.FindCount <= 0 && N.TestEdge == -1 {
		N.SetStatus(found)
		ReportMsg := messageType{
			Source:          node,
			Target:          N.Port[N.Fragment.CoreDirectionPort].NodeV,
			MessageBodyType: report,
			MWOE:            N.MWOE,
		}
		N.ParentGraph.SendMessage(ReportMsg)

		// Process pending {report} message
		if N.HasPendingReportMessage {
			N.HasPendingReportMessage = false
			// After this core node {report}s its MWOE info to the other core node.
			// Now this core node knows the fragment's MWOE.
			if N.PendingReportMessage.MWOE.CompareTo(N.MWOE) > 0 {
				// The MWOE is in this core's branch: it is the core closer to the MWOE.
				logger.Printf("[%d] starts {changecore} to [%d] (delayed), MWOE: [%d]-[%d] (weight %d)",
					node, N.Port[N.MWOEDirectionPort].NodeV, N.MWOE.NodeU, N.MWOE.NodeV, N.MWOE.Weight)
				ChangeCoreMsg := messageType{
					Source:          node,
					Target:          N.Port[N.MWOEDirectionPort].NodeV,
					MessageBodyType: changecore,
					Fragment:        N.Fragment,
					MWOE:            N.MWOE,
				}
				//N.ParentGraph.LogResult()
				N.ParentGraph.SendMessage(ChangeCoreMsg)
			} else {
				// The MWOE is in the other core's branch. Ignore it.
				// The other core will know the result after this core sends {report}.
				logger.Printf("[%d] ignored {report} from [%d] (delayed)", node, N.PendingReportMessage.Source)

				if N.PendingReportMessage.MWOE.IsInfinity() && N.MWOE.IsInfinity() {
					// No MWOE found by both core nodes
					logger.Printf("Fragment [%d]-[%d] (lv %d) found no MWOE (delayed)",
						N.Fragment.CoreEdge.NodeU, N.Fragment.CoreEdge.NodeV, N.Fragment.Level)
					logger.Printf("[%d] EXIT", node)
				}
			}
		}
	}
}

func (N *nodeType) ChangeCore() {
	node := N.ID
	N.Fragment.CoreDirectionPort = N.MWOEDirectionPort
	if N.Port[N.MWOEDirectionPort].MSTStatus == branch {
		// Not the node that found the MWOE: relay
		ChangeCoreMsg := messageType{
			Source:          node,
			Target:          N.Port[N.MWOEDirectionPort].NodeV,
			MessageBodyType: changecore,
			Fragment:        N.Fragment,
			MWOE:            N.MWOE,
		}
		//N.ParentGraph.LogResult()
		N.ParentGraph.SendMessage(ChangeCoreMsg)
	} else {
		// The node that found the MWOE: {connect}
		N.SetEdgeStatus(N.MWOEDirectionPort, branch)
		ConnectMsg := messageType{
			Source:          node,
			Target:          N.Port[N.MWOEDirectionPort].NodeV,
			MessageBodyType: connect,
			Fragment:        N.Fragment,
		}
		//N.ParentGraph.LogResult()
		N.ParentGraph.SendMessage(ConnectMsg)

		// Process pending connect mesages
		for i := 1; i <= N.PendingConnectMessageSize; i++ {
			pcMsg := N.PendingConnectMessage[i]
			p := N.FindPort(pcMsg.Source)
			if pcMsg.Fragment.Level < N.Fragment.Level ||
				(pcMsg.Fragment.Level == N.Fragment.Level && N.Port[p].MSTStatus == branch) {
				logger.Printf("[%d] retrieved pending {connect} from [%d]", node, pcMsg.Source)
				N.ResponseToConnect(pcMsg)
				N.DeletePendingConnectMessage(i)
				i--
				logger.Printf("[%d] has %d pending {connect} messages", node, N.PendingConnectMessageSize)
			}
		}
	}
}

func (N *nodeType) ResponseToConnect(recvMsg messageType) {
	node := N.ID
	p := N.FindPort(recvMsg.Source)

	// From lower level to higher level: request to be absorbed
	if recvMsg.Fragment.Level < N.Fragment.Level {
		logger.Printf("[%d] is going to merge/absorb [%d]'s fragment, status=%v", node, recvMsg.Source, nodeStatusTypeString[N.Status])
		N.SetEdgeStatus(p, branch)

		// Initiate message is only sent via the newly-added branch edge to the newly-absorbed fragment
		// with status: find or found
		InitiateMsg := messageType{
			Source:          node,
			Target:          N.Port[p].NodeV,
			MessageBodyType: initiate,
			Fragment:        N.Fragment,
			CoreStatus:      N.GetStatus(),
		}
		N.ParentGraph.SendMessage(InitiateMsg)
		if N.GetStatus() == find {
			N.FindCount++
		}
		logger.Printf("[%d] replied {connect} from [%d], status = %s, FindCount = %d",
			node, recvMsg.Source, nodeStatusTypeString[N.GetStatus()], N.FindCount)
	} else                            // Merge or absorb
	if N.Port[p].MSTStatus == basic { // B-(connect)->A ***BEFORE*** A-(connect)->B
		// Delay the message until one of the following prerequisites is fullfiled:
		// 1. A's level increases, and b[merge]a becomes a[absorb]b
		// 2. A has absorbed some other fragments and removed some MWOEs, making A-B the current MWOE
		// wait
		// Process pending connect message when:
		// 1. A receives an {initiate} message and its level increases to A>=B.
		N.StorePendingConnectMessage(recvMsg)
		logger.Printf("[%d] received {connect} from [%d] BEFORE sending {connect} to [%d]; {connect} message stored, size = %d",
			node, recvMsg.Source, recvMsg.Source, N.PendingConnectMessageSize)
	} else { // B-(connect)->A ***AFTER*** A-(connect)->B; A-(initiate)->B. <A,B> becomes the new core.
		logger.Printf("[%d] received {connect} from [%d] AFTER sending {connect} to [%d]", node, recvMsg.Source, recvMsg.Source)
		NewFragment := fragmentType{
			CoreEdge:          N.Port[p],
			Level:             N.Fragment.Level + 1,
			CoreDirectionPort: p,
		}
		NewFragment.Fix()
		N.Fragment = NewFragment

		InitiateMsg := messageType{
			Source:          node,
			Target:          N.Port[p].NodeV,
			MessageBodyType: initiate,
			Fragment:        NewFragment,
			CoreStatus:      find,
		}
		N.ParentGraph.SendMessage(InitiateMsg)
		logger.Printf("[%d] send {initiate} over new core edge [%d]-[%d]", node, node, N.Port[p].NodeV)
	}
}
