package themis

import (
	"github.com/gitferry/bamboo/crypto"
	"github.com/gitferry/bamboo/identity"
)

type FairOrderer struct {
}

func NewFairOrderer() *FairOrderer {
	return &FairOrderer{}
}

func (fo *FairOrderer) ComputeFairOrder(proposals map[identity.NodeID]*OrderedList, n int, f int) ([]crypto.Identifier, error) {
	// 1. 统计所有交易及其出现次数
	allTxMap := make(map[crypto.Identifier]int)
	for _, ol := range proposals {
		ol.Sort()
		for _, tx := range ol.Cmds {
			allTxMap[tx]++
		}
	}

	// 2. 识别 Solid 交易 (在至少 f+1 个副本中出现)
	var solidTxs []crypto.Identifier
	for tx, count := range allTxMap {
		if count >= f+1 {
			solidTxs = append(solidTxs, tx)
		}
	}

	if len(solidTxs) == 0 {
		return nil, nil
	}

	// 3. 构建拓扑图
	g := NewTopologyGraph(solidTxs)

	// 计算每对交易之间的顺序关系
	for i := 0; i < len(solidTxs); i++ {
		for j := i + 1; j < len(solidTxs); j++ {
			txA := solidTxs[i]
			txB := solidTxs[j]

			countAB := 0 // A before B
			countBA := 0 // B before A

			for _, ol := range proposals {
				posA := -1
				posB := -1
				for p, tx := range ol.Cmds {
					if tx == txA {
						posA = p
					}
					if tx == txB {
						posB = p
					}
				}

				if posA != -1 && posB != -1 {
					if posA < posB {
						countAB++
					} else {
						countBA++
					}
				}
			}

			// 如果至少有 f+1 个副本认为 A 在 B 之前
			if countAB >= f+1 {
				g.AddEdge(txA, txB)
			}
			if countBA >= f+1 {
				g.AddEdge(txB, txA)
			}
		}
	}

	// 4. 计算最终排序
	return g.Finalize(), nil
}
