package core

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"strings"
)

type Transport string

const (
	TransportWebTransport Transport = "webtransport"
	TransportHysteria2 Transport = "hysteria2"
	TransportTurtleor  Transport = "turtleor"
)

type Node struct {
	Name     string
	Host     string
	Password string
}

var Nodes = []Node{
	{Name: "***", Host: "***", Password: "***"},
	{Name: "***", Host: "***", Password: "***"},
	{Name: "***", Host: "***", Password: "***"},
}

const (
	GostServerPort = 12345
	LocalProxyPort = 23456

	LocalProxyPortMaxTry = 10

	ObfsHysteria2 = "***"
	ObfsTurtleor  = "***"
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
	case TransportWebTransport:
		sel.Label = fmt.Sprintf("%s(wt)->direct", node.Name)
		sel.Port = GostServerPort
	case TransportHysteria2:
		sel.Label = fmt.Sprintf("%s(hy2)->direct", node.Name)
		sel.Port, err = randPort(30000, 50000)
	case TransportTurtleor:
		sel.Label = fmt.Sprintf("%s(hy2)->warpplus(M-H3)", node.Name)
		sel.Port, err = randPort(10000, 20000)
	default:
		return Selection{}, fmt.Errorf("unknown transport: %s", t)
	}
	if err != nil {
		return Selection{}, err
	}
	return sel, nil
}

func (t Transport) ObfsPassword() string {
	if t == TransportTurtleor {
		return ObfsTurtleor
	}
	return ObfsHysteria2
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
