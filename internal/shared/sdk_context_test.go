package shared

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestMergeContextsCancelsWhenSecondaryCancels(t *testing.T) {
	primary, cancelPrimary := context.WithCancel(context.Background())
	defer cancelPrimary()

	secondary, cancelSecondary := context.WithCancel(context.Background())
	ctx, cancel := MergeContexts(primary, secondary)
	defer cancel()

	cancelSecondary()

	select {
	case <-ctx.Done():
		if !errors.Is(ctx.Err(), context.Canceled) {
			t.Fatalf("expected merged context to be canceled, got %v", ctx.Err())
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for merged context cancellation")
	}
}

func TestMergeContextsIsAlreadyCanceledWhenSecondaryPreCanceled(t *testing.T) {
	secondary, cancelSecondary := context.WithCancel(context.Background())
	cancelSecondary()

	ctx, cancel := MergeContexts(context.Background(), secondary)
	defer cancel()

	if !errors.Is(ctx.Err(), context.Canceled) {
		t.Fatalf("expected pre-canceled secondary to produce canceled context, got %v", ctx.Err())
	}
}

func TestMergeContextsUsesEarlierDeadline(t *testing.T) {
	now := time.Now()
	primary, cancelPrimary := context.WithDeadline(context.Background(), now.Add(2*time.Second))
	defer cancelPrimary()

	secondary, cancelSecondary := context.WithDeadline(context.Background(), now.Add(time.Second))
	defer cancelSecondary()

	ctx, cancel := MergeContexts(primary, secondary)
	defer cancel()

	deadline, ok := ctx.Deadline()
	if !ok {
		t.Fatal("expected merged context to have a deadline")
	}
	if !deadline.Equal(now.Add(time.Second)) {
		t.Fatalf("expected earliest deadline to win, got %s", deadline)
	}
}

func TestMergeContextsUsesSecondaryWhenPrimaryIsNil(t *testing.T) {
	secondary, cancelSecondary := context.WithCancel(context.Background())
	ctx, cancel := MergeContexts(nil, secondary)
	defer cancel()

	cancelSecondary()

	select {
	case <-ctx.Done():
		if !errors.Is(ctx.Err(), context.Canceled) {
			t.Fatalf("expected merged context to be canceled, got %v", ctx.Err())
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for merged context cancellation")
	}
}

func TestMergeContextsCancelFuncCancelsMergedContext(t *testing.T) {
	ctx, cancel := MergeContexts(context.Background(), context.Background())
	cancel()

	if !errors.Is(ctx.Err(), context.Canceled) {
		t.Fatalf("expected cancel func to cancel merged context, got %v", ctx.Err())
	}
}

func TestMergeContextsPreservesPrimaryContextValues(t *testing.T) {
	type contextKey string

	primary := context.WithValue(context.Background(), contextKey("trace_id"), "trace-123")
	secondary, cancelSecondary := context.WithCancel(context.Background())
	defer cancelSecondary()

	ctx, cancel := MergeContexts(primary, secondary)
	defer cancel()

	if got := ctx.Value(contextKey("trace_id")); got != "trace-123" {
		t.Fatalf("expected merged context to preserve primary values, got %#v", got)
	}
}
