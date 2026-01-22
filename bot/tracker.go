package bot

import (
	"context"
	"fmt"
	"math/big"
	"strings"

	"defi-notifier/net"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
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
	chain  string
	client *ethclient.Client

	trackedBlockNum uint64

	concernedAddresses []common.Address
	concernedTopics    [][]common.Hash

	HE common.Address
}

func NewTracker(chain, endpoint string, addresses []common.Address, HE common.Address) *Tracker {
	client, err := ethclient.Dial(endpoint)
	if err != nil {
		zap.S().Fatalf("Failed to connect to [%s] client: %v", chain, err)
	}

	latestBlockNum, err := client.BlockNumber(context.Background())
	if err != nil {
		zap.S().Fatalf("Failed to get [%s] latest block number: %v", chain, err)
	}

	tracker := &Tracker{
		chain: chain,

		client: client,

		trackedBlockNum: latestBlockNum,

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
	}

	zap.S().Infof("Initialized [%s] tracker, starting from block %d", chain, latestBlockNum)

	return tracker
}

func (t *Tracker) GetFilterLogs() {
	latestBlockNum, err := t.client.BlockNumber(context.Background())
	if err != nil {
		zap.S().Errorf("Failed to get latest %s block number: %v", t.chain, err)
		return
	}

	ethQ := ethereum.FilterQuery{
		// FromBlock: big.NewInt(int64(t.trackedBlockNum)),
		// ToBlock:   big.NewInt(int64(latestBlockNum)),
		FromBlock: big.NewInt(24278469),
		ToBlock:   big.NewInt(24278469),
		Addresses: t.concernedAddresses,
		Topics:    t.concernedTopics,
	}

	logs, err := t.client.FilterLogs(context.Background(), ethQ)
	if err != nil {
		zap.S().Errorf("Failed to filter logs: %v", err)
		return
	}

	for _, vLog := range logs {
		if vLog.Topics[0] == AddedBlackListTopic {
			usr := common.BytesToAddress(vLog.Data).Hex()
			zap.S().Infof("Detected AddedBlackList event on [%s] for address: %s, tx_hash: %s", t.chain, usr, vLog.TxHash.Hex())

			if usr == t.HE.Hex() {
				slackMsg := fmt.Sprintf("Found `%s` - :usdtlogo: blacklisted address: `%s`, %s", t.chain, usr, formatTxUrl(t.chain, vLog.TxHash.Hex()))
				net.ReportNotificationToSlack(slackMsg, true)
			}
		} else if vLog.Topics[0] == BlacklistedTopic {
			usr := common.HexToAddress(vLog.Topics[1].Hex()).Hex()
			zap.S().Infof("Detected Blacklisted event on [%s] for address: %s, tx_hash: %s", t.chain, usr, vLog.TxHash.Hex())

			if usr == t.HE.Hex() {
				slackMsg := fmt.Sprintf("Found `%s` - :usdclogo: blacklisted address: `%s`, %s", t.chain, usr, formatTxUrl(t.chain, vLog.TxHash.Hex()))
				net.ReportNotificationToSlack(slackMsg, true)
			}
		} else if vLog.Topics[0] == BlockPlacedTopic {
			usr := common.HexToAddress(vLog.Topics[1].Hex()).Hex()
			zap.S().Infof("Detected BlockPlaced event on [%s] for address: %s, tx_hash: %s", t.chain, usr, vLog.TxHash.Hex())

			if usr == t.HE.Hex() {
				slackMsg := fmt.Sprintf("Found `%s` - :usdclogo: blacklisted address: `%s`, %s", t.chain, usr, formatTxUrl(t.chain, vLog.TxHash.Hex()))
				net.ReportNotificationToSlack(slackMsg, true)
			}
		} else {
			txID := new(big.Int).SetBytes(vLog.Topics[1].Bytes()).Uint64()
			zap.S().Infof("Detected Submission event on [%s] for tx_id: %d, tx_hash: %s", t.chain, txID, vLog.TxHash.Hex())

			tx, _, rpcErr := t.client.TransactionByHash(context.Background(), vLog.TxHash)
			if rpcErr != nil {
				zap.S().Errorf("Failed to get transaction by hash %s: %v", vLog.TxHash.Hex(), err)
				continue
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
				t.chain, formatTxUrl(t.chain, vLog.TxHash.Hex()), txID,
				common.BytesToAddress(destination).Hex(), hexutil.Encode(data), action)

			if strings.Contains(txData, strings.ToLower(t.HE.Hex())[2:]) {
				net.ReportNotificationToSlack(slackMsg, true)
			}
		}
	}

	t.trackedBlockNum = latestBlockNum + 1

	zap.S().Infof("Success fetch [%s] logs from block %d to %d, found %d logs",
		t.chain, ethQ.FromBlock.Uint64(), ethQ.ToBlock.Uint64(), len(logs))
}

func (t *Tracker) Stop() {
	t.client.Close()
}

func (t *Tracker) GetChain() string {
	return t.chain
}

func (t *Tracker) GetTrackedBlockNum() uint64 {
	return t.trackedBlockNum
}

func formatTxUrl(chain, txHash string) string {
	switch strings.ToLower(chain) {
	case "tron":
		return fmt.Sprintf(":clippy:<https://tronscan.io/#/transaction/%s|TxHash>", txHash)
	case "base":
		return fmt.Sprintf(":clippy:<https://basescan.io/tx/%s|TxHash>", txHash)
	case "plasma":
		return fmt.Sprintf(":clippy:<https://plasmascan.io/tx/%s|TxHash>", txHash)
	default:
		return fmt.Sprintf(":clippy:<https://etherscan.io/tx/%s|TxHash>", txHash)
	}
}
