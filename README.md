## What is Bamboo?

**Bamboo** is a prototyping and evaluation framework that studies the next generation BFT (Byzantine fault-tolerant) protocols specific for blockchains, namely chained-BFT, or cBFT.
By leveraging Bamboo, developers can prototype a brand new cBFT protocol in around 300 LoC and evaluate using rich benchmark facilities.

Bamboo is designed based on an observation that the core of cBFT protocols can be abstracted into 4 rules: **Proposing**, **Voting**, **State Updating**, and **Commit**.
Therefore, Bamboo abstracts the 4 rules into a *Safety* module and provides implementations of the rest of the components that can be shared across cBFT protocols, leaving the safety module to be specified by developers.

*Warning*: **Bamboo** is still under heavy development, with more features and protocols to include.

Bamboo details can be found in this [technical report](https://arxiv.org/abs/2103.00777). The paper is to appear at [ICDCS 2021](https://icdcs2021.us/).

## What is cBFT?
At a high level, cBFT protocols share a unifying *propose-vote* paradigm in which they assign transactions coming from the clients a unique order in the global ledger.
A blockchain is a sequence of blocks cryptographically linked together by hashes.
Each block in a blockchain contains a hash of its parent block along with a batch of transactions and other metadata.  

Similar to classic BFT protocols, cBFT protocols are driven by leader nodes and operate in a view-by-view manner.
Each participant takes actions on receipt of messages according to four protocol-specific rules: **Proposing**, **Voting**, **State Updating**, and **Commit**.
Each view has a designated leader chosen at random, which proposes a block according to the **Proposing** rule and populates the network.
On receiving a block, replicas take actions according to the **Voting** rule and update their local state according to the **State Updating** rule.
For each view, replicas should certify the validity of the proposed block by forming a *Quorum Certificate* (or QC) for the block.
A block with a valid QC is considered certified.
The basic structure of a blockchain is depicted in the figure below.

![blockchain](https://github.com/gitferry/bamboo/blob/master/doc/propose-vote.jpeg?raw=true)

Forks happen because of conflicting blocks, which is a scenario in which two blocks do not extend each other.
Conflicting blocks might arise because of network delays or proposers deliberately ignoring the tail of the blockchain.
Replicas finalize a block whenever the block satisfies the **Commit** rule based on their local state.
Once a block is finalized, the entire prefix of the chain is also finalized. Rules dictate that all finalized blocks remain in a single chain.
Finalized blocks can be removed from memory to persistent storage for garbage collection.

## Key Features

### Advanced Consensus Mechanisms
- **Phalanx Multi-Proposer**: Enhanced HotStuff with configurable number of concurrent proposers for improved throughput
- **Themis Fair Ordering**: Cryptographic fairness guarantees preventing leader manipulation of transaction order
- **Memory Pool Management**: Efficient transaction buffering and deduplication

### Robust Engineering
- **Zero-Value Protection**: Comprehensive edge case handling for production stability
- **Configurable Parameters**: Fine-grained control over consensus behavior
- **Extensive Monitoring**: Real-time metrics and performance insights

## What is included?

Protocols:
- [x] [HotStuff and two-chain HotStuff](https://dl.acm.org/doi/10.1145/3293611.3331591)
- [x] [Streamlet](https://dl.acm.org/doi/10.1145/3419614.3423256)
- [x] [Fast-HotStuff](https://arxiv.org/abs/2010.11454)
- [x] [Phalanx](https://arxiv.org/abs/2012.01636) - Multi-proposer consensus with fairness
- [x] [Themis](https://arxiv.org/abs/2101.03715) - Fair transaction ordering algorithm
- [ ] [LBFT](https://arxiv.org/abs/2012.01636)
- [ ] [SFT](https://arxiv.org/abs/2101.03715)

Features:
- [x] Benchmarking
- [x] Fault injection
- [x] Fair transaction ordering (Themis)
- [x] Multi-proposer consensus (Phalanx)
- [x] Memory pool management
- [x] Advanced monitoring and metrics
- [x] Configurable consensus parameters
- [x] Zero-value protection for edge cases


# How to build

1. Install [Go](https://golang.org/dl/) (version 1.14 or higher recommended).

2. Clone Bamboo-Phalanx repository.

3. Build the project:
```
cd bamboo-phalanx
# Build server and client binaries
go build -o bin/server ./server
go build -o bin/client ./client

# Or build all packages
go build ./...
```

# How to run

Users can run Bamboo-based cBFT protocols in simulation (single process) or deployment.

## Simulation
In simulation mode, replicas are running in separate Goroutines and messages are passing via Go channel.

1. ```cd bamboo-phalanx/bin```

2. Configure node addresses in `ips.txt` (local testing uses `127.0.0.1` with ports starting from `8070`)

3. Customize protocol parameters in `config.json`:
   ```json
   {
     "protocol": "phalanx",           // or "hotstuff", "streamlet", "fasthotstuff"
     "phalanx_multi": 3,              // number of proposers (0 for disabled)
     "themis": {
       "enabled": true,               // enable fair transaction ordering
       "proposal_wait": 100           // milliseconds to wait for proposals
     }
   }
   ```

4. Run simulation:
   ```
   bash simulation.sh
   ```

5. Start client traffic:
   ```
   bash runClient.sh
   ```

6. Stop simulation gracefully:
   ```
   bash closeClient.sh
   bash stop.sh
   ```

Logs are generated as `client/server.xxx.log` where `xxx` is the process ID.

## Deploy
Bamboo-Phalanx can be deployed in a real network cluster.

1. ```cd bamboo-phalanx/bin/deploy```

2. Prepare deployment configuration:
   - `pub_ips.txt`: External/public IPs of server nodes
   - `ips.txt`: Internal/private IPs of server nodes
   - `clients.txt`: IPs of client machines

3. Configure deployment scripts in `deploy.sh` and `setup_cli.sh` with SSH credentials

4. Deploy binaries and configuration:
   ```
   bash deploy.sh      # Deploy to servers
   bash setup_cli.sh   # Setup client machines
   ```

5. Update configuration files:
   ```
   bash update_conf.sh
   ```

6. Start the cluster:
   ```
   bash start.sh       # Start server nodes
   ```

7. On client machine, start traffic generation:
   ```
   bash ./runClient.sh  # Adjustable concurrent clients in script
   ```

8. Graceful shutdown:
   ```
   bash ./closeClient.sh
   bash ./pkill.sh
   ```

# Monitor
Real-time monitoring and metrics are available during operation:

## HTTP Query Interface
Access node statistics via browser:
```
http://127.0.0.1:8070/query
```
Replace with actual node address for remote deployments.

## Available Metrics
- Throughput and latency measurements
- View/change numbers
- Phalanx safety/risk rates
- Memory pool status
- Transaction ordering fairness metrics
- Network connectivity statistics

## Advanced Monitoring
- Custom metrics via `/metrics` endpoint
- JSON-formatted responses for programmatic access
- Configurable logging levels
- Performance profiling support

## Project Status

**Current Version**: Enhanced Bamboo with Phalanx and Themis integration
**Status**: Actively maintained with production-ready features

### Recent Improvements
- Fixed critical zero-value division errors in Phalanx metrics
- Integrated Themis fair transaction ordering algorithm
- Enhanced memory pool management and deduplication
- Improved monitoring and diagnostic capabilities
- Added comprehensive configuration options

### Documentation
- [THEMIS_GO_IMPLEMENTATION.md](THEMIS_GO_IMPLEMENTATION.md) - Detailed Themis integration guide
- [PHALANX_MULTI_ZERO_FIX.md](PHALANX_MULTI_ZERO_FIX.md) - Phalanx edge case fixes
- [THEMIS_INTEGRATION_PLAN.md](THEMIS_INTEGRATION_PLAN.md) - Integration roadmap

## Contributing

We welcome contributions! Please:
1. Fork the repository
2. Create a feature branch
3. Submit pull requests with clear descriptions
4. Ensure all tests pass

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
