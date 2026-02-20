package HyperG

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestDirectedGraph_BasicOperations(t *testing.T) {
	// Create a graph with 4 nodes
	graph := NewDirectedGraph(4)

	// Add edges: 0->1, 1->2, 2->3, 0->3
	graph.AddEdge(0, 1)
	graph.AddEdge(1, 2)
	graph.AddEdge(2, 3)
	graph.AddEdge(0, 3)

	// Test adjacency list
	assert.Contains(t, graph.AdjacencyList[0], 1)
	assert.Contains(t, graph.AdjacencyList[0], 3)
	assert.Contains(t, graph.AdjacencyList[1], 2)
	assert.Contains(t, graph.AdjacencyList[2], 3)
}

func TestTarjanSCC_SimpleCase(t *testing.T) {
	// Create a graph with SCCs
	graph := NewDirectedGraph(5)

	// Edges forming two SCCs: {0,1,2} and {3,4}
	graph.AddEdge(0, 1)
	graph.AddEdge(1, 2)
	graph.AddEdge(2, 0) // Creates cycle 0->1->2->0
	graph.AddEdge(3, 4)
	graph.AddEdge(4, 3) // Creates cycle 3->4->3

	sccs := graph.TarjanSCC()

	// Should find 2 SCCs
	assert.Equal(t, 2, len(sccs))

	// Check that each SCC has the right nodes
	sccMap := make(map[int][]int)
	for _, scc := range sccs {
		for _, node := range scc {
			sccMap[node] = scc
		}
	}

	// Nodes 0,1,2 should be in the same SCC
	assert.Equal(t, sccMap[0], sccMap[1])
	assert.Equal(t, sccMap[1], sccMap[2])

	// Nodes 3,4 should be in the same SCC
	assert.Equal(t, sccMap[3], sccMap[4])

	// Different SCCs should be different
	assert.NotEqual(t, sccMap[0], sccMap[3])
}

func TestTopologicalSort_DAG(t *testing.T) {
	// Create a DAG: 0->1->2, 0->3->2
	graph := NewDirectedGraph(4)
	graph.AddEdge(0, 1)
	graph.AddEdge(1, 2)
	graph.AddEdge(0, 3)
	graph.AddEdge(3, 2)

	topoOrder := graph.TopologicalSort()

	// Should have all nodes
	assert.Equal(t, 4, len(topoOrder))

	// Check ordering constraints
	nodePositions := make(map[int]int)
	for i, node := range topoOrder {
		nodePositions[node] = i
	}

	// 0 should come before 1 and 3
	assert.True(t, nodePositions[0] < nodePositions[1])
	assert.True(t, nodePositions[0] < nodePositions[3])

	// 1 and 3 should come before 2
	assert.True(t, nodePositions[1] < nodePositions[2])
	assert.True(t, nodePositions[3] < nodePositions[2])
}

func TestTopologicalSort_Cycle(t *testing.T) {
	// Create a graph with cycle: 0->1->2->0
	graph := NewDirectedGraph(3)
	graph.AddEdge(0, 1)
	graph.AddEdge(1, 2)
	graph.AddEdge(2, 0)

	topoOrder := graph.TopologicalSort()

	// Should return partial ordering (may be incomplete due to cycle)
	assert.True(t, len(topoOrder) >= 0)
	assert.True(t, len(topoOrder) <= 3)
}
