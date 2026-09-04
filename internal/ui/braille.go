package ui

import "github.com/charmbracelet/lipgloss"

var dotBit = [4][2]byte{
	{0x01, 0x08},
	{0x02, 0x10},
	{0x04, 0x20},
	{0x40, 0x80},
}

const brailleBase = 0x2800

type Canvas struct {
	cellsW, cellsH int
	dots           [][]byte
	color          [][]lipgloss.Color
	set            [][]bool
}

func NewCanvas(cellsW, cellsH int) *Canvas {
	c := &Canvas{cellsW: cellsW, cellsH: cellsH}
	c.dots = make([][]byte, cellsH)
	c.color = make([][]lipgloss.Color, cellsH)
	c.set = make([][]bool, cellsH)
	for y := range cellsH {
		c.dots[y] = make([]byte, cellsW)
		c.color[y] = make([]lipgloss.Color, cellsW)
		c.set[y] = make([]bool, cellsW)
	}
	return c
}

func (c *Canvas) PixelW() int { return c.cellsW * 2 }
func (c *Canvas) PixelH() int { return c.cellsH * 4 }

func (c *Canvas) SetPixel(px, py int, col lipgloss.Color, priority bool) {
	if px < 0 || py < 0 || px >= c.PixelW() || py >= c.PixelH() {
		return
	}
	cx, cy := px/2, py/4
	sx, sy := px%2, py%4

	if c.set[cy][cx] && !priority {
		c.dots[cy][cx] |= dotBit[sy][sx]
		return
	}

	c.dots[cy][cx] |= dotBit[sy][sx]
	c.color[cy][cx] = col
	c.set[cy][cx] = true
}

func (c *Canvas) Line(x0, y0, x1, y1 int, col lipgloss.Color) {
	dx := abs(x1 - x0)
	dy := -abs(y1 - y0)
	sx, sy := 1, 1
	if x0 > x1 {
		sx = -1
	}
	if y0 > y1 {
		sy = -1
	}
	err := dx + dy

	for {
		c.SetPixel(x0, y0, col, false)
		if x0 == x1 && y0 == y1 {
			break
		}
		e2 := 2 * err
		if e2 >= dy {
			err += dy
			x0 += sx
		}
		if e2 <= dx {
			err += dx
			y0 += sy
		}
	}
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func (c *Canvas) Cell(x, y int) (rune, lipgloss.Color) {
	if c.dots[y][x] == 0 {
		return ' ', ""
	}
	return rune(brailleBase + int(c.dots[y][x])), c.color[y][x]
}
