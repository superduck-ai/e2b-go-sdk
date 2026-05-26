package e2b

import (
	"context"
	"fmt"
)

type paginator[T any] struct {
	HasNext   bool
	NextToken string
	fetchFn   func(ctx context.Context, nextToken string) ([]T, string, error)
}

// SandboxPaginator iterates over paginated sandbox results.
type SandboxPaginator struct {
	*paginator[SandboxInfo]
}

// SnapshotPaginator iterates over paginated snapshot results.
type SnapshotPaginator struct {
	*paginator[SnapshotInfo]
}

// newPaginator creates a new paginator with the given fetch function.
func newPaginator[T any](fetchFn func(ctx context.Context, nextToken string) ([]T, string, error)) *paginator[T] {
	return &paginator[T]{HasNext: true, fetchFn: fetchFn}
}

// newPaginatorWithInitialToken creates a paginator with an initial next token.
func newPaginatorWithInitialToken[T any](fetchFn func(ctx context.Context, nextToken string) ([]T, string, error), initialToken string) *paginator[T] {
	return &paginator[T]{HasNext: true, NextToken: initialToken, fetchFn: fetchFn}
}

// NextItems fetches the next page of items.
func (p *paginator[T]) NextItems() ([]T, error) {
	if !p.HasNext {
		return nil, fmt.Errorf("No more items to fetch")
	}
	items, nextToken, err := p.fetchFn(context.Background(), p.NextToken)
	if err != nil {
		return nil, err
	}
	p.NextToken = nextToken
	p.HasNext = nextToken != ""
	return items, nil
}
