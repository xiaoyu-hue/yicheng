package main

// 一键成城 · 郡图（4.0）：一郡之地的布局与渲染。
// 郡城与辖县由现有生成器按子种子真生成，以素面（Plain）形态缩印入图；
// 地形（大河/支流/山峦/田野）、官道乡道与郡级标注在本文件生成与渲染。
// 城市生成核心不因郡图改动，三种旧格局的输出保持不变。

import (
	"fmt"
	"math"
	"math/rand"
	"strings"
)

// TownMark 郡图上的一座城（郡城或县城）。
type TownMark struct {
	City   *City
	At     Pt
	Scale  float64
	Label  string
	County bool
}

// VillageMark 郡图上的一个村镇点。
type VillageMark struct {
	At    Pt
	Name  string
	Roofs []Building
}

// Region 一郡之地的全部数据。
type Region struct {
	Seed       int64
	Name       string // 郡名
	SizeLabel  string
	Theme      string
	ThemeLabel string
	NoChrome   bool // 无边框形态（4.1 海报用）：不带纸面与标题
	Towns      []TownMark
	Villages   []VillageMark
	Rivers     [][]Pt
	Roads      [][]Pt // 官道（郡城—县城）
	Lanes      [][]Pt // 乡道（村镇—最近的城）
	Mounts     []Pt   // 山符中心
	Fields     []Field
}

var regionHeads = []string{"沧澜", "临渊", "望舒", "安澜", "云津", "沐川", "长洲", "星泽", "清晏", "岱舆", "衡岳", "沧海"}
var villageNames = []string{"柳林村", "白石镇", "桑梓村", "临河集", "望亭镇", "枫桥渡", "石桥村", "杏花村", "渔梁镇", "青山堡", "芦花村", "梅溪镇"}

// renderRequest 统一渲染入口：region 走郡图，其余走单城。
func renderRequest(seed int64, size, mode, theme string) string {
	if mode == "region" {
		return GenerateRegion(seed, size, theme).SVG()
	}
	return GenerateFull(seed, size, mode, theme).SVG()
}

// pushOut 若点离折线太近，沿远离最近点的方向推到 minD 之外（城避河用）。
func pushOut(p Pt, line []Pt, minD float64) Pt {
	best := math.Inf(1)
	var near Pt
	for i := 0; i+1 < len(line); i++ {
		a, bb := line[i], line[i+1]
		ab := bb.sub(a)
		l2 := ab.X*ab.X + ab.Y*ab.Y
		t := 0.0
		if l2 > 0 {
			t = ((p.X-a.X)*ab.X + (p.Y-a.Y)*ab.Y) / l2
		}
		if t < 0 {
			t = 0
		}
		if t > 1 {
			t = 1
		}
		q := a.add(ab.mul(t))
		if d := dist(p, q); d < best {
			best, near = d, q
		}
	}
	if best >= minD {
		return p
	}
	dir := p.sub(near)
	l := math.Hypot(dir.X, dir.Y)
	if l < 1e-6 {
		dir, l = Pt{0, -1}, 1
	}
	return near.add(dir.mul(minD / l))
}

// roadWith 一条略带弯曲的路：a→b，中点沿垂直方向偏移 off。
func roadWith(a, b Pt, off float64) []Pt {
	mid := a.add(b).mul(0.5)
	d := b.sub(a)
	l := math.Hypot(d.X, d.Y)
	if l == 0 {
		return []Pt{a, b}
	}
	perp := Pt{-d.Y / l, d.X / l}
	return []Pt{a, mid.add(perp.mul(off)), b}
}

// GenerateRegion 按主种子生成一郡：布局、地形、道路、村镇全在此定死。
func GenerateRegion(seed int64, size, theme string) *Region {
	counties, sizeLabel := 4, "中郡"
	switch size {
	case "town":
		counties, sizeLabel = 3, "小郡"
	case "capital":
		counties, sizeLabel = 5, "大郡"
	}
	rng := rand.New(rand.NewSource(seed ^ 0x5EED))
	r := &Region{Seed: seed, SizeLabel: sizeLabel}
	r.Theme, r.ThemeLabel = normalizeTheme(theme)
	nh := int64(len(regionHeads))
	r.Name = regionHeads[((seed%nh)+nh)%nh] + "郡"

	// 大河与支流（先定，城要避让）
	baseY := 615 + rng.Float64()*60
	phase := rng.Float64() * 6.28
	var mainRiver []Pt
	for x := -20.0; x <= 1300; x += 26 {
		mainRiver = append(mainRiver, Pt{x, baseY + math.Sin(x/210+phase)*36})
	}
	bx := 250 + rng.Float64()*480
	ph2 := rng.Float64() * 6.28
	var trib []Pt
	for y := 30.0; y <= baseY; y += 22 {
		trib = append(trib, Pt{bx + math.Sin(y/90+ph2)*20, y})
	}
	r.Rivers = [][]Pt{mainRiver, trib}

	// 郡城（大城）居中偏北
	capAt := Pt{640 + (rng.Float64()-0.5)*90, 320 + (rng.Float64()-0.5)*50}
	capAt = pushOut(capAt, mainRiver, 150)
	capCity := GenerateMode(seed*7+1, "capital", "organic")
	capCity.Plain = true
	r.Towns = append(r.Towns, TownMark{capCity, capAt, 0.34, r.Name, false})

	// 辖县环布四方：多次取点，取离已有城最远的一个（避免城挤在一起）
	for i := 0; i < counties; i++ {
		var pos Pt
		bestD := -1.0
		for try := 0; try < 24; try++ {
			ang := float64(i)/float64(counties)*2*math.Pi + rng.Float64()*0.7 - 0.35
			rad := 350 + rng.Float64()*110
			cand := Pt{clampF(640+math.Cos(ang)*rad*1.18, 205, 1075),
				clampF(490+math.Sin(ang)*rad*0.84, 190, 790)}
			md := math.Inf(1)
			for _, t := range r.Towns {
				if d := dist(cand, t.At); d < md {
					md = d
				}
			}
			if md > bestD {
				bestD, pos = md, cand
			}
			if md >= 310 {
				break
			}
		}
		pos = pushOut(pos, mainRiver, 125)
		// 避河推移后若又贴近别的城，再沿远离方向推开
		for _, t := range r.Towns {
			if d := dist(pos, t.At); d < 265 && d > 1e-6 {
				pos = t.At.add(pos.sub(t.At).mul(265 / d))
				pos = Pt{clampF(pos.X, 205, 1075), clampF(pos.Y, 190, 790)}
			}
		}
		sz := "city"
		if i%2 == 1 {
			sz = "town"
		}
		ct := GenerateMode(seed*7+2+int64(i), sz, "organic")
		ct.Plain = true
		r.Towns = append(r.Towns, TownMark{ct, pos, 0.27, ct.Name, true})
	}

	// 县城若重名，改用未用过的城名头作标注（只改郡图标注，不动城本身）
	used := map[string]bool{}
	for i := range r.Towns {
		lbl := r.Towns[i].Label
		if used[lbl] {
			for _, h := range cityHeads {
				if cand := h + "城"; !used[cand] {
					lbl = cand
					break
				}
			}
			r.Towns[i].Label = lbl
		}
		used[lbl] = true
	}

	// 村镇散点
	vnames := append([]string(nil), villageNames...)
	rng.Shuffle(len(vnames), func(i, j int) { vnames[i], vnames[j] = vnames[j], vnames[i] })
	nv := 6 + rng.Intn(5)
	for i := 0; i < nv; i++ {
		var pos Pt
		for try := 0; try < 24; try++ {
			pos = Pt{90 + rng.Float64()*1100, 110 + rng.Float64()*740}
			ok := true
			for _, t := range r.Towns {
				if dist(pos, t.At) < 140 {
					ok = false
				}
			}
			for _, v := range r.Villages {
				if dist(pos, v.At) < 95 {
					ok = false
				}
			}
			if ok {
				break
			}
		}
		pos = pushOut(pos, mainRiver, 55)
		vm := VillageMark{At: pos, Name: vnames[i%len(vnames)]}
		for k := 0; k < 4; k++ {
			vm.Roofs = append(vm.Roofs, Building{pos.X + (rng.Float64()-0.5)*26, pos.Y + (rng.Float64()-0.5)*16, 6, 4, rng.Float64() * 3})
		}
		r.Villages = append(r.Villages, vm)
	}

	// 官道（郡城连各县）与乡道（村镇连最近的城）
	for _, t := range r.Towns[1:] {
		r.Roads = append(r.Roads, roadWith(capAt, t.At, (rng.Float64()-0.5)*70))
	}
	for _, v := range r.Villages {
		best, bd := capAt, math.Inf(1)
		for _, t := range r.Towns {
			if d := dist(v.At, t.At); d < bd {
				bd, best = d, t.At
			}
		}
		r.Lanes = append(r.Lanes, roadWith(best, v.At, (rng.Float64()-0.5)*36))
	}

	// 山峦：取离城远的角落放两簇山符
	corners := []Pt{{120, 150}, {1160, 160}, {140, 830}, {1140, 820}}
	rng.Shuffle(len(corners), func(i, j int) { corners[i], corners[j] = corners[j], corners[i] })
	added := 0
	for _, cn := range corners {
		if added >= 2 {
			break
		}
		near := false
		for _, t := range r.Towns {
			if dist(cn, t.At) < 240 {
				near = true
			}
		}
		if near {
			continue
		}
		added++
		for k := 0; k < 6; k++ {
			r.Mounts = append(r.Mounts, Pt{cn.X + (rng.Float64()-0.5)*130, cn.Y + (rng.Float64()-0.5)*80})
		}
	}

	// 田野：散布的田块（渲染时垫在河与城之下）
	for i := 0; i < 12; i++ {
		cx, cy := 80+rng.Float64()*1120, 90+rng.Float64()*780
		w, h := 60+rng.Float64()*80, 36+rng.Float64()*44
		ang := rng.Float64() * math.Pi
		var poly []Pt
		for _, s := range [][2]float64{{-1, -1}, {1, -1}, {1, 1}, {-1, 1}} {
			lx, ly := s[0]*w/2, s[1]*h/2
			poly = append(poly, Pt{cx + lx*math.Cos(ang) - ly*math.Sin(ang), cy + lx*math.Sin(ang) + ly*math.Cos(ang)})
		}
		r.Fields = append(r.Fields, Field{poly, i % 2})
	}
	return r
}

// SVG 渲染郡图（1280×960，与单城同幅、同纸面与画框）。
func (r *Region) SVG() string {
	var b strings.Builder
	b.WriteString(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 1280 960" width="1280" height="960">`)
	b.WriteString(r.bodySVG(palFor(r.Theme)))
	b.WriteString(`</svg>`)
	return b.String()
}

// bodySVG 郡图的主体图元（不含 <svg> 外壳），海报排版时也用它。
func (r *Region) bodySVG(p palette) string {
	var b strings.Builder
	if !r.NoChrome {
		fmt.Fprintf(&b, `<rect width="1280" height="960" fill="%s"/>`, p.paper)
		prng := rand.New(rand.NewSource(r.Seed ^ 0xBEEF))
		for i := 0; i < 260; i++ {
			x, y := prng.Float64()*1280, prng.Float64()*960
			rr := 8 + prng.Float64()*34
			fmt.Fprintf(&b, `<ellipse cx="%s" cy="%s" rx="%s" ry="%s" fill="%s" opacity="0.05"/>`,
				f1(x), f1(y), f1(rr), f1(rr), p.spot)
		}
	}

	// 田野
	for _, fd := range r.Fields {
		fill := p.fieldA
		if fd.Tone == 1 {
			fill = p.fieldB
		}
		fmt.Fprintf(&b, `<polygon points="%s" fill="%s" opacity="0.5"/>`, ptsAttr(fd.Poly), fill)
	}

	// 大河与支流
	for _, river := range r.Rivers {
		fmt.Fprintf(&b, `<polyline points="%s" fill="none" stroke="%s" stroke-width="44"/>`, ptsAttr(river), p.riverO)
		fmt.Fprintf(&b, `<polyline points="%s" fill="none" stroke="%s" stroke-width="30"/>`, ptsAttr(river), p.riverI)
	}

	// 山符
	for _, m := range r.Mounts {
		fmt.Fprintf(&b, `<path d="M%s,%s L%s,%s L%s,%s M%s,%s L%s,%s L%s,%s" fill="none" stroke="%s" stroke-width="2" opacity="0.75"/>`,
			f1(m.X-16), f1(m.Y+9), f1(m.X), f1(m.Y-11), f1(m.X+16), f1(m.Y+9),
			f1(m.X-8), f1(m.Y+9), f1(m.X), f1(m.Y-2), f1(m.X+8), f1(m.Y+9), p.sub)
	}

	// 乡道与官道
	for _, ln := range r.Lanes {
		fmt.Fprintf(&b, `<polyline points="%s" fill="none" stroke="%s" stroke-width="5" opacity="0.85"/>`, ptsAttr(ln), p.road)
	}
	for _, rd := range r.Roads {
		fmt.Fprintf(&b, `<polyline points="%s" fill="none" stroke="%s" stroke-width="8"/>`, ptsAttr(rd), p.road)
	}

	// 桥：路与河交汇处
	var allRoads [][]Pt
	allRoads = append(allRoads, r.Roads...)
	allRoads = append(allRoads, r.Lanes...)
	var drawn []Pt
	for _, river := range r.Rivers {
		for vi := 0; vi+1 < len(river); vi++ {
			for _, rl := range allRoads {
				for ri := 0; ri+1 < len(rl); ri++ {
					at, ok := segInt(river[vi], river[vi+1], rl[ri], rl[ri+1])
					if !ok {
						continue
					}
					dupe := false
					for _, q := range drawn {
						if dist(at, q) < 22 {
							dupe = true
						}
					}
					if dupe {
						continue
					}
					drawn = append(drawn, at)
					dirv := rl[ri+1].sub(rl[ri])
					l := math.Hypot(dirv.X, dirv.Y)
					if l == 0 {
						continue
					}
					dirv = dirv.mul(1 / l)
					a, d := at.add(dirv.mul(-15)), at.add(dirv.mul(15))
					fmt.Fprintf(&b, `<line x1="%s" y1="%s" x2="%s" y2="%s" stroke="%s" stroke-width="12"/>`,
						f1(a.X), f1(a.Y), f1(d.X), f1(d.Y), p.road)
				}
			}
		}
	}

	// 各城缩印（素面形态）＋城名标注
	for _, t := range r.Towns {
		fmt.Fprintf(&b, `<g transform="translate(%s %s) scale(%s) translate(-640 -480)">`,
			f1(t.At.X), f1(t.At.Y), fmt.Sprintf("%.3f", t.Scale))
		b.WriteString(t.City.bodySVG(p))
		b.WriteString(`</g>`)
	}
	for _, t := range r.Towns {
		half := t.City.R * t.Scale * 1.2
		size, weight := 19, "bold"
		if !t.County {
			size = 25
		}
		fmt.Fprintf(&b, `<text x="%s" y="%s" font-family="serif" font-size="%d" font-weight="%s" fill="%s" text-anchor="middle">%s</text>`,
			f1(t.At.X), f1(t.At.Y-half-10), size, weight, p.title, t.Label)
	}

	// 村镇：屋顶小簇＋名字
	fmt.Fprintf(&b, `<g fill="%s" opacity="0.55">`, p.building)
	for _, v := range r.Villages {
		for _, bd := range v.Roofs {
			fmt.Fprintf(&b, `<g transform="translate(%s %s) rotate(%s)"><rect x="%s" y="%s" width="%s" height="%s"/></g>`,
				f1(bd.CX), f1(bd.CY), f1(bd.Ang*180/math.Pi),
				f1(-bd.HW), f1(-bd.HH), f1(2*bd.HW), f1(2*bd.HH))
		}
	}
	b.WriteString(`</g>`)
	for _, v := range r.Villages {
		fmt.Fprintf(&b, `<text x="%s" y="%s" font-family="serif" font-size="14" fill="%s" text-anchor="middle">%s</text>`,
			f1(v.At.X), f1(v.At.Y+27), p.wardText, v.Name)
	}

	// 标题、指北针、落款、画框（海报形态不带）
	if !r.NoChrome {
		subtitle := fmt.Sprintf("一键成城 · 种子 %d · %s · 一郡之地", r.Seed, r.SizeLabel)
		if r.ThemeLabel != "" {
			subtitle += " · " + r.ThemeLabel
		}
		fmt.Fprintf(&b, `<text x="56" y="86" font-family="serif" font-size="46" font-weight="bold" fill="%s">%s</text>`, p.title, r.Name)
		fmt.Fprintf(&b, `<text x="58" y="118" font-family="serif" font-size="19" fill="%s">%s</text>`, p.sub, subtitle)
		fmt.Fprintf(&b, `<text x="1222" y="66" font-family="serif" font-size="22" font-weight="bold" fill="%s" text-anchor="middle">北</text>`, p.title)
		fmt.Fprintf(&b, `<polygon points="1222,78 1214,98 1230,98" fill="%s"/>`, p.title)
		n := int64(len(captions))
		cap := captions[((r.Seed%n)+n)%n]
		fmt.Fprintf(&b, `<text x="58" y="922" font-family="serif" font-size="18" fill="%s">%s</text>`, p.sub, cap)
		fmt.Fprintf(&b, `<rect x="18" y="18" width="1244" height="924" fill="none" stroke="%s" stroke-width="3"/>`, p.frame)
		fmt.Fprintf(&b, `<rect x="28" y="28" width="1224" height="904" fill="none" stroke="%s" stroke-width="1"/>`, p.frame)
	}
	return b.String()
}
