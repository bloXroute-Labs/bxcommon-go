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

// MainnetNum - for Ethereum main net blockchain network number
const MainnetNum NetworkNum = 5

// BSCMainnetNum - for BSC main net blockchain network number
const BSCMainnetNum NetworkNum = 10

// BaseChainID -- Base chain ID
const BaseChainID = 8453

// BSCChainID - BSC chain ID
const BSCChainID = 56

const XLayerChainID = 196

// HyperliquidChaind - Hyperliquicd chain ID
const HyperliquidChainID = 999

// EthChainID - eth chain ID
const EthChainID NetworkID = 1

// HoleskyChainID - Holesky Testnet chain ID
const HoleskyChainID = 17000

// BaseMainnetNum - for Base main net blockchain network number
const BaseMainnetNum NetworkNum = 456

const XLayerMainnetNum NetworkNum = 567

// BSCTestnetNum - for BSC-Testnet blockchain network number
const BSCTestnetNum NetworkNum = 42

// HoleskyNum - for Holesky Testnet network number
const HoleskyNum NetworkNum = 49

// BlockchainNetworkToNetworkNum converts blockchain network to number
var BlockchainNetworkToNetworkNum = map[string]NetworkNum{
	Mainnet:       MainnetNum,
	BSCMainnet:    BSCMainnetNum,
	BSCTestnet:    BSCTestnetNum,
	Holesky:       HoleskyNum,
	BaseMainnet:   BaseMainnetNum,
	XLayerMainnet: XLayerMainnetNum,
}

// NetworkNumToChainID - Mapping from networkNum to chainID
var NetworkNumToChainID = map[NetworkNum]NetworkID{
	MainnetNum:       EthChainID,
	BSCMainnetNum:    BSCChainID,
	HoleskyNum:       HoleskyChainID,
	BaseMainnetNum:   BaseChainID,
	XLayerMainnetNum: XLayerChainID,
}

// NetworkNumToBlockchainNetwork - Mapping from networkNum to blockchain network
var NetworkNumToBlockchainNetwork = map[NetworkNum]string{
	MainnetNum:       Mainnet,
	BSCMainnetNum:    BSCMainnet,
	BSCTestnetNum:    BSCTestnet,
	HoleskyNum:       Holesky,
	BaseMainnetNum:   BaseMainnet,
	XLayerMainnetNum: XLayerMainnet,
}

var (
	BSCMainnetLorentzTime = time.Date(2025, 4, 29, 5, 5, 0, 0, time.UTC)
	BSCTestnetLorentzTime = time.Date(2025, 4, 8, 5, 5, 0, 0, time.UTC)
	// TODO Update these times when Maxwell is activated on BSC Mainnet and BSC Testnet
	BSCMainnetMaxwellTime time.Time
	BSCTestnetMaxwellTime time.Time
)

// NetworkToBlockDuration defines block interval for each network
func NetworkToBlockDuration(network string) time.Duration {
	switch network {
	case Mainnet:
		return 12 * time.Second
	case BSCMainnet, BSCTestnet:
		return 750 * time.Millisecond
	case Holesky:
		return 12 * time.Second
	default:
		return 0
	}
}
