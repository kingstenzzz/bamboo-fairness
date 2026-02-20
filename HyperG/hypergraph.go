package HyperG

// buildHypergraph constructs the hypergraph with weights and dependencies
func (hg *HyperGSorter) buildHypergraph(prefMatrix [][]int, clusters [][]int) *Hypergraph {
	b := len(prefMatrix)
	if b == 0 {
		return &Hypergraph{
			Hyperedges:     []Hyperedge{},
			Dependencies:   []HypergraphDependency{},
			InternalOrders: make(map[int][]int),
		}
	}

	// Convert clusters to hyperedges
	hyperedges := make([]Hyperedge, 0)
	for _, cluster := range clusters {
		if len(cluster) > 0 {
			hyperedges = append(hyperedges, Hyperedge{
				Transactions: cluster,
				Weight:       0.0,
			})
		}
	}

	// Compute hyperedge weights (internal consensus)
	weights := make(map[int]float64)
	for idx, he := range hyperedges {
		s := len(he.Transactions)
		if s == 1 {
			weights[idx] = 1.0
			continue
		}

		// Calculate internal consensus
		totalPairs := 0
		agreedPairs := 0

		for i := 0; i < s; i++ {
			for j := i + 1; j < s; j++ {
				txI := he.Transactions[i]
				txJ := he.Transactions[j]

				if txI < b && txJ < b {
					minVotes := min(prefMatrix[txI][txJ], prefMatrix[txJ][txI])
					agreedPairs += minVotes
					totalPairs++
				}
			}
		}

		if totalPairs > 0 {
			weights[idx] = float64(agreedPairs) / float64(totalPairs)
		} else {
			weights[idx] = 0.0
		}
	}

	// Update weights in hyperedges
	for i := range hyperedges {
		if weight, exists := weights[i]; exists {
			hyperedges[i].Weight = weight
		}
	}

	// Compute inter-hyperedge dependencies
	deps := make([]HypergraphDependency, 0)
	H := len(hyperedges)

	for i := 0; i < H; i++ {
		hi := hyperedges[i].Transactions
		for j := i + 1; j < H; j++ {
			hj := hyperedges[j].Transactions

			// Calculate preference counts between hyperedges
			pij := 0 // preferences from hi to hj
			pji := 0 // preferences from hj to hi

			for _, txI := range hi {
				for _, txJ := range hj {
					if txI < b && txJ < b {
						pij += prefMatrix[txI][txJ]
						pji += prefMatrix[txJ][txI]
					}
				}
			}

			// Apply margin rule: 1.2x preference threshold
			margin := 1.2
			if float64(pij) > margin*float64(pji) {
				deps = append(deps, HypergraphDependency{
					Source: i,
					Target: j,
					Weight: pij,
				})
			} else if float64(pji) > margin*float64(pij) {
				deps = append(deps, HypergraphDependency{
					Source: j,
					Target: i,
					Weight: pji,
				})
			}
		}
	}

	// Compute internal ordering within each hyperedge
	internalOrders := make(map[int][]int)

	for idx, he := range hyperedges {
		if len(he.Transactions) == 1 {
			internalOrders[idx] = []int{he.Transactions[0]}
		} else {
			// Use net preference score for internal ordering
			netScores := make(map[int]int)
			for _, tx := range he.Transactions {
				netScores[tx] = 0
			}

			// Calculate net preference scores
			for i, txI := range he.Transactions {
				for j, txJ := range he.Transactions {
					if i != j && txI < b && txJ < b {
						netScore := prefMatrix[txI][txJ] - prefMatrix[txJ][txI]
						netScores[txI] += netScore
					}
				}
			}

			// Sort by net scores (descending)
			sortedTxs := make([]int, len(he.Transactions))
			copy(sortedTxs, he.Transactions)

			for i := 0; i < len(sortedTxs)-1; i++ {
				for j := i + 1; j < len(sortedTxs); j++ {
					if netScores[sortedTxs[i]] < netScores[sortedTxs[j]] {
						sortedTxs[i], sortedTxs[j] = sortedTxs[j], sortedTxs[i]
					}
				}
			}

			internalOrders[idx] = sortedTxs
		}
	}

	return &Hypergraph{
		Hyperedges:     hyperedges,
		Dependencies:   deps,
		InternalOrders: internalOrders,
	}
}

// extractFinalOrdering extracts the final transaction ordering from the hypergraph
func (hg *HyperGSorter) extractFinalOrdering(hypergraph *Hypergraph) []int {
	// Topologically sort hyperedges based on dependencies
	heOrder := hg.topologicalSortHyperedges(hypergraph)

	// Extract final ordering by concatenating internal orders
	finalOrder := make([]int, 0)

	for _, idx := range heOrder {
		if order, exists := hypergraph.InternalOrders[idx]; exists {
			finalOrder = append(finalOrder, order...)
		}
	}

	return finalOrder
}

// topologicalSortHyperedges performs topological sort on hyperedges
func (hg *HyperGSorter) topologicalSortHyperedges(hypergraph *Hypergraph) []int {
	H := len(hypergraph.Hyperedges)
	if H == 0 {
		return []int{}
	}

	// Build adjacency list and in-degrees
	adj := make([][]int, H)
	indeg := make([]int, H)

	// Initialize adjacency list
	for i := 0; i < H; i++ {
		adj[i] = make([]int, 0)
	}

	// Build graph from dependencies
	for _, dep := range hypergraph.Dependencies {
		adj[dep.Source] = append(adj[dep.Source], dep.Target)
		indeg[dep.Target]++
	}

	// Kahn's algorithm for topological sort
	queue := make([]int, 0)
	result := make([]int, 0)

	// Add nodes with zero in-degree
	for i := 0; i < H; i++ {
		if indeg[i] == 0 {
			queue = append(queue, i)
		}
	}

	// Process nodes
	for len(queue) > 0 {
		u := queue[0]
		queue = queue[1:]
		result = append(result, u)

		// Reduce in-degree of neighbors
		for _, v := range adj[u] {
			indeg[v]--
			if indeg[v] == 0 {
				queue = append(queue, v)
			}
		}
	}

	// Handle cycles by adding remaining nodes
	// In practice, hypergraph dependencies should form a DAG, but we handle cycles gracefully
	seen := make(map[int]bool)
	for _, node := range result {
		seen[node] = true
	}

	for i := 0; i < H; i++ {
		if !seen[i] {
			result = append(result, i)
		}
	}

	return result
}

// updateStats updates the statistics based on hypergraph results
func (hg *HyperGSorter) updateStats(hypergraph *Hypergraph) {
	if hypergraph == nil {
		return
	}

	hg.stats.NHyperedges = len(hypergraph.Hyperedges)

	if len(hypergraph.Hyperedges) > 0 {
		totalSize := 0
		maxSize := 0
		minSize := len(hypergraph.Hyperedges[0].Transactions)

		for _, he := range hypergraph.Hyperedges {
			size := len(he.Transactions)
			totalSize += size
			if size > maxSize {
				maxSize = size
			}
			if size < minSize {
				minSize = size
			}
		}

		hg.stats.AvgHyperedgeSize = float64(totalSize) / float64(len(hypergraph.Hyperedges))
		hg.stats.MaxHyperedgeSize = maxSize
		hg.stats.MinHyperedgeSize = minSize
	} else {
		hg.stats.AvgHyperedgeSize = 0.0
		hg.stats.MaxHyperedgeSize = 0
		hg.stats.MinHyperedgeSize = 0
	}
}
