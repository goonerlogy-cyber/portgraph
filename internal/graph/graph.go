package graph

import (
	"math"
	"math/rand/v2"

	"github.com/arshnah/portgraph/internal/netpoll"
)

type Kind int

const (
	KindProcess Kind = iota
	KindRemote
)

type Node struct {
	ID     string
	Label  string
	Kind   Kind
	Proto  netpoll.Proto
	Weight int
	X, Y   float64
	vx, vy float64
}

type Edge struct {
	From, To string
	Proto    netpoll.Proto
}

type Graph struct {
	Nodes       map[string]*Node
	Edges       []Edge
	temperature float64
	width       float64
	height      float64
}

func New() *Graph {
	return &Graph{
		Nodes:       map[string]*Node{},
		temperature: 10,
	}
}

func (g *Graph) Update(conns []netpoll.Conn, width, height float64) {
	g.width = width
	g.height = height

	seenNodes := map[string]bool{}
	var edges []Edge

	weight := map[string]int{}
	for _, c := range conns {
		procID := "proc:" + processLabel(c)
		remoteID := "remote:" + c.RemoteIP
		weight[procID]++
		weight[remoteID]++
	}

	for _, c := range conns {
		procID := "proc:" + processLabel(c)
		remoteID := "remote:" + c.RemoteIP

		g.ensureNode(procID, processLabel(c), KindProcess, c.Proto, weight[procID])
		g.ensureNode(remoteID, c.RemoteIP, KindRemote, c.Proto, weight[remoteID])

		seenNodes[procID] = true
		seenNodes[remoteID] = true

		edges = append(edges, Edge{From: procID, To: remoteID, Proto: c.Proto})
	}

	for id := range g.Nodes {
		if !seenNodes[id] {
			delete(g.Nodes, id)
		}
	}

	g.Edges = edges
}

func processLabel(c netpoll.Conn) string {
	if c.Process != "" {
		return c.Process
	}
	return "?"
}

func (g *Graph) ensureNode(id, label string, kind Kind, proto netpoll.Proto, weight int) {
	n, ok := g.Nodes[id]
	if !ok {
		n = &Node{
			ID:    id,
			Label: label,
			Kind:  kind,
			Proto: proto,
			X:     g.width/2 + (rand.Float64()-0.5)*g.width*0.5,
			Y:     g.height/2 + (rand.Float64()-0.5)*g.height*0.5,
		}
		g.Nodes[id] = n
	}
	n.Weight = weight
	n.Proto = proto
}

func (g *Graph) Step(iterations int) {
	n := len(g.Nodes)
	if n < 2 || g.width <= 0 || g.height <= 0 {
		return
	}

	area := g.width * g.height
	k := math.Sqrt(area / float64(n))

	nodes := make([]*Node, 0, n)
	for _, node := range g.Nodes {
		nodes = append(nodes, node)
	}

	for iter := 0; iter < iterations; iter++ {
		for _, v := range nodes {
			v.vx, v.vy = 0, 0
		}

		for i := range nodes {
			for j := range nodes {
				if i == j {
					continue
				}
				dx := nodes[i].X - nodes[j].X
				dy := nodes[i].Y - nodes[j].Y
				dist := math.Hypot(dx, dy)
				if dist < 0.01 {
					dist = 0.01
				}
				force := (k * k) / dist
				nodes[i].vx += dx / dist * force
				nodes[i].vy += dy / dist * force
			}
		}

		for _, e := range g.Edges {
			from, okF := g.Nodes[e.From]
			to, okT := g.Nodes[e.To]
			if !okF || !okT {
				continue
			}
			dx := from.X - to.X
			dy := from.Y - to.Y
			dist := math.Hypot(dx, dy)
			if dist < 0.01 {
				dist = 0.01
			}
			force := (dist * dist) / k
			fx := dx / dist * force
			fy := dy / dist * force
			from.vx -= fx
			from.vy -= fy
			to.vx += fx
			to.vy += fy
		}

		temp := math.Max(g.temperature, k*0.05)
		for _, v := range nodes {
			disp := math.Hypot(v.vx, v.vy)
			if disp < 0.01 {
				disp = 0.01
			}
			limited := math.Min(disp, temp)
			v.X += v.vx / disp * limited
			v.Y += v.vy / disp * limited
			v.X = clamp(v.X, 1, g.width-1)
			v.Y = clamp(v.Y, 1, g.height-1)
		}
	}

	g.temperature = math.Max(g.temperature*0.98, k*0.05)
}

func clamp(v, lo, hi float64) float64 {
	if hi < lo {
		return lo
	}
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func (g *Graph) TopNodes(n int) []*Node {
	all := make([]*Node, 0, len(g.Nodes))
	for _, node := range g.Nodes {
		all = append(all, node)
	}
	for i := 0; i < len(all); i++ {
		for j := i + 1; j < len(all); j++ {
			if all[j].Weight > all[i].Weight {
				all[i], all[j] = all[j], all[i]
			}
		}
	}
	if len(all) > n {
		all = all[:n]
	}
	return all
}
