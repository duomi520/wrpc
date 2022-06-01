package wrpc

import "time"

//DefaultDeadlineDuration IO超时
const DefaultDeadlineDuration = 10 * time.Second

//DefaultHeartbeatDuration 心跳包
const DefaultHeartbeatDuration time.Duration = time.Second * 5

//DefaultHeartbeatPeriodDuration 心跳包间距
const DefaultHeartbeatPeriodDuration time.Duration = (DefaultHeartbeatDuration * 9) / 10

type favContextKey string

var ContextKey favContextKey = "metadata"
