package main

import (
	"log"
	"os"

	"image/color"

	"github.com/tdewolff/canvas"
	"github.com/tdewolff/canvas/renderers"

	"github.com/rustyoz/svg"
)

var textBeforePaint string
var scale float64

var fontFamily *canvas.FontFamily

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
					ctx.MoveTo(ins.M[0], ins.M[1])
					ctx.Arc(*ins.Radius, *ins.Radius, 0, 0, 360)
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
						face := fontFamily.Face(100.0, stroke, canvas.FontRegular, canvas.FontNormal)
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

	ctx.SetStrokeWidth(10.0)
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
	di, _ = doc.Groups[1].ParseDrawingInstructions()
	render(ctx, canvas.Black, di)

	colorDc := color.RGBA{28, 151, 160, 255}
	colorAulas := colorDc
	for _, e := range doc.Groups[2].Elements {
		var p *svg.Path
		p = e.(*svg.Path)

		renderAula(ctx, colorAulas, p)
	}

	di, _ = doc.Groups[3].ParseDrawingInstructions()
	render(ctx, canvas.Lightgray, di)

	renderers.Write("mapa.png", c)
}
