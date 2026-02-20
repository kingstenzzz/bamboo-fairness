package hyperg

// Hyperedge represents a hyperedge in the hypergraph
type Hyperedge struct {
	Transactions []int   // Transaction indices
	Weight       float64 // Internal consensus weight
}

// HypergraphDependency represents a dependency between hyperedges
type HypergraphDependency struct {
	Source int // Source hyperedge index
	Target int // Target hyperedge index
	Weight int // Dependency weight
}

// Hypergraph represents the complete hypergraph structure
type Hypergraph struct {
	Hyperedges     []Hyperedge
	Dependencies   []HypergraphDependency
	InternalOrders map[int][]int // Internal ordering within each hyperedge
}

// StronglyConnectedComponent represents a strongly connected component in a graph
type StronglyConnectedComponent struct {
	Nodes []int
}

// DirectedGraph represents a directed graph for SCC detection
type DirectedGraph struct {
	AdjacencyList [][]int
	NodeCount     int
}

// NewDirectedGraph creates a new directed graph
func NewDirectedGraph(nodeCount int) *DirectedGraph {
	graph := &DirectedGraph{
		NodeCount: nodeCount,
	}
	graph.AdjacencyList = make([][]int, nodeCount)
	for i := range graph.AdjacencyList {
		graph.AdjacencyList[i] = make([]int, 0)
	}
	return graph
}

// AddEdge adds a directed edge from source to target
func (g *DirectedGraph) AddEdge(source, target int) {
	if source >= 0 && source < g.NodeCount && target >= 0 && target < g.NodeCount {
		g.AdjacencyList[source] = append(g.AdjacencyList[source], target)
	}
}

// TarjanSCC implements Tarjan's algorithm for finding strongly connected components
func (g *DirectedGraph) TarjanSCC() [][]int {
	disc := make([]int, g.NodeCount)      // Discovery times
	low := make([]int, g.NodeCount)       // Low values
	onStack := make([]bool, g.NodeCount)  // Nodes currently on stack
	stack := make([]int, 0)               // Stack for DFS
	time := 0                             // Time counter
	sccs := make([][]int, 0)              // Result SCCs

	// Initialize arrays
	for i := 0; i < g.NodeCount; i++ {
		disc[i] = -1
		low[i] = -1
		onStack[i] = false
	}

	// Recursive helper function
	var tarjanUtil func(int)
	tarjanUtil = func(u int) {
		disc[u] = time
		low[u] = time
		time++
		stack = append(stack, u)
		onStack[u] = true

		// Visit all neighbors
		for _, v := range g.AdjacencyList[u] {
			if disc[v] == -1 {
				// Unvisited node
				tarjanUtil(v)
				low[u] = min(low[u], low[v])
			} else if onStack[v] {
				// Back edge to ancestor
				low[u] = min(low[u], disc[v])
			}
		}

		// If u is root of SCC
		if low[u] == disc[u] {
			scc := make([]int, 0)
			for {
				v := stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				onStack[v] = false
				scc = append(scc, v)
				if v == u {
					break
				}
			}
			if len(scc) > 0 {
				sccs = append(sccs, scc)
			}
		}
	}

	// Call helper for all unvisited nodes
	for i := 0; i < g.NodeCount; i++ {
		if disc[i] == -1 {
			tarjanUtil(i)
		}
	}

	return sccs
}

// TopologicalSort performs topological sort on a DAG
func (g *DirectedGraph) TopologicalSort() []int {
	inDegree := make([]int, g.NodeCount)
	
	// Calculate in-degrees
	for u := 0; u < g.NodeCount; u++ {
		for _, v := range g.AdjacencyList[u] {
			inDegree[v]++
		}
	}
	
	// Queue for nodes with zero in-degree
	queue := make([]int, 0)
	result := make([]int, 0)
	
	// Add all nodes with zero in-degree
	for i := 0; i < g.NodeCount; i++ {
		if inDegree[i] == 0 {
			queue = append(queue, i)
		}
	}
	
	// Process nodes
	for len(queue) > 0 {
		u := queue[0]
		queue = queue[1:]
		result = append(result, u)
		
		// Reduce in-degree of neighbors
		for _, v := range g.AdjacencyList[u] {
			inDegree[v]--
			if inDegree[v] == 0 {
				queue = append(queue, v)
			}
		}
	}
	
	// Check for cycles
	if len(result) != g.NodeCount {
		// Graph has cycle, return partial result
		return result
	}
	
	return result
}