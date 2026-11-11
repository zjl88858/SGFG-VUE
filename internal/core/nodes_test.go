package core

import (
	"strings"
	"testing"
)

func TestConfiguredOMHYNodesAndModes(t *testing.T) {
	if len(Nodes) != 2 || Nodes[0].Name == "" || Nodes[1].Name == "" || Nodes[0].Name == Nodes[1].Name {
		t.Fatal("expected two distinct nodes")
	}
	for _, mode := range []Transport{TransportOMHYDirect, TransportOMHYTurtleor} {
		min, max := mode.portRange()
		for i := 0; i < 64; i++ {
			sel, err := SelectNode(mode)
			if err != nil || sel.Port < min || sel.Port > max {
				t.Fatal("mode/port selection failed", mode, err)
			}
			if err := sel.BlinkConfig().validate(); err != nil {
				t.Fatal("invalid configured Blink parameters")
			}
		}
		sel, err := SelectNodeExcluding(mode, map[string]bool{Nodes[0].Name: true})
		if err != nil || sel.Node.Name != Nodes[1].Name {
			t.Fatal("node exclusion failed")
		}
	}
	for _, old := range []Transport{"webtransport", "hysteria2", "turtleor", "invalid"} {
		if _, err := BuildSelection(old, Nodes[0]); err == nil {
			t.Fatal("retired mode accepted", old)
		}
	}
}

func TestBlinkConfigMapsAllOptionsAndExplicitZeros(t *testing.T) {
	zero := 0
	b := BlinkConfig{Password: "direct-test-secret-012345", MaxPacketSize: 1150, Fragmentation: true, PaddingMax: &zero, JunkPackets: &zero, MaxSessions: 3072, MaxReassemblyBytes: 2097152, VerifyPacketsPerSecond: 4096}
	if err := b.validate(); err != nil {
		t.Fatal(err)
	}
	o := b.options()
	if o.MaxPacketSize != 1150 || !o.Fragmentation || o.PaddingMax == nil || *o.PaddingMax != 0 || o.JunkPackets == nil || *o.JunkPackets != 0 || o.MaxSessions != 3072 || o.MaxReassemblyBytes != 2097152 || o.VerifyPacketsPerSecond != 4096 || o.Server {
		t.Fatal("Blink option loss")
	}
	turtleor := b
	turtleor.Password = "turtle-test-secret-012345"
	sel, err := BuildSelection(TransportOMHYTurtleor, Node{Name: "test", DirectBlink: b, TurtleorBlink: turtleor})
	if err != nil {
		t.Fatal(err)
	}
	if sel.BlinkConfig().Password != turtleor.Password {
		t.Fatal("mode-specific Blink key not selected")
	}
}

func TestBlinkConfigRejectsInvalidOptionsWithoutLeakingSecrets(t *testing.T) {
	for _, change := range []func(*BlinkConfig){
		func(b *BlinkConfig) { b.Fragmentation = false },
		func(b *BlinkConfig) { v := 129; b.PaddingMax = &v },
		func(b *BlinkConfig) { v := 5; b.JunkPackets = &v },
		func(b *BlinkConfig) { b.MaxSessions = 5000 },
		func(b *BlinkConfig) { b.MaxReassemblyBytes = 1 },
		func(b *BlinkConfig) { b.VerifyPacketsPerSecond = 2 },
	} {
		b := omhyTestNode().DirectBlink
		change(&b)
		err := b.validate()
		if err == nil {
			t.Fatal("invalid Blink configuration accepted")
		}
		if strings.Contains(err.Error(), b.Password) {
			t.Fatal("configuration error leaked a secret")
		}
	}
}
