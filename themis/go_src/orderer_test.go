package themis

import (
	"testing"

	"github.com/gitferry/bamboo/crypto"
	"github.com/gitferry/bamboo/identity"
	"github.com/stretchr/testify/assert"
)

func TestFairOrderer_ComputeFairOrder(t *testing.T) {
	tx1 := crypto.MakeID("tx1")
	tx2 := crypto.MakeID("tx2")
	tx3 := crypto.MakeID("tx3")

	// 副本 1 提议: tx1, tx2, tx3
	ol1 := NewOrderedList([]crypto.Identifier{tx1, tx2, tx3}, []int64{10, 20, 30})
	// 副本 2 提议: tx2, tx1, tx3
	ol2 := NewOrderedList([]crypto.Identifier{tx2, tx1, tx3}, []int64{15, 25, 35})
	// 副本 3 提议: tx1, tx2, tx3
	ol3 := NewOrderedList([]crypto.Identifier{tx1, tx2, tx3}, []int64{5, 15, 25})

	proposals := map[identity.NodeID]*OrderedList{
		"node1": ol1,
		"node2": ol2,
		"node3": ol3,
	}

	fo := NewFairOrderer()
	// n=3, f=1. 需要至少 f+1=2 个副本同意顺序
	result, err := fo.ComputeFairOrder(proposals, 3, 1)
	assert.NoError(t, err)
	assert.Equal(t, 3, len(result))

	// tx1 在 tx2 之前的有: node1, node3 (2个)
	// tx2 在 tx1 之前的有: node2 (1个)
	// 所以 tx1 应该在 tx2 之前

	// tx2 在 tx3 之前的有: node1, node2, node3 (3个)

	assert.Equal(t, tx1, result[0])
	assert.Equal(t, tx2, result[1])
	assert.Equal(t, tx3, result[2])
}
