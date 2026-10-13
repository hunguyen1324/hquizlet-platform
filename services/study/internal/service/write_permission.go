package service

import "context"

type studyWriteKey struct{}
type studyWritePermission struct{ userID, setID int64 }

// WithStudyWritePermission carries a server-evaluated resource permission across
// the handler/service boundary. Call only after authz.Can succeeds for this write.
func WithStudyWritePermission(ctx context.Context, userID, setID int64) context.Context {
	return context.WithValue(ctx, studyWriteKey{}, studyWritePermission{userID, setID})
}

func hasStudyWritePermission(ctx context.Context, userID, setID int64) bool {
	grant, ok := ctx.Value(studyWriteKey{}).(studyWritePermission)
	return ok && userID > 0 && setID > 0 && grant.userID == userID && grant.setID == setID
}
