package wrpc

import (
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
	tokens   int64
	guardian *utils.Guardian
	//退出标志 1-退出
	stopFlag int32
	logger   utils.ILogger
}

func (t *TokenBucketLimiter) limit(b []byte, w WriterFunc) ([]byte, error) {
	t.Wait(1)
	return b, nil
}

func NewTokenBucketLimiter(limitRate, limitSize int64, snippet time.Duration, l utils.ILogger) *TokenBucketLimiter {
	t := &TokenBucketLimiter{
		LimitRate: limitRate,
		LimitSize: limitSize,
		Snippet:   snippet,
		tokens:    limitSize,
		stopFlag:  0,
		logger:    l,
	}
	if t.Snippet == 0 {
		//默认1秒
		t.Snippet = 1000 * time.Millisecond
	}
	t.guardian = utils.NewGuardian(snippet, l)
	t.guardian.AddJob(t.run)
	return t
}

//Wait 阻塞等待,申请n个令牌，取不到足够数量时阻塞。
func (t *TokenBucketLimiter) Wait(n int64) {
	for {
		if atomic.LoadInt32(&t.stopFlag) == 1 {
			return
		}
		new := atomic.AddInt64(&t.tokens, -n)
		if new > -1 {
			return
		}
		atomic.AddInt64(&t.tokens, n)
		time.Sleep(t.Snippet)
	}
}

func (t *TokenBucketLimiter) run() bool {
	if atomic.LoadInt32(&t.stopFlag) == 1 {
		return true
	}
	//竟态下，准确度低，影响不大。
	new := atomic.AddInt64(&t.tokens, t.LimitRate)
	if new > t.LimitSize {
		atomic.StoreInt64(&t.tokens, t.LimitSize)
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
