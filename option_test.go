package wrpc

import (
	"github.com/duomi520/utils"
	"reflect"
	"testing"
)

func TestOptions(t *testing.T) {
	logger, _ := utils.NewWLogger(utils.DebugLevel, "")
	o1 := NewOptions(WithLogger(logger))
	o2 := NewOptions()
	if reflect.ValueOf(o1.Logger).IsNil() || reflect.ValueOf(o2.Logger).IsNil() {
		t.Fatal(o1.Logger, o2.Logger)
	}
}
