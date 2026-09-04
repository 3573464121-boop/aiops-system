package app

import (
	"context"
	"testing"

	"aiops-mvp/internal/domain"
	"aiops-mvp/internal/tools"
)

func TestMemoryScopeIsolation(t *testing.T) {
	service := New(tools.NewService(nil, nil, nil), nil)
	alice := WithActorTeam(context.Background(), "USR-alice", "alice", "viewer", "team-a")
	bob := WithActorTeam(context.Background(), "USR-bob", "bob", "viewer", "team-a")
	eve := WithActorTeam(context.Background(), "USR-eve", "eve", "viewer", "team-b")
	admin := WithActorTeam(context.Background(), "USR-admin", "admin", "admin", "team-a")

	personal, err := service.CreateMemory(alice, domain.MemoryRequest{Scope: "personal", Content: "alice private database note"})
	if err != nil {
		t.Fatal(err)
	}
	team, err := service.CreateMemory(alice, domain.MemoryRequest{Scope: "team", Content: "team-a shared runbook"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.CreateMemory(admin, domain.MemoryRequest{Scope: "global", Content: "global operations policy"}); err != nil {
		t.Fatal(err)
	}
	if _, err = service.CreateMemory(admin, domain.MemoryRequest{Scope: "product", ProductID: "payment", Content: "payment timeout baseline"}); err != nil {
		t.Fatal(err)
	}

	assertMemoryCount(t, service, alice, 4)
	assertMemoryCount(t, service, bob, 3)
	assertMemoryCount(t, service, eve, 2)

	if err = service.DeleteMemory(bob, personal.ID); err == nil {
		t.Fatal("another user must not delete a personal memory")
	}
	if err = service.DeleteMemory(bob, team.ID); err == nil {
		t.Fatal("a non-owner viewer must not delete a team memory")
	}
	if err = service.DeleteMemory(admin, team.ID); err != nil {
		t.Fatalf("team admin should delete a team memory: %v", err)
	}
	if _, err = service.CreateMemory(bob, domain.MemoryRequest{Scope: "global", Content: "forbidden"}); err == nil {
		t.Fatal("viewer must not create a global memory")
	}
	if got := service.recallMemories(eve, "payment", "alice private database note", 10); containsMemory(got, personal.ID) {
		t.Fatal("personal memory leaked into another user's recall")
	}
}

func assertMemoryCount(t *testing.T, service *Service, ctx context.Context, want int) {
	t.Helper()
	got, err := service.ListMemories(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != want {
		t.Fatalf("memory count: want %d, got %d (%+v)", want, len(got), got)
	}
}

func containsMemory(memories []domain.Memory, id string) bool {
	for _, memory := range memories {
		if memory.ID == id {
			return true
		}
	}
	return false
}
