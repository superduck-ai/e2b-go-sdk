package e2b

import "context"

// Paginator provides a generic way to iterate over paginated API results.
type Paginator[T any] struct {
	HasNext   bool
	NextToken string
	fetchFn   func(ctx context.Context, nextToken string) ([]T, string, error)
}

// NewPaginator creates a new paginator with the given fetch function.
func NewPaginator[T any](fetchFn func(ctx context.Context, nextToken string) ([]T, string, error)) *Paginator[T] {
	return &Paginator[T]{HasNext: true, fetchFn: fetchFn}
}

// NewPaginatorWithInitialToken creates a paginator with an initial next token.
func NewPaginatorWithInitialToken[T any](fetchFn func(ctx context.Context, nextToken string) ([]T, string, error), initialToken string) *Paginator[T] {
	return &Paginator[T]{HasNext: true, NextToken: initialToken, fetchFn: fetchFn}
}

// NextItems fetches the next page of items. Returns nil when no more pages are available.
func (p *Paginator[T]) NextItems(ctx context.Context) ([]T, error) {
	if !p.HasNext {
		return nil, nil
	}
	items, nextToken, err := p.fetchFn(ctx, p.NextToken)
	if err != nil {
		return nil, err
	}
	p.NextToken = nextToken
	p.HasNext = nextToken != ""
	return items, nil
}
