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
		t.Wait(1)
	}
}

func TestLimiter(t *testing.T) {
	Default()
	defer Stop()
	logger, _ := utils.NewWLogger(utils.ErrorLevel, "")
	defer logger.Close()
	limiter := NewTokenBucketLimiter(1, 1, 10*time.Millisecond, logger)
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
	for i := 0; i < 10; i++ {
		var reply string
		prev := time.Now()
		if err := c.Call(context.TODO(), "hi.SayHello", "linda", &reply); err != nil {
			t.Fatal(err.Error())
		}
		fmt.Println(i, time.Since(prev), limiter.tokens)
	}
	c.Close()
}

/*
0 510.7µs 0
1 16.4487ms 0
2 30.8437ms 1
3 0s 0
4 15.5885ms 0
5 15.5804ms 0
6 30.5975ms 1
7 0s 0
8 15.3328ms 0
9 30.3292ms 0
*/
