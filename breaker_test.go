package wrpc

import (
	"testing"
	"time"
)

func markSuccess(c *CircuitBreaker, count int) {
	for i := 0; i < count; i++ {
		c.MarkSuccess()
	}
}

func markFailed(c *CircuitBreaker, count int) {
	for i := 0; i < count; i++ {
		c.MarkFailed()
	}
}

func TestCircuitBreakerClose(t *testing.T) {
	c := NewCircuitBreaker(100, 2)
	markSuccess(c, 80)
	if err := c.AllowRequest(); err != nil {
		t.Fatal(err.Error())
	}
	markSuccess(c, 120)
	if err := c.AllowRequest(); err != nil {
		t.Fatal(err.Error())
	}
}

func TestCircuitBreakerOpen(t *testing.T) {
	c := NewCircuitBreaker(100, 2)
	markFailed(c, 10000)
	if err := c.AllowRequest(); err == nil {
		t.Fatal("熔断器未开启")
	}
}

func TestCircuitBreakerHalfOpen(t *testing.T) {
	c := NewCircuitBreaker(100, 2)
	markFailed(c, 10000)
	if err := c.AllowRequest(); err == nil {
		t.Fatal("熔断器未开启")
	}
	time.Sleep(2 * time.Second)
	if err := c.AllowRequest(); err != nil {
		t.Fatal(err.Error())
	}
	markSuccess(c, 10000)
	if err := c.AllowRequest(); err != nil {
		t.Fatal(err.Error())
	}
}
func BenchmarkCircuitBreaker(b *testing.B) {
	c := NewCircuitBreaker(100, 2)
	b.ResetTimer()
	for i := 0; i <= b.N; i++ {
		c.AllowRequest()
		if i%2 == 0 {
			c.MarkSuccess()
		} else {
			c.MarkFailed()
		}
	}
}

// https://github.com/go-kratos/kratos/blob/v1.0.x/pkg/net/netutil/breaker/sre_breaker_test.go
