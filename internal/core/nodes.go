package core

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"strings"
)

type Transport string

const (
	TransportOMHYDirect   Transport = "omhy-direct"
	TransportOMHYTurtleor Transport = "omhy-turtleor"
)

type Node struct {
	Name          string
	Host          string
	Password      string
	DirectBlink   BlinkConfig
	TurtleorBlink BlinkConfig
}

// Deployment-specific information lives only in this file. config_demo is a
// private reference directory, not a build/runtime dependency; it may be removed
// before publication. Review these values before publishing source or binaries.
var Nodes = []Node{
	{
		Name:          "argon",
		Host:          "",
		Password:      "",
		DirectBlink:   nodeBlink(""),
		TurtleorBlink: nodeBlink(""),
	},
	{
		Name:          "barium",
		Host:          "",
		Password:      "",
		DirectBlink:   nodeBlink(""),
		TurtleorBlink: nodeBlink(""),
	},
}

// Shared Blink parameters from the supplied reference profiles. Per-mode
// authentication secrets and these defaults can all be edited in nodes.go.
func nodeBlink(password string) BlinkConfig {
	padding, junk := 16, 3
	return BlinkConfig{
		Password: password, MaxPacketSize: 1150, Fragmentation: true,
		PaddingMax: &padding, JunkPackets: &junk, MaxSessions: 3072,
		// Zero selects upstream defaults for the remaining resource budgets.
		MaxReassemblyBytes: 0, VerifyPacketsPerSecond: 0,
	}
}

const (
	LocalProxyPort = 7788

	LocalProxyPortMaxTry = 10

	DirectPortMin   = 10520
	DirectPortMax   = 10522
	TurtleorPortMin = 11520
	TurtleorPortMax = 11522
)

type Selection struct {
	Transport Transport
	Node      Node
	Label     string
	Port      int
}

func NodeByName(name string) (Node, bool) {
	for _, n := range Nodes {
		if n.Name == name {
			return n, true
		}
	}
	return Node{}, false
}

func SelectNode(t Transport) (Selection, error) {
	return SelectNodeExcluding(t, nil)
}

func SelectNodeExcluding(t Transport, exclude map[string]bool) (Selection, error) {
	if !t.Valid() {
		return Selection{}, fmt.Errorf("unknown transport: %s", t)
	}
	var candidates []Node
	for _, n := range Nodes {
		if !exclude[n.Name] {
			candidates = append(candidates, n)
		}
	}
	if len(candidates) == 0 {
		return Selection{}, fmt.Errorf("no nodes available")
	}
	n, err := rand.Int(rand.Reader, big.NewInt(int64(len(candidates))))
	if err != nil {
		return Selection{}, err
	}
	return BuildSelection(t, candidates[n.Int64()])
}

func BuildSelection(t Transport, node Node) (Selection, error) {
	var err error
	sel := Selection{Transport: t, Node: node}
	switch t {
	case TransportOMHYDirect:
		sel.Label = fmt.Sprintf("%s(omhy)->direct", node.Name)
		sel.Port, err = randPort(DirectPortMin, DirectPortMax)
	case TransportOMHYTurtleor:
		sel.Label = fmt.Sprintf("%s(omhy)->turtleor", node.Name)
		sel.Port, err = randPort(TurtleorPortMin, TurtleorPortMax)
	default:
		return Selection{}, fmt.Errorf("unknown transport: %s", t)
	}
	if err != nil {
		return Selection{}, err
	}
	return sel, nil
}

func (t Transport) Valid() bool {
	return t == TransportOMHYDirect || t == TransportOMHYTurtleor
}

func (t Transport) portRange() (int, int) {
	if t == TransportOMHYTurtleor {
		return TurtleorPortMin, TurtleorPortMax
	}
	return DirectPortMin, DirectPortMax
}

func (s Selection) BlinkConfig() BlinkConfig {
	if s.Transport == TransportOMHYTurtleor {
		return s.Node.TurtleorBlink
	}
	return s.Node.DirectBlink
}

func randPort(min, max int) (int, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(int64(max-min+1)))
	if err != nil {
		return 0, err
	}
	return min + int(n.Int64()), nil
}

func GeneratePassword() string {
	const source = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
	nBig, err := rand.Int(rand.Reader, big.NewInt(9))
	if err != nil {
		return ""
	}
	cnt := 8 + int(nBig.Int64())
	var sb strings.Builder
	for i := 0; i < cnt; i++ {
		idx, err := rand.Int(rand.Reader, big.NewInt(int64(len(source))))
		if err != nil {
			return ""
		}
		sb.WriteByte(source[idx.Int64()])
	}
	return sb.String()
}
