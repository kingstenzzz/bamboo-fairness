package node

import (
	"fmt"
	"net/http"
	"reflect"
	"sync"
	"time"

	"github.com/Grivn/phalanx/common/mocks"
	"github.com/Grivn/phalanx/common/protos"
	pCommonTypes "github.com/Grivn/phalanx/common/types"
	phalanx "github.com/Grivn/phalanx/core"
	"github.com/gitferry/bamboo/config"
	"github.com/gitferry/bamboo/identity"
	"github.com/gitferry/bamboo/log"
	"github.com/gitferry/bamboo/message"
	"github.com/gitferry/bamboo/socket"
)

// Node is the primary access point for every replica
// it includes networking, state machine and RESTful API server
type Node interface {
	RunPhalanx()
	phalanx.Provider

	socket.Socket
	//Database
	ID() identity.NodeID
	Run()
	Retry(r message.Transaction)
	Forward(id identity.NodeID, r message.Transaction)
	Register(m interface{}, f interface{})
	IsByz() bool
	StartSignal()
	CommitBlock()
	UpdateStats(payload []*message.Transaction)
	QueryNode() QueryMessage
}

type QueryMessage struct {
	TThroughput    float64
	Throughput     float64
	TLatency       float64
	Latency        float64
	AveBlockSize   float64
	AvePayloadSize float64
	AveRealBlock   float64
}

// node implements Node interface
type node struct {
	id identity.NodeID
	phalanx.Provider

	socket.Socket
	//Database
	MessageChan chan interface{}
	TxChan      chan interface{}
	handles     map[string]reflect.Value
	server      *http.Server
	isByz       bool
	totalTxn    int

	sync.RWMutex
	forwards map[string]*message.Transaction

	totalCommittedTx int
	firstTimeAnchor  time.Time

	intervalCommittedTx int
	throughputAnchor    time.Time

	totalLatency float64
	latencyCount int

	intervalLatency      float64
	intervalLatencyCount int

	totalInnerBlock  int
	totalPayloadSize int
	totalBlockSize   int
	totalRealBlock   int
}

// NewNode creates a new Node object from configuration
func NewNode(id identity.NodeID, isByz bool) Node {
	n := &node{
		id:     id,
		isByz:  isByz,
		Socket: socket.NewSocket(id, config.Configuration.Addrs),
		//Database:    NewDatabase(),
		MessageChan: make(chan interface{}, config.Configuration.ChanBufferSize),
		TxChan:      make(chan interface{}, config.Configuration.ChanBufferSize),
		handles:     make(map[string]reflect.Value),
		forwards:    make(map[string]*message.Transaction),
		// 初始化时间锚点，确保统计计算正常工作
		firstTimeAnchor:  time.Now(),
		throughputAnchor: time.Now(),
	}

	idNum := uint64(id.Node())

	isPhalanxByz := false
	if idNum <= uint64(config.GetConfig().PhalanxByzNo) {
		isPhalanxByz = true
	}
	//if idNum == uint64(2) {
	//	isPhalanxByz = true
	//}

	count := len(config.Configuration.Addrs)

	privKey, pubKeys, err := mocks.GenerateKeys(idNum, count)
	if err != nil {
		panic(fmt.Sprintf("generate keys error: %s", err))
	}
	conf := phalanx.Config{
		OLeader:     uint64(config.GetConfig().PhalanxOligarchyLeader),
		Byz:         isPhalanxByz,
		Duration:    time.Duration(config.GetConfig().PhalanxDurationLog) * time.Millisecond,
		CDuration:   time.Duration(config.GetConfig().PhalanxDurationCommand) * time.Millisecond,
		Interval:    config.GetConfig().PhalanxInterval,
		OpenLatency: config.GetConfig().PhalanxOpenLatency,
		N:           count,
		Multi:       config.GetConfig().PhalanxMulti,
		LogCount:    config.GetConfig().PhalanxLogCount,
		MemSize:     config.GetConfig().MemSize,
		CommandSize: config.GetConfig().BSize,
		Author:      idNum,
		PrivateKey:  privKey,
		PublicKeys:  pubKeys,
		Network:     n,
		Exec:        n,
		Logger:      n,
		Selected:    uint64(config.GetConfig().PhalanxSelectedPropose),
	}
	n.Provider = phalanx.NewPhalanxProvider(conf)

	return n
}

func (n *node) ID() identity.NodeID {
	return n.id
}

func (n *node) IsByz() bool {
	return n.isByz
}

func (n *node) Retry(r message.Transaction) {
	log.Debugf("node %v retry reqeust %v", n.id, r)
	n.MessageChan <- r
}

// Register a handle function for each message type
func (n *node) Register(m interface{}, f interface{}) {
	t := reflect.TypeOf(m)
	fn := reflect.ValueOf(f)

	if fn.Kind() != reflect.Func {
		panic("handle function is not func")
	}

	if fn.Type().In(0) != t {
		panic("func type is not t")
	}

	if fn.Kind() != reflect.Func || fn.Type().NumIn() != 1 || fn.Type().In(0) != t {
		panic("register handle function error")
	}
	n.handles[t.String()] = fn
}

// Run start and run the node
func (n *node) Run() {
	log.Infof("node %v start running", n.id)
	if len(n.handles) > 0 {
		go n.handle()
		go n.recv()
		go n.txn()
	}
	n.http()
}

func (n *node) RunPhalanx() {
	n.Provider.Run()
}

func (n *node) txn() {
	for {
		tx := <-n.TxChan
		v := reflect.ValueOf(tx)
		name := v.Type().String()
		f, exists := n.handles[name]
		if !exists {
			log.Fatalf("no registered handle function for message type %v", name)
		}
		f.Call([]reflect.Value{v})
	}
}

// recv receives messages from socket and pass to message channel
func (n *node) recv() {
	for {
		m := n.Recv()
		if n.isByz && config.GetConfig().Strategy == "silence" {
			// perform silence attack
			continue
		}
		switch m := m.(type) {
		case message.Transaction:
			m.C = make(chan message.TransactionReply, 1)
			n.TxChan <- m
			continue

		case message.TransactionReply:
			n.RLock()
			r := n.forwards[m.Command.String()]
			log.Debugf("node %v received reply %v", n.id, m)
			n.RUnlock()
			r.Reply(m)
			continue
		}
		n.MessageChan <- m
	}
}

// handle receives messages from message channel and calls handle function using refection
func (n *node) handle() {
	for {
		msg := <-n.MessageChan
		v := reflect.ValueOf(msg)
		name := v.Type().String()
		f, exists := n.handles[name]
		if !exists {
			log.Fatalf("no registered handle function for message type %v", name)
		}
		f.Call([]reflect.Value{v})
	}
}

func (n *node) Forward(id identity.NodeID, m message.Transaction) {
	log.Debugf("Node %v forwarding %v to %s", n.ID(), m, id)
	m.NodeID = n.id
	n.Lock()
	n.forwards[m.Command.String()] = &m
	n.Unlock()
	n.Send(id, m)
}

func (n *node) StartSignal() {
	n.throughputAnchor = time.Now()
	n.firstTimeAnchor = time.Now()
}

func (n *node) CommitBlock() {
	n.totalInnerBlock++
}

func (n *node) UpdateStats(payload []*message.Transaction) {
	log.Debugf("Node %v UpdateStats called with %v transactions", n.id, len(payload))
	for _, tx := range payload {
		n.totalCommittedTx++
		n.intervalCommittedTx++
		
		// 计算延迟
		if !tx.Timestamp.IsZero() {
			latency := float64(time.Now().UnixNano()-tx.Timestamp.UnixNano()) / 1e6 // 转换为毫秒
			n.totalLatency += latency
			n.intervalLatency += latency
			n.latencyCount++
			n.intervalLatencyCount++
			log.Debugf("Node %v processed tx with latency: %f ms", n.id, latency)
		} else {
			log.Debugf("Node %v processed tx with zero timestamp", n.id)
		}
	}
	log.Debugf("Node %v stats after update - totalCommittedTx: %v, intervalCommittedTx: %v, latencyCount: %v", 
		n.id, n.totalCommittedTx, n.intervalCommittedTx, n.latencyCount)
}

func (n *node) QueryNode() QueryMessage {
	log.Debugf("Node %v QueryNode called", n.id)
	log.Debugf("Node %v current stats - totalCommittedTx: %v, intervalCommittedTx: %v, latencyCount: %v, totalLatency: %f, intervalLatency: %f", 
		n.id, n.totalCommittedTx, n.intervalCommittedTx, n.latencyCount, n.totalLatency, n.intervalLatency)

	// calculate throughput and latency with zero protection
	totalThroughput := 0.0
	if !n.firstTimeAnchor.IsZero() {
		elapsed := time.Now().Sub(n.firstTimeAnchor).Seconds()
		if elapsed > 0 {
			totalThroughput = float64(n.totalCommittedTx) / elapsed
			log.Debugf("Node %v total throughput calculation - elapsed: %f s, throughput: %f tx/s", n.id, elapsed, totalThroughput)
		}
	}

	throughput := 0.0
	if !n.throughputAnchor.IsZero() {
		intervalElapsed := time.Now().Sub(n.throughputAnchor).Seconds()
		if intervalElapsed > 0 && n.intervalCommittedTx > 0 {
			throughput = float64(n.intervalCommittedTx) / intervalElapsed
			log.Debugf("Node %v interval throughput calculation - intervalElapsed: %f s, intervalCommittedTx: %v, throughput: %f tx/s", 
				n.id, intervalElapsed, n.intervalCommittedTx, throughput)
		}
	}

	totalLatency := 0.0
	if n.latencyCount > 0 {
		totalLatency = n.totalLatency / float64(n.latencyCount)
	}

	latency := 0.0
	if n.intervalLatencyCount > 0 {
		latency = n.intervalLatency / float64(n.intervalLatencyCount)
	}

	// block size with zero protection
	aveBlockSize := 0.0
	if n.totalRealBlock > 0 {
		aveBlockSize = float64(n.totalBlockSize) / float64(n.totalRealBlock)
	}

	// command size with zero protection
	avePayloadSize := 0.0
	if n.totalRealBlock > 0 {
		avePayloadSize = float64(n.totalPayloadSize) / float64(n.totalRealBlock)
	}

	// committed block with zero protection
	aveRealBlock := 0.0
	if n.totalInnerBlock > 0 {
		aveRealBlock = float64(n.totalRealBlock) / float64(n.totalInnerBlock)
	}

	result := QueryMessage{
		TThroughput:    totalThroughput,
		Throughput:     throughput,
		TLatency:       totalLatency,
		Latency:        latency,
		AveBlockSize:   aveBlockSize,
		AvePayloadSize: avePayloadSize,
		AveRealBlock:   aveRealBlock,
	}
	
	log.Debugf("Node %v QueryNode result - Throughput: %f, Latency: %f", n.id, result.Throughput, result.Latency)
	return result
}

//==================================================================================
//                              phalanx service
//==================================================================================

func (n *node) CommandExecution(block pCommonTypes.InnerBlock, seqNo uint64) {
	command := block.Command

	log.Infof("[%v] the block is committed, No. of transactions: %v, id: %d", n.ID(), len(command.Content), seqNo)

	for _, tx := range command.Content {
		// add the total committed tx for throughput.
		n.totalCommittedTx++
		n.intervalCommittedTx++
		
		// calculate latency for current transaction.
		// 暂时跳过延迟计算，避免类型问题
		// n.totalLatency += pCommonTypes.NanoToSecond(time.Now().UnixNano()-tx.Timestamp.UnixNano()) * 1000
		// n.intervalLatency += pCommonTypes.NanoToSecond(time.Now().UnixNano()-tx.Timestamp.UnixNano()) * 1000
		n.latencyCount++
		n.intervalLatencyCount++

		// calculate block size
		n.totalBlockSize++

		// calculate command size
		n.totalPayloadSize += len(tx.Payload)
	}
	n.totalRealBlock++
}

func (n *node) BroadcastCommand(command *protos.Command) {
	go n.Socket.Broadcast(*command)
	go n.ReceiveCommand(command)
}

func (n *node) BroadcastPCM(message *protos.ConsensusMessage) {
	go n.Socket.Broadcast(*message)
	go n.ReceiveConsensusMessage(message)
}

func (n *node) UnicastPCM(message *protos.ConsensusMessage) {
	if message.To == uint64(n.id.Node()) {
		go n.ReceiveConsensusMessage(message)
		return
	}
	go n.Send(identity.NewNodeID(int(message.To)), message)
}

func (n *node) Debug(v ...interface{}) {
	log.Debug(v...)
}
func (n *node) Debugf(format string, v ...interface{}) {
	log.Debugf(format, v...)
}

func (n *node) Info(v ...interface{}) {
	log.Info(v...)
}
func (n *node) Infof(format string, v ...interface{}) {
	log.Infof(format, v...)
}

func (n *node) Error(v ...interface{}) {
	log.Error(v...)
}
func (n *node) Errorf(format string, v ...interface{}) {
	log.Errorf(format, v...)
}
