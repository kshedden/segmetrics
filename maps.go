package main

import (
	"compress/gzip"
	"encoding/gob"
	"flag"
	"fmt"
	"io"
	"os"
	"path"
	"strconv"
	"strings"

	"github.com/fogleman/gg"
	shp "github.com/jonas-p/go-shp"
	"github.com/kshedden/segregation/seglib"
	colorful "github.com/lucasb-eyer/go-colorful"
	"github.com/paulmach/orb"
)

var (
	// The census region boundaries
	shapefile = "gz_2010_##_140_00_500k.shp"

	// The segregation metrics
	segmetricsfile = "segregation_!!!!!_2010_#####.gob.gz"

	// Only show regions intersecting with this bounding box
	bbox orb.Bound

	// Anchor points for the color map
	keypoints GradientTable

	// Scaling factors for the final image
	xf, yf float64

	// The minimum and mazimumn attribute values
	mina, maxa float64

	// The attribute to plot
	attrf func(*seglib.Region) float64

	// If true, rescale the z values to range from 0 to 1
	scale01 bool
)

// This table contains the "keypoints" of the colorgradient you want to generate.
// The position of each keypoint has to live in the range [0,1]
type GradientTable []struct {
	Col colorful.Color
	Pos float64
}

// This is the meat of the gradient computation. It returns a HCL-blend between
// the two colors around `t`.
// Note: It relies heavily on the fact that the gradient keypoints are sorted.
func (self GradientTable) GetInterpolatedColorFor(t float64) colorful.Color {
	for i := 0; i < len(self)-1; i++ {
		c1 := self[i]
		c2 := self[i+1]
		if c1.Pos <= t && t <= c2.Pos {
			// We are in between c1 and c2. Go blend them!
			t := (t - c1.Pos) / (c2.Pos - c1.Pos)
			return c1.Col.BlendHcl(c2.Col, t).Clamped()
		}
	}

	// Nothing found? Means we're at (or past) the last gradient keypoint.
	return self[len(self)-1].Col
}

// This is a very nice thing Golang forces you to do!
// It is necessary so that we can write out the literal of the colortable below.
func MustParseHex(s string) colorful.Color {
	c, err := colorful.Hex(s)
	if err != nil {
		panic("MustParseHex: " + err.Error())
	}
	return c
}

func getSeg(fname string) map[string]*seglib.Region {

	inf, err := os.Open(fname)
	if err != nil {
		panic(err)
	}
	defer inf.Close()

	ing, err := gzip.NewReader(inf)
	if err != nil {
		panic(err)
	}

	regions := make(map[string]*seglib.Region)
	dec := gob.NewDecoder(ing)
	first := true
	for {
		var r seglib.Region
		err := dec.Decode(&r)
		if err == io.EOF {
			break
		} else if err != nil {
			panic(err)
		}

		if !bbox.Contains(r.Location) {
			continue
		}

		// Update the attribute range
		v := attrf(&r)
		if first {
			mina, maxa = v, v
			first = false
		} else {
			if v < mina {
				mina = v
			}
			if v > maxa {
				maxa = v
			}
		}

		id := r.StateId + r.Tract
		regions[id] = &r
	}

	if scale01 {
		fmt.Printf("Scaling to %f %f\n", mina, maxa)
	}

	return regions
}

func trans(z orb.Point) orb.Point {

	return [2]float64{xf * (z[0] - bbox.Min[0]), 1000 - yf*(z[1]-bbox.Min[1])}
}

func setupCmap() {

	keypoints = GradientTable{
		{MustParseHex("#5e4fa2"), 0.0},
		{MustParseHex("#3288bd"), 0.1},
		{MustParseHex("#66c2a5"), 0.2},
		{MustParseHex("#abdda4"), 0.3},
		{MustParseHex("#e6f598"), 0.4},
		{MustParseHex("#ffffbf"), 0.5},
		{MustParseHex("#fee090"), 0.6},
		{MustParseHex("#fdae61"), 0.7},
		{MustParseHex("#f46d43"), 0.8},
		{MustParseHex("#d53e4f"), 0.9},
		{MustParseHex("#9e0142"), 1.0},
	}
}

func parseBbox(boxs string) {

	f := strings.Split(boxs, ",")
	if len(f) != 4 {
		panic("Malformed bbox string")
	}

	var v []float64
	for _, x := range f {
		z, err := strconv.ParseFloat(x, 64)
		if err != nil {
			panic(err)
		}
		v = append(v, z)
	}

	bbox = orb.Bound{Min: orb.Point{v[0], v[1]}, Max: orb.Point{v[2], v[3]}}
}

func main() {

	aname := flag.String("attribute", "", "Attribute name")
	outfile := flag.String("outfile", "", "Output file name")
	bboxf := flag.String("bbox", "", "Bounding box")
	buffer := flag.Int("buffer", 0, "Buffer population")
	state := flag.String("state", "", "State")
	region := flag.String("region", "", "tract or blockgroup")
	flag.Parse()

	switch *region {
	case "tract":
	case "blockgroup":
	default:
		panic("region must be either 'tract' or 'blockgroup'")
	}

	scale01 = false
	switch *aname {
	case "LocalEntropy":
		attrf = func(r *seglib.Region) float64 { return r.LocalEntropy }
		scale01 = true
	case "RegionalEntropy":
		attrf = func(r *seglib.Region) float64 { return r.RegionalEntropy }
		scale01 = true
	case "PBlack":
		attrf = func(r *seglib.Region) float64 { return r.PBlack }
	case "PWhite":
		attrf = func(r *seglib.Region) float64 { return r.PWhite }
	case "BlackIsolation":
		attrf = func(r *seglib.Region) float64 { return r.BlackIsolation }
		scale01 = true
	case "WhiteIsolation":
		attrf = func(r *seglib.Region) float64 { return r.WhiteIsolation }
		scale01 = true
	case "BODissimilarity":
		attrf = func(r *seglib.Region) float64 { return r.BODissimilarity }
		scale01 = true
	case "WODissimilarity":
		attrf = func(r *seglib.Region) float64 { return r.WODissimilarity }
		scale01 = true
	default:
		panic(fmt.Sprintf("Unknown attribute '%s'", *aname))
	}

	shapefile = strings.Replace(shapefile, "##", *state, 1)

	parseBbox(*bboxf)

	setupCmap()

	// Scaling factors to fill the page
	xf = 1000 / (bbox.Max[0] - bbox.Min[0])
	yf = 1000 / (bbox.Max[1] - bbox.Min[1])

	segmetricsfile = strings.Replace(segmetricsfile, "#####", fmt.Sprintf("%d", *buffer), 1)
	segmetricsfile = strings.Replace(segmetricsfile, "!!!!!", *region, 1)
	regions := getSeg(segmetricsfile)

	// Open a shapefile for reading
	sf := path.Join("shapefiles", *region, shapefile)
	shapef, err := shp.Open(sf)
	if err != nil {
		panic(err)
	}
	defer shapef.Close()

	// fields from the attribute table (DBF)
	fields := shapef.Fields()

	dc := gg.NewContext(1000, 1000)

	// loop through all features in the shapefile
	for shapef.Next() {
		n, p := shapef.Shape()

		attrs := make(map[string]string)
		for k, f := range fields {
			attrs[f.String()] = shapef.ReadAttribute(n, k)
		}
		id := attrs["STATE"] + attrs["TRACT"]
		segdata, ok := regions[id]
		if !ok {
			continue
		}

		q := p.(*shp.Polygon)
		var ls []orb.Point
		for _, pt := range q.Points {
			ls = append(ls, [2]float64{pt.X, pt.Y})
		}
		if !orb.LineString(ls).Bound().Intersects(bbox) {
			continue
		}

		// Draw the boundary
		z := trans(ls[0])
		dc.MoveTo(z[0], z[1])
		for j := 1; j < len(ls); j++ {
			z = trans(ls[j])
			dc.LineTo(z[0], z[1])
		}
		dc.ClosePath()
		cval := attrf(segdata)
		if scale01 {
			cval = (cval - mina) / (maxa - mina)
		}
		color := keypoints.GetInterpolatedColorFor(cval)
		dc.SetColor(color)
		dc.Fill()
		dc.ClearPath()
	}

	if *outfile == "" {
		panic("File name is required")
	}
	dc.SavePNG(*outfile)
}
