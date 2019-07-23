package main

import (
	"compress/gzip"
	"encoding/gob"
	"fmt"
	"io"
	"math"
	"os"
	"sort"
	"strings"

	"github.com/kshedden/segregation/seglib"
	"gonum.org/v1/gonum/floats"
)

var (
	sumlevel seglib.RegionType
)

type locPoly struct {
	y, x []float64
}

func newlocPoly(y, x []float64) *locPoly {

	if len(y) != len(x) {
		panic("Mismatched lengths\n")
	}

	xx := make([]float64, len(x))
	copy(xx, x)
	x = xx

	inds := make([]int, len(x))
	floats.Argsort(x, inds)

	yy := make([]float64, len(y))
	for i, j := range inds {
		yy[i] = y[j]
	}
	y = yy

	return &locPoly{
		y: y,
		x: x,
	}
}

func (lp *locPoly) fit(x, bw float64) float64 {

	i0 := sort.SearchFloat64s(lp.x, x-bw)
	i1 := sort.SearchFloat64s(lp.x, x+bw)

	// Get the weighted means
	var ybar, xbar, wt float64
	for i := i0; i < i1; i++ {
		u := (lp.x[i] - x) / bw
		w := 0.75 * (1 - u*u)
		wt += w
		ybar += w * lp.y[i]
		xbar += w * (lp.x[i] - x)
	}
	ybar /= wt
	xbar /= wt

	// Get the weighted covariance
	var xycov, xvar float64
	for i := i0; i < i1; i++ {
		u := (lp.x[i] - x) / bw
		w := 0.75 * (1 - u*u)
		u = lp.x[i] - x - xbar
		xycov += w * u * (lp.y[i] - ybar)
		xvar += w * u * u
	}
	xycov /= wt
	xvar /= wt

	b := xycov / xvar
	a := ybar - b*xbar

	return a
}

func processUrban(regs []*seglib.Region, sel func(*seglib.Region) float64, set func(*seglib.Region, float64)) {

	var x, y []float64
	for _, r := range regs {
		if r.CBSA != "99999" {
			x = append(x, math.Log(1+float64(r.CBSATotalPop)))
			y = append(y, sel(r))
		}
	}

	lp := newlocPoly(y, x)

	for _, r := range regs {
		if r.CBSA != "99999" {
			yh := lp.fit(math.Log(1+float64(r.CBSATotalPop)), 0.5)
			set(r, sel(r)-yh)
		}
	}
}

func processRural(regs []*seglib.Region, sel func(*seglib.Region) float64, set func(*seglib.Region, float64)) {

	var x, y []float64
	for _, r := range regs {
		if r.CBSA == "99999" {
			x = append(x, math.Log(1+float64(r.PCBSATotalPop)))
			y = append(y, sel(r))
		}
	}

	lp := newlocPoly(y, x)

	for _, r := range regs {
		if r.CBSA == "99999" {
			yh := lp.fit(math.Log(1+float64(r.PCBSATotalPop)), 0.5)
			set(r, sel(r)-yh)
		}
	}
}

func load(inName string) []*seglib.Region {

	lname := strings.ToLower(inName)
	switch {
	case strings.Contains(lname, "tract"):
		sumlevel = seglib.Tract
	case strings.Contains(lname, "blockgroup"):
		sumlevel = seglib.BlockGroup
	case strings.Contains(lname, "cousub"):
		sumlevel = seglib.CountySubdivision
	default:
		panic("Unknown summary level")
	}

	inf, err := os.Open(inName)
	if err != nil {
		panic(err)
	}
	defer inf.Close()

	ing, err := gzip.NewReader(inf)
	if err != nil {
		panic(err)
	}

	dec := gob.NewDecoder(ing)

	var regs []*seglib.Region
	for {
		var r seglib.Region
		err := dec.Decode(&r)
		if err == io.EOF {
			break
		} else if err != nil {
			panic(err)
		}
		regs = append(regs, &r)
	}

	return regs
}

func main() {

	inName := os.Args[1]
	if !strings.HasSuffix(inName, ".gob.gz") {
		panic("Invalid input file\n")
	}
	fmt.Printf("Reading unnormalized results from from '%s'\n", inName)

	regs := load(inName)

	for _, t := range []struct {
		sel func(*seglib.Region) float64
		set func(*seglib.Region, float64)
	}{
		{
			sel: func(r *seglib.Region) float64 { return r.BODissimilarity },
			set: func(r *seglib.Region, v float64) { r.BODissimilarityResid = v },
		},
		{
			sel: func(r *seglib.Region) float64 { return r.WODissimilarity },
			set: func(r *seglib.Region, v float64) { r.WODissimilarityResid = v },
		},
		{
			sel: func(r *seglib.Region) float64 { return r.BlackIsolation },
			set: func(r *seglib.Region, v float64) { r.BlackIsolationResid = v },
		},
		{
			sel: func(r *seglib.Region) float64 { return r.WhiteIsolation },
			set: func(r *seglib.Region, v float64) { r.WhiteIsolationResid = v },
		},
	} {
		processUrban(regs, t.sel, t.set)
		if sumlevel == seglib.CountySubdivision {
			processRural(regs, t.sel, t.set)
		}
	}

	outName := strings.Replace(inName, ".gob.gz", "_norm.gob.gz", 1)
	if outName == inName {
		panic("Invalid input file\n")
	}
	fmt.Printf("Writing normalized results to to '%s'\n", outName)

	outf, err := os.Create(outName)
	if err != nil {
		panic(err)
	}
	defer outf.Close()

	outg := gzip.NewWriter(outf)
	defer outg.Close()

	enc := gob.NewEncoder(outg)

	for _, r := range regs {
		err := enc.Encode(r)
		if err != nil {
			panic(err)
		}
	}
}
