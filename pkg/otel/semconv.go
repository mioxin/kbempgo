package otel

import (
	"go.opentelemetry.io/otel/attribute"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
)

var (
	DBSystemPrometheus = semconv.DBSystemKey.String("prometheus")
	DBSystemEtcd       = semconv.DBSystemKey.String("etcd")
	DBSystemVault      = semconv.DBSystemKey.String("vault")
)

var (
	DBKeyKey          = attribute.Key("db.key")
	IdentityUser      = attribute.Key("identity.user")
	IdentityUserID    = attribute.Key("identity.user_id")
	IdentityProject   = attribute.Key("identity.project")
	IdentityProjectID = attribute.Key("identity.project_id")
)

// Must wrapper for functions returning (ret, err)
func Must[T any](ret T, err error) T {
	if err != nil {
		panic(err)
	}
	return ret
}
