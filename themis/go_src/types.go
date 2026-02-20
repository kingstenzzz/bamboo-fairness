package themis

import (
	"sort"

	"github.com/gitferry/bamboo/crypto"
)

type OrderedList struct {
	Cmds       []crypto.Identifier
	Timestamps []int64
}

func NewOrderedList(cmds []crypto.Identifier, timestamps []int64) *OrderedList {
	return &OrderedList{
		Cmds:       cmds,
		Timestamps: timestamps,
	}
}

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
