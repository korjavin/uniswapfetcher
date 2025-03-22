package uniswap

import (
	"context"
	"fmt"
	"math/big"
	"strconv"
	"time"

	"github.com/ethereum/go-ethereum/common"
)

// Client is the interface for interacting with Uniswap
type Client interface {
	// GetPositions fetches all positions for a given wallet address
	GetPositions(ctx context.Context, req PositionRequest) ([]Position, error)

	// Close closes the client and releases any resources
	Close()
}

// FormatPositionSummary formats a position into a human-readable summary
func FormatPositionSummary(position Position) PositionSummary {
	inRange := false
	if position.CurrentPrice != nil && position.PriceLower != nil && position.PriceUpper != nil {
		inRange = position.CurrentPrice.Cmp(position.PriceLower) >= 0 && position.CurrentPrice.Cmp(position.PriceUpper) <= 0
	}

	return PositionSummary{
		ID:            position.ID.String(),
		Version:       string(position.Version),
		TokenPair:     fmt.Sprintf("%s/%s", position.Token0.Symbol, position.Token1.Symbol),
		Amounts:       fmt.Sprintf("%s %s, %s %s", formatBigInt(position.Amount0, int(position.Token0.Decimals)), position.Token0.Symbol, formatBigInt(position.Amount1, int(position.Token1.Decimals)), position.Token1.Symbol),
		PriceRange:    fmt.Sprintf("%s - %s", formatBigFloat(position.PriceLower), formatBigFloat(position.PriceUpper)),
		UnclaimedFees: fmt.Sprintf("%s %s, %s %s", formatBigInt(position.UnclaimedFees0, int(position.Token0.Decimals)), position.Token0.Symbol, formatBigInt(position.UnclaimedFees1, int(position.Token1.Decimals)), position.Token1.Symbol),
		CreatedAt:     position.CreatedAt.Format("2006-01-02 15:04:05"),
		InRange:       inRange,
	}
}

// Helper functions for formatting big numbers
func formatBigInt(n *big.Int, decimals int) string {
	if n == nil {
		return "0"
	}

	// Create a copy of n to avoid modifying the original
	value := new(big.Int).Set(n)

	// If decimals is 0, just return the string representation
	if decimals == 0 {
		return value.String()
	}

	// Convert to a decimal representation
	// First, get the integer part
	divisor := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(decimals)), nil)
	intPart := new(big.Int).Div(value, divisor)

	// Then, get the fractional part
	remainder := new(big.Int).Mod(value, divisor)

	// Format the fractional part with leading zeros
	fracStr := remainder.String()
	for len(fracStr) < decimals {
		fracStr = "0" + fracStr
	}

	// Trim trailing zeros
	for len(fracStr) > 0 && fracStr[len(fracStr)-1] == '0' {
		fracStr = fracStr[:len(fracStr)-1]
	}

	// Combine integer and fractional parts
	if len(fracStr) > 0 {
		return intPart.String() + "." + fracStr
	}

	return intPart.String()
}

func formatBigFloat(n *big.Float) string {
	if n == nil {
		return "0"
	}

	// Format with 4 decimal places
	return fmt.Sprintf("%.4f", n)
}

// parsePositionData parses position data from the API response
func parsePositionData(data *PositionData) []Position {
	var positions []Position
	for _, p := range data.Positions {
		id, _ := new(big.Int).SetString(p.ID, 10)
		pos := Position{
			ID:             id,
			Version:        VersionV3, // Default to V3
			Owner:          common.HexToAddress(p.Owner),
			Amount0:        parseBigInt(p.DepositedToken0),
			Amount1:        parseBigInt(p.DepositedToken1),
			UnclaimedFees0: parseBigInt(p.CollectedFeesToken0),
			UnclaimedFees1: parseBigInt(p.CollectedFeesToken1),
			Token0: Token{
				Symbol:   p.Token0.Symbol,
				Decimals: uint8(mustParseInt(p.Token0.Decimals)),
			},
			Token1: Token{
				Symbol:   p.Token1.Symbol,
				Decimals: uint8(mustParseInt(p.Token1.Decimals)),
			},
			CreatedAt:    time.Now(), // Use block timestamp if available
			CurrentPrice: parseBigFloat(p.Pool.Token0Price),
			PriceLower:   nil, // Calculate from tick range if available
			PriceUpper:   nil, // Calculate from tick range if available
		}
		positions = append(positions, pos)
	}
	return positions
}

func mustParseInt(s string) int64 {
	i, _ := strconv.ParseInt(s, 10, 64)
	return i
}

func parseBigInt(s string) *big.Int {
	i := new(big.Int)
	i.SetString(s, 10)
	return i
}

func parseBigFloat(s string) *big.Float {
	f := new(big.Float)
	f.SetString(s)
	return f
}
