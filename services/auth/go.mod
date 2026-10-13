module github.com/hunguyen1324/hquizlet-platform/services/auth

go 1.22

require (
	github.com/hunguyen1324/hquizlet-platform/packages/authz v0.0.0
	github.com/jackc/pgx/v5 v5.7.2
	golang.org/x/crypto v0.31.0
)

replace github.com/hunguyen1324/hquizlet-platform/packages/authz => ../../packages/authz
