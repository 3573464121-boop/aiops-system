package app

import "context"

type actorContextKey string

const (
	actorIDKey       actorContextKey = "actor_id"
	actorUsernameKey actorContextKey = "actor_username"
	actorRoleKey     actorContextKey = "actor_role"
	actorTeamKey     actorContextKey = "actor_team"
)

func WithActor(ctx context.Context, userID, username, role string) context.Context {
	return WithActorTeam(ctx, userID, username, role, "")
}

func WithActorTeam(ctx context.Context, userID, username, role, teamID string) context.Context {
	ctx = context.WithValue(ctx, actorIDKey, userID)
	ctx = context.WithValue(ctx, actorUsernameKey, username)
	ctx = context.WithValue(ctx, actorRoleKey, role)
	ctx = context.WithValue(ctx, actorTeamKey, teamID)
	return ctx
}

func actorFromContext(ctx context.Context) (userID, username, role, teamID string) {
	if ctx == nil {
		return "", "", "", ""
	}
	userID, _ = ctx.Value(actorIDKey).(string)
	username, _ = ctx.Value(actorUsernameKey).(string)
	role, _ = ctx.Value(actorRoleKey).(string)
	teamID, _ = ctx.Value(actorTeamKey).(string)
	return userID, username, role, teamID
}
