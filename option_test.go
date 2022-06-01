package wrpc

import (
	"github.com/duomi520/utils"
	"testing"
)

func TestOptions(t *testing.T) {
	logger, _ := utils.NewWLogger(utils.DebugLevel, "")
	o1 := NewOptions(WithLogger(logger))
	o2 := NewOptions()
	if o1.Logger == nil || o2.Logger == nil {
		t.Fatal(o1.Logger, o2.Logger)
	}
}
