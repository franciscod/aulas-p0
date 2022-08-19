package main

import (
	"log"
	"math"
	"os"
	"strconv"

	"image/color"

	"github.com/tdewolff/canvas"
	"github.com/tdewolff/canvas/renderers"

	"github.com/rustyoz/svg"

	"github.com/asim/quadtree"
)

type Edge struct {
	u, v int
}

var textBeforePaint string
var scale float64

var fontFamily *canvas.FontFamily

const fontSize = 100.0

func render(ctx *canvas.Context, di chan *svg.DrawingInstruction) *quadtree.Point {
	ps := []*quadtree.Point{}

	done := false
	for !done {
		select {
		case ins, ok := <-di:
			if !ok {
				done = true
				break
			}
			if ins == nil {
				continue
			}
			switch ins.Kind {
			case svg.MoveInstruction:
				{
					ctx.MoveTo(ins.M[0], ins.M[1])
					ps = append(ps, quadtree.NewPoint(ins.M[0], ins.M[1], nil))
				}
			case svg.CircleInstruction:
				{
					r := *ins.Radius
					ctx.MoveTo(ins.M[0]+r, ins.M[1])
					ctx.Arc(r, r, 0, 0, 360)
				}
			case svg.CurveInstruction:
				{
					ctx.CubeTo(
						ins.CurvePoints.C1[0],
						ins.CurvePoints.C1[1],
						ins.CurvePoints.C2[0],
						ins.CurvePoints.C2[1],
						ins.CurvePoints.T[0],
						ins.CurvePoints.T[1],
					)
					ps = append(ps, quadtree.NewPoint(ins.CurvePoints.T[0], ins.CurvePoints.T[1], nil))
				}
			case svg.LineInstruction:
				{
					ctx.LineTo(ins.M[0], ins.M[1])
					ps = append(ps, quadtree.NewPoint(ins.M[0], ins.M[1], nil))
				}
			case svg.CloseInstruction:
				{
					ctx.Close()
					x, y := ctx.Pos()
					ps = append(ps, quadtree.NewPoint(x, y, nil))
				}

			case svg.PaintInstruction:
				{
					if textBeforePaint != "" {
						x, y := ctx.Pos()
						face := fontFamily.Face(fontSize, ctx.Style.StrokeColor, canvas.FontRegular, canvas.FontNormal)
						text := canvas.NewTextBox(face, textBeforePaint, 0.0, 0.0, canvas.Left, canvas.Top, 0.0, 0.0)

						coordView := canvas.Identity
						coordView = coordView.ReflectYAbout(ctx.Height() * 1.45)
						coord := coordView.Mul(ctx.CoordView()).Dot(canvas.Point{X: x + 10.0, Y: y + 10.0})
						m := ctx.View().Translate(coord.X, coord.Y)
						ctx.RenderText(text, m)
						textBeforePaint = ""
					}
					ctx.Stroke()
				}
			default:
				log.Println("wtf")
				panic(ins.Kind)
			}
		}
	}

	cx := 0.0
	cy := 0.0
	l := float64(len(ps))
	for _, p := range ps {
		x, y := p.Coordinates()
		cx += x
		cy += y
	}

	return quadtree.NewPoint(cx/l, cy/l, nil)
}
func renderAula(ctx *canvas.Context, p *svg.Path) *quadtree.Point {

	di, _ := p.ParseDrawingInstructions()

	textBeforePaint = p.ID
	return render(ctx, di)
}

var dist [1000][1000]float64
var next [1000][1000]int

func path(u, v int) []int {
	if next[u][v] == -1 {
		return []int{}
	}
	path := []int{u}

	for u != v {
		u = next[u][v]
		path = append(path, u)
	}
	return path
}

func mapita(src, dst string) {
	fontFamily = canvas.NewFontFamily("noto")
	if err := fontFamily.LoadLocalFont("NotoSans-Regular", canvas.FontRegular); err != nil {
		panic(err)
	}

	c := canvas.New(1000, 500)
	ctx := canvas.NewContext(c)
	ctx.SetCoordSystem(canvas.CartesianIV)

	fill, err := canvas.ParseSVG("L1000 0 L1000 500 L0 500 Z")
	ctx.SetFillColor(color.RGBA{0xfd, 0xfd, 0xfd, 0xff})
	ctx.DrawPath(0, 0, fill)

	scale := 0.4
	xmin := 400.0
	ymin := 100.0

	ctx.SetView(canvas.Identity.Translate(0.0, 0.0).Scale(scale, scale).Translate(-xmin, -ymin))

	reader, err := os.Open("../data/mapa.svg")
	doc, err := svg.ParseSvgFromReader(reader, "mapa", 1)

	if err != nil {
		panic(err)
	}

	var di chan *svg.DrawingInstruction

	if doc.Groups[2].ID != "divisiones" {
		panic(doc.Groups[2].ID)
	}
	di, _ = doc.Groups[2].ParseDrawingInstructions()
	ctx.SetStrokeWidth(3.0) // divisiones finitas
	ctx.SetStrokeColor(canvas.Lightgray)
	render(ctx, di)

	if doc.Groups[1].ID != "paredes" {
		panic(doc.Groups[1].ID)
	}
	di, _ = doc.Groups[1].ParseDrawingInstructions()
	ctx.SetStrokeWidth(6.0) // paredes gruesas
	ctx.SetStrokeColor(canvas.Black)
	render(ctx, di)

	if doc.Groups[4].ID != "puntitos" {
		panic(doc.Groups[4].ID)
	}

	centerPoint := quadtree.NewPoint(0.0, 0.0, nil)
	halfPoint := quadtree.NewPoint(3000.0, 3000.0, nil)
	bb := quadtree.NewAABB(centerPoint, halfPoint)
	qtree := quadtree.New(bb, 0, nil)

	srcI := -1
	dstI := -1
	i := 0
	ps := make([]*quadtree.Point, 0)

	for _, e := range doc.Groups[4].Elements {
		c, ok := e.(*svg.Circle)
		if !ok {
			log.Println("Expected a circle and wasn't")
			p, wasPath := e.(*svg.Path)
			if wasPath {
				log.Println("-- path with ID:", p.ID)
			}
		}
		p := quadtree.NewPoint(c.Cx, c.Cy, i)
		ok = qtree.Insert(p)
		if !ok {
			panic("out of bounds")
		}
		ps = append(ps, p)
		i++
	}
	// log.Println(len(ps), "puntitos")

	if doc.Groups[3].ID != "aulas" {
		panic(doc.Groups[3].ID)
	}

	ctx.SetStrokeColor(canvas.Transparent)
	for _, e := range doc.Groups[3].Elements {
		var p *svg.Path
		p = e.(*svg.Path)

		c := renderAula(ctx, p)

		if p.ID == src {
			srcI = i
		}
		if p.ID == dst {
			dstI = i
		}

		ps = append(ps, c)
		i++
	}

	if len(ps) > 1000 {
		panic("muchos puntitos")
	}

	es := make([]*Edge, 0)

	for i, p := range ps {
		dist := 60.0
		knc := p
		knd := quadtree.NewPoint(dist, dist, nil)
		knbb := quadtree.NewAABB(knc, knd)

		maxPoints := 4
		for _, point := range qtree.KNearest(knbb, maxPoints, nil) {
			var j int
			j = point.Data().(int)

			es = append(es, &Edge{i, j})
			es = append(es, &Edge{j, i})
		}
	}

	for u := range ps {
		for v := range ps {
			dist[u][v] = 100000000000
			next[u][v] = -1
		}
	}

	for _, e := range es {
		ux, uy := ps[e.u].Coordinates()
		vx, vy := ps[e.v].Coordinates()
		dist[e.u][e.v] = math.Sqrt((ux-vx)*(ux-vx) + (uy-vy)*(uy-vy)) // The weight of the edge (u, v)
		next[e.u][e.v] = e.v
	}

	for i := range ps {
		dist[i][i] = 0
		next[i][i] = i
	}

	for k := range ps {
		for i := range ps {
			for j := range ps {
				if dist[i][j] > dist[i][k]+dist[k][j] {
					dist[i][j] = dist[i][k] + dist[k][j]
					next[i][j] = next[i][k]
				}
			}
		}
	}

	// log.Println(src, srcI)
	// log.Println(dst, dstI)
	camino := path(srcI, dstI)

	px, py := ps[srcI].Coordinates()

	ctx.SetStrokeJoiner(canvas.RoundJoin)
	ctx.SetStrokeCapper(canvas.RoundCap)
	ctx.SetStrokeColor(canvas.Gold)
	ctx.SetStrokeWidth(20.0) // camino bien visible
	ctx.MoveTo(px, py)
	for _, u := range camino {
		x, y := ps[u].Coordinates()
		ctx.LineTo(x, y)
	}
	ctx.Stroke()

	drawn := map[string]bool{}
	ctx.SetStrokeWidth(4.0) // puntitos intermedios
	ctx.SetStrokeColor(color.RGBA{0, 0, 0, 10})
	for _, e := range es {
		if e.u >= e.v {
			continue
		}
		arc := "" + strconv.Itoa(e.u) + ":" + strconv.Itoa(e.v)
		if drawn[arc] {
			continue
		}

		x, y := ps[e.u].Coordinates()
		px, py := ps[e.v].Coordinates()
		ctx.MoveTo(px, py)
		ctx.LineTo(x, y)
		ctx.Stroke()

		drawn[arc] = true
	}

	di, _ = doc.Groups[4].ParseDrawingInstructions()
	render(ctx, di)

	ctx.SetStrokeWidth(8.0) // aulas gruesas
	ctx.SetStrokeColor(color.RGBA{28, 151, 160, 255})
	for _, e := range doc.Groups[3].Elements {
		var p *svg.Path
		p = e.(*svg.Path)

		renderAula(ctx, p)
	}

	renderers.Write("mapa.png", c)
}

func main() {
	mapita("pab1", "kiosko")
}
