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

// MainnetNum - for Ethereum main net blockchain network number
const MainnetNum NetworkNum = 5

// BSCMainnetNum - for BSC main net blockchain network number
const BSCMainnetNum NetworkNum = 10

// BaseChainID -- Base chain ID
const BaseChainID = 8453

// BSCChainID - BSC chain ID
const BSCChainID = 56

// EthChainID - eth chain ID
const EthChainID NetworkID = 1

// HoleskyChainID - Holesky Testnet chain ID
const HoleskyChainID = 17000

// BaseMainnetNum - for Base main net blockchain network number
const BaseMainnetNum NetworkNum = 456

// BSCTestnetNum - for BSC-Testnet blockchain network number
const BSCTestnetNum NetworkNum = 42

// HoleskyNum - for Holesky Testnet network number
const HoleskyNum NetworkNum = 49

// BlockchainNetworkToNetworkNum converts blockchain network to number
var BlockchainNetworkToNetworkNum = map[string]NetworkNum{
	Mainnet:    MainnetNum,
	BSCMainnet: BSCMainnetNum,
	BSCTestnet: BSCTestnetNum,
	Holesky:    HoleskyNum,
}

// NetworkToBlockDuration defines block interval for each network
var NetworkToBlockDuration = map[string]time.Duration{
	Mainnet:    12 * time.Second,
	BSCMainnet: 3 * time.Second,
	Holesky:    12 * time.Second,
	BSCTestnet: 3 * time.Second,
}

// NetworkNumToChainID - Mapping from networkNum to chainID
var NetworkNumToChainID = map[NetworkNum]NetworkID{
	MainnetNum:    EthChainID,
	BSCMainnetNum: BSCChainID,
	HoleskyNum:    HoleskyChainID,
}

// NetworkNumToBlockchainNetwork - Mapping from networkNum to blockchain network
var NetworkNumToBlockchainNetwork = map[NetworkNum]string{
	MainnetNum:    Mainnet,
	BSCMainnetNum: BSCMainnet,
	BSCTestnetNum: BSCTestnet,
	HoleskyNum:    Holesky,
}
