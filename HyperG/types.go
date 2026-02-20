package hyperg

import (
	"github.com/gitferry/bamboo/crypto"
	"sort"
)

// OrderedList represents a node's local transaction ordering with timestamps
type OrderedList struct {
	Cmds       []crypto.Identifier
	Timestamps []int64
}

// NewOrderedList creates a new ordered list
func NewOrderedList(cmds []crypto.Identifier, timestamps []int64) *OrderedList {
	return &OrderedList{
		Cmds:       cmds,
		Timestamps: timestamps,
	}
}

// Sort sorts the ordered list by timestamps
func (ol *OrderedList) Sort() {
	if len(ol.Cmds) != len(ol.Timestamps) {
		return
	}
	
	indices := make([]int, len(ol.Cmds))
	for i := range indices {
		indices[i] = i
	}
	
	sort.Slice(indices, func(i, j int) bool {
		return ol.Timestamps[indices[i]] < ol.Timestamps[indices[j]]
	})
	
	newCmds := make([]crypto.Identifier, len(ol.Cmds))
	newTimestamps := make([]int64, len(ol.Timestamps))
	
	for i, idx := range indices {
		newCmds[i] = ol.Cmds[idx]
		newTimestamps[i] = ol.Timestamps[idx]
	}
	
	ol.Cmds = newCmds
	ol.Timestamps = newTimestamps
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

// HyperGConfig holds configuration parameters for HyperG algorithm
type HyperGConfig struct {
	Enabled      bool    `json:"enabled"`
	Gamma        float64 `json:"gamma"`         // Fairness parameter (0.0-1.0)
	Delta        int     `json:"delta"`         // Clustering distance threshold
	ProposalWait int     `json:"proposal_wait"` // Wait time for proposals in ms
}