module github.com/nebula-strike/nebula-server

go 1.21

// Match heroiclabs/nakama v3.21.1 so the .so plugin loads (protobuf + nakama-common must align)
require (
	github.com/heroiclabs/nakama-common v1.31.0
	google.golang.org/protobuf v1.31.0 // indirect
)
