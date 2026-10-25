package sockfilter

import (
	"encoding/binary"
	"testing"
)

func TestHelperAndGUIBypassBeforeProcessRules(t *testing.T) {
	rules := makeRules([]string{"sgfg-ng.exe", "sgfg-driver-helper.exe"}, 123, 456, 0, 123)
	if len(rules) != 7 {
		t.Fatal("unexpected rule count", len(rules))
	}
	for i, pid := range []uint32{123, 456} {
		if binary.LittleEndian.Uint32(rules[i][4:8]) != pid || binary.LittleEndian.Uint32(rules[i][79:83]) != 0 {
			t.Fatal("proxy/helper must bypass before basename rules")
		}
	}
	if binary.LittleEndian.Uint32(rules[2][79:83]) != 16 {
		t.Fatal("missing TCP filter rule")
	}
}
