package wrpc

import (
	"log/slog"
	"os"
	"reflect"
	"testing"
)

func TestOptions(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	o1 := NewOptions(WithLogger(logger))
	o2 := NewOptions()
	if reflect.ValueOf(o1.Logger).IsNil() || reflect.ValueOf(o2.Logger).IsNil() {
		t.Fatal(o1.Logger, o2.Logger)
	}
}
