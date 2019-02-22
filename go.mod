module github.com/jorgemarey/conoxy

require (
	github.com/DataDog/datadog-go v0.0.0-20180822151419-281ae9f2d895 // indirect
	github.com/armon/go-radix v1.0.0 // indirect
	github.com/circonus-labs/circonus-gometrics v2.2.6+incompatible // indirect
	github.com/circonus-labs/circonusllhist v0.1.3 // indirect
	github.com/hashicorp/consul v1.4.2
	github.com/hashicorp/go-retryablehttp v0.5.2 // indirect
	github.com/hashicorp/go-rootcerts v1.0.0 // indirect
	github.com/hashicorp/go-version v1.1.0 // indirect
	github.com/hashicorp/hcl v1.0.0 // indirect
	github.com/hashicorp/raft v1.0.0 // indirect
	github.com/hashicorp/serf v0.8.2 // indirect
	github.com/hashicorp/yamux v0.0.0-20181012175058-2f1d1f20f75d // indirect
	github.com/mitchellh/copystructure v1.0.0 // indirect
	github.com/mitchellh/go-testing-interface v1.0.0 // indirect
	github.com/mitchellh/hashstructure v1.0.0 // indirect
	github.com/mitchellh/mapstructure v1.1.2
	github.com/pkg/errors v0.8.1 // indirect
	github.com/tv42/httpunix v0.0.0-20150427012821-b75d8614f926 // indirect
	golang.org/x/crypto v0.0.0-20190128193316-c7b33c32a30b // indirect
)

# We need a change on consul to close the listener on close proxy
replace github.com/hashicorp/consul => ../../hashicorp/consul
