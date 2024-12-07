module phyer.click/texus

replace (
	v5sdk_go/config => ./submodules/okex/config
	v5sdk_go/rest => ./submodules/okex/rest
	v5sdk_go/utils => ./submodules/okex/utils
	v5sdk_go/ws => ./submodules/okex/ws
)

go 1.21

require (
	github.com/bitly/go-simplejson v0.5.0
	github.com/go-redis/redis v6.15.9+incompatible
	github.com/sirupsen/logrus v1.9.3
	v5sdk_go/rest v0.0.0-00010101000000-000000000000
	v5sdk_go/ws v0.0.0-00010101000000-000000000000
)

require (
	github.com/bmizerany/assert v0.0.0-20160611221934-b7ed37b82869 // indirect
	github.com/gorilla/websocket v1.5.1 // indirect
	github.com/kr/pretty v0.3.0 // indirect
	github.com/onsi/ginkgo v1.16.4 // indirect
	github.com/onsi/gomega v1.16.0 // indirect
	golang.org/x/net v0.17.0 // indirect
	golang.org/x/sys v0.13.0 // indirect
	v5sdk_go/config v0.0.0-00010101000000-000000000000 // indirect
	v5sdk_go/utils v0.0.0-00010101000000-000000000000 // indirect
)
