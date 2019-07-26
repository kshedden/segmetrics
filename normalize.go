package main

import (
	"compress/gzip"
	"encoding/gob"
	"fmt"
	"io"
	"math"
	"os"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/kshedden/segregation/seglib"
	"gonum.org/v1/gonum/floats"
	"gonum.org/v1/gonum/mat"
)

var (
	sumlevel seglib.RegionType

	// 99999 for 2010, 9999 for 2000
	nullCBSA string
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
		if u <= -1 || u >= 1 {
			continue
		}
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
		if u <= -1 || u >= 1 {
			continue
		}
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

type locPoly3d struct {
	y []float64
	x [][3]float64
}

func newlocPoly3d(y []float64, x [][3]float64) *locPoly3d {

	if len(y) != len(x) {
		panic("Mismatched lengths\n")
	}

	return &locPoly3d{
		y: y,
		x: x,
	}
}

func (lp *locPoly3d) fit(x [3]float64, bw float64) (float64, error) {

	// Get the weighted covariance
	xyg := make([]float64, 3)
	xxg := make([]float64, 9)
	var wt float64
	for i := range lp.y {
		u := (lp.x[i][1] - x[1]) / bw
		if u <= -1 || u >= 1 {
			continue
		}
		w := 0.75 * (1 - u*u)
		u = (lp.x[i][2] - x[2]) / bw
		if u <= -1 || u >= 1 {
			continue
		}
		w *= 0.75 * (1 - u*u)
		wt += w
		for j1 := 0; j1 < 3; j1++ {
			xyg[j1] += w * (lp.x[i][j1] - x[j1]) * lp.y[i]
			for j2 := 0; j2 < 3; j2++ {
				xxg[3*j1+j2] += w * (lp.x[i][j1] - x[j1]) * (lp.x[i][j2] - x[j2])
			}
		}
	}

	rslt := mat.NewDense(3, 1, make([]float64, 3))
	err := rslt.Solve(mat.NewDense(3, 3, xxg), mat.NewDense(3, 1, xyg))
	if err != nil {
		fmt.Printf("xxg=%v\n", xxg)
		fmt.Printf("xyg=%v\n", xyg)
		return 0, fmt.Errorf("linalg error")
	}

	return rslt.At(0, 0), nil
}

func processUrban(regs []*seglib.Region,
	sel1 func(*seglib.Region) float64,
	sel2 func(*seglib.Region) float64,
	set2 func(*seglib.Region, float64)) {

	// Pass 1 to remove the mean trend
	var x, y []float64
	for _, r := range regs {
		if r.CBSA != nullCBSA && r.RegionPop > 0 {
			x = append(x, math.Log(1+float64(r.CBSATotalPop)))
			y = append(y, sel1(r))
		}
	}
	lp := newlocPoly(y, x)

	// Write demeaned data to resid variable
	var wg sync.WaitGroup
	for _, r := range regs {
		if r.CBSA != nullCBSA && r.RegionPop > 0 {
			wg.Add(1)
			go func(r *seglib.Region) {
				yh := lp.fit(math.Log(1+float64(r.CBSATotalPop)), 0.5)
				set2(r, sel1(r)-yh)
				wg.Done()
			}(r)
		}
	}
	wg.Wait()

	// Pass 2 to rescale the dispersion
	y = y[0:0]
	for _, r := range regs {
		if r.CBSA != nullCBSA && r.RegionPop > 0 {
			y = append(y, math.Log(math.Abs(sel2(r))))
		}
	}
	lp = newlocPoly(y, x)

	for _, r := range regs {
		if r.CBSA != nullCBSA && r.RegionPop > 0 {
			wg.Add(1)
			go func(r *seglib.Region) {
				yh := lp.fit(math.Log(1+float64(r.CBSATotalPop)), 0.75)
				set2(r, sel2(r)/math.Exp(yh))
				wg.Done()
			}(r)
		}
	}
	wg.Wait()
}

func processRural(regs []*seglib.Region,
	sel1 func(*seglib.Region) float64,
	sel2 func(*seglib.Region) float64,
	set2 func(*seglib.Region, float64)) {

	// Pass 1 to remove the mean trend
	var x [][3]float64
	var y []float64
	for _, r := range regs {
		if r.CBSA == nullCBSA && r.RegionPop > 0 {
			vx := [3]float64{1, math.Log(1 + float64(r.RegionPop)), math.Log(1 + float64(r.PCBSATotalPop))}
			x = append(x, vx)
			y = append(y, sel1(r))
		}
	}
	lp := newlocPoly3d(y, x)

	// Write demeaned data to resid variable
	var wg sync.WaitGroup
	for _, r := range regs {
		if r.CBSA == nullCBSA && r.RegionPop > 0 {
			wg.Add(1)
			go func(r *seglib.Region) {
				vx := [3]float64{0, math.Log(1 + float64(r.RegionPop)), math.Log(1 + float64(r.PCBSATotalPop))}
				bw := 1.0
				for {
					yh, err := lp.fit(vx, bw)
					if err == nil {
						set2(r, sel1(r)-yh)
						break
					}
					bw *= 2
				}
				wg.Done()
			}(r)
		}
	}
	wg.Wait()

	// Pass 2 to rescale the dispersion
	y = y[0:0]
	for _, r := range regs {
		if r.CBSA == nullCBSA && r.RegionPop > 0 {
			y = append(y, math.Log(math.Abs(sel2(r))))
		}
	}
	lp = newlocPoly3d(y, x)

	for _, r := range regs {
		if r.CBSA == nullCBSA && r.RegionPop > 0 {
			wg.Add(1)
			go func(r *seglib.Region) {
				vx := [3]float64{0, math.Log(1 + float64(r.RegionPop)), math.Log(1 + float64(r.PCBSATotalPop))}
				bw := 1.0
				for {
					yh, err := lp.fit(vx, bw)
					if err == nil {
						va := math.Exp(yh)
						if va < 1e-2 {
							va = 1e-2
						}
						set2(r, sel2(r)/va)
						break
					}
					bw *= 2
				}
				wg.Done()
			}(r)
		}
	}
	wg.Wait()
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

	fp := regexp.MustCompile(`[_\.]`).Split(inName, -1)
	switch fp[2] {
	case "2010":
		nullCBSA = "99999"
	case "2000":
		nullCBSA = "9999"
	default:
		panic(fmt.Sprintf("unknown year: %s\n", fp[2]))
	}

	regs := load(inName)

	for _, t := range []struct {
		sel1 func(*seglib.Region) float64
		sel2 func(*seglib.Region) float64
		set2 func(*seglib.Region, float64)
	}{
		{
			sel1: func(r *seglib.Region) float64 { return r.BODissimilarity },
			sel2: func(r *seglib.Region) float64 { return r.BODissimilarityResid },
			set2: func(r *seglib.Region, v float64) { r.BODissimilarityResid = v },
		},
		{
			sel1: func(r *seglib.Region) float64 { return r.WODissimilarity },
			sel2: func(r *seglib.Region) float64 { return r.WODissimilarityResid },
			set2: func(r *seglib.Region, v float64) { r.WODissimilarityResid = v },
		},
		{
			sel1: func(r *seglib.Region) float64 { return r.BlackIsolation },
			sel2: func(r *seglib.Region) float64 { return r.BlackIsolationResid },
			set2: func(r *seglib.Region, v float64) { r.BlackIsolationResid = v },
		},
		{
			sel1: func(r *seglib.Region) float64 { return r.WhiteIsolation },
			sel2: func(r *seglib.Region) float64 { return r.WhiteIsolationResid },
			set2: func(r *seglib.Region, v float64) { r.WhiteIsolationResid = v },
		},
	} {
		processUrban(regs, t.sel1, t.sel2, t.set2)
		if sumlevel == seglib.CountySubdivision {
			processRural(regs, t.sel1, t.sel2, t.set2)
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
