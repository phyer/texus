package core

import (
	"context"
)

type Core struct {
	// Add any necessary fields here
}

func (c *Core) GetScoreList(count int) []string {
	// Implementation of GetScoreList
}

func (c *Core) GetScoreListWithContext(ctx context.Context, count int) ([]string, error) {
	done := make(chan []string, 1)
	errChan := make(chan error, 1)

	go func() {
		list := c.GetScoreList(count)
		select {
		case done <- list:
		case <-ctx.Done():
		}
	}()

	select {
	case list := <-done:
		return list, nil
	case err := <-errChan:
		return nil, err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}
