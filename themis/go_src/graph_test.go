package themis

import (
	"testing"

	"github.com/gitferry/bamboo/crypto"
	"github.com/stretchr/testify/assert"
)

func TestTopologyGraph_Simple(t *testing.T) {
	tx1 := crypto.MakeID("tx1")
	tx2 := crypto.MakeID("tx2")
	tx3 := crypto.MakeID("tx3")
	nodes := []crypto.Identifier{tx1, tx2, tx3}

	g := NewTopologyGraph(nodes)
	g.AddEdge(tx1, tx2)
	g.AddEdge(tx2, tx3)

	result := g.Finalize()
	assert.Equal(t, 3, len(result))
	assert.Equal(t, tx1, result[0])
	assert.Equal(t, tx2, result[1])
	assert.Equal(t, tx3, result[2])
}

func TestTopologyGraph_SCC(t *testing.T) {
	tx1 := crypto.MakeID("tx1")
	tx2 := crypto.MakeID("tx2")
	tx3 := crypto.MakeID("tx3")
	nodes := []crypto.Identifier{tx1, tx2, tx3}

	// Create a cycle tx1 -> tx2 -> tx1, and tx2 -> tx3
	g := NewTopologyGraph(nodes)
	g.AddEdge(tx1, tx2)
	g.AddEdge(tx2, tx1)
	g.AddEdge(tx2, tx3)

	result := g.Finalize()
	assert.Equal(t, 3, len(result))
	// tx3 must be after tx1 and tx2 because of tx2 -> tx3 and SCC(tx1, tx2)
	assert.Equal(t, tx3, result[2])
}
