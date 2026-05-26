package e2b

import (
	"context"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestPaginatorNextItemsErrorsWhenExhausted(t *testing.T) {
	paginator := newPaginator(func(ctx context.Context, nextToken string) ([]int, string, error) {
		return []int{1}, "", nil
	})

	items, err := paginator.NextItems()
	if err != nil {
		t.Fatalf("unexpected error on first page: %v", err)
	}
	if len(items) != 1 || items[0] != 1 {
		t.Fatalf("unexpected items: %#v", items)
	}
	if paginator.HasNext {
		t.Fatal("expected paginator to be exhausted after page without next token")
	}

	_, err = paginator.NextItems()
	if err == nil {
		t.Fatal("expected error when fetching past the end of paginator")
	}
	if err.Error() != "No more items to fetch" {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestPaginatorNextItemsDoesNotExposeContextParameter(t *testing.T) {
	paginatorType := reflect.TypeOf(&paginator[int]{})
	method, ok := paginatorType.MethodByName("NextItems")
	if !ok {
		t.Fatal("expected paginator.NextItems to exist")
	}
	if method.Type.NumIn() != 1 {
		t.Fatalf("expected paginator.NextItems to accept no arguments, got %d inputs", method.Type.NumIn()-1)
	}
}

func TestPaginatorConstructorsAreNotExported(t *testing.T) {
	source, err := os.ReadFile("paginator.go")
	if err != nil {
		t.Fatalf("failed to read paginator.go: %v", err)
	}

	text := string(source)
	if strings.Contains(text, "func NewPaginator(") {
		t.Fatal("did not expect NewPaginator to be exported")
	}
	if strings.Contains(text, "func NewPaginatorWithInitialToken(") {
		t.Fatal("did not expect NewPaginatorWithInitialToken to be exported")
	}
	if strings.Contains(text, "type Paginator[") {
		t.Fatal("did not expect Paginator to be exported")
	}
	if !strings.Contains(text, "type SandboxPaginator struct") {
		t.Fatal("expected SandboxPaginator to be exported")
	}
	if !strings.Contains(text, "type SnapshotPaginator struct") {
		t.Fatal("expected SnapshotPaginator to be exported")
	}
}
