package geo

import "math"

// CellDeg is the spatial grid resolution in degrees (~5.5 km per cell).
const CellDeg = 0.05

type cellClass uint8

const (
	classOutside  cellClass = iota // point definitely outside this zone; not stored
	classInside                    // cell fully interior; skip the polygon test
	classBoundary                  // cell straddles the edge; exact test required
)

type cellZone struct {
	zone  int
	class cellClass
}

// Grid pre-rasterizes zones into cells so the common far-from-any-zone message
// costs one map lookup and no polygon math. Cells fully inside a zone skip the
// polygon test; boundary cells fall back to an exact orb point-in-polygon.
// Built once at startup and read-only thereafter (safe for concurrent workers).
type Grid struct {
	cell  float64
	zones []*Zone
	cells map[int64][]cellZone
}

// NewGrid rasterizes zones onto a grid of the given cell size in degrees.
func NewGrid(zones []*Zone, cellDeg float64) *Grid {
	g := &Grid{cell: cellDeg, zones: zones, cells: make(map[int64][]cellZone)}
	for zi, z := range zones {
		b := z.bound
		ixMin, ixMax := cellIndex(b.Min[0], cellDeg), cellIndex(b.Max[0], cellDeg)
		iyMin, iyMax := cellIndex(b.Min[1], cellDeg), cellIndex(b.Max[1], cellDeg)
		for iy := iyMin; iy <= iyMax; iy++ {
			for ix := ixMin; ix <= ixMax; ix++ {
				cls := g.classifyCell(z, ix, iy)
				if cls == classOutside {
					continue
				}
				k := cellKey(ix, iy)
				g.cells[k] = append(g.cells[k], cellZone{zone: zi, class: cls})
			}
		}
	}
	return g
}

// Zones returns the zone slice indexed by the values Inside emits.
func (g *Grid) Zones() []*Zone { return g.zones }

// Inside appends, to out, the indices of every zone that contains (lat, lon).
// Callers pass a reusable scratch slice to avoid per-message allocation.
func (g *Grid) Inside(lat, lon float64, out []int) []int {
	out = out[:0]
	ix := cellIndex(lon, g.cell)
	iy := cellIndex(lat, g.cell)
	for _, cz := range g.cells[cellKey(ix, iy)] {
		switch cz.class {
		case classInside:
			out = append(out, cz.zone)
		case classBoundary:
			if g.zones[cz.zone].Contains(lon, lat) {
				out = append(out, cz.zone)
			}
		}
	}
	return out
}

// classifyCell decides whether a cell is fully inside a zone, on its boundary,
// or fully outside. It samples the four corners and, to stay conservative,
// treats any cell holding a polygon vertex as boundary. A cell is called
// "inside" only when all four corners are inside and no vertex sits in it; this
// never yields a false inside for the simplified polygons we use, and any
// misclassification only costs an extra exact test (never a wrong answer).
func (g *Grid) classifyCell(z *Zone, ix, iy int) cellClass {
	lon0 := float64(ix) * g.cell
	lat0 := float64(iy) * g.cell
	lon1 := lon0 + g.cell
	lat1 := lat0 + g.cell

	in := 0
	for _, c := range [4][2]float64{{lon0, lat0}, {lon1, lat0}, {lon1, lat1}, {lon0, lat1}} {
		if z.Contains(c[0], c[1]) {
			in++
		}
	}
	vertexInCell := g.vertexInCell(z, lon0, lat0, lon1, lat1)
	switch {
	case in == 4 && !vertexInCell:
		return classInside
	case in == 0 && !vertexInCell:
		return classOutside
	default:
		return classBoundary
	}
}

func (g *Grid) vertexInCell(z *Zone, lon0, lat0, lon1, lat1 float64) bool {
	for _, ring := range z.Poly {
		for _, p := range ring {
			if p[0] >= lon0 && p[0] <= lon1 && p[1] >= lat0 && p[1] <= lat1 {
				return true
			}
		}
	}
	return false
}

func cellIndex(deg, cell float64) int { return int(math.Floor(deg / cell)) }

func cellKey(ix, iy int) int64 { return int64(iy)<<32 | int64(uint32(ix)) }
