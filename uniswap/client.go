package uniswap

import (
	"context"
	"fmt"
	"math/big"
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
		Amounts:       fmt.Sprintf("%s %s, %s %s", formatBigInt(position.DepositedToken0, int(position.Token0.Decimals)), position.Token0.Symbol, formatBigInt(position.DepositedToken1, int(position.Token1.Decimals)), position.Token1.Symbol),
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

	// For display purposes, we want to show the actual value, not the raw value
	// For example, if the token has 6 decimals and the value is 1000000, we want to show 1
	// If the token has 18 decimals and the value is 1000000000000000000, we want to show 1
	divisor := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(decimals)), nil)

	// Check if the value is a multiple of the divisor (no fractional part)
	if new(big.Int).Mod(value, divisor).Cmp(big.NewInt(0)) == 0 {
		// If it's a clean division, just show the integer part
		return new(big.Int).Div(value, divisor).String()
	}

	// For values that have a fractional part, we'll format with the appropriate decimal places
	// First, get the integer part
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
