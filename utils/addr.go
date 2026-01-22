package utils

import (
	"log"

	"github.com/btcsuite/btcd/btcutil/base58"
	"github.com/ethereum/go-ethereum/common"
)

func MustDecodeBase58(addr string) common.Address {
	decoded, _, err := base58.CheckDecode(addr)
	if err != nil {
		log.Fatal(err)
	}
	return common.BytesToAddress(decoded)
}

func ConvertEthAddresses(addrs []string) []common.Address {
	result := make([]common.Address, 0, len(addrs))
	for _, addr := range addrs {
		result = append(result, common.HexToAddress(addr))
	}
	return result
}

func ConvertTronAddresses(addrs []string) []common.Address {
	result := make([]common.Address, 0, len(addrs))
	for _, addr := range addrs {
		result = append(result, MustDecodeBase58(addr))
	}
	return result
}
