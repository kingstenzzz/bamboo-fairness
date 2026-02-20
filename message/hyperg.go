package message

import (
	"github.com/gitferry/bamboo/crypto"
	"github.com/gitferry/bamboo/identity"
	"github.com/gitferry/bamboo/types"
)

// HyperGProposal represents a proposal message containing a node's local transaction ordering
type HyperGProposal struct {
	View       types.View
	Proposer   identity.NodeID
	Cmds       []crypto.Identifier
	Timestamps []int64
}

// NewHyperGProposal creates a new HyperG proposal message
func NewHyperGProposal(view types.View, proposer identity.NodeID, cmds []crypto.Identifier, timestamps []int64) *HyperGProposal {
	return &HyperGProposal{
		View:       view,
		Proposer:   proposer,
		Cmds:       cmds,
		Timestamps: timestamps,
	}
}

// HyperGResult represents the result of HyperG fair ordering computation
type HyperGResult struct {
	View         types.View
	OrderedCmds  []crypto.Identifier
	Stats        *HyperGStats
	ErrorMessage string
}

// HyperGStats contains statistics about the HyperG algorithm execution
type HyperGStats struct {
	TotalTime          float64
	PrefMatrixTime     float64
	ClusteringTime     float64
	HypergraphTime     float64
	ExtractionTime     float64
	NHyperedges        int
	AvgHyperedgeSize   float64
	MaxHyperedgeSize   int
	MinHyperedgeSize   int
	Gamma              float64
	Delta              int
	Threshold          int
}

// HyperGAck represents an acknowledgment message for HyperG proposals
type HyperGAck struct {
	View     types.View
	Receiver identity.NodeID
	Sender   identity.NodeID
	Accepted bool
}

// NewHyperGAck creates a new HyperG acknowledgment message
func NewHyperGAck(view types.View, receiver, sender identity.NodeID, accepted bool) *HyperGAck {
	return &HyperGAck{
		View:     view,
		Receiver: receiver,
		Sender:   sender,
		Accepted: accepted,
	}
}