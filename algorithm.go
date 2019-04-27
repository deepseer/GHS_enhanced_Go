package main

import (
	"time"
)

func (G *graphType) UniversalWatcher() int {
	G.LastUpdate = time.Now()
	for time.Now().Sub(G.LastUpdate).Nanoseconds() < 500000000 { //500ms
		time.Sleep(time.Nanosecond * 100000000) //100ms
	}
	logger.Printf("GHS last updated at %v\n current time is %v", G.LastUpdate, time.Now())
	for i := 0; i < G.Size; i++ {
		G.Node[i].TerminateSignal <- 1
	}
	time.Sleep(time.Nanosecond * 10000000) //100ms
	return 0
}

func (N *nodeType) ProcessNode() {
	node := N.ID
	logger.Printf("[%d] process start", node)

	for {
		select {
		case <-N.TerminateSignal:
			logger.Printf(">> [%d] received terminate signal while <%s>", node, nodeStatusTypeString[N.Status])
			return
		case recvMsg := <-N.IncomingMessage:
			logger.Printf(">> [%d] received {%s} from [%d]", node, messageBodyTypeString[recvMsg.MessageBodyType], recvMsg.Source)

			switch recvMsg.MessageBodyType {

			case wakeup:
				// Simplified network detection method: wakeups the weight of adjecent edges, and wake up
				// adjecent nodes in the process
				if N.Status == sleeping {
					logger.Printf("[%d] activated", node)
					N.SetStatus(find)

					p := N.FindPort(recvMsg.Source)

					logger.Printf("[%d] received {wakeup} from [%d] (port /%d/)", node, recvMsg.Source, p)

					for i := 1; i <= N.PortCount; i++ {
						if i != p {
							WakeupMsg := messageType{
								Source:          node,
								Target:          N.Port[i].NodeV,
								MessageBodyType: wakeup,
							}
							N.ParentGraph.SendMessage(WakeupMsg)
							N.Port[i].Activated = true
						}
					}

					MWOE := N.FindOutboundEdge()
					if MWOE <= N.PortCount {
						N.SetStatus(found)
						N.SetEdgeStatus(MWOE, branch)
						ConnectMsg := messageType{
							Source:          node,
							Target:          N.Port[MWOE].NodeV,
							MessageBodyType: connect,
							Fragment:        N.Fragment,
						}
						//N.ParentGraph.LogResult()
						N.ParentGraph.SendMessage(ConnectMsg)
					}
				}

			case connect:
				N.ResponseToConnect(recvMsg)

				// Core broadcasts to all its childs, and they will start to find its
				// Minimal Weight Outbound Edge (MWOE)
				// Also works as updating fragment information after a fragment merge without searching for a new MWOE
				// CoreStatus: find (begin search), found (inform update only)
			case initiate:
				// Drop outdated initiate messages (if any... does it really exist?)
				if recvMsg.Fragment.Level >= N.Fragment.Level {
					p := N.FindPort(recvMsg.Source)
					N.Fragment.CoreDirectionPort = p
					N.Fragment.CoreEdge = recvMsg.Fragment.CoreEdge
					N.Fragment.Level = recvMsg.Fragment.Level
					N.SetStatus(recvMsg.CoreStatus)
					N.PrintOneNodeStatus()
					// Flood {initiate} to childs
					floodcount := 0
					for i := 1; i <= N.PortCount; i++ {
						if i != p && N.Port[i].MSTStatus == branch {
							InitiateMsg := messageType{
								Source:          node,
								Target:          N.Port[i].NodeV,
								MessageBodyType: initiate,
								Fragment:        N.Fragment,
								CoreStatus:      recvMsg.CoreStatus,
							}
							N.ParentGraph.SendMessage(InitiateMsg)
							if N.GetStatus() == find {
								N.FindCount++
							}
							floodcount++
						}
					}
					logger.Printf("[%d] (status=%s) flooded %d {initiate} messages, FindCount = %d",
						node, nodeStatusTypeString[N.GetStatus()], floodcount, N.FindCount)

					// Process pending connect mesages
					for i := 1; i <= N.PendingConnectMessageSize; i++ {
						pcMsg := N.PendingConnectMessage[i]
						if pcMsg.Fragment.Level < N.Fragment.Level {
							logger.Printf("[%d] retrieved pending {connect} from [%d]", node, pcMsg.Source)
							N.ResponseToConnect(pcMsg)
							N.DeletePendingConnectMessage(i)
							i--
							logger.Printf("[%d] has %d pending {connect} messages", node, N.PendingConnectMessageSize)
						}
					}

					// Process pending test messages
					for i := 1; i <= N.PendingTestMessageSize; i++ {
						if N.PendingTestMessage[i].Fragment.Level <= N.Fragment.Level {
							ptMsg := N.PendingTestMessage[i]
							logger.Printf("[%d] retrieved pending {test} from [%d]: {CoreEdge:%d-%d, lv:%d} <== {CoreEdge:%d-%d, lv:%d}", node, ptMsg.Source,
								N.Fragment.CoreEdge.NodeU, N.Fragment.CoreEdge.NodeV, N.Fragment.Level,
								ptMsg.Fragment.CoreEdge.NodeU, ptMsg.Fragment.CoreEdge.NodeV, ptMsg.Fragment.Level)
							if ptMsg.Fragment.CoreEdge.CompareTo(N.Fragment.CoreEdge) == 0 {
								p := N.FindPort(ptMsg.Source)
								if N.Port[p].MSTStatus == basic {
									N.SetEdgeStatus(p, rejected)
								}
								// (*) The exception that is mentioned above
								if N.TestEdge != ptMsg.Source {
									logger.Printf("[%d] (rejected) pending {test} from [%d]", node, ptMsg.Source)
									RejectMsg := messageType{
										Source:          node,
										Target:          ptMsg.Source,
										MessageBodyType: reject,
										Fragment:        N.Fragment,
									}
									N.ParentGraph.SendMessage(RejectMsg)
								} else {
									logger.Printf("[%d] silently (rejected) pending {test} from [%d]", node, ptMsg.Source)
								}
							} else {
								AcceptMsg := messageType{
									Source:          node,
									Target:          ptMsg.Source,
									MessageBodyType: accept,
									Fragment:        N.Fragment,
								}
								N.ParentGraph.SendMessage(AcceptMsg)
								logger.Printf("[%d] accepted {test} from [%d]", node, recvMsg.Source)
							}
							N.DeletePendingTestMessage(i)
							i--
							logger.Printf("[%d] has %d pending {connect} messages", node, N.PendingConnectMessageSize)
						}
					}
					// Find Minimal Weight Outbound Edge
					//
					//     After a node [n] in a lower-level fragment [F] connects (and is absorbed by) a node [n'] in a higher-level
					// fragment [F']. [n'] sends an initiate message to [n], with [n']'s current node status
					//
					// find: [n'] has not sent its report to its parent. Every node formerly in fragment [F] should join the search.
					// found: [n'] has sent its report to its parent. It has found an MWOE different (with less weight) from the edge (n, n').
					//
					//     Since edge (n, n') is [F]'s MWOE, [F+F']'s MWOE can't appear in [F]. Nodes formerly in fragment [F] does not need
					// to search for a new MWOE, but have to be informed that their fragment has changed to [F'].
					//
					//     Normal initiate message from the core is always "find", and every node informed should search for its MWOE.
					if N.GetStatus() == find {
						N.Test()
					}

				} else {
					logger.Printf("[%d] (lv %d) received an outdated initiate message from [%d] (lv %d)",
						node, N.Fragment.Level, recvMsg.Source, recvMsg.Fragment.Level)
				}

			case test:
				// A node receiving a test message does the following:
				// • If the id in the message is the same as the id of the fragment the
				//   node belongs to – a reject message is sent back. (*)
				// • If the id in the message differs from the id of the fragment the node
				//   belongs to and the level in the message is lower or equal to that of
				//   the node's fragment – an accept message is sent back.
				// • If the id in the message differs from the id of the fragment the node
				//   belongs to and the level in the message is higher than that of the
				//   node's fragment – no reply is sent until the situation has changed.
				// (*) an exception to this rule is when the node that sent the test message
				// receives a test message along the same edge with the same id. In such
				// a case a reject message is not sent (the other side would get the
				// message sent by this node and know that this edge is not an outgoing
				// edge). This is done in order to obtain a small decrease in message
				// complexity.
				logger.Printf("[%d] received {test} from [%d]: {CoreEdge:%d-%d, lv:%d} <== {CoreEdge:%d-%d, lv:%d}", node, recvMsg.Source,
					N.Fragment.CoreEdge.NodeU, N.Fragment.CoreEdge.NodeV, N.Fragment.Level,
					recvMsg.Fragment.CoreEdge.NodeU, recvMsg.Fragment.CoreEdge.NodeV, recvMsg.Fragment.Level)
				if recvMsg.Fragment.CoreEdge.CompareTo(N.Fragment.CoreEdge) == 0 {
					p := N.FindPort(recvMsg.Source)
					if N.Port[p].MSTStatus == basic {
						N.SetEdgeStatus(p, rejected)
					}
					// (*) The exception that is mentioned above
					if N.TestEdge != recvMsg.Source {
						logger.Printf("[%d] (rejected) {test} from [%d]", node, recvMsg.Source)
						RejectMsg := messageType{
							Source:          node,
							Target:          recvMsg.Source,
							MessageBodyType: reject,
							Fragment:        N.Fragment,
						}
						N.ParentGraph.SendMessage(RejectMsg)
					} else {
						logger.Printf("[%d] silently (rejected) {test} from [%d]", node, recvMsg.Source)
						N.TestEdge = -1
						N.Test()
					}
				} else if recvMsg.Fragment.Level <= N.Fragment.Level {
					AcceptMsg := messageType{
						Source:          node,
						Target:          recvMsg.Source,
						MessageBodyType: accept,
						Fragment:        N.Fragment,
					}
					N.ParentGraph.SendMessage(AcceptMsg)
					logger.Printf("[%d] accepted {test} from [%d]", node, recvMsg.Source)
				} else {
					// wait
					// push into priority queue
					N.StorePendingTestMessage(recvMsg)
					logger.Printf("[%d] (lv %d) received {test} from [%d] (lv %d); {test} message stored",
						node, N.Fragment.Level, recvMsg.Source, recvMsg.Fragment.Level)
				}

			case accept:
				// {accept}
				logger.Printf("[%d] received {accept} from [%d], FindCount = %d", node, recvMsg.Source, N.FindCount)
				N.TestEdge = -1
				p := N.FindPort(recvMsg.Source)
				if N.MWOE.CompareTo(N.Port[p]) > 0 {
					N.MWOEDirectionPort = p
					N.MWOE = N.Port[p]
					logger.Printf("[%d] changed its MWOE to %d-%d (weight %d)", node, node, N.Port[p].NodeV, N.Port[p].Weight)
				}
				N.Report()

			case reject:
				// {reject}
				logger.Printf("[%d] received {reject} from [%d]", node, recvMsg.Source)
				N.TestEdge = -1
				p := N.FindPort(recvMsg.Source)
				if p >= 0 && N.Port[p].MSTStatus == basic {
					N.SetEdgeStatus(p, rejected)
				}
				N.Test()

			case report:
				logger.Printf("[%d] received {report} from [%d]: [%d]'s current FindCount = %d, MWOE info: [%d]-[%d] (weight %d), direction=%d ([%d])",
					node, recvMsg.Source, node, N.FindCount,
					N.MWOE.NodeU, N.MWOE.NodeV, N.MWOE.Weight, N.MWOEDirectionPort, N.Port[N.MWOEDirectionPort].NodeV)
				p := N.FindPort(recvMsg.Source)

				// From its "child"
				if p != N.Fragment.CoreDirectionPort {
					logger.Printf("[%d] (status=%s) received a {report} from its child: relay or drop message",
						node, nodeStatusTypeString[N.Status])
					N.FindCount--
					if recvMsg.MWOE.CompareTo(N.MWOE) < 0 {
						N.MWOE = recvMsg.MWOE
						N.MWOEDirectionPort = p
						logger.Printf("[%d] updated its MWOE: [%d]-[%d] (weight %d), direction=%d ([%d]",
							node, N.MWOE.NodeU, N.MWOE.NodeV, N.MWOE.Weight, N.MWOEDirectionPort, N.Port[N.MWOEDirectionPort].NodeV)
					} else {
						logger.Printf("[%d] dropped {report} from [%d]", node, recvMsg.Source)
					}
					N.Report()

					// See below:
					// If 1) This node is a core node;
					//    2) This core node has received the other core node's {report};
					//    3) This message is the last {report} which the node is waiting for;
					//    4) This node has decided its own MWOE.
					// This node will report to the other core node immediately.

				} else { // From its "parent": only happens when the node farther from the MWOE receives a message
					//     from the node closer to the MWOE
					// send {changecore} and merge/absorb
					logger.Printf("[%d] (status=%s) is the core node farther from the MWOE in the {report}: decide next action",
						node, nodeStatusTypeString[N.Status])
					if N.GetStatus() == found {
						// This core node has {report}ed its MWOE info to the other core node before.
						// Now this core node knows this fragment's MWOE.
						if recvMsg.MWOE.CompareTo(N.MWOE) > 0 {
							// The MWOE is in this core's branch: it is the core closer to the MWOE.
							logger.Printf("[%d] starts {changecore} to [%d], MWOE: [%d]-[%d] (weight %d)",
								node, N.Port[N.MWOEDirectionPort].NodeV, N.MWOE.NodeU, N.MWOE.NodeV, N.MWOE.Weight)
							N.ChangeCore()
						} else {
							// The MWOE is in the other core's branch. Ignore it.
							// The other core will know the result after this core sends {report}.
							logger.Printf("[%d] ignored {report} from [%d]", node, recvMsg.Source)

							if recvMsg.MWOE.IsInfinity() && N.MWOE.IsInfinity() {
								// No MWOE found by both core nodes
								logger.Printf("Fragment [%d]-[%d] (lv %d) found no MWOE",
									N.Fragment.CoreEdge.NodeU, N.Fragment.CoreEdge.NodeV, N.Fragment.Level)
								logger.Printf("[%d] EXIT", node)
							}
						}
					} else {
						// This core node has NOT decided and {report}ed its branch's MWOE info to the other core node yet.
						// The {report} message will be stored, and processed after
						//     this node has collected all {report}/{accept}/{reject} from its branch and is ready to make its own {report}.
						logger.Printf("[%d] (status=%s) (TestEdge=%d) has not collected all {report}/{accept}/{reject} from its branch; {report} message stored",
							node, nodeStatusTypeString[N.Status], N.TestEdge)
						N.HasPendingReportMessage = true
						N.PendingReportMessage = recvMsg
					}

				}

			case changecore:
				// First from the core node that is farther to the MWOE to the core node that is closer to the MWOE
				logger.Printf("[%d] received {changecore} from [%d], MWOEDirectionPort = %d",
					node, recvMsg.Source, N.MWOEDirectionPort)
				N.ChangeCore()

			} // switch recvMsg.MessageBodyType
		} // select
	} // for loop

}
