module toollab-core

go 1.26.1

require (
	github.com/devpablocristo/core/ai/go v0.0.0
	github.com/devpablocristo/core/errors/go v0.0.0
	github.com/devpablocristo/core/http/go v0.0.0
	github.com/devpablocristo/core/security/go v0.0.0
	github.com/go-chi/chi/v5 v5.2.5
	github.com/google/uuid v1.6.0
	github.com/mattn/go-sqlite3 v1.14.34
	golang.org/x/oauth2 v0.35.0
	golang.org/x/tools v0.42.0
)

require (
	cloud.google.com/go/compute/metadata v0.3.0 // indirect
	golang.org/x/mod v0.33.0 // indirect
	golang.org/x/sync v0.19.0 // indirect
)

replace github.com/devpablocristo/core/ai/go => ../../core/ai/go

replace github.com/devpablocristo/core/errors/go => ../../core/errors/go

replace github.com/devpablocristo/core/http/go => ../../core/http/go

replace github.com/devpablocristo/core/security/go => ../../core/security/go
