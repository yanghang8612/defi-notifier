package bot

import (
	"context"
	"fmt"
	"math/big"
	"strings"
	"sync"

	"defi-notifier/net"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"go.uber.org/zap"
)

var (
	AddedBlackListTopic = crypto.Keccak256Hash([]byte("AddedBlackList(address)"))
	BlacklistedTopic    = crypto.Keccak256Hash([]byte("Blacklisted(address)"))
	BlockPlacedTopic    = crypto.Keccak256Hash([]byte("BlockPlaced(address)"))
	SubmissionTopic     = crypto.Keccak256Hash([]byte("Submission(uint256)"))

	actionsMap = map[string]string{
		"0xf2fde38b": "transferOwnership(address)",
		"0x8456cb59": "pause()",
		"0x3f4ba83a": "unpause()",
		"0x0ecb93c0": "addBlackList(address)",
		"0xe4997dc5": "removeBlackList(address)",
		"0xf3bdc228": "destroyBlackFunds(address)",
		"0x0753c30c": "deprecate(address)",
		"0xcc872b66": "issue(uint256)",
		"0xdb006a75": "redeem(uint256)",
		"0xc0324c77": "setParams(uint256,uint256)",
	}
)

type Tracker struct {
	chain    string
	client   *ethclient.Client
	explorer string

	latestBlockNum  uint64
	trackedBlockNum uint64
	writeLock       sync.Mutex

	concernedAddresses []common.Address
	concernedTopics    [][]common.Hash

	HE common.Address

	converter func(common.Address) string
}

func NewTracker(chain, endpoint, explorer string, addresses []common.Address, HE common.Address, converter func(common.Address) string) *Tracker {
	client, err := ethclient.Dial(endpoint)
	if err != nil {
		zap.S().Fatalf("Failed to connect to [%s] client: %v", chain, err)
	}

	latestBlockNum, err := client.BlockNumber(context.Background())
	if err != nil {
		zap.S().Fatalf("Failed to get [%s] latest block number: %v", chain, err)
	}

	tracker := &Tracker{
		chain:    chain,
		client:   client,
		explorer: explorer,

		latestBlockNum: latestBlockNum,

		concernedAddresses: addresses,
		concernedTopics: [][]common.Hash{
			{
				AddedBlackListTopic,
				BlacklistedTopic,
				BlockPlacedTopic,
				SubmissionTopic,
			},
		},

		HE: HE,

		converter: converter,
	}

	zap.S().Infof("Initialized [%s] tracker, starting from block %d", chain, latestBlockNum)

	return tracker
}

func (t *Tracker) GetFilterLogs() {
	latestBlockNum, err := t.client.BlockNumber(context.Background())
	if err != nil {
		zap.S().Errorf("Failed to get latest [%s] block number: %v", t.chain, err)
		return
	}

	for {
		if t.latestBlockNum > latestBlockNum {
			break
		}

		var toBlock uint64
		if latestBlockNum-t.latestBlockNum > 1000 {
			toBlock = t.latestBlockNum + 1000
		} else {
			toBlock = latestBlockNum
		}
		ethQ := ethereum.FilterQuery{
			FromBlock: big.NewInt(int64(t.latestBlockNum)),
			ToBlock:   big.NewInt(int64(toBlock)),
			Addresses: t.concernedAddresses,
			Topics:    t.concernedTopics,
		}

		logs, rpcErr := t.client.FilterLogs(context.Background(), ethQ)
		if rpcErr != nil {
			zap.S().Errorf("Failed to filter logs on [%s]: %v", t.chain, rpcErr)
			return
		}

		for _, vLog := range logs {
			t.handleLog(vLog)
		}

		t.writeLock.Lock()

		t.trackedBlockNum += toBlock - t.latestBlockNum + 1
		t.latestBlockNum = toBlock + 1

		t.writeLock.Unlock()

		zap.S().Infof("Success fetch [%s] logs from block %d to %d, found %d logs",
			t.chain, ethQ.FromBlock.Uint64(), ethQ.ToBlock.Uint64(), len(logs))
	}
}

func (t *Tracker) handleLog(log types.Log) {
	switch log.Topics[0] {
	case AddedBlackListTopic:
		var usr string
		if len(log.Data) > 0 {
			usr = t.converter(common.BytesToAddress(log.Data))
		} else {
			usr = t.converter(common.HexToAddress(log.Topics[1].Hex()))
		}
		zap.S().Infof("Detected AddedBlackList event on [%s] for address: %s, tx_hash: %s", t.chain, usr, log.TxHash.Hex())

		slackMsg := fmt.Sprintf("Found `%s` - :usdtlogo: blacklisted address: `%s`, %s", t.chain, usr, formatTxUrl(t.explorer, log.TxHash.Hex()))
		if usr == t.converter(t.HE) {
			net.ReportToMainChannel(slackMsg, true)
		} else {
			net.ReportToBackupChannel(slackMsg, false)
		}
	case BlacklistedTopic:
		usr := t.converter(common.HexToAddress(log.Topics[1].Hex()))
		zap.S().Infof("Detected Blacklisted event on [%s] for address: %s, tx_hash: %s", t.chain, usr, log.TxHash.Hex())

		slackMsg := fmt.Sprintf("Found `%s` - :usdclogo: blacklisted address: `%s`, %s", t.chain, usr, formatTxUrl(t.explorer, log.TxHash.Hex()))
		if usr == t.converter(t.HE) {
			net.ReportToMainChannel(slackMsg, true)
		} else {
			net.ReportToBackupChannel(slackMsg, false)
		}
	case BlockPlacedTopic:
		usr := t.converter(common.HexToAddress(log.Topics[1].Hex()))
		zap.S().Infof("Detected BlockPlaced event on [%s] for address: %s, tx_hash: %s", t.chain, usr, log.TxHash.Hex())

		slackMsg := fmt.Sprintf("Found `%s` - :usdclogo: blacklisted address: `%s`, %s", t.chain, usr, formatTxUrl(t.explorer, log.TxHash.Hex()))
		if usr == t.converter(t.HE) {
			net.ReportToMainChannel(slackMsg, true)
		} else {
			net.ReportToBackupChannel(slackMsg, false)
		}
	default:
		txID := new(big.Int).SetBytes(log.Topics[1].Bytes()).Uint64()
		zap.S().Infof("Detected Submission event on [%s] for tx_id: %d, tx_hash: %s", t.chain, txID, log.TxHash.Hex())

		tx, _, rpcErr := t.client.TransactionByHash(context.Background(), log.TxHash)
		if rpcErr != nil {
			zap.S().Errorf("Failed to get transaction on [%s] by hash %s: %v", t.chain, log.TxHash.Hex(), rpcErr)
			return
		}
		destination := tx.Data()[4 : 4+32]
		data := tx.Data()[4+32*4:]

		txData := hexutil.Encode(data)
		action := "unknown"
		if len(txData) >= 10 {
			if act, ok := actionsMap[txData[:10]]; ok {
				action = act
			}
		}

		slackMsg := fmt.Sprintf("Found `%s` - :usdtlogo: multi-sig submission: %s\n"+
			"> TxId: `%d`\n"+
			"> Destination: %s\n"+
			"> Data: %s\n"+
			"> Action: `%s`\n",
			t.chain, formatTxUrl(t.explorer, log.TxHash.Hex()), txID,
			common.BytesToAddress(destination).Hex(), hexutil.Encode(data), action)

		if strings.Contains(txData, strings.ToLower(t.HE.Hex())[2:]) {
			net.ReportToMainChannel(slackMsg, true)
		} else {
			net.ReportToBackupChannel(slackMsg, false)
		}
	}
}

func (t *Tracker) Stop() {
	t.client.Close()
}

func (t *Tracker) GetChain() string {
	return t.chain
}

func (t *Tracker) GetLatestBlockNum() uint64 {
	return t.latestBlockNum
}

func (t *Tracker) GetTrackedBlockNum() uint64 {
	t.writeLock.Lock()
	defer t.writeLock.Unlock()

	trackedBlockNum := t.trackedBlockNum
	t.trackedBlockNum = 0

	return trackedBlockNum
}

func formatTxUrl(explorer, txHash string) string {
	if strings.HasSuffix(explorer, "/") {
		explorer = explorer[:len(explorer)-1]
	}
	if strings.Contains(explorer, "tron") {
		txHash = strings.TrimPrefix(txHash, "0x")
	}
	return fmt.Sprintf(":clippy:<%s/%s|TxHash>", explorer, txHash)
}
