package wrpc

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/duomi520/utils"
)

func BenchmarkWait(b *testing.B) {
	logger, _ := utils.NewWLogger(utils.ErrorLevel, "")
	t := NewTokenBucketLimiter(1000, 16*1024, 10*time.Millisecond, logger)
	time.Sleep(100 * time.Millisecond)
	for i := 0; i < b.N; i++ {
		t.Take(1)
	}
}

func TestWait(t *testing.T) {
	logger, _ := utils.NewWLogger(utils.ErrorLevel, "")
	defer logger.Close()
	limiter := NewTokenBucketLimiter(1, 1, 10*time.Millisecond, logger)
	defer limiter.Close()
	for i := 0; i < 10; i++ {
		prev := time.Now()
		if err := limiter.Take(1); err != nil {
			time.Sleep(10 * time.Millisecond)
		}
		fmt.Println(i, time.Since(prev), limiter.tokens)
	}
}

/*
0 0s 0
1 19.1924ms 1
2 0s 0
3 15.5182ms -1
4 0s 0
5 15.2105ms -1
6 0s 0
7 15.2859ms -1
8 0s 0
9 15.258ms 1
*/
func TestLimiter(t *testing.T) {
	Default()
	defer Stop()
	logger, _ := utils.NewWLogger(utils.ErrorLevel, "")
	defer logger.Close()
	limiter := NewTokenBucketLimiter(100, 10000, 10*time.Millisecond, logger)
	defer limiter.Close()
	o := NewOptions(WithLogger(logger), WithIntletHook(limiter.limit))
	s := NewService(o)
	s.TCPServer(":4567")
	defer s.Stop()
	hi := new(hi)
	s.RegisterRPC("hi", hi)
	c, err := NewTCPClient("127.0.0.1:4567", NewOptions(WithLogger(logger)))
	if err != nil {
		t.Fatal(err.Error())
	}
	for j := 1; j < 10; j++ {
		prev := time.Now()
		accept, reject := 0, 0
		for i := 0; i < 10000; i++ {
			var reply string
			if err := c.Call(context.TODO(), "hi.SayHello", "linda", &reply); err != nil {
				reject++
				d := j * 1000000
				time.Sleep(time.Duration(d))
			}
			accept++
		}
		fmt.Println(j, time.Since(prev), accept, reject)
	}
	c.Close()
}

/*
1 403.2754ms 10000 0
2 835.2098ms 10000 38
3 1.6915661s 10000 99
4 1.8195403s 10000 100
5 1.7860291s 10000 99
6 1.8188981s 10000 99
7 1.8755421s 10000 99
8 1.8108858s 10000 92
9 1.8412894s 10000 100
*/
func TestTake(t *testing.T) {
	Default()
	defer Stop()
	logger, _ := utils.NewWLogger(utils.ErrorLevel, "")
	defer logger.Close()
	limiter := NewTokenBucketLimiter(1, 1, 1000*time.Millisecond, logger)
	defer limiter.Close()
	o := NewOptions(WithLogger(logger), WithIntletHook(limiter.limit))
	s := NewService(o)
	s.TCPServer(":4567")
	defer s.Stop()
	hi := new(hi)
	s.RegisterRPC("hi", hi)
	c, err := NewTCPClient("127.0.0.1:4567", NewOptions(WithLogger(logger)))
	if err != nil {
		t.Fatal(err.Error())
	}
	limiter.Take(10)
	var reply string
	if err := c.Call(context.TODO(), "hi.SayHello", "linda", &reply); err != nil {
		fmt.Println(err.Error())
	} else {
		t.Fatal("未拦截")
	}
	c.Close()
}

// TokenBucketLimiter.Take：被限流
