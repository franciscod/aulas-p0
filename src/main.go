package main

import (
	"log"
	"os"

	"image/color"

	"github.com/tdewolff/canvas"
	"github.com/tdewolff/canvas/renderers"

	"github.com/rustyoz/svg"

	"github.com/asim/quadtree"
)

var textBeforePaint string
var scale float64

var fontFamily *canvas.FontFamily

const fontSize = 100.0

func render(ctx *canvas.Context, stroke color.RGBA, di chan *svg.DrawingInstruction) {
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
				}
			case svg.LineInstruction:
				{
					ctx.LineTo(ins.M[0], ins.M[1])
				}
			case svg.CloseInstruction:
				{
					ctx.Close()
				}

			case svg.PaintInstruction:
				{
					if textBeforePaint != "" {
						x, y := ctx.Pos()
						face := fontFamily.Face(fontSize, stroke, canvas.FontRegular, canvas.FontNormal)
						text := canvas.NewTextBox(face, textBeforePaint, 0.0, 0.0, canvas.Left, canvas.Top, 0.0, 0.0)

						coordView := canvas.Identity
						coordView = coordView.ReflectYAbout(ctx.Height() * 1.45)
						coord := coordView.Mul(ctx.CoordView()).Dot(canvas.Point{x + 10.0, y + 10.0})
						m := ctx.View().Translate(coord.X, coord.Y)
						ctx.RenderText(text, m)
						textBeforePaint = ""
					}
					ctx.SetStrokeColor(stroke)
					ctx.Stroke()
				}
			default:
				log.Println("wtf")
				panic(ins.Kind)
			}
		}
	}

}
func renderAula(ctx *canvas.Context, color color.RGBA, p *svg.Path) {

	di, _ := p.ParseDrawingInstructions()

	textBeforePaint = p.ID
	render(ctx, color, di)
}

func main() {

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
	ctx.SetStrokeWidth(3.0)
	render(ctx, canvas.Lightgray, di)

	if doc.Groups[1].ID != "paredes" {
		panic(doc.Groups[1].ID)
	}
	di, _ = doc.Groups[1].ParseDrawingInstructions()
	ctx.SetStrokeWidth(6.0)
	render(ctx, canvas.Black, di)

	if doc.Groups[4].ID != "puntitos" {
		panic(doc.Groups[4].ID)
	}

	centerPoint := quadtree.NewPoint(0.0, 0.0, nil)
	halfPoint := quadtree.NewPoint(3000.0, 3000.0, nil)
	bb := quadtree.NewAABB(centerPoint, halfPoint)
	qtree := quadtree.New(bb, 0, nil)

	ctx.SetStrokeWidth(4.0)
	di, _ = doc.Groups[4].ParseDrawingInstructions()
	render(ctx, canvas.Lightskyblue, di)

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
		p := quadtree.NewPoint(c.Cx, c.Cy, c)
		ok = qtree.Insert(p)
		if !ok {
			panic("out of bounds")
		}
		ps = append(ps, p)
	}
	log.Println(len(ps), "puntitos")

	for _, p := range ps {
		px, py := p.Coordinates()
		dist := 60.0
		knc := quadtree.NewPoint(px, py, nil)
		knd := quadtree.NewPoint(dist, dist, nil)
		knbb := quadtree.NewAABB(knc, knd)

		maxPoints := 4
		for _, point := range qtree.KNearest(knbb, maxPoints, nil) {
			var c *svg.Circle
			c = point.Data().(*svg.Circle)

			x, y := knc.Coordinates()
			ctx.MoveTo(c.Cx, c.Cy)
			ctx.LineTo(x, y)
			ctx.Stroke()
		}
	}

	if doc.Groups[3].ID != "aulas" {
		panic(doc.Groups[3].ID)
	}
	colorDc := color.RGBA{28, 151, 160, 255}
	colorAulas := colorDc

	ctx.SetStrokeWidth(8.0)
	for _, e := range doc.Groups[3].Elements {
		var p *svg.Path
		p = e.(*svg.Path)

		renderAula(ctx, colorAulas, p)
	}

	renderers.Write("mapa.png", c)
}
