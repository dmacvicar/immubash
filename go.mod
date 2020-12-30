module github.com/dmacvicar/immubash

go 1.15

require (
	github.com/codenotary/immudb v0.8.1
	github.com/google/uuid v1.1.2 // indirect
	github.com/iovisor/gobpf v0.0.0-20200614202714-e6b321d32103
	github.com/renstrom/shortuuid v3.0.0+incompatible
	google.golang.org/grpc v1.34.0
)

replace github.com/iovisor/gobpf v0.0.0-20200614202714-e6b321d32103 => github.com/dmacvicar/gobpf v0.0.0-20201119134629-33d7cfb78ad2
