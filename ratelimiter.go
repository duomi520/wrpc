package wrpc

import (
	"io"

	"github.com/duomi520/utils"
)

type Limiter struct {
	utils.TokenBucketLimiter
}

func (l *Limiter) Ratelimit(proto func([]byte, io.Writer) error) func([]byte, io.Writer) error {
	return func(req []byte, rw io.Writer) error {
		err := l.Take(1)
		if err != nil {
			return err
		}
		return proto(req, rw)
	}
}

/*
import (
	"io"

	"github.com/go-kratos/aegis/ratelimit"
	"github.com/go-kratos/aegis/ratelimit/bbr"
)

type Limiter struct {
	bbr.BBR
}

func (l *Limiter) Ratelimit(proto func([]byte, io.Writer) error) func([]byte, io.Writer) error {
	return func(req []byte, rw io.Writer) error {
		done, err := l.Allow()
		if err != nil {
			return err
		}
		err = proto(req, rw)
		done(ratelimit.DoneInfo{})
		return err
	}
}
*/
