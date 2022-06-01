package wrpc

import (
	"encoding/json"
	"github.com/duomi520/utils"
	"strings"
	"testing"
)

func TestFrame(t *testing.T) {
	meta := make(map[any]any, 16)
	meta["charset"] = "utf-8"
	var tests = []Frame{
		{utils.StatusRequest16, 1, "wang", nil, "hi"},
		{utils.StatusResponse16, 2, "劳动节5.1", meta, "International Labour Day"},
	}
	for i := range tests {
		data, err := tests[i].MarshalBinary(json.Marshal, func(size int) []byte { return make([]byte, size) })
		if err != nil {
			t.Fatal(err.Error())
		}
		f := &Frame{}
		l, err := f.UnmarshalHeader(data)
		if err != nil {
			t.Fatal(err.Error())
		}
		err = json.Unmarshal(data[l:], &f.Payload)
		if err != nil {
			t.Fatal(err.Error())
		}
		if !strings.EqualFold(f.ServiceMethod, tests[i].ServiceMethod) {
			t.Errorf("expected %s got %s", tests[i].ServiceMethod, f.ServiceMethod)
		}
		if !strings.EqualFold(f.Payload.(string), tests[i].Payload.(string)) {
			t.Errorf("expected %s got %s", tests[i].Payload, f.Payload)
		}
		if !strings.EqualFold(GetPayload(json.Unmarshal, data).(string), tests[i].Payload.(string)) {
			t.Errorf("expected %s got %s", tests[i].Payload.(string), GetPayload(json.Unmarshal, data).(string))
		}
		if tests[i].Status != GetStatus(data) {
			t.Errorf("expected %d got %d", tests[i].Status, GetStatus(data))
		}
		if f.Metadata != nil {
			if !strings.EqualFold(f.Metadata["charset"].(string), meta["charset"].(string)) {
				t.Errorf("expected %s got %s", meta["charset"], f.Metadata["charset"])
			}
		}
	}
}
