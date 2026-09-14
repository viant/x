package module

import (
	"context"
	"testing"
)

func TestSelectedWorkspaceValidatesPatterns(t *testing.T) {
	workspace := (&BuildSelection{}).Workspace()
	for _, tc := range []struct {
		name             string
		include, exclude []string
	}{
		{"missing include", nil, nil},
		{"empty include", []string{" "}, nil},
		{"malformed include", []string{"["}, nil},
		{"empty exclude", []string{"example.com/app/..."}, []string{""}},
		{"malformed exclude", []string{"example.com/app/..."}, []string{"["}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := workspace.Walk(context.Background(), tc.include, tc.exclude, func(File) error { return nil }); err == nil {
				t.Fatal("invalid selection accepted")
			}
		})
	}
	if err := workspace.Walk(context.Background(), []string{"example.com/app/..."}, nil, nil); err == nil {
		t.Fatal("nil visitor accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := workspace.Walk(ctx, []string{"example.com/app/..."}, nil, func(File) error { return nil }); err != context.Canceled {
		t.Fatalf("cancellation: %v", err)
	}
}
