package uniswap

import (
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
)

// PositionVersion represents the Uniswap version (V3 or V4)
type PositionVersion string

const (
	// VersionV3 represents Uniswap V3
	VersionV3 PositionVersion = "V3"
	// VersionV4 represents Uniswap V4
	VersionV4 PositionVersion = "V4"
)

// Token represents an ERC20 token
type Token struct {
	Address  common.Address `json:"address"`
	Symbol   string         `json:"symbol"`
	Decimals uint8          `json:"decimals"`
}

// Position represents a Uniswap position (either V3 or V4)
type Position struct {
	// Common fields for both V3 and V4
	ID        *big.Int        `json:"id"`
	Version   PositionVersion `json:"version"`
	Owner     common.Address  `json:"owner"`
	Token0    Token           `json:"token0"`
	Token1    Token           `json:"token1"`
	Amount0   *big.Int        `json:"amount0"`
	Amount1   *big.Int        `json:"amount1"`
	FeeTier   uint32          `json:"feeTier"`
	CreatedAt time.Time       `json:"createdAt"`

	// V3 specific fields
	TickLower int      `json:"tickLower,omitempty"`
	TickUpper int      `json:"tickUpper,omitempty"`
	Liquidity *big.Int `json:"liquidity,omitempty"`

	// Fee information
	UnclaimedFees0 *big.Int `json:"unclaimedFees0"`
	UnclaimedFees1 *big.Int `json:"unclaimedFees1"`

	// Price range
	PriceLower   *big.Float `json:"priceLower"`
	PriceUpper   *big.Float `json:"priceUpper"`
	CurrentPrice *big.Float `json:"currentPrice"`

	// V4 specific fields (if any)
	// Add V4 specific fields here as needed
}

// PositionSummary provides a human-readable summary of a position
type PositionSummary struct {
	ID            string `json:"id"`
	Version       string `json:"version"`
	TokenPair     string `json:"tokenPair"`
	Amounts       string `json:"amounts"`
	PriceRange    string `json:"priceRange"`
	UnclaimedFees string `json:"unclaimedFees"`
	CreatedAt     string `json:"createdAt"`
	InRange       bool   `json:"inRange"`
}

// PositionRequest represents a request to fetch positions for a wallet
type PositionRequest struct {
	WalletAddress common.Address
	IncludeV3     bool
	IncludeV4     bool
}
