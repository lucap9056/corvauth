package state

import (
	"context"
	"testing"
	"time"
)

func TestMemoryCache_TakeIsSingleUse(t *testing.T) {
	ctx := context.Background()
	c, err := NewCache(nil)
	if err != nil {
		t.Fatalf("NewCache: %v", err)
	}

	want := Session{Verifier: "v1", RedirectURL: "https://app.example/callback"}
	if err := c.Set(ctx, "s1", want); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if got, _ := c.Take(ctx, "s1"); got != want {
		t.Fatalf("Take = %+v; want %+v", got, want)
	}
	if got, _ := c.Take(ctx, "s1"); got != (Session{}) {
		t.Fatalf("second Take = %+v; want empty", got)
	}
}

func TestMemoryCache_Expires(t *testing.T) {
	ctx := context.Background()
	c, err := NewCache(nil, WithTTL(50*time.Millisecond))
	if err != nil {
		t.Fatalf("NewCache: %v", err)
	}

	c.Set(ctx, "s1", Session{Verifier: "v1"})
	time.Sleep(100 * time.Millisecond)

	if got, _ := c.Take(ctx, "s1"); got != (Session{}) {
		t.Fatalf("Take after TTL = %+v; want empty", got)
	}
}

func TestNewCache_InvalidMaximumSize(t *testing.T) {
	if _, err := NewCache(nil, WithMaximumSize(0)); err == nil {
		t.Fatal("expected error for zero maximum size")
	}
}
