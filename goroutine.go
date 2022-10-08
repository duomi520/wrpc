package wrpc

import "github.com/panjf2000/ants/v2"

func wgo(task func()) {
	ants.Submit(task)
}

// https://github.com/panjf2000/ants/blob/master/README_ZH.md
// https://blog.csdn.net/darjun/article/details/117576252
