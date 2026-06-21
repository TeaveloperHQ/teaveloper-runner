package ui

import _ "embed"

// 트레이 상태 아이콘(.ico). 빌드에 함께 포함된다.
var (
	//go:embed assets/connected.ico
	iconConnected []byte
	//go:embed assets/disconnected.ico
	iconDisconnected []byte
	//go:embed assets/connecting.ico
	iconConnecting []byte
	//go:embed assets/localdown.ico
	iconLocalDown []byte
)
