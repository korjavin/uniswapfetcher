package uniswap

import (
	"context"
	"fmt"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"go.uber.org/zap"
)

// V4 contract addresses
var (
	// Uniswap V4 Pool Manager contract address
	V4PoolManagerAddress = common.HexToAddress("0x8D5CF870354ffFa6F7fB096A2C247A59DA0B7E72") // Example address, replace with actual

	// Uniswap V4 Factory contract address
	V4FactoryAddress = common.HexToAddress("0x1F98431c8aD98523631AE4a59f267346ea31F984") // Example address, replace with actual
)

// V4Client is the interface for interacting with Uniswap V4
type V4Client interface {
	// GetPositions fetches all V4 positions for a given wallet address
	GetPositions(ctx context.Context, walletAddress common.Address) ([]Position, error)
}

// V4ClientImpl implements the V4Client interface
type V4ClientImpl struct {
	ethClient      *ethclient.Client
	poolManagerABI *abi.ABI
	logger         *zap.SugaredLogger
}

// NewV4Client creates a new Uniswap V4 client
func NewV4Client(ethClient *ethclient.Client, logger *zap.SugaredLogger) (V4Client, error) {
	// Parse ABIs for the contracts we need to interact with
	poolManagerABI, err := abi.JSON(strings.NewReader(poolManagerABIJson))
	if err != nil {
		return nil, fmt.Errorf("failed to parse pool manager ABI: %w", err)
	}

	logger.Infow("Initialized Uniswap V4 client",
		"poolManagerAddress", V4PoolManagerAddress.Hex(),
		"factoryAddress", V4FactoryAddress.Hex())

	return &V4ClientImpl{
		ethClient:      ethClient,
		poolManagerABI: &poolManagerABI,
		logger:         logger,
	}, nil
}

// GetPositions fetches all V4 positions for a given wallet address
func (c *V4ClientImpl) GetPositions(ctx context.Context, walletAddress common.Address) ([]Position, error) {
	c.logger.Infow("Fetching V4 positions", "wallet", walletAddress.Hex())

	// IMPORTANT: Uniswap V4 is still in development and not yet deployed on mainnet
	c.logger.Warnw("Uniswap V4 is still in development and not yet deployed on mainnet")

	// Return an empty array since V4 is not yet available
	return []Position{}, nil
}

// ABIs for the contracts we need to interact with
const poolManagerABIJson = `[
	{
		"inputs": [],
		"name": "getPoolManager",
		"outputs": [
			{
				"internalType": "address",
				"name": "",
				"type": "address"
			}
		],
		"stateMutability": "view",
		"type": "function"
	}
]`
