module github.com/forkbikash/daytona/libs/sdk-go

go 1.21

require (
	github.com/forkbikash/daytona/libs/api-client-go v0.3.0
	github.com/gorilla/websocket v1.5.3
	github.com/joho/godotenv v1.5.1
)

// For local development within the monorepo.
// This directive is ignored when the module is fetched from a remote repository.
replace github.com/forkbikash/daytona/libs/api-client-go => ../api-client-go
