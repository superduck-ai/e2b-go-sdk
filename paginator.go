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
	fetchWithOpts func(ctx context.Context, nextToken string, opts *SandboxApiOpts) ([]SandboxInfo, string, error)
}

// SnapshotPaginator iterates over paginated snapshot results.
type SnapshotPaginator struct {
	*paginator[SnapshotInfo]
	fetchWithOpts func(ctx context.Context, nextToken string, opts *SandboxApiOpts) ([]SnapshotInfo, string, error)
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
	return p.NextItemsContext(context.Background())
}

// NextItemsContext fetches the next page of items using the provided context.
func (p *paginator[T]) NextItemsContext(ctx context.Context) ([]T, error) {
	return p.nextItemsWithFetch(ctx, p.fetchFn)
}

func (p *paginator[T]) nextItemsWithFetch(ctx context.Context, fetchFn func(ctx context.Context, nextToken string) ([]T, string, error)) ([]T, error) {
	if !p.HasNext {
		return nil, fmt.Errorf("No more items to fetch")
	}
	items, nextToken, err := fetchFn(ctx, p.NextToken)
	if err != nil {
		return nil, err
	}
	p.NextToken = nextToken
	p.HasNext = nextToken != ""
	return items, nil
}

// NextItems fetches the next page of sandbox items. Optional per-call opts
// override the paginator's request config for this page only.
func (p *SandboxPaginator) NextItems(opts ...*SandboxApiOpts) ([]SandboxInfo, error) {
	return p.NextItemsContext(context.Background(), opts...)
}

// NextItemsContext fetches the next page of sandbox items using the provided
// context. Optional per-call opts override the paginator's request config for
// this page only.
func (p *SandboxPaginator) NextItemsContext(ctx context.Context, opts ...*SandboxApiOpts) ([]SandboxInfo, error) {
	fetchFn := p.paginator.fetchFn
	if p.fetchWithOpts != nil {
		fetchFn = func(ctx context.Context, nextToken string) ([]SandboxInfo, string, error) {
			return p.fetchWithOpts(ctx, nextToken, firstSandboxApiOpts(opts))
		}
	}
	return p.paginator.nextItemsWithFetch(ctx, fetchFn)
}

// NextItems fetches the next page of snapshot items. Optional per-call opts
// override the paginator's request config for this page only.
func (p *SnapshotPaginator) NextItems(opts ...*SandboxApiOpts) ([]SnapshotInfo, error) {
	return p.NextItemsContext(context.Background(), opts...)
}

// NextItemsContext fetches the next page of snapshot items using the provided
// context. Optional per-call opts override the paginator's request config for
// this page only.
func (p *SnapshotPaginator) NextItemsContext(ctx context.Context, opts ...*SandboxApiOpts) ([]SnapshotInfo, error) {
	fetchFn := p.paginator.fetchFn
	if p.fetchWithOpts != nil {
		fetchFn = func(ctx context.Context, nextToken string) ([]SnapshotInfo, string, error) {
			return p.fetchWithOpts(ctx, nextToken, firstSandboxApiOpts(opts))
		}
	}
	return p.paginator.nextItemsWithFetch(ctx, fetchFn)
}

func firstSandboxApiOpts(opts []*SandboxApiOpts) *SandboxApiOpts {
	if len(opts) == 0 {
		return nil
	}
	return opts[0]
}
