package main

import (
	"flag"
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"os"
	"runtime/pprof"
	"runtime/trace"
	"strconv"
	"sync"
	"time"

	"github.com/gitferry/bamboo"
	"github.com/gitferry/bamboo/config"
	"github.com/gitferry/bamboo/crypto"
	"github.com/gitferry/bamboo/identity"
	"github.com/gitferry/bamboo/log"
	"github.com/gitferry/bamboo/replica"
)

var algorithm = flag.String("algorithm", "hotstuff", "BFT consensus algorithm")
var id = flag.String("id", "", "NodeID of the node")
var simulation = flag.Bool("sim", false, "simulation mode")

// Debug related config keys
const (
	DefaultURL          = "0.0.0.0"
	PprofEnable         = "pprof.enable"
	PprofPort           = "pprof.port"
	PprofDetailEnable   = "pprof.detail"
	PprofRecordDuration = "pprof.duration"

	MemsizeEnable = "memsize.enable"
	MemsizePort   = "memsize.port"
)

func setupDebug() {
	if config.GetConfig().Pprof {
		addr := DefaultURL + ":" + "10001"
		go func() {
			_ = http.ListenAndServe(addr, nil)
		}()
		go recordPProf(5 * time.Second)
	}
}

func recordPProf(duration time.Duration) {
	var (
		cpuProfile   string
		memProfile   string
		traceProfile string
		cpuFile      *os.File
		memFile      *os.File
		traceFile    *os.File
	)

	dir := "./debug"
	exist, err := pathExists(dir)
	if err != nil {
		return
	}
	if !exist {
		err := os.Mkdir(dir, os.ModePerm)
		if err != nil {
			return
		}
	}
	cpuProfile = fmt.Sprint("./debug/cpu_", time.Now().Format("2006-01-02-15-04-05"))
	memProfile = fmt.Sprint("./debug/mem_", time.Now().Format("2006-01-02-15-04-05"))
	traceProfile = fmt.Sprint("./debug/trace_", time.Now().Format("2006-01-02-15-04-05"))
	cpuFile, _ = os.Create(cpuProfile)
	_ = pprof.StartCPUProfile(cpuFile)
	traceFile, _ = os.Create(traceProfile)
	_ = trace.Start(traceFile)
	tick := time.NewTicker(duration)

	for {
		select {
		case <-tick.C:
			pprof.StopCPUProfile()
			_ = cpuFile.Close()
			trace.Stop()
			_ = traceFile.Close()
			memFile, _ = os.Create(memProfile)
			_ = pprof.WriteHeapProfile(memFile)
			_ = memFile.Close()

			cpuProfile = fmt.Sprint("./debug/cpu_", time.Now().Format("2006-01-02-15-04-05"))
			memProfile = fmt.Sprint("./debug/mem_", time.Now().Format("2006-01-02-15-04-05"))
			traceProfile = fmt.Sprint("./debug/trace_", time.Now().Format("2006-01-02-15-04-05"))
			cpuFile, _ = os.Create(cpuProfile)
			_ = pprof.StartCPUProfile(cpuFile)
			traceFile, _ = os.Create(traceProfile)
			_ = trace.Start(traceFile)
		}
	}
}

func pathExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func initReplica(id identity.NodeID, isByz bool) {
	log.Infof("node %v starting...", id)
	if isByz {
		log.Infof("node %v is Byzantine", id)
	}

	r := replica.NewReplica(id, *algorithm, isByz)
	r.Start()
}

func main() {
	bamboo.Init()
	// the private and public keys are generated here
	errCrypto := crypto.SetKeys()
	if errCrypto != nil {
		log.Fatal("Could not generate keys:", errCrypto)
	}
	if *simulation {
		var wg sync.WaitGroup
		wg.Add(1)
		config.Simulation()
		for id := range config.GetConfig().Addrs {
			isByz := false
			if id.Node() <= config.GetConfig().ByzNo {
				isByz = true
			}
			go initReplica(id, isByz)
		}
		wg.Wait()
	} else {
		setupDebug()
		isByz := false
		i, _ := strconv.Atoi(*id)
		if i <= config.GetConfig().ByzNo {
			isByz = true
		}
		initReplica(identity.NodeID(*id), isByz)
	}
}
