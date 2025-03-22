package uniswap

import (
	"context"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"go.uber.org/zap"
)

// V3 contract addresses
var (
	// Uniswap V3 NFT Position Manager contract address
	V3PositionManagerAddress = common.HexToAddress("0xC36442b4a4522E871399CD717aBDD847Ab11FE88")

	// Uniswap V3 Factory contract address
	V3FactoryAddress = common.HexToAddress("0x1F98431c8aD98523631AE4a59f267346ea31F984")

	// Common ERC20 tokens
	TokenAddressToSymbol = map[string]TokenInfo{
		"0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48": {Symbol: "USDC", Decimals: 6},
		"0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2": {Symbol: "WETH", Decimals: 18},
		"0xdAC17F958D2ee523a2206206994597C13D831ec7": {Symbol: "USDT", Decimals: 6},
		"0x2260FAC5E5542a773Aa44fBCfeDf7C193bc2C599": {Symbol: "WBTC", Decimals: 8},
		"0x6B175474E89094C44Da98b954EedeAC495271d0F": {Symbol: "DAI", Decimals: 18},
		// Add more tokens as needed
	}
)

// TokenInfo contains token metadata
type TokenInfo struct {
	Symbol   string
	Decimals uint8
}

// V3Client is the interface for interacting with Uniswap V3
type V3Client interface {
	// GetPositions fetches all V3 positions for a given wallet address
	GetPositions(ctx context.Context, walletAddress common.Address) ([]Position, error)
}

// V3ClientImpl implements the V3Client interface
type V3ClientImpl struct {
	ethClient          *ethclient.Client
	positionManagerABI *abi.ABI
	erc721ABI          *abi.ABI
	erc20ABI           *abi.ABI
	logger             *zap.SugaredLogger
}

// NewV3Client creates a new Uniswap V3 client
func NewV3Client(ethClient *ethclient.Client, logger *zap.SugaredLogger) (V3Client, error) {
	// Parse ABIs for the contracts we need to interact with
	positionManagerABI, err := abi.JSON(strings.NewReader(positionManagerABIJson))
	if err != nil {
		return nil, fmt.Errorf("failed to parse position manager ABI: %w", err)
	}

	erc721ABI, err := abi.JSON(strings.NewReader(erc721ABIJson))
	if err != nil {
		return nil, fmt.Errorf("failed to parse ERC721 ABI: %w", err)
	}

	erc20ABI, err := abi.JSON(strings.NewReader(erc20ABIJson))
	if err != nil {
		return nil, fmt.Errorf("failed to parse ERC20 ABI: %w", err)
	}

	logger.Infow("Initialized Uniswap V3 client",
		"positionManagerAddress", V3PositionManagerAddress.Hex(),
		"factoryAddress", V3FactoryAddress.Hex())

	return &V3ClientImpl{
		ethClient:          ethClient,
		positionManagerABI: &positionManagerABI,
		erc721ABI:          &erc721ABI,
		erc20ABI:           &erc20ABI,
		logger:             logger,
	}, nil
}

// GetPositions fetches all V3 positions for a given wallet address
func (c *V3ClientImpl) GetPositions(ctx context.Context, walletAddress common.Address) ([]Position, error) {
	c.logger.Infow("Fetching V3 positions", "wallet", walletAddress.Hex())

	// 1. Get the balance of NFTs for this wallet
	balanceOf, err := c.getTokenBalance(ctx, V3PositionManagerAddress, walletAddress)
	if err != nil {
		return nil, fmt.Errorf("failed to get NFT balance: %w", err)
	}

	c.logger.Debugw("NFT balance", "balance", balanceOf)

	// If no positions, return empty array
	if balanceOf.Cmp(big.NewInt(0)) == 0 {
		return []Position{}, nil
	}

	// 2. Get all token IDs owned by this wallet
	tokenIDs, err := c.getOwnedTokenIDs(ctx, walletAddress, balanceOf)
	if err != nil {
		return nil, fmt.Errorf("failed to get owned token IDs: %w", err)
	}

	// 3. Get position details for each token ID
	var positions []Position
	for _, tokenID := range tokenIDs {
		position, err := c.getPositionDetails(ctx, tokenID, walletAddress)
		if err != nil {
			c.logger.Warnw("Failed to get position details", "tokenID", tokenID, "error", err)
			continue
		}
		positions = append(positions, position)
	}

	c.logger.Infow("Fetched V3 positions", "count", len(positions))
	return positions, nil
}

// getTokenBalance gets the balance of tokens for a wallet
func (c *V3ClientImpl) getTokenBalance(ctx context.Context, tokenAddress, walletAddress common.Address) (*big.Int, error) {
	callData, err := c.erc721ABI.Pack("balanceOf", walletAddress)
	if err != nil {
		return nil, fmt.Errorf("failed to pack balanceOf call: %w", err)
	}

	result, err := c.ethClient.CallContract(ctx, ethereum.CallMsg{
		To:   &tokenAddress,
		Data: callData,
	}, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to call balanceOf: %w", err)
	}

	var balance *big.Int
	err = c.erc721ABI.UnpackIntoInterface(&balance, "balanceOf", result)
	if err != nil {
		return nil, fmt.Errorf("failed to unpack balanceOf result: %w", err)
	}

	return balance, nil
}

// getOwnedTokenIDs gets all token IDs owned by a wallet
func (c *V3ClientImpl) getOwnedTokenIDs(ctx context.Context, walletAddress common.Address, balance *big.Int) ([]*big.Int, error) {
	var tokenIDs []*big.Int

	// For each token index, get the token ID
	for i := int64(0); i < balance.Int64(); i++ {
		callData, err := c.erc721ABI.Pack("tokenOfOwnerByIndex", walletAddress, big.NewInt(i))
		if err != nil {
			return nil, fmt.Errorf("failed to pack tokenOfOwnerByIndex call: %w", err)
		}

		result, err := c.ethClient.CallContract(ctx, ethereum.CallMsg{
			To:   &V3PositionManagerAddress,
			Data: callData,
		}, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to call tokenOfOwnerByIndex: %w", err)
		}

		var tokenID *big.Int
		err = c.erc721ABI.UnpackIntoInterface(&tokenID, "tokenOfOwnerByIndex", result)
		if err != nil {
			return nil, fmt.Errorf("failed to unpack tokenOfOwnerByIndex result: %w", err)
		}

		tokenIDs = append(tokenIDs, tokenID)
	}

	return tokenIDs, nil
}

// getPositionDetails gets the details of a position
func (c *V3ClientImpl) getPositionDetails(ctx context.Context, tokenID *big.Int, walletAddress common.Address) (Position, error) {
	// Call positions(tokenId) on the position manager contract
	callData, err := c.positionManagerABI.Pack("positions", tokenID)
	if err != nil {
		return Position{}, fmt.Errorf("failed to pack positions call: %w", err)
	}

	result, err := c.ethClient.CallContract(ctx, ethereum.CallMsg{
		To:   &V3PositionManagerAddress,
		Data: callData,
	}, nil)
	if err != nil {
		return Position{}, fmt.Errorf("failed to call positions: %w", err)
	}

	// Unpack the result
	var positionResult struct {
		Nonce                    *big.Int
		Operator                 common.Address
		Token0                   common.Address
		Token1                   common.Address
		Fee                      *big.Int
		TickLower                *big.Int
		TickUpper                *big.Int
		Liquidity                *big.Int
		FeeGrowthInside0LastX128 *big.Int
		FeeGrowthInside1LastX128 *big.Int
		TokensOwed0              *big.Int
		TokensOwed1              *big.Int
	}
	err = c.positionManagerABI.UnpackIntoInterface(&positionResult, "positions", result)
	if err != nil {
		return Position{}, fmt.Errorf("failed to unpack positions result: %w", err)
	}

	// Get token symbols and decimals
	token0Symbol, token0Decimals := c.getTokenInfo(ctx, positionResult.Token0)
	token1Symbol, token1Decimals := c.getTokenInfo(ctx, positionResult.Token1)

	// Calculate price range and current price
	// Note: This is a simplified calculation and may not be accurate for all token pairs
	sqrtPriceX96, err := c.getCurrentSqrtPriceX96(ctx, positionResult.Token0, positionResult.Token1, positionResult.Fee.Uint64())
	if err != nil {
		c.logger.Warnw("Failed to get current price", "error", err)
		sqrtPriceX96 = big.NewInt(0)
	}

	// Calculate amounts
	amount0, amount1, err := c.calculateAmounts(ctx, tokenID, positionResult.Liquidity)
	if err != nil {
		c.logger.Warnw("Failed to calculate amounts", "error", err)
		amount0 = big.NewInt(0)
		amount1 = big.NewInt(0)
	}

	// Calculate unclaimed fees
	unclaimedFees0, unclaimedFees1, err := c.calculateUnclaimedFees(ctx, tokenID)
	if err != nil {
		c.logger.Warnw("Failed to calculate unclaimed fees", "error", err)
		unclaimedFees0 = big.NewInt(0)
		unclaimedFees1 = big.NewInt(0)
	}

	// Get creation timestamp
	createdAt, err := c.getPositionCreationTime(ctx, tokenID)
	if err != nil {
		c.logger.Warnw("Failed to get position creation time", "error", err)
		createdAt = time.Now()
	}

	// Calculate price range
	tickToPrice := func(tick int64) *big.Float {
		// Convert tick to price using the formula: price = 1.0001^tick
		// This is a simplified calculation
		price := big.NewFloat(1.0001)
		price.SetPrec(256)
		price = price.SetMantExp(price, int(tick))
		return price
	}

	priceLower := tickToPrice(positionResult.TickLower.Int64())
	priceUpper := tickToPrice(positionResult.TickUpper.Int64())

	// Calculate current price from sqrtPriceX96
	currentPrice := new(big.Float)
	if sqrtPriceX96.Cmp(big.NewInt(0)) > 0 {
		sqrtPrice := new(big.Float).SetInt(sqrtPriceX96)
		sqrtPrice.Quo(sqrtPrice, new(big.Float).SetInt(new(big.Int).Exp(big.NewInt(2), big.NewInt(96), nil)))
		currentPrice.Mul(sqrtPrice, sqrtPrice)
	} else {
		// If we couldn't get the current price, use a value in the middle of the range
		currentPrice.Add(priceLower, priceUpper)
		currentPrice.Quo(currentPrice, big.NewFloat(2))
	}

	position := Position{
		ID:             tokenID,
		Version:        VersionV3,
		Owner:          walletAddress,
		Token0:         Token{Address: positionResult.Token0, Symbol: token0Symbol, Decimals: token0Decimals},
		Token1:         Token{Address: positionResult.Token1, Symbol: token1Symbol, Decimals: token1Decimals},
		Amount0:        amount0,
		Amount1:        amount1,
		FeeTier:        uint32(positionResult.Fee.Uint64()),
		CreatedAt:      createdAt,
		TickLower:      int(positionResult.TickLower.Int64()),
		TickUpper:      int(positionResult.TickUpper.Int64()),
		Liquidity:      positionResult.Liquidity,
		UnclaimedFees0: unclaimedFees0,
		UnclaimedFees1: unclaimedFees1,
		PriceLower:     priceLower,
		PriceUpper:     priceUpper,
		CurrentPrice:   currentPrice,
	}

	c.logger.Infow("Fetched V3 position",
		"positionId", position.ID.String(),
		"tokenPair", fmt.Sprintf("%s/%s", position.Token0.Symbol, position.Token1.Symbol))

	return position, nil
}

// getTokenInfo gets the symbol and decimals for a token
func (c *V3ClientImpl) getTokenInfo(ctx context.Context, tokenAddress common.Address) (string, uint8) {
	// Check if we have the token info cached
	if info, ok := TokenAddressToSymbol[strings.ToLower(tokenAddress.Hex())]; ok {
		return info.Symbol, info.Decimals
	}

	// Otherwise, query the blockchain
	symbol := c.getTokenSymbol(ctx, tokenAddress)
	decimals := c.getTokenDecimals(ctx, tokenAddress)

	return symbol, decimals
}

// getTokenSymbol gets the symbol for a token
func (c *V3ClientImpl) getTokenSymbol(ctx context.Context, tokenAddress common.Address) string {
	callData, err := c.erc20ABI.Pack("symbol")
	if err != nil {
		c.logger.Warnw("Failed to pack symbol call", "error", err)
		return "UNKNOWN"
	}

	result, err := c.ethClient.CallContract(ctx, ethereum.CallMsg{
		To:   &tokenAddress,
		Data: callData,
	}, nil)
	if err != nil {
		c.logger.Warnw("Failed to call symbol", "error", err)
		return "UNKNOWN"
	}

	var symbol string
	err = c.erc20ABI.UnpackIntoInterface(&symbol, "symbol", result)
	if err != nil {
		c.logger.Warnw("Failed to unpack symbol result", "error", err)
		return "UNKNOWN"
	}

	return symbol
}

// getTokenDecimals gets the decimals for a token
func (c *V3ClientImpl) getTokenDecimals(ctx context.Context, tokenAddress common.Address) uint8 {
	callData, err := c.erc20ABI.Pack("decimals")
	if err != nil {
		c.logger.Warnw("Failed to pack decimals call", "error", err)
		return 18 // Default to 18 decimals
	}

	result, err := c.ethClient.CallContract(ctx, ethereum.CallMsg{
		To:   &tokenAddress,
		Data: callData,
	}, nil)
	if err != nil {
		c.logger.Warnw("Failed to call decimals", "error", err)
		return 18 // Default to 18 decimals
	}

	var decimals uint8
	err = c.erc20ABI.UnpackIntoInterface(&decimals, "decimals", result)
	if err != nil {
		c.logger.Warnw("Failed to unpack decimals result", "error", err)
		return 18 // Default to 18 decimals
	}

	return decimals
}

// getCurrentSqrtPriceX96 gets the current sqrt price for a pool
func (c *V3ClientImpl) getCurrentSqrtPriceX96(ctx context.Context, token0, token1 common.Address, fee uint64) (*big.Int, error) {
	// This is a simplified implementation
	// In a real implementation, we would:
	// 1. Get the pool address from the factory
	// 2. Call slot0() on the pool to get the current sqrt price

	// For now, return a default value
	return big.NewInt(0), nil
}

// calculateAmounts calculates the amounts of token0 and token1 in a position
func (c *V3ClientImpl) calculateAmounts(ctx context.Context, tokenID, liquidity *big.Int) (*big.Int, *big.Int, error) {
	// This is a simplified implementation
	// In a real implementation, we would:
	// 1. Get the pool address
	// 2. Get the current tick
	// 3. Calculate the amounts based on the liquidity, tick, and tick range

	// For now, return default values
	return big.NewInt(0), big.NewInt(0), nil
}

// calculateUnclaimedFees calculates the unclaimed fees for a position
func (c *V3ClientImpl) calculateUnclaimedFees(ctx context.Context, tokenID *big.Int) (*big.Int, *big.Int, error) {
	// This is a simplified implementation
	// In a real implementation, we would:
	// 1. Get the pool address
	// 2. Get the current fee growth
	// 3. Calculate the unclaimed fees

	// For now, return default values
	return big.NewInt(0), big.NewInt(0), nil
}

// getPositionCreationTime gets the creation time of a position
func (c *V3ClientImpl) getPositionCreationTime(ctx context.Context, tokenID *big.Int) (time.Time, error) {
	// This is a simplified implementation
	// In a real implementation, we would:
	// 1. Query the transfer event for the token ID
	// 2. Get the block timestamp

	// For now, return the current time
	return time.Now().Add(-30 * 24 * time.Hour), nil
}

// ABIs for the contracts we need to interact with
const positionManagerABIJson = `[
	{
		"inputs": [
			{
				"internalType": "uint256",
				"name": "tokenId",
				"type": "uint256"
			}
		],
		"name": "positions",
		"outputs": [
			{
				"internalType": "uint96",
				"name": "nonce",
				"type": "uint96"
			},
			{
				"internalType": "address",
				"name": "operator",
				"type": "address"
			},
			{
				"internalType": "address",
				"name": "token0",
				"type": "address"
			},
			{
				"internalType": "address",
				"name": "token1",
				"type": "address"
			},
			{
				"internalType": "uint24",
				"name": "fee",
				"type": "uint24"
			},
			{
				"internalType": "int24",
				"name": "tickLower",
				"type": "int24"
			},
			{
				"internalType": "int24",
				"name": "tickUpper",
				"type": "int24"
			},
			{
				"internalType": "uint128",
				"name": "liquidity",
				"type": "uint128"
			},
			{
				"internalType": "uint256",
				"name": "feeGrowthInside0LastX128",
				"type": "uint256"
			},
			{
				"internalType": "uint256",
				"name": "feeGrowthInside1LastX128",
				"type": "uint256"
			},
			{
				"internalType": "uint128",
				"name": "tokensOwed0",
				"type": "uint128"
			},
			{
				"internalType": "uint128",
				"name": "tokensOwed1",
				"type": "uint128"
			}
		],
		"stateMutability": "view",
		"type": "function"
	}
]`

const erc721ABIJson = `[
	{
		"inputs": [
			{
				"internalType": "address",
				"name": "owner",
				"type": "address"
			}
		],
		"name": "balanceOf",
		"outputs": [
			{
				"internalType": "uint256",
				"name": "",
				"type": "uint256"
			}
		],
		"stateMutability": "view",
		"type": "function"
	},
	{
		"inputs": [
			{
				"internalType": "address",
				"name": "owner",
				"type": "address"
			},
			{
				"internalType": "uint256",
				"name": "index",
				"type": "uint256"
			}
		],
		"name": "tokenOfOwnerByIndex",
		"outputs": [
			{
				"internalType": "uint256",
				"name": "",
				"type": "uint256"
			}
		],
		"stateMutability": "view",
		"type": "function"
	}
]`

const erc20ABIJson = `[
	{
		"inputs": [],
		"name": "symbol",
		"outputs": [
			{
				"internalType": "string",
				"name": "",
				"type": "string"
			}
		],
		"stateMutability": "view",
		"type": "function"
	},
	{
		"inputs": [],
		"name": "decimals",
		"outputs": [
			{
				"internalType": "uint8",
				"name": "",
				"type": "uint8"
			}
		],
		"stateMutability": "view",
		"type": "function"
	}
]`
