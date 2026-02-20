package HyperG

import (
	"github.com/gitferry/bamboo/identity"
	"time"
)

// HyperGSorter implements the HyperOrder-X Phase 1 algorithm
type HyperGSorter struct {
	n         int     // total number of nodes
	f         int     // number of byzantine nodes
	gamma     float64 // fairness parameter
	delta     int     // clustering distance threshold
	threshold int     // voting threshold
	stats     *HyperGStats
}

// NewHyperGSorter creates a new HyperG sorter instance
func NewHyperGSorter(n, f int, gamma float64, delta int) *HyperGSorter {
	threshold := max(1, int(float64(n)*(1-gamma))+f+1)
	return &HyperGSorter{
		n:         n,
		f:         f,
		gamma:     gamma,
		delta:     delta,
		threshold: threshold,
		stats:     &HyperGStats{},
	}
}

// ComputeFairOrder computes the fair transaction ordering using HyperG algorithm
func (hg *HyperGSorter) ComputeFairOrder(proposals map[identity.NodeID]*OrderedList, n, f int) ([]int, error) {
	startTotal := time.Now()

	// Update parameters if provided
	if n > 0 {
		hg.n = n
	}
	if f >= 0 {
		hg.f = f
	}
	hg.threshold = max(1, int(float64(hg.n)*(1-hg.gamma))+hg.f+1)

	// 1. Compute pairwise preferences
	t0 := time.Now()
	prefMatrix := hg.computePairwisePreferences(proposals)
	hg.stats.PrefMatrixTime = time.Since(t0).Seconds()

	// 2. Detect Condorcet clusters
	t0 = time.Now()
	clusters := hg.detectCondorcetClusters(proposals, prefMatrix)
	hg.stats.ClusteringTime = time.Since(t0).Seconds()

	// 3. Build hypergraph
	t0 = time.Now()
	hypergraph := hg.buildHypergraph(prefMatrix, clusters)
	hg.stats.HypergraphTime = time.Since(t0).Seconds()

	// 4. Extract final ordering
	t0 = time.Now()
	finalOrderIndices := hg.extractFinalOrdering(hypergraph)
	hg.stats.ExtractionTime = time.Since(t0).Seconds()

	// Convert indices back to identifiers (placeholder implementation)
	finalOrder := make([]int, len(finalOrderIndices))
	copy(finalOrder, finalOrderIndices)

	hg.stats.TotalTime = time.Since(startTotal).Seconds()
	hg.stats.Gamma = hg.gamma
	hg.stats.Delta = hg.delta
	hg.stats.Threshold = hg.threshold

	// Update statistics
	hg.updateStats(hypergraph)

	return finalOrder, nil
}

// GetStats returns the algorithm execution statistics
func (hg *HyperGSorter) GetStats() *HyperGStats {
	return hg.stats
}

// Helper functions
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
