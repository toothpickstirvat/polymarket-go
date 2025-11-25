package polymarket

import (
	"context"
	"errors"
	"fmt"
	"time"

	polymarketdata "github.com/ivanzzeth/polymarket-go-data-client"
	"github.com/shopspring/decimal"
)

// AutoRedeemConfig configuration for automatic redemption
type AutoRedeemConfig struct {
	// PollingInterval polling interval (default 60 seconds)
	PollingInterval time.Duration

	// Enabled whether to enable auto redeem (default false)
	Enabled bool

	// OnError error callback (optional)
	OnError func(error)

	// OnSuccess success callback (optional)
	OnSuccess func(tokenID string, amount decimal.Decimal)
}

// AutoMergeConfig configuration for automatic merge
type AutoMergeConfig struct {
	// PollingInterval polling interval (default 60 seconds)
	PollingInterval time.Duration

	// Enabled whether to enable auto merge (default false)
	Enabled bool

	// MinMergeAmount minimum merge amount threshold (to avoid frequent small merges)
	MinMergeAmount decimal.Decimal

	// OnError error callback (optional)
	OnError func(error)

	// OnSuccess success callback (optional)
	OnSuccess func(conditionID string, amount decimal.Decimal)
}

// startAutoRedeem starts the automatic redemption service (internal use only)
// This is called automatically by NewClient if AutoRedeemConfig is provided
// Periodically checks user positions, and automatically redeems if the market is resolved
func (c *Client) startAutoRedeem(ctx context.Context, config *AutoRedeemConfig) error {
	if config == nil {
		return fmt.Errorf("config cannot be nil")
	}

	c.autoMu.Lock()
	defer c.autoMu.Unlock()

	// Stop existing service if running
	if c.autoRedeemCancel != nil {
		c.autoRedeemCancel()
	}

	// Set default polling interval
	if config.PollingInterval == 0 {
		config.PollingInterval = 60 * time.Second
	}

	c.autoRedeemConfig = config

	// If not enabled, just store config and return
	if !config.Enabled {
		return nil
	}

	// Create cancellable context
	redeemCtx, cancel := context.WithCancel(ctx)
	c.autoRedeemCancel = cancel

	// Start background goroutine
	go c.autoRedeemLoop(redeemCtx)

	return nil
}

// StopAutoRedeem stops the automatic redemption service
func (c *Client) StopAutoRedeem() error {
	c.autoMu.Lock()
	defer c.autoMu.Unlock()

	if c.autoRedeemCancel != nil {
		c.autoRedeemCancel()
		c.autoRedeemCancel = nil
	}

	return nil
}

// startAutoMerge starts the automatic merge service (internal use only)
// This is called automatically by NewClient if AutoMergeConfig is provided
// Periodically checks user's complementary token pairs and automatically merges them to USDC
func (c *Client) startAutoMerge(ctx context.Context, config *AutoMergeConfig) error {
	if config == nil {
		return fmt.Errorf("config cannot be nil")
	}

	c.autoMu.Lock()
	defer c.autoMu.Unlock()

	// Stop existing service if running
	if c.autoMergeCancel != nil {
		c.autoMergeCancel()
	}

	// Set default polling interval
	if config.PollingInterval == 0 {
		config.PollingInterval = 60 * time.Second
	}

	// Set default min merge amount
	if config.MinMergeAmount.IsZero() {
		config.MinMergeAmount = decimal.NewFromFloat(0.1) // Default 0.1 USDC
	}

	c.autoMergeConfig = config

	// If not enabled, just store config and return
	if !config.Enabled {
		return nil
	}

	// Create cancellable context
	mergeCtx, cancel := context.WithCancel(ctx)
	c.autoMergeCancel = cancel

	// Start background goroutine
	go c.autoMergeLoop(mergeCtx)

	return nil
}

// StopAutoMerge stops the automatic merge service
func (c *Client) StopAutoMerge() error {
	c.autoMu.Lock()
	defer c.autoMu.Unlock()

	if c.autoMergeCancel != nil {
		c.autoMergeCancel()
		c.autoMergeCancel = nil
	}

	return nil
}

// startAutoManagement starts the complete automatic management service (internal use only)
// This is called automatically by NewClient if auto management configs are provided
func (c *Client) startAutoManagement(ctx context.Context, redeemConfig *AutoRedeemConfig, mergeConfig *AutoMergeConfig) error {
	// Start auto redeem if configured
	if redeemConfig != nil {
		if err := c.startAutoRedeem(ctx, redeemConfig); err != nil {
			return fmt.Errorf("failed to start auto redeem: %w", err)
		}
	}

	// Start auto merge if configured
	if mergeConfig != nil {
		if err := c.startAutoMerge(ctx, mergeConfig); err != nil {
			// If merge fails, stop redeem to keep consistent state
			if stopErr := c.StopAutoRedeem(); stopErr != nil {
				// Return combined error if stop also fails
				return fmt.Errorf("failed to start auto merge: %w (additionally, failed to stop auto redeem: %v)", err, stopErr)
			}
			return fmt.Errorf("failed to start auto merge: %w", err)
		}
	}

	return nil
}

// StopAutoManagement stops all automatic management services
// It attempts to stop all services even if some fail, and returns a combined error if any fail
func (c *Client) StopAutoManagement() error {
	var errs []error

	if err := c.StopAutoRedeem(); err != nil {
		errs = append(errs, fmt.Errorf("stop auto redeem: %w", err))
	}

	if err := c.StopAutoMerge(); err != nil {
		errs = append(errs, fmt.Errorf("stop auto merge: %w", err))
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}

	return nil
}

// autoRedeemLoop automatic redemption loop
func (c *Client) autoRedeemLoop(ctx context.Context) {
	ticker := time.NewTicker(c.autoRedeemConfig.PollingInterval)
	defer ticker.Stop()

	// Run immediately once
	c.processAutoRedeem(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.processAutoRedeem(ctx)
		}
	}
}

// autoMergeLoop automatic merge loop
func (c *Client) autoMergeLoop(ctx context.Context) {
	ticker := time.NewTicker(c.autoMergeConfig.PollingInterval)
	defer ticker.Stop()

	// Run immediately once
	c.processAutoMerge(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.processAutoMerge(ctx)
		}
	}
}

// processAutoRedeem processes automatic redemption logic
func (c *Client) processAutoRedeem(ctx context.Context) {
	config := c.autoRedeemConfig
	if config == nil || !config.Enabled {
		return
	}

	// Get user's redeemable positions from DataClient
	redeemableTrue := true
	// TODO: Implement pagination to fetch all positions if user has more than 500 positions
	// Currently limited to 500 positions per API constraint
	positions, err := c.dataClient.GetPositions(ctx, &polymarketdata.GetPositionsParams{
		User:       c.funderAddr.Hex(),
		Redeemable: &redeemableTrue,
		Limit:      500, // API max limit
	})
	if err != nil {
		if config.OnError != nil {
			config.OnError(fmt.Errorf("failed to get positions: %w", err))
		}
		return
	}

	// Process each redeemable position
	for _, position := range positions {
		conditionID := position.ConditionId
		balance := position.Size

		// Skip zero balance
		if balance.IsZero() {
			continue
		}

		// Try to redeem
		if position.NegativeRisk {
			// For NegRisk markets, need to find complementary position
			var complementaryBalance decimal.Decimal
			for _, pos := range positions {
				if pos.ConditionId == conditionID && pos.Asset != position.Asset {
					complementaryBalance = pos.Size
					break
				}
			}

			// Try NegRisk redeem
			amounts := []decimal.Decimal{balance, complementaryBalance}
			_, err := c.RedeemNegRisk(ctx, conditionID, amounts)
			if err != nil {
				if config.OnError != nil {
					config.OnError(fmt.Errorf("failed to redeem NegRisk condition %s: %w", conditionID, err))
				}
				continue
			}

			// NegRisk redeem succeeded
			if config.OnSuccess != nil {
				totalRedeemed := balance.Add(complementaryBalance)
				config.OnSuccess(position.Asset, totalRedeemed)
			}
		} else {
			// Standard market redeem
			_, err := c.Redeem(ctx, conditionID)
			if err != nil {
				if config.OnError != nil {
					config.OnError(fmt.Errorf("failed to redeem condition %s: %w", conditionID, err))
				}
				continue
			}

			// Standard redeem succeeded
			if config.OnSuccess != nil {
				config.OnSuccess(position.Asset, balance)
			}
		}
	}
}

// processAutoMerge processes automatic merge logic
func (c *Client) processAutoMerge(ctx context.Context) {
	config := c.autoMergeConfig
	if config == nil || !config.Enabled {
		return
	}

	// Get user's actual positions from DataClient with mergeable filter
	mergeableTrue := true
	// TODO: Implement pagination to fetch all positions if user has more than 500 positions
	// Currently limited to 500 positions per API constraint
	positions, err := c.dataClient.GetPositions(ctx, &polymarketdata.GetPositionsParams{
		User:       c.funderAddr.Hex(),
		Mergeable:  &mergeableTrue,
		Limit:      500, // API max limit
	})
	if err != nil {
		if config.OnError != nil {
			config.OnError(fmt.Errorf("failed to get positions: %w", err))
		}
		return
	}

	// Build a map of conditionID -> positions
	conditionMap := make(map[string][]polymarketdata.Position)
	for _, position := range positions {
		conditionMap[position.ConditionId] = append(conditionMap[position.ConditionId], position)
	}

	// Process each condition - look for complementary pairs
	for conditionID, positionsInCondition := range conditionMap {
		// Need at least 2 positions to merge
		if len(positionsInCondition) < 2 {
			continue
		}

		// Find the minimum balance across all positions in this condition
		minBalance := positionsInCondition[0].Size
		for _, pos := range positionsInCondition[1:] {
			if pos.Size.LessThan(minBalance) {
				minBalance = pos.Size
			}
		}

		// Check if amount meets threshold
		if minBalance.LessThan(config.MinMergeAmount) {
			continue
		}

		// Perform merge
		_, err := c.Merge(ctx, conditionID, minBalance)
		if err != nil {
			if config.OnError != nil {
				config.OnError(fmt.Errorf("failed to merge condition %s: %w", conditionID, err))
			}
			continue
		}

		// Trigger success callback
		if config.OnSuccess != nil {
			config.OnSuccess(conditionID, minBalance)
		}
	}
}
