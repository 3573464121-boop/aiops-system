package app

import "context"

type actorContextKey string

const (
	actorIDKey       actorContextKey = "actor_id"
	actorUsernameKey actorContextKey = "actor_username"
	actorRoleKey     actorContextKey = "actor_role"
)

func WithActor(ctx context.Context, userID, username, role string) context.Context {
	ctx = context.WithValue(ctx, actorIDKey, userID)
	ctx = context.WithValue(ctx, actorUsernameKey, username)
	ctx = context.WithValue(ctx, actorRoleKey, role)
	return ctx
}

func actorFromContext(ctx context.Context) (userID, username, role string) {
	if ctx == nil {
		return "", "", ""
	}
	userID, _ = ctx.Value(actorIDKey).(string)
	username, _ = ctx.Value(actorUsernameKey).(string)
	role, _ = ctx.Value(actorRoleKey).(string)
	return userID, username, role
}
