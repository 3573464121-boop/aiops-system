package auth

import (
	"testing"
	"time"
)

func TestIssueWithTeamRoundTrip(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	signer := NewSigner("test-secret", time.Hour)
	token, err := signer.IssueWithTeam("USR-1", "alice", "viewer", "team-a", now)
	if err != nil {
		t.Fatal(err)
	}
	claims, err := signer.Verify(token, now.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if claims.UserID != "USR-1" || claims.Username != "alice" || claims.Role != "viewer" || claims.TeamID != "team-a" {
		t.Fatalf("unexpected claims: %+v", claims)
	}
}

func TestLegacyIssueHasEmptyTeam(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	signer := NewSigner("test-secret", time.Hour)
	token, err := signer.Issue("USR-1", "alice", "viewer", now)
	if err != nil {
		t.Fatal(err)
	}
	claims, err := signer.Verify(token, now)
	if err != nil {
		t.Fatal(err)
	}
	if claims.TeamID != "" {
		t.Fatalf("legacy token team must be empty, got %q", claims.TeamID)
	}
}
