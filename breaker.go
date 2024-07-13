package wrpc

import (
	"errors"
	"math"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"
)

const (
	StatusBreakerClosed int32 = iota + 1
	StatusBreakerOpen
	StatusBreakerHalfOpen
)

//回路状态
//关闭状态：服务正常，并维护一个失败率统计，当失败率达到阀值时，转到开启状态
//开启状态：服务异常，在该状态下，发起请求时会立即返回错误
//半开启装态：这时熔断器只允许一个请求通过. 当该请求调用成功时, 熔断器恢复到关闭状态. 若该请求失败, 熔断器继续保持打开状态, 接下来的请求被禁止通过

// CircuitBreaker 回路 SRE过载保护算法
// 降低 K 值会使自适应限流算法更加激进（允许客户端在算法启动时拒绝更多本地请求）
// 增加 K 值会使自适应限流算法不再那么激进（允许服务端在算法启动时尝试接收更多的请求，与上面相反）
type CircuitBreaker struct {
	r        *rand.Rand
	randLock sync.Mutex
	stat     *breakerRollingWindow
	// 比率
	k float64
	// 触发熔断的最少请求数量（请求少于该值时不会触发熔断）
	request int64
	state   int32
}

// NewCircuitBreakcer 新加回路
func NewCircuitBreaker(request int64, k float64) *CircuitBreaker {
	b := &breakerRollingWindow{
		//64*8.388608=536.870912 ms
		interval: 64,
	}
	c := &CircuitBreaker{
		r:       rand.New(rand.NewSource(time.Now().UnixNano())),
		stat:    b,
		k:       k,
		request: request,
		state:   StatusBreakerOpen,
	}
	return c
}
func (c *CircuitBreaker) AllowRequest() error {
	success, total := c.stat.history()
	k := c.k * float64(success)
	//check overflow requests = K * success
	if total < c.request || float64(total) < k {
		// total <= K * success 时，关闭熔断；
		if atomic.LoadInt32(&c.state) == StatusBreakerOpen {
			atomic.CompareAndSwapInt32(&c.state, StatusBreakerOpen, StatusBreakerClosed)
		}
		return nil
	}
	// 否则，熔断器开启（说明 total > K * success）
	if atomic.LoadInt32(&c.state) == StatusBreakerClosed {
		atomic.CompareAndSwapInt32(&c.state, StatusBreakerClosed, StatusBreakerOpen)
	}
	// 根据 googleSRE 的算法计算拿到客户端请求拒绝的概率
	dr := math.Max(0, (float64(total)-k)/float64(total+1))
	c.randLock.Lock()
	drop := c.r.Float64() < dr
	c.randLock.Unlock()
	if drop {
		return errors.New("CircuitBreaker.AllowRequest: 熔断器开启")
	}
	return nil
}

func (c *CircuitBreaker) MarkSuccess() {
	c.stat.add(1)
}

func (c *CircuitBreaker) MarkFailed() {
	c.stat.add(0)
}

// 环形窗口
// 最小样本窗口 1024*1024*8=8,388,608 约8ms
// 512个窗口 512*8,388,608=4,294,967,296 约4.3秒
// 2^3=8,2^9=512,2^10=1024

// breakerRollingWindow
type breakerRollingWindow struct {
	sum   [512]int64
	count [512]int64
	round [512]int64
	//间隔几个样本窗口
	interval int64
}

func (b *breakerRollingWindow) add(n int64) {
	now := time.Now().UnixNano()
	//offset := now / 4294967296
	offset := now >> (10 + 10 + 3 + 9)
	//index := now % 4294967296 / 8388608
	index := (now & (4294967296 - 1)) >> (10 + 10 + 3)
	if atomic.LoadInt64(&b.round[index]) == offset {
		atomic.AddInt64(&b.sum[index], n)
		atomic.AddInt64(&b.count[index], 1)
	} else {
		atomic.StoreInt64(&b.sum[index], n)
		atomic.StoreInt64(&b.count[index], 1)
		atomic.StoreInt64(&b.round[index], offset)
	}
}

func (b *breakerRollingWindow) history() (success, total int64) {
	now := time.Now().UnixNano()
	offset := now >> (10 + 10 + 3 + 9)
	index := (now & (4294967296 - 1)) >> (10 + 10 + 3)
	if index >= b.interval {
		for i := index - b.interval; i <= index; i++ {
			if atomic.LoadInt64(&b.round[i]) == offset {
				success = success + atomic.LoadInt64(&b.sum[i])
				total = total + atomic.LoadInt64(&b.count[i])
			}
		}
	} else {
		for i := int64(0); i <= index; i++ {
			if atomic.LoadInt64(&b.round[i]) == offset {
				success = success + atomic.LoadInt64(&b.sum[i])
				total = total + atomic.LoadInt64(&b.count[i])
			}
		}
		for i := 512 + index - b.interval; i < 512; i++ {
			if atomic.LoadInt64(&b.round[i]) == (offset - 1) {
				success = success + atomic.LoadInt64(&b.sum[i])
				total = total + atomic.LoadInt64(&b.count[i])
			}
		}
	}
	return success, total
}

// https://mp.weixin.qq.com/s/DGRnUhyv6SS_E36ZQKGpPA

// https://pandaychen.github.io/2020/05/10/A-GOOGLE-SRE-BREAKER/
// https://pandaychen.github.io/2020/07/12/KRATOS-BREAKER-ANALYSIS/
// https://zhuanlan.zhihu.com/p/433883634
// https://www.jianshu.com/p/9b07eb8a362c

// https://blog.csdn.net/qq_16399991/article/details/127239852
// https://github.com/zhoushuguang/go-zero/blob/master/core/breaker/googlebreaker.go
// https://github.com/zhoushuguang/go-zero/blob/e7acadb15df23bdcea84838f251bc66f2181d3bc/core/collection/rollingwindow.go#L15
