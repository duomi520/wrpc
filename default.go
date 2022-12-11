package wrpc

import (
	"runtime"

	"github.com/duomi520/utils"
)

var tcpClientGuardian *utils.Guardian

//Default
func Default() {
	logger, _ := utils.NewWLogger(utils.ErrorLevel, "")
	tcpClientGuardian = utils.NewGuardian(DefaultHeartbeatDuration, logger)
}

//Stop
func Stop() {
	tcpClientGuardian.Release()
}

func formatRecover() ([]byte, any) {
	if r := recover(); r != nil {
		const size = 65536
		buf := make([]byte, size)
		end := runtime.Stack(buf, false)
		if end > size {
			end = size
		}
		return buf[:end], r
	}
	return nil, nil
}
