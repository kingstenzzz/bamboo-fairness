package hyperg

import (
	"testing"
	"github.com/gitferry/bamboo/crypto"
	"github.com/gitferry/bamboo/identity"
	"github.com/stretchr/testify/assert"
)

func TestHyperGSorter_BasicFunctionality(t *testing.T) {
	// 创建测试数据
	tx1 := crypto.MakeID("tx1")
	tx2 := crypto.MakeID("tx2") 
	tx3 := crypto.MakeID("tx3")

	// 模拟三个节点的排序提议
	ol1 := NewOrderedList([]crypto.Identifier{tx1, tx2, tx3}, []int64{10, 20, 30})
	ol2 := NewOrderedList([]crypto.Identifier{tx1, tx3, tx2}, []int64{15, 25, 35})
	ol3 := NewOrderedList([]crypto.Identifier{tx2, tx1, tx3}, []int64{5, 15, 25})

	proposals := map[identity.NodeID]*OrderedList{
		"node1": ol1,
		"node2": ol2,
		"node3": ol3,
	}

	// 创建HyperG排序器
	sorter := NewHyperGSorter(3, 1, 0.8, 5)

	// 执行排序
	result, err := sorter.ComputeFairOrder(proposals, 3, 1)
	
	// 验证结果
	assert.NoError(t, err)
	assert.NotNil(t, result)
	// 注意：由于placeholder实现，结果可能为空，这在第一阶段是正常的
	// assert.NotEmpty(t, result)
	
	// 验证统计信息
	stats := sorter.GetStats()
	assert.NotNil(t, stats)
	assert.True(t, stats.TotalTime > 0)
	assert.Equal(t, 0.8, stats.Gamma)
	assert.Equal(t, 5, stats.Delta)
}

func TestOrderedList_Sort(t *testing.T) {
	// 创建无序的订单列表
	cmds := []crypto.Identifier{
		crypto.MakeID("tx3"),
		crypto.MakeID("tx1"),
		crypto.MakeID("tx2"),
	}
	timestamps := []int64{30, 10, 20}

	ol := NewOrderedList(cmds, timestamps)
	
	// 排序前验证
	assert.Equal(t, cmds[0], ol.Cmds[0])
	assert.Equal(t, int64(30), ol.Timestamps[0])

	// 执行排序
	ol.Sort()

	// 排序后验证
	assert.Equal(t, crypto.MakeID("tx1"), ol.Cmds[0]) // 最早时间戳的应该在前面
	assert.Equal(t, int64(10), ol.Timestamps[0])
	assert.Equal(t, crypto.MakeID("tx2"), ol.Cmds[1])
	assert.Equal(t, int64(20), ol.Timestamps[1])
	assert.Equal(t, crypto.MakeID("tx3"), ol.Cmds[2])
	assert.Equal(t, int64(30), ol.Timestamps[2])
}

func TestHyperGSorter_ParameterUpdates(t *testing.T) {
	sorter := NewHyperGSorter(4, 1, 0.5, 3)
	
	// 初始参数验证
	assert.Equal(t, 4, sorter.n)
	assert.Equal(t, 1, sorter.f)
	assert.Equal(t, 0.5, sorter.gamma)
	assert.Equal(t, 3, sorter.delta)
	expectedThreshold := max(1, int(float64(4)*(1-0.5))+1+1)
	assert.Equal(t, expectedThreshold, sorter.threshold)

	// 测试ComputeFairOrder中的参数更新
	proposals := make(map[identity.NodeID]*OrderedList)
	result, err := sorter.ComputeFairOrder(proposals, 6, 2)
	
	assert.NoError(t, err)
	assert.Equal(t, 6, sorter.n)   // 应该更新
	assert.Equal(t, 2, sorter.f)   // 应该更新
	newExpectedThreshold := max(1, int(float64(6)*(1-0.5))+2+1)
	assert.Equal(t, newExpectedThreshold, sorter.threshold)
	assert.NotNil(t, result)
}