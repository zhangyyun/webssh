module github.com/myml/webssh

go 1.12

require (
	github.com/gorilla/websocket v1.4.3-0.20220104015952-9111bb834a68
	github.com/nacos-group/nacos-sdk-go v1.0.9
	github.com/pkg/errors v0.9.1
	github.com/spf13/cobra v1.3.0
	golang.org/x/crypto v0.0.0-20211108221036-ceb1ce70b4fa
	golang.org/x/sys v0.0.0-20211214234402-4825e8c3871d // indirect
)

replace github.com/gorilla/websocket v1.4.3-0.20220104015952-9111bb834a68 => github.com/zhangyyun/websocket v1.4.3-0.20220211023552-0a0759617553
