package main

// 一键成城 · 生成核心
// 流水线（借鉴 Watabou 城市生成器的公开思路，代码全原创）：
//   撒点分区(Voronoi) → 放松(Lloyd) → 定城墙 → 修街道(BFS) → 派坊市
// 全程只用 math/rand 的显式种子源：同一种子必然生成同一座城。

import (
	"math"
	"math/rand"
	"sort"
)

type Pt struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

func (a Pt) add(b Pt) Pt      { return Pt{a.X + b.X, a.Y + b.Y} }
func (a Pt) sub(b Pt) Pt      { return Pt{a.X - b.X, a.Y - b.Y} }
func (a Pt) mul(s float64) Pt { return Pt{a.X * s, a.Y * s} }
func dist(a, b Pt) float64    { d := a.sub(b); return math.Hypot(d.X, d.Y) }

type Ward struct {
	Kind string // 衙署 / 市集 / 寺庙 / 学宫 / 园林 / 驿站 / 民居
	Name string
	Fill string
}

type Gate struct {
	Name  string // 东门 / 南门 / 西门 / 北门
	Mid   Pt
	Label Pt
	Cell  int
}

type Building struct{ CX, CY, HW, HH, Ang float64 }

// Field 城外田垄：一块带朝向的田地（渲染层按 Tone 选两种田色）。
type Field struct {
	Poly []Pt
	Tone int
}

type City struct {
	Seed       int64
	Size       string
	SizeLabel  string
	Mode       string // organic 山水格局 / grid 棋盘格局
	ModeLabel  string
	Theme      string // 渲染主题（皮肤）："" 宣纸 / spring 春 / autumn 秋 / winter 冬 / night 夜
	ThemeLabel string
	Plain      bool   // 素面形态（4.0 郡图缩印用）：渲染时只画城本身
	NoChrome   bool   // 无边框形态（4.1 海报用）：不带纸面与标题，其余全画
	Name       string // 城名，如「沧澜城」
	W, H       float64
	Center     Pt
	R          float64
	Boundary   []Pt
	Cells      [][]Pt
	Cents      []Pt
	Inner      []int
	Wards      map[int]Ward
	WallEdges  [][2]Pt
	Gates      []Gate
	Roads      [][]Pt
	Ring       []Pt
	River      []Pt
	Buildings  []Building
	Fields     []Field    // 城外田垄（2.0）
	Villages   []Building // 城外村落屋舍（2.0，借用 Building 形状）
	Canals     [][]Pt     // 水乡运河（3.0，仅水乡格局）
	Boats      []Building // 水乡的船（3.0，借用 Building 形状：中心＋朝向）
}

// ---------- 几何工具 ----------

// clipHalfPlane 保留多边形中离 pi 不比 pj 远的部分（Voronoi 半平面裁剪）。
func clipHalfPlane(poly []Pt, pi, pj Pt) []Pt {
	mid := pi.add(pj).mul(0.5)
	d := pj.sub(pi)
	inside := func(p Pt) bool { return p.sub(mid).X*d.X+p.sub(mid).Y*d.Y <= 0 }
	inter := func(a, b Pt) Pt {
		da := a.sub(mid).X*d.X + a.sub(mid).Y*d.Y
		db := b.sub(mid).X*d.X + b.sub(mid).Y*d.Y
		t := 0.5
		if da != db {
			t = da / (da - db)
		}
		return Pt{a.X + (b.X-a.X)*t, a.Y + (b.Y-a.Y)*t}
	}
	var out []Pt
	if len(poly) == 0 {
		return out
	}
	prev := poly[len(poly)-1]
	pin := inside(prev)
	for _, cur := range poly {
		cin := inside(cur)
		if cin {
			if !pin {
				out = append(out, inter(prev, cur))
			}
			out = append(out, cur)
		} else if pin {
			out = append(out, inter(prev, cur))
		}
		prev, pin = cur, cin
	}
	return out
}

func polyCentroid(poly []Pt) Pt {
	if len(poly) == 0 {
		return Pt{}
	}
	var a, cx, cy float64
	for i := range poly {
		p1, p2 := poly[i], poly[(i+1)%len(poly)]
		cr := p1.X*p2.Y - p2.X*p1.Y
		a += cr
		cx += (p1.X + p2.X) * cr
		cy += (p1.Y + p2.Y) * cr
	}
	a *= 0.5
	if math.Abs(a) < 1e-6 {
		var s Pt
		for _, p := range poly {
			s = s.add(p)
		}
		return s.mul(1 / float64(len(poly)))
	}
	return Pt{cx / (6 * a), cy / (6 * a)}
}

func pointInPoly(p Pt, poly []Pt) bool {
	inside := false
	for i := range poly {
		p1, p2 := poly[i], poly[(i+1)%len(poly)]
		if (p1.Y > p.Y) != (p2.Y > p.Y) {
			xin := (p2.X-p1.X)*(p.Y-p1.Y)/(p2.Y-p1.Y) + p1.X
			if p.X < xin {
				inside = !inside
			}
		}
	}
	return inside
}

// distPtSeg 点到线段的距离。
func distPtSeg(p, a, b Pt) float64 {
	ab := b.sub(a)
	l2 := ab.X*ab.X + ab.Y*ab.Y
	if l2 == 0 {
		return dist(p, a)
	}
	t := ((p.X-a.X)*ab.X + (p.Y-a.Y)*ab.Y) / l2
	if t < 0 {
		t = 0
	}
	if t > 1 {
		t = 1
	}
	return dist(p, a.add(ab.mul(t)))
}

// distToPolyline 点到折线的最短距离。
func distToPolyline(p Pt, line []Pt) float64 {
	best := math.Inf(1)
	for i := 0; i+1 < len(line); i++ {
		if d := distPtSeg(p, line[i], line[i+1]); d < best {
			best = d
		}
	}
	return best
}

// ---------- 名字库 ----------

var cityHeads = []string{"沧澜", "临渊", "望舒", "安澜", "云津", "沐川", "长洲", "星泽", "清晏", "岱舆"}
var fangNames = []string{"安平坊", "长乐坊", "永宁坊", "兴善坊", "清泰坊", "怀德坊", "崇仁坊", "延福坊",
	"嘉会坊", "丰邑坊", "临川坊", "望京坊", "归义坊", "敦化坊", "昭庆坊", "景行坊", "承晖坊", "乐善坊"}
var templeNames = []string{"栖云寺", "宝林寺", "清凉寺", "慈恩寺", "栖霞寺"}

// fangName 取第 k 个民居坊名：坊名库用完一轮后，后续轮次改后缀为「里」「巷」，保证全城不重名。
func fangName(pool []string, k int) string {
	base := pool[k%len(pool)]
	switch k / len(pool) {
	case 0:
		return base
	case 1:
		return base[:len(base)-len("坊")] + "里"
	default:
		return base[:len(base)-len("坊")] + "巷"
	}
}

var gardenNames = []string{"听雨园", "半亩园", "漱石园", "沧浪亭园", "怡园"}
var waterNames = []string{"水市", "河市", "桥市", "津市"}
var dockNames = []string{"南码头", "北码头", "东码头", "西码头"}
var captions = []string{
	"城外有河，城里有坊——每换一颗种子，就是一座新城。",
	"城门四面，坊市井然；此城只存于这颗种子里。",
	"一砖一瓦皆由算法排布，换个种子，又是一番人间。",
	"图上无真城，世间有此理：先有墙，再有街，后有坊。",
}

// ---------- 生成 ----------

type sizeSpec struct {
	label string
	r     float64
	rings [][2]float64 // {个数, 半径比例}
}

var sizeSpecs = map[string]sizeSpec{
	"town":    {"小城", 300, [][2]float64{{5, 0.38}, {8, 0.70}, {10, 0.97}}},
	"city":    {"中城", 320, [][2]float64{{6, 0.36}, {11, 0.68}, {13, 0.96}}},
	"capital": {"大城", 340, [][2]float64{{7, 0.34}, {13, 0.64}, {17, 0.94}}},
}

// Generate 山水格局（默认）：有机坊界的古城。
func Generate(seed int64, size string) *City { return GenerateMode(seed, size, "organic") }

// GenerateFull 在 GenerateMode 之上叠加渲染主题（皮肤）。
// 主题只影响渲染层配色，不参与生成——同种子同格局换皮肤，城的布局不变。
func GenerateFull(seed int64, size, mode, theme string) *City {
	c := GenerateMode(seed, size, mode)
	c.Theme, c.ThemeLabel = normalizeTheme(theme)
	return c
}

// normalizeTheme 归一化主题名，未知值按默认宣纸处理。
func normalizeTheme(theme string) (string, string) {
	switch theme {
	case "spring":
		return "spring", "春"
	case "autumn":
		return "autumn", "秋"
	case "winter":
		return "winter", "冬"
	case "night":
		return "night", "夜"
	}
	return "", ""
}

// GenerateMode 按格局生成：organic 山水 / grid 棋盘 / water 水乡，其余值按山水处理。
func GenerateMode(seed int64, size, mode string) *City {
	switch mode {
	case "grid":
		return generateGrid(seed, size)
	case "water":
		return generateOrganic(seed, size, true)
	default:
		return generateOrganic(seed, size, false)
	}
}

// generateOrganic 有机坊界一系（山水与水乡共用）：
// 水乡只在山水的基础上加运河、临水坊与船，其余流水线完全一致。
func generateOrganic(seed int64, size string, water bool) *City {
	spec, ok := sizeSpecs[size]
	if !ok {
		size, spec = "city", sizeSpecs["city"]
	}
	rng := rand.New(rand.NewSource(seed))
	c := &City{Seed: seed, Size: size, SizeLabel: spec.label, W: 1280, H: 960,
		Center: Pt{640, 480}, R: spec.r, Wards: map[int]Ward{},
		Mode: "organic", ModeLabel: "山水格局"}
	if water {
		c.Mode, c.ModeLabel = "water", "水乡格局"
	}
	c.Name = cityHeads[rng.Intn(len(cityHeads))] + "城"

	// 1) 撒点：中心一点 + 若干环
	pts := []Pt{{c.Center.X + rng.NormFloat64()*6, c.Center.Y + rng.NormFloat64()*6}}
	for _, ring := range spec.rings {
		n := int(ring[0])
		for i := 0; i < n; i++ {
			ang := 2*math.Pi*float64(i)/float64(n) + (rng.Float64()-0.5)*0.32
			rad := c.R * ring[1] * (0.9 + rng.Float64()*0.18)
			pts = append(pts, Pt{c.Center.X + rad*math.Cos(ang), c.Center.Y + rad*math.Sin(ang)})
		}
	}
	// 有机城界：带起伏的 64 边形
	for i := 0; i < 64; i++ {
		ang := 2 * math.Pi * float64(i) / 64
		rad := c.R * 1.04 * (1 + 0.045*math.Sin(ang*3+1.7) + 0.03*math.Sin(ang*7+0.4))
		c.Boundary = append(c.Boundary, Pt{c.Center.X + rad*math.Cos(ang), c.Center.Y + rad*math.Sin(ang)})
	}
	voronoi := func(points []Pt) [][]Pt {
		cells := make([][]Pt, len(points))
		for i, pi := range points {
			cell := append([]Pt(nil), c.Boundary...)
			for j, pj := range points {
				if i != j {
					cell = clipHalfPlane(cell, pi, pj)
					if len(cell) == 0 {
						break
					}
				}
			}
			cells[i] = cell
		}
		return cells
	}
	c.Cells = voronoi(pts)
	for k := 0; k < 2; k++ { // Lloyd 放松两轮，坊块更匀整
		for i, cell := range c.Cells {
			if len(cell) >= 3 {
				pts[i] = polyCentroid(cell)
			}
		}
		c.Cells = voronoi(pts)
	}
	for _, cell := range c.Cells {
		if len(cell) >= 3 {
			c.Cents = append(c.Cents, polyCentroid(cell))
		} else {
			c.Cents = append(c.Cents, Pt{})
		}
	}

	// 2) 城墙：内坊的外缘边
	for i, cen := range c.Cents {
		if dist(cen, c.Center) < c.R*0.84 {
			c.Inner = append(c.Inner, i)
		}
	}
	innerSet := map[int]bool{}
	for _, i := range c.Inner {
		innerSet[i] = true
	}
	type ekey [4]int
	ek := func(a, b Pt) ekey {
		ka := [2]int{int(math.Round(a.X)), int(math.Round(a.Y))}
		kb := [2]int{int(math.Round(b.X)), int(math.Round(b.Y))}
		if ka[0] < kb[0] || (ka[0] == kb[0] && ka[1] <= kb[1]) {
			return ekey{ka[0], ka[1], kb[0], kb[1]}
		}
		return ekey{kb[0], kb[1], ka[0], ka[1]}
	}
	owners := map[ekey][]int{}
	for i, cell := range c.Cells {
		for k := range cell {
			key := ek(cell[k], cell[(k+1)%len(cell)])
			owners[key] = append(owners[key], i)
		}
	}
	adjSet := map[int]map[int]bool{}
	addAdj := func(a, b int) {
		if adjSet[a] == nil {
			adjSet[a] = map[int]bool{}
		}
		adjSet[a][b] = true
	}
	for _, ow := range owners {
		if len(ow) == 2 {
			addAdj(ow[0], ow[1])
			addAdj(ow[1], ow[0])
		}
	}
	adj := map[int][]int{}
	for i, set := range adjSet {
		for v := range set {
			adj[i] = append(adj[i], v)
		}
		sort.Ints(adj[i])
	}
	seenWall := map[ekey]bool{}
	for _, i := range c.Inner {
		cell := c.Cells[i]
		for k := range cell {
			a, b := cell[k], cell[(k+1)%len(cell)]
			key := ek(a, b)
			ow := owners[key]
			isWall := len(ow) == 1
			if len(ow) == 2 && innerSet[ow[0]] != innerSet[ow[1]] {
				isWall = true
			}
			if isWall && !seenWall[key] {
				seenWall[key] = true
				c.WallEdges = append(c.WallEdges, [2]Pt{a, b})
			}
		}
	}

	// 3) 城门：四个方向上最贴合的墙边
	dirs := []struct {
		name string
		d    Pt
	}{{"东门", Pt{1, 0}}, {"南门", Pt{0, 1}}, {"西门", Pt{-1, 0}}, {"北门", Pt{0, -1}}}
	for _, dir := range dirs {
		var bestMid Pt
		bestCell, bestScore := -1, -2.0
		for _, e := range c.WallEdges {
			mid := e[0].add(e[1]).mul(0.5)
			v := mid.sub(c.Center)
			ln := math.Hypot(v.X, v.Y)
			if ln == 0 {
				continue
			}
			score := (v.X*dir.d.X + v.Y*dir.d.Y) / ln
			if score > bestScore {
				bestScore, bestMid = score, mid
				bestCell = -1
				for _, ow := range owners[ek(e[0], e[1])] {
					if innerSet[ow] {
						bestCell = ow
					}
				}
			}
		}
		out := bestMid.sub(c.Center)
		out = out.mul(38 / math.Hypot(out.X, out.Y))
		c.Gates = append(c.Gates, Gate{Name: dir.name, Mid: bestMid, Label: bestMid.add(out), Cell: bestCell})
	}

	// 4) 街道：城门 → 城心 的最短路（沿坊界邻接图 BFS）+ 环城道
	centerCell := 0
	for i, cen := range c.Cents {
		if dist(cen, c.Center) < dist(c.Cents[centerCell], c.Center) {
			centerCell = i
		}
	}
	bfs := func(src, dst int) []int {
		prev := map[int]int{src: -1}
		queue := []int{src}
		for len(queue) > 0 {
			u := queue[0]
			queue = queue[1:]
			if u == dst {
				break
			}
			for _, v := range adj[u] {
				if _, seen := prev[v]; !seen {
					prev[v] = u
					queue = append(queue, v)
				}
			}
		}
		var path []int
		for u := dst; u != -1; u = prev[u] {
			path = append([]int{u}, path...)
			if u == src {
				break
			}
		}
		return path
	}
	for _, g := range c.Gates {
		if g.Cell < 0 {
			continue
		}
		line := []Pt{g.Mid}
		for _, ci := range bfs(g.Cell, centerCell) {
			line = append(line, c.Cents[ci])
		}
		c.Roads = append(c.Roads, line)
	}
	var ringCells []int
	gateCells := map[int]bool{}
	for _, g := range c.Gates {
		gateCells[g.Cell] = true
	}
	for _, i := range c.Inner {
		touch := gateCells[i]
		for _, nb := range adj[i] {
			if !innerSet[nb] {
				touch = true
			}
		}
		if touch {
			ringCells = append(ringCells, i)
		}
	}
	sort.Slice(ringCells, func(a, b int) bool {
		pa, pb := c.Cents[ringCells[a]].sub(c.Center), c.Cents[ringCells[b]].sub(c.Center)
		return math.Atan2(pa.Y, pa.X) < math.Atan2(pb.Y, pb.X)
	})
	for _, i := range ringCells {
		c.Ring = append(c.Ring, c.Cents[i])
	}

	// 4.5) 水乡运河（仅水乡格局）：主运河东西穿城，支渠自运河南下入河
	var canalNear map[int]bool
	var canalPhase float64
	if water {
		canalPhase = rng.Float64() * 6.28
		var mainCanal []Pt
		for x := c.Center.X - c.R*0.98; x <= c.Center.X+c.R*0.98; x += 16 {
			y := c.Center.Y + math.Sin((x-c.Center.X)/95+canalPhase)*c.R*0.20
			mainCanal = append(mainCanal, Pt{x, y})
		}
		bx := c.Center.X + c.R*(0.15+rng.Float64()*0.25)
		baseY := c.Center.Y + math.Sin((bx-c.Center.X)/95+canalPhase)*c.R*0.20
		ph2 := rng.Float64() * 6.28
		var branch []Pt
		for y := baseY; y <= 856; y += 16 {
			branch = append(branch, Pt{bx + math.Sin(y/80+ph2)*14, y})
		}
		c.Canals = [][]Pt{mainCanal, branch}
		canalNear = map[int]bool{}
		for _, i := range c.Inner {
			if distToPolyline(c.Cents[i], mainCanal) < 58 {
				canalNear[i] = true
			}
		}
	}

	// 5) 派坊：按位置与规矩给每坊定身份与名字
	sorted := append([]int(nil), c.Inner...)
	sort.Slice(sorted, func(a, b int) bool {
		return dist(c.Cents[sorted[a]], c.Center) < dist(c.Cents[sorted[b]], c.Center)
	})
	c.Wards[centerCell] = Ward{"衙署", c.Name[:len(c.Name)-len("城")] + "县衙", "#e0b096"}
	used := map[string]bool{c.Wards[centerCell].Name: true}
	pick := func(idx int) int {
		if idx < 0 {
			idx = 0
		}
		if idx >= len(sorted) {
			idx = len(sorted) - 1
		}
		return sorted[idx]
	}
	temple := pick(len(sorted) * 2 / 3)
	c.Wards[temple] = Ward{"寺庙", templeNames[rng.Intn(len(templeNames))], "#ecd3a0"}
	school := pick(len(sorted) / 3)
	if _, dup := c.Wards[school]; !dup {
		c.Wards[school] = Ward{"学宫", "明伦学宫", "#dde0bd"}
	}
	gardens := append([]string(nil), gardenNames...)
	rng.Shuffle(len(gardens), func(i, j int) { gardens[i], gardens[j] = gardens[j], gardens[i] })
	gi := 0
	for _, idx := range []int{len(sorted) / 5, len(sorted) * 4 / 5, len(sorted) - 1} {
		p := pick(idx)
		if _, dup := c.Wards[p]; !dup {
			c.Wards[p] = Ward{"园林", gardens[gi%len(gardens)], "#bccf9d"}
			gi++
		}
	}
	if len(c.Gates) > 0 {
		if gc := c.Gates[0].Cell; gc >= 0 {
			if _, dup := c.Wards[gc]; !dup {
				c.Wards[gc] = Ward{"驿站", "迎送驿", "#e8d2ac"}
			}
		}
	}
	fangs := append([]string(nil), fangNames...)
	rng.Shuffle(len(fangs), func(i, j int) { fangs[i], fangs[j] = fangs[j], fangs[i] })
	fi := 0
	wc := 0 // 临水坊计数（仅水乡格局用到）
	for _, i := range c.Inner {
		if _, done := c.Wards[i]; done {
			continue
		}
		if water && canalNear[i] {
			if wc%2 == 0 {
				c.Wards[i] = Ward{"水市", waterNames[(wc/2)%len(waterNames)], "#9fc6bd"}
			} else {
				c.Wards[i] = Ward{"码头", dockNames[(wc/2)%len(dockNames)], "#cbb28a"}
			}
			wc++
			continue
		}
		d := c.Cents[i].sub(c.Center)
		if dist(c.Cents[i], c.Center) < c.R*0.45 {
			name := ""
			if math.Abs(d.X) >= math.Abs(d.Y) {
				if d.X >= 0 {
					name = "东市"
				} else {
					name = "西市"
				}
			} else if d.Y >= 0 {
				name = "南市"
			} else {
				name = "北市"
			}
			if !used[name] {
				used[name] = true
				c.Wards[i] = Ward{"市集", name, "#f2d894"}
				continue
			}
		}
		c.Wards[i] = Ward{"民居", fangName(fangs, fi), "#f0e4c6"}
		fi++
	}

	// 6) 屋舍：民居/市集坊内撒小屋顶（渲染层只管画）
	for _, i := range c.Inner {
		w := c.Wards[i]
		if w.Kind != "民居" && w.Kind != "市集" {
			continue
		}
		cell := c.Cells[i]
		if len(cell) < 3 {
			continue
		}
		minX, maxX, minY, maxY := cell[0].X, cell[0].X, cell[0].Y, cell[0].Y
		for _, p := range cell {
			minX, maxX = math.Min(minX, p.X), math.Max(maxX, p.X)
			minY, maxY = math.Min(minY, p.Y), math.Max(maxY, p.Y)
		}
		for t := 0; t < 46; t++ {
			p := Pt{minX + rng.Float64()*(maxX-minX), minY + rng.Float64()*(maxY-minY)}
			if !pointInPoly(p, cell) || dist(p, c.Cents[i]) < 26 {
				continue
			}
			c.Buildings = append(c.Buildings, Building{p.X, p.Y,
				5 + rng.Float64()*4, 3.5 + rng.Float64()*2, rng.Float64() * math.Pi})
		}
	}

	// 6.5) 水乡的船（仅水乡格局）：沿主运河泊几艘
	if water {
		for i := 0; i < 6; i++ {
			x := c.Center.X - c.R*0.88 + rng.Float64()*c.R*1.76
			y := c.Center.Y + math.Sin((x-c.Center.X)/95+canalPhase)*c.R*0.20
			c.Boats = append(c.Boats, Building{x, y, 10, 3.5, (rng.Float64() - 0.5) * 0.6})
		}
	}

	// 7) 城南河
	phase := rng.Float64() * 6.28
	for x := 30.0; x <= c.W-30; x += 18 {
		y := 852 + 30*math.Sin(x/180+phase) + 10*math.Sin(x/57+phase*2)
		c.River = append(c.River, Pt{x, y})
	}

	// 8) 城外景（独立随机源，不扰动上面的生成序列）
	addOutskirts(c)
	return c
}

// ---------- 城外景（两种格局共用） ----------

func clampF(v, lo, hi float64) float64 { return math.Max(lo, math.Min(hi, v)) }

// addOutskirts 生成城外田垄与村落。用独立随机源（由种子派生），
// 不得改用生成主流程的 rng——那会扰动城内生成序列，让旧种子变城。
func addOutskirts(c *City) {
	rng := rand.New(rand.NewSource(c.Seed*31 + 20261120))
	// 田垄 7 块，散布城外一圈；落到河道高度的往北挪开
	for i := 0; i < 7; i++ {
		ang := rng.Float64() * 2 * math.Pi
		rr := c.R * (1.42 + rng.Float64()*0.33)
		fx := clampF(c.Center.X+rr*math.Cos(ang), 130, c.W-130)
		fy := clampF(c.Center.Y+rr*math.Sin(ang), 140, c.H-120)
		if fy > 770 {
			fy -= 180
		}
		w, h := 64+rng.Float64()*72, 40+rng.Float64()*38
		rot := (rng.Float64() - 0.5) * 0.9
		cs, sn := math.Cos(rot), math.Sin(rot)
		var poly []Pt
		for _, s := range [][2]float64{{-1, -1}, {1, -1}, {1, 1}, {-1, 1}} {
			lx, ly := s[0]*w/2, s[1]*h/2
			poly = append(poly, Pt{fx + lx*cs - ly*sn, fy + lx*sn + ly*cs})
		}
		c.Fields = append(c.Fields, Field{Poly: poly, Tone: rng.Intn(2)})
	}
	// 村落两簇
	for v := 0; v < 2; v++ {
		ang := rng.Float64() * 2 * math.Pi
		rr := c.R * (1.34 + rng.Float64()*0.22)
		vx := clampF(c.Center.X+rr*math.Cos(ang), 110, c.W-110)
		vy := clampF(c.Center.Y+rr*math.Sin(ang), 120, c.H-130)
		if vy > 770 {
			vy -= 180
		}
		n := 9 + rng.Intn(5)
		for i := 0; i < n; i++ {
			px := clampF(vx+(rng.Float64()-0.5)*88, 50, c.W-50)
			py := clampF(vy+(rng.Float64()-0.5)*66, 60, c.H-50)
			c.Villages = append(c.Villages, Building{px, py,
				4.5 + rng.Float64()*2.5, 3 + rng.Float64()*1.6, rng.Float64() * math.Pi})
		}
	}
}

// ---------- 棋盘格局（长安风） ----------

type gridSpec struct {
	n int     // 每边坊数
	s float64 // 城半边长
}

var gridSpecs = map[string]gridSpec{
	"town":    {5, 252},
	"city":    {6, 268},
	"capital": {7, 288},
}

// generateGrid 棋盘格局：方正城墙、棋盘街坊、十字大道。
// 只替换「切坊」方式，派坊、屋舍、城南河、城外景与山水格局共用同一套规矩。
func generateGrid(seed int64, size string) *City {
	spec, ok := sizeSpecs[size]
	if !ok {
		size, spec = "city", sizeSpecs["city"]
	}
	gs := gridSpecs[size]
	rng := rand.New(rand.NewSource(seed))
	c := &City{Seed: seed, Size: size, SizeLabel: spec.label, W: 1280, H: 960,
		Center: Pt{640, 480}, R: gs.s, Wards: map[int]Ward{},
		Mode: "grid", ModeLabel: "棋盘格局"}
	c.Name = cityHeads[rng.Intn(len(cityHeads))] + "城"

	n := gs.n
	cell := 2 * gs.s / float64(n)
	cx, cy := c.Center.X, c.Center.Y
	ox, oy := cx-gs.s, cy-gs.s
	idx := func(r, col int) int { return r*n + col }
	for r := 0; r < n; r++ {
		for col := 0; col < n; col++ {
			x0, y0 := ox+float64(col)*cell, oy+float64(r)*cell
			c.Cells = append(c.Cells, []Pt{{x0, y0}, {x0 + cell, y0}, {x0 + cell, y0 + cell}, {x0, y0 + cell}})
			c.Cents = append(c.Cents, Pt{x0 + cell/2, y0 + cell/2})
			c.Inner = append(c.Inner, idx(r, col))
		}
	}
	// 城界：切角方形
	S, ch := gs.s, 20.0
	c.Boundary = []Pt{
		{cx - S + ch, cy - S}, {cx + S - ch, cy - S}, {cx + S, cy - S + ch}, {cx + S, cy + S - ch},
		{cx + S - ch, cy + S}, {cx - S + ch, cy + S}, {cx - S, cy + S - ch}, {cx - S, cy - S + ch},
	}
	// 城墙：四边各 n 段
	for i := 0; i < n; i++ {
		t0, t1 := -S+float64(i)*cell, -S+float64(i+1)*cell
		c.WallEdges = append(c.WallEdges,
			[2]Pt{{cx + t0, cy - S}, {cx + t1, cy - S}},
			[2]Pt{{cx + t0, cy + S}, {cx + t1, cy + S}},
			[2]Pt{{cx - S, cy + t0}, {cx - S, cy + t1}},
			[2]Pt{{cx + S, cy + t0}, {cx + S, cy + t1}})
	}
	// 城门：四边正中
	for _, g := range []struct {
		name string
		mid  Pt
		out  Pt
		cell int
	}{
		{"东门", Pt{cx + S, cy}, Pt{1, 0}, idx(n/2, n-1)},
		{"南门", Pt{cx, cy + S}, Pt{0, 1}, idx(n-1, n/2)},
		{"西门", Pt{cx - S, cy}, Pt{-1, 0}, idx(n/2, 0)},
		{"北门", Pt{cx, cy - S}, Pt{0, -1}, idx(0, n/2)},
	} {
		c.Gates = append(c.Gates, Gate{Name: g.name, Mid: g.mid, Label: g.mid.add(g.out.mul(38)), Cell: g.cell})
	}
	// 街道：十字大道 + 环城道
	c.Roads = append(c.Roads,
		[]Pt{{cx, cy - S}, {cx, cy + S}},
		[]Pt{{cx - S, cy}, {cx + S, cy}})
	in := S - cell
	c.Ring = []Pt{{cx - in, cy - in}, {cx + in, cy - in}, {cx + in, cy + in}, {cx - in, cy + in}}
	// 派坊：衙署居北中（唐长安皇城位），东西两市对峙，寺、学、园、驿按角安置
	put := func(r, col int, w Ward) { c.Wards[idx(r, col)] = w }
	put(0, n/2, Ward{"衙署", c.Name[:len(c.Name)-len("城")] + "县衙", "#e0b096"})
	put(n/2, n-1, Ward{"市集", "东市", "#f2d894"})
	put(n/2, 0, Ward{"市集", "西市", "#f2d894"})
	put(n-1, 0, Ward{"寺庙", templeNames[rng.Intn(len(templeNames))], "#ecd3a0"})
	put(n-1, n-1, Ward{"学宫", "明伦学宫", "#dde0bd"})
	gardens := append([]string(nil), gardenNames...)
	rng.Shuffle(len(gardens), func(i, j int) { gardens[i], gardens[j] = gardens[j], gardens[i] })
	put(0, 0, Ward{"园林", gardens[0], "#bccf9d"})
	put(n-1, n/2, Ward{"园林", gardens[1], "#bccf9d"})
	put(n/2, n-2, Ward{"驿站", "迎送驿", "#e8d2ac"})
	fangs := append([]string(nil), fangNames...)
	rng.Shuffle(len(fangs), func(i, j int) { fangs[i], fangs[j] = fangs[j], fangs[i] })
	fi := 0
	for _, i := range c.Inner {
		if _, done := c.Wards[i]; !done {
			c.Wards[i] = Ward{"民居", fangName(fangs, fi), "#f0e4c6"}
			fi++
		}
	}
	// 屋舍：民居/市集坊内撒小屋顶
	for _, i := range c.Inner {
		w := c.Wards[i]
		if w.Kind != "民居" && w.Kind != "市集" {
			continue
		}
		poly := c.Cells[i]
		for t := 0; t < 34; t++ {
			p := Pt{poly[0].X + rng.Float64()*cell, poly[0].Y + rng.Float64()*cell}
			if dist(p, c.Cents[i]) < cell*0.24 {
				continue
			}
			c.Buildings = append(c.Buildings, Building{p.X, p.Y,
				5 + rng.Float64()*4, 3.5 + rng.Float64()*2, rng.Float64() * math.Pi})
		}
	}
	// 城南河（与山水格局同一条规矩）
	phase := rng.Float64() * 6.28
	for x := 30.0; x <= c.W-30; x += 18 {
		y := 852 + 30*math.Sin(x/180+phase) + 10*math.Sin(x/57+phase*2)
		c.River = append(c.River, Pt{x, y})
	}
	addOutskirts(c)
	return c
}
