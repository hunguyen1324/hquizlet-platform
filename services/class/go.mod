module github.com/hunguyen1324/hquizlet-platform/services/class

go 1.23.0

require (
	github.com/hunguyen1324/hquizlet-platform/packages/authz v0.0.0
	github.com/DATA-DOG/go-sqlmock v1.5.2
	github.com/jackc/pgx/v5 v5.7.2
	github.com/nats-io/nats.go v1.37.0
	github.com/pkg/sftp v1.13.10
	golang.org/x/crypto v0.41.0
)

replace github.com/hunguyen1324/hquizlet-platform/packages/authz => ../../packages/authz

require (
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/puddle/v2 v2.2.2 // indirect
	github.com/klauspost/compress v1.17.2 // indirect
	github.com/kr/fs v0.1.0 // indirect
	github.com/nats-io/nkeys v0.4.7 // indirect
	github.com/nats-io/nuid v1.0.1 // indirect
	golang.org/x/sync v0.16.0 // indirect
	golang.org/x/sys v0.35.0 // indirect
	golang.org/x/text v0.28.0 // indirect
)
