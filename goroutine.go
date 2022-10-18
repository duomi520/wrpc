package wrpc

import (
	"github.com/duomi520/utils"
	"github.com/panjf2000/ants/v2"
)

func wgo(task func()) {
	ants.Submit(task)
}

var defaultGuardian *utils.Guardian

//StartGuardian
func StartGuardian() {
	defaultGuardian = utils.NewGuardian(DefaultHeartbeatDuration)
	go defaultGuardian.Run()
}

//StopGuardian
func StopGuardian() {
	defaultGuardian.Release()
}

// https://github.com/panjf2000/ants/blob/master/README_ZH.md
// https://blog.csdn.net/darjun/article/details/117576252
