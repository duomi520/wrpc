package wrpc

import (
	"errors"
	"sync/atomic"
	"time"

	"github.com/duomi520/utils"
)

//Limiter 限流器 Token Bucket(令牌桶)
//每隔一段时间加入一批令牌，达到上限后，不再增加。
type TokenBucketLimiter struct {
	//限流器速率，每秒处理的令牌数
	LimitRate int64
	//限流器大小，存放令牌的最大值
	LimitSize int64
	//加入的时间间隔
	Snippet time.Duration
	//令牌
	tokens int64
	//定时执行
	guardian *utils.Guardian
	//退出标志 1-退出
	stopFlag int32
	logger   utils.ILogger
}

func (t *TokenBucketLimiter) limit(b []byte, w WriterFunc) ([]byte, error) {
	err := t.Take(1)
	return b, err
}

//NewTokenBucketLimiter limitRate, limitSize,snippet数值较小时，准确度低。
func NewTokenBucketLimiter(limitRate, limitSize int64, snippet time.Duration, l utils.ILogger) *TokenBucketLimiter {
	t := &TokenBucketLimiter{
		LimitRate: limitRate,
		LimitSize: limitSize,
		Snippet:   snippet,
		tokens:    limitSize,

		stopFlag: 0,
		logger:   l,
	}
	if t.Snippet == 0 {
		//默认0.1秒
		t.Snippet = 100 * time.Millisecond
	}
	t.guardian = utils.NewGuardian(snippet, l)
	t.guardian.AddJob(t.run)
	return t
}

//Wait 等待,申请n个令牌，取不到足够数量时返回错误。
func (t *TokenBucketLimiter) Take(n int64) error {
	if atomic.LoadInt32(&t.stopFlag) == 1 {
		return nil
	}
	new := atomic.AddInt64(&t.tokens, -n)
	if new > -1 {
		return nil
	}
	return errors.New("TokenBucketLimiter.Take：被限流")
}

func (t *TokenBucketLimiter) run() bool {
	if atomic.LoadInt32(&t.stopFlag) == 1 {
		return true
	}
	//竟态下，牺牲准确度。
	new := atomic.AddInt64(&t.tokens, t.LimitRate)
	if new > t.LimitSize {
		atomic.StoreInt64(&t.tokens, t.LimitSize)
	}
	if new < t.LimitRate {
		atomic.StoreInt64(&t.tokens, t.LimitRate)
	}
	return false
}

//Close 关闭。
func (t *TokenBucketLimiter) Close() {
	atomic.StoreInt32(&t.stopFlag, 1)
	t.guardian.Release()
}

// https://golang.org/x/time/rate
// https://studygolang.com/articles/27454#reply0
// https://zhuanlan.zhihu.com/p/89820414
// https://github.com/juju/ratelimit
// https://github.com/uber-go/ratelimit
// https://mp.weixin.qq.com/s/T_LvVfAOzgANO1XSCViJrw
// https://mp.weixin.qq.com/s/YCvUTwpe0jUdwcKyQQj7hA

// TODO
// https://pandaychen.github.io/2020/07/12/KRATOS-LIMITER/
// https://github.com/alibaba/Sentinel/wiki/%E7%B3%BB%E7%BB%9F%E8%87%AA%E9%80%82%E5%BA%94%E9%99%90%E6%B5%81
// https://segmentfault.com/a/1190000041950209
// https://my.oschina.net/u/4545365/blog/5281780
