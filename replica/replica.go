package replica

import (
	"encoding/gob"
	"fmt"
	"sort"
	"time"

	pCommonProto "github.com/Grivn/phalanx/common/protos"
	pCommonTypes "github.com/Grivn/phalanx/common/types"
	fhs "github.com/gitferry/bamboo/fasthostuff"
	"github.com/gitferry/bamboo/lbft"

	"go.uber.org/atomic"

	"github.com/gitferry/bamboo/HyperG"
	"github.com/gitferry/bamboo/blockchain"
	"github.com/gitferry/bamboo/config"
	"github.com/gitferry/bamboo/crypto"
	"github.com/gitferry/bamboo/election"
	"github.com/gitferry/bamboo/hotstuff"
	"github.com/gitferry/bamboo/identity"
	"github.com/gitferry/bamboo/log"
	"github.com/gitferry/bamboo/mempool"
	"github.com/gitferry/bamboo/message"
	"github.com/gitferry/bamboo/node"
	"github.com/gitferry/bamboo/pacemaker"
	"github.com/gitferry/bamboo/streamlet"
	"github.com/gitferry/bamboo/tchs"
	"github.com/gitferry/bamboo/themis/go_src"
	"github.com/gitferry/bamboo/types"
)

type Replica struct {
	node.Node
	Safety
	election.Election

	openPhalanx   bool
	themis        bool
	fo            *themis.FairOrderer
	themisProps   map[types.View]map[identity.NodeID]*themis.OrderedList
	proposedViews map[types.View]bool
	hyperg        bool
	hypergSorter  *HyperG.HyperGSorter
	HyperGProps   map[types.View]map[identity.NodeID]*HyperG.OrderedList

	pd              *mempool.Producer
	pm              *pacemaker.Pacemaker
	start           chan bool // signal to start the node
	isStarted       atomic.Bool
	isByz           bool
	timer           *time.Timer // timeout for each view
	committedBlocks chan *blockchain.Block
	forkedBlocks    chan *blockchain.Block
	eventChan       chan interface{}

	/* for monitoring node statistics */
	thrus                string
	lastViewTime         time.Time
	startTime            time.Time
	tmpTime              time.Time
	voteStart            time.Time
	totalCreateDuration  time.Duration
	totalProcessDuration time.Duration
	totalProposeDuration time.Duration
	totalDelay           time.Duration
	totalRoundTime       time.Duration
	totalVoteTime        time.Duration
	totalBlockSize       int
	receivedNo           int
	roundNo              int
	voteNo               int
	totalCommittedTx     int
	latencyNo            int
	proposedNo           int
	processedNo          int
	committedNo          int

	preTotalSafe int
	preTotalRisk int

	preTotalSafeM int
	preTotalRiskM int

	preTotalSafeTA int
	preTotalRiskTA int
}

// NewReplica creates a new replica instance
func NewReplica(id identity.NodeID, alg string, isByz bool) *Replica {
	r := new(Replica)
	r.Node = node.NewNode(id, isByz)
	if isByz {
		log.Infof("[%v] is Byzantine", r.ID())
	}
	if config.GetConfig().Master == "0" {
		r.Election = election.NewRotation(config.GetConfig().N())
	} else {
		r.Election = election.NewStatic(config.GetConfig().Master)
	}
	r.isByz = isByz
	r.pd = mempool.NewProducer()
	r.pm = pacemaker.NewPacemaker(config.GetConfig().N())
	r.start = make(chan bool)
	r.eventChan = make(chan interface{}, 1024)
	r.committedBlocks = make(chan *blockchain.Block, 100)
	r.forkedBlocks = make(chan *blockchain.Block, 100)
	r.Register(blockchain.Block{}, r.HandleBlock)
	r.Register(blockchain.Vote{}, r.HandleVote)
	r.Register(pacemaker.TMO{}, r.HandleTmo)
	r.Register(message.Transaction{}, r.handleTxn)
	r.Register(message.Query{}, r.handleQuery)
	r.Register(message.ThemisProposal{}, r.HandleThemisProposal)
	r.Register(message.HyperGProposal{}, r.HandleHyperGProposal)
	r.Register(pCommonProto.ConsensusMessage{}, r.HandleConsensusMessage)
	r.Register(pCommonProto.Command{}, r.HandleCommand)
	gob.Register(blockchain.Block{})
	gob.Register(blockchain.Vote{})
	gob.Register(pacemaker.TC{})
	gob.Register(pacemaker.TMO{})
	gob.Register(message.ThemisProposal{})
	gob.Register(message.HyperGProposal{})
	gob.Register(pCommonProto.ConsensusMessage{})
	gob.Register(pCommonProto.Command{})

	// 根据 phalanx_multi 值决定是否开启 Phalanx
	r.openPhalanx = config.GetConfig().PhalanxMulti > 0
	r.themis = config.GetConfig().Themis.Enabled
	if r.themis {
		r.fo = themis.NewFairOrderer()
		r.themisProps = make(map[types.View]map[identity.NodeID]*themis.OrderedList)
		r.proposedViews = make(map[types.View]bool)
	}

	r.hyperg = config.GetConfig().HyperG.Enabled
	if r.hyperg {
		r.hypergSorter = HyperG.NewHyperGSorter(
			config.GetConfig().N(),
			config.GetConfig().ByzNo,
			config.GetConfig().HyperG.Gamma,
			config.GetConfig().HyperG.Delta,
		)
		r.HyperGProps = make(map[types.View]map[identity.NodeID]*HyperG.OrderedList)
	}

	// Is there a better way to reduce the number of parameters?
	switch alg {
	case "hotstuff":
		r.Safety = hotstuff.NewHotStuff(r.Node, r.pm, r.Election, r.committedBlocks, r.forkedBlocks)
	case "tchs":
		r.Safety = tchs.NewTchs(r.Node, r.pm, r.Election, r.committedBlocks, r.forkedBlocks)
	case "streamlet":
		r.Safety = streamlet.NewStreamlet(r.Node, r.pm, r.Election, r.committedBlocks, r.forkedBlocks)
	case "lbft":
		r.Safety = lbft.NewLbft(r.Node, r.pm, r.Election, r.committedBlocks, r.forkedBlocks)
	case "fasthotstuff":
		r.Safety = fhs.NewFhs(r.Node, r.pm, r.Election, r.committedBlocks, r.forkedBlocks)
	default:
		r.Safety = hotstuff.NewHotStuff(r.Node, r.pm, r.Election, r.committedBlocks, r.forkedBlocks)
	}
	return r
}

/* Message Handlers */

func (r *Replica) HandleBlock(block blockchain.Block) {
	r.receivedNo++
	r.startSignal()
	log.Debugf("[%v] received a block from %v, view is %v, id: %x, prevID: %x", r.ID(), block.Proposer, block.View, block.ID, block.PrevID)
	r.eventChan <- block
}

func (r *Replica) HandleVote(vote blockchain.Vote) {
	if vote.View < r.pm.GetCurView() {
		return
	}
	r.startSignal()
	log.Debugf("[%v] received a vote frm %v, blockID is %x", r.ID(), vote.Voter, vote.BlockID)
	r.eventChan <- vote
}

func (r *Replica) HandleTmo(tmo pacemaker.TMO) {
	if tmo.View < r.pm.GetCurView() {
		return
	}
	log.Debugf("[%v] received a timeout from %v for view %v", r.ID(), tmo.NodeID, tmo.View)
	r.eventChan <- tmo
}

func (r *Replica) HandleThemisProposal(tp message.ThemisProposal) {
	log.Infof("[%v] HandleThemisProposal called: received ThemisProposal from %v, view %v", r.ID(), tp.Proposer, tp.View)
	select {
	case r.eventChan <- tp:
		log.Infof("[%v] ThemisProposal sent to eventChan successfully", r.ID())
	default:
		log.Infof("[%v] eventChan is full, ThemisProposal from %v dropped", r.ID(), tp.Proposer)
	}
}

func (r *Replica) HandleThemisProposalEvent(tp message.ThemisProposal) {
	if _, ok := r.themisProps[tp.View]; !ok {
		r.themisProps[tp.View] = make(map[identity.NodeID]*themis.OrderedList)
	}
	r.themisProps[tp.View][tp.Proposer] = &themis.OrderedList{
		Cmds:       tp.Cmds,
		Timestamps: tp.Timestamps,
	}
	log.Infof("[%v] received themis proposal from %v for view %v, total proposals: %v", r.ID(), tp.Proposer, tp.View, len(r.themisProps[tp.View]))

	// Check if we are the leader and have enough proposals
	if r.themis && r.IsLeader(r.ID(), tp.View) {
		props := r.themisProps[tp.View]
		required := config.GetConfig().N() - config.GetConfig().ByzNo
		if len(props) >= required {
			log.Infof("[%v] has enough proposals for view %v: %v >= %v", r.ID(), tp.View, len(props), required)
			go r.proposeBlockWithThemis(tp.View)
		}
	}
	// TODO: Clean up old views from r.themisProps
}

func (r *Replica) HandleHyperGProposal(hp message.HyperGProposal) {
	log.Debugf("[%v] received a HyperGProposal from %v, view is %v", r.ID(), hp.Proposer, hp.View)
	r.eventChan <- hp
}

func (r *Replica) HandleHyperGProposalEvent(hp message.HyperGProposal) {
	if _, ok := r.HyperGProps[hp.View]; !ok {
		r.HyperGProps[hp.View] = make(map[identity.NodeID]*HyperG.OrderedList)
	}
	r.HyperGProps[hp.View][hp.Proposer] = &HyperG.OrderedList{
		Cmds:       hp.Cmds,
		Timestamps: hp.Timestamps,
	}
	// TODO: Clean up old views from r.HyperGProps
}

func (r *Replica) HandleConsensusMessage(message pCommonProto.ConsensusMessage) {
	r.startSignal()
	log.Debugf("[%v] received a consensus-message from %v", r.ID(), message.From)
	r.eventChan <- message
}

func (r *Replica) HandleCommand(command pCommonProto.Command) {
	r.startSignal()
	log.Debugf("[%v] received a command from %v", r.ID(), command.Author)
	r.eventChan <- command
}

// handleQuery replies a query with the statistics of the node
func (r *Replica) handleQuery(m message.Query) {
	// 处理可能的除零情况
	aveCreateDuration := 0.0
	if r.proposedNo > 0 {
		aveCreateDuration = float64(r.totalCreateDuration.Milliseconds()) / float64(r.proposedNo)
	}

	aveProcessTime := 0.0
	if r.processedNo > 0 {
		aveProcessTime = float64(r.totalProcessDuration.Milliseconds()) / float64(r.processedNo)
	}

	aveVoteProcessTime := 0.0
	if r.voteNo > 0 {
		aveVoteProcessTime = float64(r.totalVoteTime.Milliseconds()) / float64(r.voteNo)
	}

	requestRate := 0.0
	elapsed := time.Now().Sub(r.startTime).Seconds()
	if elapsed > 0 {
		requestRate = float64(r.pd.TotalReceivedTxNo()) / elapsed
	}

	aveRoundTime := 0.0
	if r.roundNo > 0 {
		aveRoundTime = float64(r.totalRoundTime.Milliseconds()) / float64(r.roundNo)
	}

	// query essential information from node instance.
	nodeQuery := r.Node.QueryNode()

	// 获取 phalanx metrics（简化处理避免死锁）
	phalanxMetrics := pCommonTypes.MetricsInfo{}

	// 安全处理 phalanx 指标的除零情况
	committedCommandCount := phalanxMetrics.SafeCommandCount + phalanxMetrics.RiskCommandCount
	safeRate := 0.0
	riskRate := 0.0
	if committedCommandCount > 0 {
		safeRate = float64(phalanxMetrics.SafeCommandCount) / float64(committedCommandCount) * 100
		riskRate = float64(phalanxMetrics.RiskCommandCount) / float64(committedCommandCount) * 100
	}

	periodTotalSafe := phalanxMetrics.SafeCommandCount - r.preTotalSafe
	periodTotalRisk := phalanxMetrics.RiskCommandCount - r.preTotalRisk
	periodTotalCount := periodTotalRisk + periodTotalSafe
	periodSafeRate := 0.0
	periodRiskRate := 0.0
	if periodTotalCount > 0 {
		periodSafeRate = float64(periodTotalSafe) / float64(periodTotalCount) * 100
		periodRiskRate = float64(periodTotalRisk) / float64(periodTotalCount) * 100
	}

	r.preTotalSafe = phalanxMetrics.SafeCommandCount
	r.preTotalRisk = phalanxMetrics.RiskCommandCount

	committedCommandCountM := phalanxMetrics.MSafeCommandCount + phalanxMetrics.MRiskCommandCount
	safeRateM := 0.0
	riskRateM := 0.0
	if committedCommandCountM > 0 {
		safeRateM = float64(phalanxMetrics.MSafeCommandCount) / float64(committedCommandCountM) * 100
		riskRateM = float64(phalanxMetrics.MRiskCommandCount) / float64(committedCommandCountM) * 100
	}

	periodTotalSafeM := phalanxMetrics.MSafeCommandCount - r.preTotalSafeM
	periodTotalRiskM := phalanxMetrics.MRiskCommandCount - r.preTotalRiskM
	periodTotalCountM := periodTotalRiskM + periodTotalSafeM
	periodSafeRateM := 0.0
	periodRiskRateM := 0.0
	if periodTotalCountM > 0 {
		periodSafeRateM = float64(periodTotalSafeM) / float64(periodTotalCountM) * 100
		periodRiskRateM = float64(periodTotalRiskM) / float64(periodTotalCountM) * 100
	}

	r.preTotalSafeM = phalanxMetrics.MSafeCommandCount
	r.preTotalRiskM = phalanxMetrics.MRiskCommandCount

	committedCommandCountTA := phalanxMetrics.TASafeCommandCount + phalanxMetrics.TARiskCommandCount
	safeRateTA := 0.0
	riskRateTA := 0.0
	if committedCommandCountTA > 0 {
		safeRateTA = float64(phalanxMetrics.TASafeCommandCount) / float64(committedCommandCountTA) * 100
		riskRateTA = float64(phalanxMetrics.TARiskCommandCount) / float64(committedCommandCountTA) * 100
	}

	periodTotalSafeTA := phalanxMetrics.TASafeCommandCount - r.preTotalSafeTA
	periodTotalRiskTA := phalanxMetrics.TARiskCommandCount - r.preTotalRiskTA
	periodTotalCountTA := periodTotalRiskTA + periodTotalSafeTA
	periodSafeRateTA := 0.0
	periodRiskRateTA := 0.0
	if periodTotalCountTA > 0 {
		periodSafeRateTA = float64(periodTotalSafeTA) / float64(periodTotalCountTA) * 100
		periodRiskRateTA = float64(periodTotalRiskTA) / float64(periodTotalCountTA) * 100
	}

	r.preTotalSafeTA = phalanxMetrics.TASafeCommandCount
	r.preTotalRiskTA = phalanxMetrics.TARiskCommandCount

	r.thrus += fmt.Sprintf(
		"Time: %.2f s. "+
			"Throughput: %.2f txs/s, Latency: %.2f, "+
			"Safe Rate: %.2f%%, Risk Rate: %.2f%%, "+
			"Safe Rate (M): %.2f%%, Risk Rate (M): %.2f%%, "+
			"Safe Rate (TA): %.2f%%, Risk Rate (TA): %.2f%%, "+
			"%.1fms(p), %.1fms(c), %.1fms(s), %.1fms(o)\n",
		time.Now().Sub(r.startTime).Seconds(),
		nodeQuery.Throughput, nodeQuery.Latency,
		periodSafeRate, periodRiskRate,
		periodSafeRateM, periodRiskRateM,
		periodSafeRateTA, periodRiskRateTA,
		phalanxMetrics.CurOrderLatency,
		phalanxMetrics.CurLogLatency,
		phalanxMetrics.CurCommitStreamLatency,
		phalanxMetrics.CurCommandInfoLatency,
	)

	totalCommands := phalanxMetrics.SafeCommandCount + phalanxMetrics.RiskCommandCount
	totalFrontAttackedCommands := phalanxMetrics.FrontAttackFromSafe + phalanxMetrics.FrontAttackFromRisk
	totalFrontAttackedInterval := phalanxMetrics.FrontAttackIntervalSafe + phalanxMetrics.FrontAttackIntervalRisk
	totalFrontAttackedGiven := totalFrontAttackedCommands - totalFrontAttackedInterval

	safeFrontAttackedRate := 0.0
	riskFrontAttackedRate := 0.0
	totalFrontAttackedRate := 0.0
	totalFrontAttackedIntervalRate := 0.0
	totalFrontAttackedGivenRate := 0.0

	if totalCommands > 0 {
		safeFrontAttackedRate = float64(phalanxMetrics.FrontAttackFromSafe) / float64(totalCommands) * 100
		riskFrontAttackedRate = float64(phalanxMetrics.FrontAttackFromRisk) / float64(totalCommands) * 100
		totalFrontAttackedRate = float64(totalFrontAttackedCommands) / float64(totalCommands) * 100

		if totalFrontAttackedCommands > 0 {
			totalFrontAttackedIntervalRate = float64(totalFrontAttackedInterval) / float64(totalFrontAttackedCommands) * 100
			totalFrontAttackedGivenRate = float64(totalFrontAttackedGiven) / float64(totalFrontAttackedCommands) * 100
		}
	}

	totalCommandsM := phalanxMetrics.MSafeCommandCount + phalanxMetrics.MRiskCommandCount
	totalFrontAttackedCommandsM := phalanxMetrics.MFrontAttackFromSafe + phalanxMetrics.MFrontAttackFromRisk
	totalFrontAttackedIntervalM := phalanxMetrics.MFrontAttackIntervalSafe + phalanxMetrics.MFrontAttackIntervalRisk
	totalFrontAttackedGivenM := totalFrontAttackedCommandsM - totalFrontAttackedIntervalM

	safeFrontAttackedRateM := 0.0
	riskFrontAttackedRateM := 0.0
	totalFrontAttackedRateM := 0.0
	totalFrontAttackedIntervalRateM := 0.0
	totalFrontAttackedGivenRateM := 0.0

	if totalCommandsM > 0 {
		safeFrontAttackedRateM = float64(phalanxMetrics.MFrontAttackFromSafe) / float64(totalCommandsM) * 100
		riskFrontAttackedRateM = float64(phalanxMetrics.MFrontAttackFromRisk) / float64(totalCommandsM) * 100
		totalFrontAttackedRateM = float64(totalFrontAttackedCommandsM) / float64(totalCommandsM) * 100

		if totalFrontAttackedCommandsM > 0 {
			totalFrontAttackedIntervalRateM = float64(totalFrontAttackedIntervalM) / float64(totalFrontAttackedCommandsM) * 100
			totalFrontAttackedGivenRateM = float64(totalFrontAttackedGivenM) / float64(totalFrontAttackedCommandsM) * 100
		}
	}

	totalCommandsTA := phalanxMetrics.TASafeCommandCount + phalanxMetrics.TARiskCommandCount
	totalFrontAttackedCommandsTA := phalanxMetrics.TAFrontAttackFromSafe + phalanxMetrics.TAFrontAttackFromRisk
	totalFrontAttackedIntervalTA := phalanxMetrics.TAFrontAttackIntervalSafe + phalanxMetrics.TAFrontAttackIntervalRisk
	totalFrontAttackedGivenTA := totalFrontAttackedCommandsTA - totalFrontAttackedIntervalTA

	safeFrontAttackedRateTA := 0.0
	riskFrontAttackedRateTA := 0.0
	totalFrontAttackedRateTA := 0.0
	totalFrontAttackedIntervalRateTA := 0.0
	totalFrontAttackedGivenRateTA := 0.0

	if totalCommandsTA > 0 {
		safeFrontAttackedRateTA = float64(phalanxMetrics.TAFrontAttackFromSafe) / float64(totalCommandsTA) * 100
		riskFrontAttackedRateTA = float64(phalanxMetrics.TAFrontAttackFromRisk) / float64(totalCommandsTA) * 100
		totalFrontAttackedRateTA = float64(totalFrontAttackedCommandsTA) / float64(totalCommandsTA) * 100

		if totalFrontAttackedCommandsTA > 0 {
			totalFrontAttackedIntervalRateTA = float64(totalFrontAttackedIntervalTA) / float64(totalFrontAttackedCommandsTA) * 100
			totalFrontAttackedGivenRateTA = float64(totalFrontAttackedGivenTA) / float64(totalFrontAttackedCommandsTA) * 100
		}
	}

	status := fmt.Sprintf(
		"chain status is: %s\n"+
			"Ave. block size is %v.\n"+
			"Ave. payload size is %v.\n"+
			"Ave. real block is %v.\n"+
			"Ave. creation time is %f ms.\n"+
			"Ave. processing time is %v ms.\n"+
			"Ave. vote time is %v ms.\n"+
			"Request rate is %f txs/s.\n"+
			"Ave. round time is %f ms.\n"+
			"Ave. Throughput is %f tx/s.\n"+
			"Ave. Latency is %f ms.\n",
		r.Safety.GetChainStatus(),
		nodeQuery.AveBlockSize,
		nodeQuery.AvePayloadSize,
		nodeQuery.AveRealBlock,
		aveCreateDuration,
		aveProcessTime,
		aveVoteProcessTime,
		requestRate,
		aveRoundTime,
		nodeQuery.Throughput,
		nodeQuery.Latency,
	)

	// 根据 phalanx_multi 值决定是否显示 Phalanx 指标
	if config.GetConfig().PhalanxMulti > 0 {
		status += fmt.Sprintf(
			"Ave. Latency of Phalanx\n"+
				"     Select Command %f ms.\n"+
				"     Generate Order Log %f ms.\n"+
				"     Commit Order Log %f ms.\n"+
				"     Commit Query Stream %f ms.\n"+
				"     Commit Command Info %f ms.\n"+
				"Ave. Rate of Phalanx\n"+
				"     Order Size %d\n"+
				"     Receive Rate %.2f\n"+
				"     Log Rate %.2f\n"+
				"     Gen Log Rate %.2f\n",
			phalanxMetrics.AvePackOrderLatency,
			phalanxMetrics.AveOrderLatency,
			phalanxMetrics.AveLogLatency,
			phalanxMetrics.AveCommitStreamLatency,
			phalanxMetrics.AveCommandInfoLatency,
			phalanxMetrics.AveOrderSize,
			phalanxMetrics.CommandPS,
			phalanxMetrics.LogPS,
			phalanxMetrics.GenLogPS,
		)

		status += fmt.Sprintf(
			"Phalanx Command Rate\n"+
				"     Total Commands %d\n"+
				"     Safe Committed Commands %d(%f%%)\n"+
				"     Risk Committed Commands %d(%f%%)\n"+
				"     Front Attacked Commands %d(%f%%)\n"+
				"     Front Attacked From Safe %d(%f%%)\n"+
				"     Front Attacked From Risk %d(%f%%)\n"+
				"     Front Attacked Given %d(%f%%)\n"+
				"     Front Attacked Interval %d(%f%%)\n"+
				"     Success Rates %v\n"+
				"Phalanx Command Rate Medium\n"+
				"     Total Commands %d\n"+
				"     Safe Committed Commands %d(%f%%)\n"+
				"     Risk Committed Commands %d(%f%%)\n"+
				"     Front Attacked Commands %d(%f%%)\n"+
				"     Front Attacked From Safe %d(%f%%)\n"+
				"     Front Attacked From Risk %d(%f%%)\n"+
				"     Front Attacked Given %d(%f%%)\n"+
				"     Front Attacked Interval %d(%f%%)\n"+
				"     Success Rates %v\n"+
				"Phalanx Command Rate Time Anchor\n"+
				"     Total Commands %d\n"+
				"     Safe Committed Commands %d(%f%%)\n"+
				"     Risk Committed Commands %d(%f%%)\n"+
				"     Front Attacked Commands %d(%f%%)\n"+
				"     Front Attacked From Safe %d(%f%%)\n"+
				"     Front Attacked From Risk %d(%f%%)\n"+
				"     Front Attacked Given %d(%f%%)\n"+
				"     Front Attacked Interval %d(%f%%)\n"+
				"     Success Rates %v\n",
			totalCommands,
			phalanxMetrics.SafeCommandCount, safeRate,
			phalanxMetrics.RiskCommandCount, riskRate,
			totalFrontAttackedCommands, totalFrontAttackedRate,
			phalanxMetrics.FrontAttackFromSafe, safeFrontAttackedRate,
			phalanxMetrics.FrontAttackFromRisk, riskFrontAttackedRate,
			totalFrontAttackedGiven, totalFrontAttackedGivenRate,
			totalFrontAttackedInterval, totalFrontAttackedIntervalRate,
			phalanxMetrics.SuccessRates,
			totalCommandsM,
			phalanxMetrics.MSafeCommandCount, safeRateM,
			phalanxMetrics.MRiskCommandCount, riskRateM,
			totalFrontAttackedCommandsM, totalFrontAttackedRateM,
			phalanxMetrics.MFrontAttackFromSafe, safeFrontAttackedRateM,
			phalanxMetrics.MFrontAttackFromRisk, riskFrontAttackedRateM,
			totalFrontAttackedGivenM, totalFrontAttackedGivenRateM,
			totalFrontAttackedIntervalM, totalFrontAttackedIntervalRateM,
			phalanxMetrics.MSuccessRates,
			totalCommandsTA,
			phalanxMetrics.TASafeCommandCount, safeRateTA,
			phalanxMetrics.TARiskCommandCount, riskRateTA,
			totalFrontAttackedCommandsTA, totalFrontAttackedRateTA,
			phalanxMetrics.TAFrontAttackFromSafe, safeFrontAttackedRateTA,
			phalanxMetrics.TAFrontAttackFromRisk, riskFrontAttackedRateTA,
			totalFrontAttackedGivenTA, totalFrontAttackedGivenRateTA,
			totalFrontAttackedIntervalTA, totalFrontAttackedIntervalRateTA,
			phalanxMetrics.TASuccessRates,
		)
	} else {
		// phalanx_multi=0 时不显示 Phalanx 指标
		status += "Phalanx module is disabled (phalanx_multi=0)\n"
	}

	status += fmt.Sprintf("Throughput is: \n%v", r.thrus)

	// 添加排序算法信息
	status += "\n=== Sorting Algorithms Status ===\n"

	// Themis 状态
	if r.themis {
		status += fmt.Sprintf("Themis: ENABLED (proposal_wait: %d ms)\n", config.GetConfig().Themis.ProposalWait)
	} else {
		status += "Themis: DISABLED\n"
	}

	// HyperG 状态
	if r.hyperg {
		status += fmt.Sprintf("HyperG: ENABLED (gamma: %.2f, delta: %d, proposal_wait: %d ms)\n",
			config.GetConfig().HyperG.Gamma,
			config.GetConfig().HyperG.Delta,
			config.GetConfig().HyperG.ProposalWait)

		// 如果有统计数据，显示HyperG性能指标
		if r.hypergSorter != nil {
			stats := r.hypergSorter.GetStats()
			if stats != nil && stats.TotalTime > 0 {
				status += fmt.Sprintf("HyperG Stats:\n")
				status += fmt.Sprintf("  Total Time: %.6f s\n", stats.TotalTime)
				status += fmt.Sprintf("  Pref Matrix Time: %.6f s\n", stats.PrefMatrixTime)
				status += fmt.Sprintf("  Clustering Time: %.6f s\n", stats.ClusteringTime)
				status += fmt.Sprintf("  Hypergraph Time: %.6f s\n", stats.HypergraphTime)
				status += fmt.Sprintf("  Extraction Time: %.6f s\n", stats.ExtractionTime)
				status += fmt.Sprintf("  Hyperedges: %d\n", stats.NHyperedges)
				status += fmt.Sprintf("  Avg Hyperedge Size: %.2f\n", stats.AvgHyperedgeSize)
				status += fmt.Sprintf("  Threshold: %d\n", stats.Threshold)
			} else {
				status += fmt.Sprintf("HyperG Stats: Not yet executed\n")
			}
		}
	} else {
		status += "HyperG: DISABLED\n"
	}

	// 添加交易接收统计信息
	status += fmt.Sprintf(
		"\n=== Transaction Statistics ===\n"+
			"Total Received Tx: %d\n"+
			"Current View: %d\n"+
			"Committed Blocks: %d\n"+
			"Total Committed Tx: %d\n"+
			"Proposed Blocks: %d\n"+
			"Processed Events: %d\n",
		r.pd.TotalReceivedTxNo(),
		r.pm.GetCurView(),
		r.committedNo,
		r.totalCommittedTx,
		r.proposedNo,
		r.processedNo,
	)

	m.Reply(message.QueryReply{Info: status})
}

func (r *Replica) handleTxn(m message.Transaction) {
	log.Debugf("[%v] received a transaction from client, ID: %v", r.ID(), m.ID)
	log.Debugf("[%v] entering handleTxn, current view: %v, phalanx_multi: %v, themis_enabled: %v", r.ID(), r.pm.GetCurView(), config.GetConfig().PhalanxMulti, config.GetConfig().Themis.Enabled)

	if m.Timestamp.IsZero() {
		m.Timestamp = time.Now()
	}
	log.Debugf("[%v] about to add transaction to mempool", r.ID())
	r.pd.AddTxn(&m)
	log.Debugf("[%v] transaction added to mempool", r.ID())

	//payload, _ := json.Marshal(m)
	tx := pCommonTypes.GenerateTransaction(m.Command.Value)
	log.Debugf("[%v] generated transaction object", r.ID())

	i := 0
	for {
		if i == config.GetConfig().DupRecv {
			break
		}
		log.Debugf("[%v] processing duplicate %d/%d - about to call ReceiveTransaction", r.ID(), i+1, config.GetConfig().DupRecv)

		// 只有在phalanx_multi > 0时才调用ReceiveTransaction
		if config.GetConfig().PhalanxMulti > 0 {
			r.Node.ReceiveTransaction(tx)
			log.Debugf("[%v] processing duplicate %d/%d - ReceiveTransaction completed", r.ID(), i+1, config.GetConfig().DupRecv)
		} else {
			log.Debugf("[%v] processing duplicate %d/%d - skipping ReceiveTransaction (phalanx_multi=0)", r.ID(), i+1, config.GetConfig().DupRecv)
		}

		log.Debugf("[%v] processing duplicate %d/%d - about to call CalculateRequestTx", r.ID(), i+1, config.GetConfig().DupRecv)
		r.pd.CalculateRequestTx()
		log.Debugf("[%v] processing duplicate %d/%d - CalculateRequestTx completed", r.ID(), i+1, config.GetConfig().DupRecv)
		i++
	}
	log.Debugf("[%v] about to call startSignal", r.ID())
	r.startSignal()

	// the first leader kicks off the protocol
	log.Debugf("[%v] checking view and leadership, current view: %v", r.ID(), r.pm.GetCurView())
	if r.pm.GetCurView() == 0 {
		isLeaderView0 := r.IsLeader(r.ID(), 0)
		isLeaderView1 := r.IsLeader(r.ID(), 1)
		log.Debugf("[%v] view is 0, isLeader for view 0: %v, isLeader for view 1: %v", r.ID(), isLeaderView0, isLeaderView1)
		if isLeaderView1 {
			log.Debugf("[%v] is going to kick off the protocol", r.ID())
			r.pm.AdvanceView(0)
		} else {
			log.Debugf("[%v] not leader for view 1, leader is: %v", r.ID(), r.FindLeaderFor(1))
		}
	} else {
		log.Debugf("[%v] current view is %v, not 0", r.ID(), r.pm.GetCurView())
	}
	log.Debugf("[%v] handleTxn completed", r.ID())
}

/* Processors */

func (r *Replica) processCommittedBlock(block *blockchain.Block) {
	r.committedNo++
	r.totalCommittedTx += len(block.Payload)
	r.Node.CommitBlock()

	// 更新Node层的统计信息
	r.Node.UpdateStats(block.Payload)

	log.Infof("[%v] the block is committed, No. of transactions: %v, view: %v, current view: %v, id: %x", r.ID(), len(block.Payload), block.View, r.pm.GetCurView(), block.ID)
	if block.PBatch == nil {
		return
	}
	err := r.Node.CommitProposal(block.PBatch)
	if err != nil {
		panic(err)
	}
}

func (r *Replica) processForkedBlock(block *blockchain.Block) {
	if block.Proposer == r.ID() {
		for _, txn := range block.Payload {
			// collect txn back to mem pool
			r.pd.CollectTxn(txn)
		}
	}
	log.Infof("[%v] the block is forked, No. of transactions: %v, view: %v, current view: %v, id: %x", r.ID(), len(block.Payload), block.View, r.pm.GetCurView(), block.ID)
}

func (r *Replica) processNewView(newView types.View) {
	log.Infof("[%v] is processing new view: %v, leader is %v", r.ID(), newView, r.FindLeaderFor(newView))
	if r.themis {
		r.sendThemisProposal(newView)
	}
	if !r.IsLeader(r.ID(), newView) {
		return
	}
	r.proposeBlock(newView)

	// In Themis mode, set a timer to propose block even if not enough proposals
	if r.themis {
		go func() {
			time.Sleep(time.Duration(config.GetConfig().Themis.ProposalWait) * time.Millisecond)
			r.proposeBlockWithThemis(newView)
		}()
	}
}

func (r *Replica) sendThemisProposal(view types.View) {
	log.Debugf("[%v] is sending themis proposal for view %v", r.ID(), view)
	payload := r.pd.GeneratePayload()
	if len(payload) == 0 {
		log.Debugf("[%v] no payload to send themis proposal for view %v", r.ID(), view)
		return
	}
	log.Infof("[%v] is sending themis proposal for view %v with %v txns", r.ID(), view, len(payload))
	tp := message.ThemisProposal{
		View:       view,
		Proposer:   r.ID(),
		Cmds:       make([]crypto.Identifier, len(payload)),
		Timestamps: make([]int64, len(payload)),
	}
	for i, tx := range payload {
		// Use tx.ID directly as the identifier, avoiding serialization of channel field
		tp.Cmds[i] = crypto.Identifier{}
		copy(tp.Cmds[i][:], []byte(tx.ID)[:32])
		tp.Timestamps[i] = tx.Timestamp.UnixNano()
		// 把交易放回内存池，因为这只是排序提议，还没真正打包进区块
		r.pd.CollectTxn(tx)
	}
	leader := r.FindLeaderFor(view)
	if leader == r.ID() {
		log.Infof("[%v] is leader for view %v, handling own proposal locally", r.ID(), view)
		// Use goroutine to avoid blocking the event loop
		go r.HandleThemisProposal(tp)
	} else {
		log.Infof("[%v] sending ThemisProposal to leader %v for view %v", r.ID(), leader, view)
		r.Send(leader, tp)
	}
}

func (r *Replica) proposeBlockWithThemis(view types.View) {
	// Check if already proposed for this view
	if r.proposedViews[view] {
		log.Debugf("[%v] already proposed for view %v, skipping", r.ID(), view)
		return
	}

	// Mark as proposed
	r.proposedViews[view] = true

	props, ok := r.themisProps[view]
	var payload []*message.Transaction

	if ok && len(props) >= config.GetConfig().N()-config.GetConfig().ByzNo {
		log.Infof("[%v] computing fair order for view %v with %v proposals", r.ID(), view, len(props))
		fairOrder, err := r.fo.ComputeFairOrder(props, config.GetConfig().N(), config.GetConfig().ByzNo)
		if err == nil && len(fairOrder) > 0 {
			payload = r.pd.GeneratePayload()
			orderMap := make(map[string]int)
			for i, id := range fairOrder {
				orderMap[string(id[:])] = i
			}
			sort.Slice(payload, func(i, j int) bool {
				idI := payload[i].ID
				idJ := payload[j].ID
				posI, okI := orderMap[idI]
				posJ, okJ := orderMap[idJ]
				if okI && okJ {
					return posI < posJ
				}
				if okI {
					return true
				}
				if okJ {
					return false
				}
				return false
			})
		} else {
			log.Errorf("[%v] failed to compute fair order: %v, using unsorted payload", r.ID(), err)
			payload = r.pd.GeneratePayload()
		}
	} else {
		log.Infof("[%v] not enough proposals for view %v (got %d), proposing block anyway", r.ID(), view, len(props))
		payload = r.pd.GeneratePayload()
	}

	block := r.Safety.MakeProposal(view, payload)
	r.totalBlockSize += len(block.Payload)
	r.proposedNo++
	block.Timestamp = time.Now()
	log.Infof("[%v] proposing block for view %v with %d transactions", r.ID(), view, len(block.Payload))
	r.Node.Broadcast(block)
	_ = r.Safety.ProcessBlock(block)
	r.voteStart = time.Now()
}

func (r *Replica) proposeBlock(view types.View) {
	if r.themis {
		// In Themis mode, block proposal is triggered by HandleThemisProposalEvent
		// when enough proposals are collected. Just log and return.
		log.Debugf("[%v] proposeBlock called for view %v in Themis mode, waiting for proposals", r.ID(), view)
		return
	}

	createStart := time.Now()

	// generate different block types according to trusted target
	var block *blockchain.Block
	if r.openPhalanx && view > 1 {
		block = r.Safety.MakePProposal(view)
	} else {
		block = r.Safety.MakeProposal(view, r.pd.GeneratePayload())
	}

	r.totalBlockSize += len(block.Payload)
	r.proposedNo++
	createEnd := time.Now()
	createDuration := createEnd.Sub(createStart)
	block.Timestamp = time.Now()
	r.totalCreateDuration += createDuration
	r.Node.Broadcast(block)
	_ = r.Safety.ProcessBlock(block)
	r.voteStart = time.Now()
}

// ListenLocalEvent listens new view and timeout events
func (r *Replica) ListenLocalEvent() {
	r.lastViewTime = time.Now()
	timer := time.NewTimer(r.pm.GetTimerForView())
	for {
		select {
		case view := <-r.pm.EnteringViewEvent():
			if view >= 2 {
				r.totalVoteTime += time.Now().Sub(r.voteStart)
			}
			// measure round time
			now := time.Now()
			lasts := now.Sub(r.lastViewTime)
			r.totalRoundTime += lasts
			r.roundNo++
			r.lastViewTime = now
			r.eventChan <- view
			log.Debugf("[%v] the last view lasts %v milliseconds, current view: %v", r.ID(), lasts.Milliseconds(), view)
			// Reset timer
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			timer.Reset(r.pm.GetTimerForView())
		case <-timer.C:
			r.Safety.ProcessLocalTmo(r.pm.GetCurView())
			timer.Reset(r.pm.GetTimerForView())
		}
	}
}

// ListenCommittedBlocks listens committed blocks and forked blocks from the protocols
func (r *Replica) ListenCommittedBlocks() {
	for {
		select {
		case committedBlock := <-r.committedBlocks:
			r.processCommittedBlock(committedBlock)
		case forkedBlock := <-r.forkedBlocks:
			r.processForkedBlock(forkedBlock)
		}
	}
}

func (r *Replica) startSignal() {
	log.Debugf("[%v] startSignal called, isStarted: %v", r.ID(), r.isStarted.Load())
	if !r.isStarted.Load() {
		r.startTime = time.Now()
		log.Debugf("[%v] is boosting", r.ID())
		r.isStarted.Store(true)
		r.start <- true
		r.Node.StartSignal()
		log.Debugf("[%v] startSignal completed", r.ID())
	} else {
		log.Debugf("[%v] already started, skipping", r.ID())
	}
}

// Start starts event loop
func (r *Replica) Start() {
	go r.Node.Run()

	// 根据 openPhalanx 值决定是否运行 Phalanx
	if r.openPhalanx {
		r.Node.RunPhalanx()
	}

	// wait for the start signal
	<-r.start
	go r.ListenLocalEvent()
	go r.ListenCommittedBlocks()
	for r.isStarted.Load() {
		event := <-r.eventChan
		switch v := event.(type) {
		case types.View:
			r.processNewView(v)
		case blockchain.Block:
			startProcessTime := time.Now()
			r.totalProposeDuration += startProcessTime.Sub(v.Timestamp)
			_ = r.Safety.ProcessBlock(&v)
			r.totalProcessDuration += time.Now().Sub(startProcessTime)
			r.voteStart = time.Now()
			r.processedNo++
		case blockchain.Vote:
			startProcessTime := time.Now()
			r.Safety.ProcessVote(&v)
			processingDuration := time.Now().Sub(startProcessTime)
			r.totalVoteTime += processingDuration
			r.voteNo++
		case message.ThemisProposal:
			r.HandleThemisProposalEvent(v)
		case pacemaker.TMO:
			r.Safety.ProcessRemoteTmo(&v)
		case pCommonProto.ConsensusMessage:
			_ = r.Node.ReceiveConsensusMessage(&v)
		case pCommonProto.Command:
			r.Node.ReceiveCommand(&v)
		}
	}
}
