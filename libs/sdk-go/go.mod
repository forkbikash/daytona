module github.com/daytonaio/sdk-go

go 1.21

require (
	github.com/daytonaio/apiclient v0.0.0-00010101000000-000000000000
	github.com/gorilla/websocket v1.5.3
	github.com/joho/godotenv v1.5.1
)

// For local development within the monorepo.
// This directive is ignored when the module is fetched from a remote repository.
replace github.com/daytonaio/apiclient => ../api-client-go
