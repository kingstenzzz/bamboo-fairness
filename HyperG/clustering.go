package hyperg

import (
	"github.com/gitferry/bamboo/crypto"
	"github.com/gitferry/bamboo/identity"
)

// computePairwisePreferences calculates the pairwise preference matrix
// pref[i][j] = number of nodes that prefer transaction i before j
func (hg *HyperGSorter) computePairwisePreferences(proposals map[identity.NodeID]*OrderedList) [][]int {
	// First, collect all transactions and create mapping
	allTxs := make(map[crypto.Identifier]int)
	txList := make([]crypto.Identifier, 0)
	
	for _, ol := range proposals {
		ol.Sort() // Ensure sorted by timestamps
		for _, tx := range ol.Cmds {
			if _, exists := allTxs[tx]; !exists {
				allTxs[tx] = len(txList)
				txList = append(txList, tx)
			}
		}
	}
	
	b := len(txList)
	if b == 0 {
		return make([][]int, 0)
	}
	
	// Initialize preference matrix
	pref := make([][]int, b)
	for i := range pref {
		pref[i] = make([]int, b)
	}
	
	// Calculate preferences
	for _, ol := range proposals {
		positions := make(map[int]int)
		for pos, tx := range ol.Cmds {
			if txIdx, exists := allTxs[tx]; exists {
				positions[txIdx] = pos
			}
		}
		
		// Compare all pairs
		for i := 0; i < b; i++ {
			for j := 0; j < b; j++ {
				if i != j {
					posI, existsI := positions[i]
					posJ, existsJ := positions[j]
					
					if existsI && existsJ && posI < posJ {
						pref[i][j]++
					}
				}
			}
		}
	}
	
	return pref
}

// detectCondorcetClusters performs canonical embedding and clustering
func (hg *HyperGSorter) detectCondorcetClusters(proposals map[identity.NodeID]*OrderedList, prefMatrix [][]int) [][]int {
	b := len(prefMatrix)
	if b == 0 {
		return [][]int{}
	}
	
	// Step 1: Calculate canonical positions using threshold filtering
	txCanonicalPos := make(map[int]int)
	
	for tx := 0; tx < b; tx++ {
		countBefore := 0
		for other := 0; other < b; other++ {
			if other != tx {
				// Only count if pref[other][tx] >= threshold
				if prefMatrix[other][tx] >= hg.threshold {
					countBefore++
				}
			}
		}
		txCanonicalPos[tx] = countBefore
	}
	
	// Step 2: Pre-sort by canonical position
	type txWithPos struct {
		tx   int
		pos  int
	}
	
	sortedTxs := make([]txWithPos, 0, b)
	for tx := 0; tx < b; tx++ {
		sortedTxs = append(sortedTxs, txWithPos{tx: tx, pos: txCanonicalPos[tx]})
	}
	
	// Sort by canonical position (simple bubble sort for clarity)
	for i := 0; i < len(sortedTxs)-1; i++ {
		for j := i + 1; j < len(sortedTxs); j++ {
			if sortedTxs[i].pos > sortedTxs[j].pos {
				sortedTxs[i], sortedTxs[j] = sortedTxs[j], sortedTxs[i]
			}
		}
	}
	
	// Step 3: Geometric clustering
	hyperedges := make([][]int, 0)
	if len(sortedTxs) == 0 {
		return hyperedges
	}
	
	currentHyperedge := []int{sortedTxs[0].tx}
	
	for i := 1; i < len(sortedTxs); i++ {
		txPrev := sortedTxs[i-1].tx
		txCurr := sortedTxs[i].tx
		
		dist := txCanonicalPos[txCurr] - txCanonicalPos[txPrev]
		
		if dist < hg.delta {
			// Close distance, add to current hyperedge
			currentHyperedge = append(currentHyperedge, txCurr)
		} else {
			// Large distance, close current and start new hyperedge
			if len(currentHyperedge) > 0 {
				hyperedges = append(hyperedges, currentHyperedge)
			}
			currentHyperedge = []int{txCurr}
		}
	}
	
	// Add the last hyperedge
	if len(currentHyperedge) > 0 {
		hyperedges = append(hyperedges, currentHyperedge)
	}
	
	// Ensure all transactions are covered (single transactions as singleton hyperedges)
	clustered := make(map[int]bool)
	for _, he := range hyperedges {
		for _, tx := range he {
			clustered[tx] = true
		}
	}
	
	for tx := 0; tx < b; tx++ {
		if !clustered[tx] {
			hyperedges = append(hyperedges, []int{tx})
		}
	}
	
	return hyperedges
}