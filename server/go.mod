module github.com/lucap9056/corvauth/server

go 1.27.0

require (
	github.com/DATA-DOG/go-sqlmock v1.5.2
	github.com/golang-jwt/jwt/v5 v5.3.1
	github.com/jackc/pgx/v5 v5.8.0
	github.com/lucap9056/corvauth/database v0.2.0
	github.com/lucap9056/corvauth/jwt v0.1.0
	github.com/lucap9056/corvauth/oauth2 v0.0.0
	github.com/lucap9056/go-lifecycle/v2 v2.0.0
	github.com/maypok86/otter/v2 v2.3.0
	github.com/redis/rueidis v1.0.78
	golang.org/x/oauth2 v0.37.0
	golang.org/x/sync v0.22.0
)

require (
	cloud.google.com/go/compute/metadata v0.3.0 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/puddle/v2 v2.2.2 // indirect
	github.com/kr/text v0.2.0 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/ravener/discord-oauth2 v0.0.0-20230514095040-ae65713199b3 // indirect
	github.com/rogpeppe/go-internal v1.16.0 // indirect
	github.com/stretchr/testify v1.11.1 // indirect
	golang.org/x/sys v0.47.0 // indirect
	golang.org/x/text v0.41.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/lucap9056/corvauth/oauth2 => ../oauth2
