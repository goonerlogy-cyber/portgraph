package graph

import (
	"math"
	"testing"

	"github.com/arshnah/portgraph/internal/netpoll"
)

func TestUpdateAddsAndRemovesNodes(t *testing.T) {
	g := New()

	conns := []netpoll.Conn{
		{Proto: netpoll.TCP, RemoteIP: "1.2.3.4", Process: "curl"},
		{Proto: netpoll.UDP, RemoteIP: "5.6.7.8", Process: "dns"},
	}
	g.Update(conns, 200, 100)

	if len(g.Nodes) != 4 {
		t.Fatalf("expected 4 nodes (2 proc + 2 remote), got %d", len(g.Nodes))
	}
	if len(g.Edges) != 2 {
		t.Fatalf("expected 2 edges, got %d", len(g.Edges))
	}

	g.Update(conns[:1], 200, 100)
	if len(g.Nodes) != 2 {
		t.Fatalf("expected 2 nodes after removing one connection, got %d", len(g.Nodes))
	}
}

func TestStepKeepsNodesInBounds(t *testing.T) {
	g := New()
	conns := []netpoll.Conn{
		{Proto: netpoll.TCP, RemoteIP: "1.2.3.4", Process: "a"},
		{Proto: netpoll.TCP, RemoteIP: "5.6.7.8", Process: "b"},
		{Proto: netpoll.TCP, RemoteIP: "9.9.9.9", Process: "c"},
	}
	g.Update(conns, 100, 100)

	for range 20 {
		g.Step(5)
	}

	for _, n := range g.Nodes {
		if math.IsNaN(n.X) || math.IsNaN(n.Y) {
			t.Fatalf("node position became NaN: %+v", n)
		}
		if n.X < 0 || n.X > 100 || n.Y < 0 || n.Y > 100 {
			t.Fatalf("node escaped bounds: %+v", n)
		}
	}
}

func TestTopNodesSortedByWeight(t *testing.T) {
	g := New()
	conns := []netpoll.Conn{
		{Proto: netpoll.TCP, RemoteIP: "1.1.1.1", Process: "heavy"},
		{Proto: netpoll.TCP, RemoteIP: "2.2.2.2", Process: "heavy"},
		{Proto: netpoll.TCP, RemoteIP: "3.3.3.3", Process: "light"},
	}
	g.Update(conns, 100, 100)

	top := g.TopNodes(1)
	if len(top) != 1 {
		t.Fatalf("expected 1 node, got %d", len(top))
	}
	if top[0].Weight < 2 {
		t.Fatalf("expected the heaviest node first, got weight %d", top[0].Weight)
	}
}
