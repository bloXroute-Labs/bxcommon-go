package types

import "time"

// EthereumProtocol - string representation for the EthereumProtocol protocol
const EthereumProtocol = "Ethereum"

// BaseMainnet - for Base main net blockchain network name
const BaseMainnet = "Base-Mainnet"

// BSCMainnet - for BSC main net blockchain network name
const BSCMainnet = "BSC-Mainnet"

// Mainnet - for Ethereum main net blockchain network name
const Mainnet = "Mainnet"

// BSCTestnet - for BSC testnet blockchain network name
const BSCTestnet = "BSC-Testnet"

// Holesky - for Holesky testnet blockchain network name
const Holesky = "Holesky"

const XLayerMainnet = "X-Layer-Mainnet"

const HyperliquidMainnet = "Hyperliquid-Mainnet"

const MonadMainnet = "Monad-Mainnet"

// MainnetNum - for Ethereum main net blockchain network number
const MainnetNum NetworkNum = 5

// BSCMainnetNum - for BSC main net blockchain network number
const BSCMainnetNum NetworkNum = 10

// BaseChainID -- Base chain ID
const BaseChainID = 8453

// BSCChainID - BSC chain ID
const BSCChainID = 56

const XLayerChainID = 196

// HyperliquidChainID - Hyperliquicd chain ID
const HyperliquidChainID = 999

const MonadChainID = 143

// EthChainID - eth chain ID
const EthChainID NetworkID = 1

// HoleskyChainID - Holesky Testnet chain ID
const HoleskyChainID = 17000

// BaseMainnetNum - for Base main net blockchain network number
const BaseMainnetNum NetworkNum = 456

const XLayerMainnetNum NetworkNum = 567

// HyperliquidNum - Hyperliquid network number (internal arbitrary, so we use same as chain id)
const HyperliquidNum NetworkNum = 999

const MonadMainnetNum NetworkNum = 143

// BSCTestnetNum - for BSC-Testnet blockchain network number
const BSCTestnetNum NetworkNum = 42

// HoleskyNum - for Holesky Testnet network number
const HoleskyNum NetworkNum = 49

// BlockchainNetworkToNetworkNum converts blockchain network to number
var BlockchainNetworkToNetworkNum = map[string]NetworkNum{
	Mainnet:            MainnetNum,
	BSCMainnet:         BSCMainnetNum,
	BSCTestnet:         BSCTestnetNum,
	Holesky:            HoleskyNum,
	BaseMainnet:        BaseMainnetNum,
	XLayerMainnet:      XLayerMainnetNum,
	HyperliquidMainnet: HyperliquidNum,
	MonadMainnet:       MonadMainnetNum,
}

// NetworkNumToChainID - Mapping from networkNum to chainID
var NetworkNumToChainID = map[NetworkNum]NetworkID{
	MainnetNum:       EthChainID,
	BSCMainnetNum:    BSCChainID,
	HoleskyNum:       HoleskyChainID,
	BaseMainnetNum:   BaseChainID,
	XLayerMainnetNum: XLayerChainID,
	HyperliquidNum:   HyperliquidChainID,
	MonadMainnetNum:  MonadChainID,
}

// NetworkNumToBlockchainNetwork - Mapping from networkNum to blockchain network
var NetworkNumToBlockchainNetwork = map[NetworkNum]string{
	MainnetNum:       Mainnet,
	BSCMainnetNum:    BSCMainnet,
	BSCTestnetNum:    BSCTestnet,
	HoleskyNum:       Holesky,
	BaseMainnetNum:   BaseMainnet,
	XLayerMainnetNum: XLayerMainnet,
	HyperliquidNum:   HyperliquidMainnet,
	MonadMainnetNum:  MonadMainnet,
}

var (
	BSCMainnetFermiTime = time.Date(2026, 1, 14, 2, 30, 0, 0, time.UTC)
	BSCTestnetFermiTime = time.Date(2025, 11, 10, 2, 25, 0, 0, time.UTC)
)

// NetworkToBlockDuration defines block interval for each network
func NetworkToBlockDuration(network string) time.Duration {
	switch network {
	case Mainnet:
		return 12 * time.Second
	case BSCMainnet:
		if time.Now().After(BSCMainnetFermiTime) {
			return 450 * time.Millisecond
		}
		return 750 * time.Millisecond
	case BSCTestnet:
		if time.Now().After(BSCTestnetFermiTime) {
			return 450 * time.Millisecond
		}
		return 750 * time.Millisecond
	case Holesky:
		return 12 * time.Second
	default:
		return 0
	}
}
