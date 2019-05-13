// Reference for Getis-Ord statistics
// https://onlinelibrary.wiley.com/doi/pdf/10.1111/j.1538-4632.1995.tb00912.x

package main

import (
	"compress/gzip"
	"encoding/gob"
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"sort"

	"github.com/kshedden/segregation/seglib"
	"github.com/paulmach/orb"
	"github.com/paulmach/orb/geo"
	"github.com/paulmach/orb/quadtree"
	"gonum.org/v1/gonum/floats"
)

var (
	year int

	sumlevel seglib.RegionType

	regions []*seglib.Region

	// Maximum radius in miles
	maxradius float64

	// Target population
	targetpop int

	// Scaling parameter for exponential weights
	escale float64
)

const (
	// Physical constants
	earthRadiusMiles float64 = 3959
	metersPerMile    float64 = 1609.34
)

func getRegions() {

	var fname string
	switch sumlevel {
	case seglib.Tract:
		fname = fmt.Sprintf("segregation_raw_tract_%4d.gob.gz", year)
	case seglib.BlockGroup:
		fname = fmt.Sprintf("segregation_raw_blockgroup_%4d.gob.gz", year)
	default:
		panic("Unkown summary level\n")
	}

	fid, err := os.Open(fname)
	if err != nil {
		panic(err)
	}
	defer fid.Close()
	fmt.Printf("Reading regions from '%s'\n", fname)

	gid, err := gzip.NewReader(fid)
	if err != nil {
		panic(err)
	}
	defer gid.Close()

	dec := gob.NewDecoder(gid)

	for {
		var r seglib.Region
		err := dec.Decode(&r)
		if err == io.EOF {
			break
		} else if err != nil {
			panic(err)
		}

		regions = append(regions, &r)
	}
}

func getCBSAStats() {

	cbsa := make(map[string][3]int)

	for _, r := range regions {
		x := cbsa[r.CBSA]
		x[0] += r.TotalPop
		x[1] += r.BlackOnlyPop
		x[2] += r.WhiteOnlyPop
		cbsa[r.CBSA] = x
	}

	for _, r := range regions {
		x := cbsa[r.CBSA]
		r.CBSATotalPop = x[0]
		r.CBSABlackOnlyPop = x[1]
		r.CBSAWhiteOnlyPop = x[2]
	}
}

func main() {

	flag.IntVar(&year, "year", 0, "Census year")
	var sl string
	flag.StringVar(&sl, "sumlevel", "", "Summary level ('blockgroup' or 'tract')")
	flag.IntVar(&targetpop, "targetpop", 25000, "Target population")
	flag.Float64Var(&maxradius, "maxradius", 30, "Maximum radius in miles")
	flag.Float64Var(&escale, "escale", 2.0, "Exponential scaling parameter")
	var outname string
	flag.StringVar(&outname, "outfile", "", "File name for output")
	flag.Parse()

	if outname == "" {
		msg := "Output file name must be provided\n"
		os.Stderr.WriteString(msg)
		os.Exit(1)
	}

	switch sl {
	case "blockgroup":
		sumlevel = seglib.BlockGroup
	case "tract":
		sumlevel = seglib.Tract
	default:
		msg := fmt.Sprintf("Unkown sumlevel '%s'\n", sl)
		panic(msg)
	}

	if year != 2010 {
		msg := fmt.Sprintf("Invalid year '%d'\n", year)
		panic(msg)
	}

	getRegions()

	getCBSAStats()

	qt := quadtree.New(orb.Bound{Min: orb.Point{-180, -60},
		Max: orb.Point{20, 80}})

	for _, r := range regions {
		qt.Add(r)
	}

	fid, err := os.Create(outname)
	if err != nil {
		panic(err)
	}
	defer fid.Close()
	fmt.Printf("Writing regions to '%s'\n", outname)

	gid := gzip.NewWriter(fid)
	defer gid.Close()
	enc := gob.NewEncoder(gid)

	buf := make([]orb.Pointer, 1000)
	nbds := make([]*seglib.Region, 1000)
	dists := make([]float64, 1000)
	inds := make([]int, 1000)
	for _, r := range regions {

		// Find enough regions to reach the target population
		var nbd []orb.Pointer
		for k := 8; k <= 256; k *= 2 {

			nbd = qt.KNearest(buf, r.Location, k)

			pop := r.TotalPop
			for _, n := range nbd {
				pop += n.(*seglib.Region).TotalPop
			}

			if pop > targetpop {
				break
			}
		}

		// Sort by increasing distance from the center
		dists = dists[0:len(nbd)]
		for j, n := range nbd {
			dists[j] = geo.Distance(r.Location, n.(*seglib.Region).Location)
		}
		inds = inds[0:len(nbd)]
		floats.Argsort(dists, inds)
		nbds = nbds[0:0]
		for _, j := range inds {
			nbds = append(nbds, nbd[j].(*seglib.Region))
		}

		// Exclude anything equal to or greater than the maximum radius
		i := sort.SearchFloat64s(dists, maxradius*metersPerMile)
		if i == 0 {
			os.Stderr.WriteString("Skipping region:\n")
			os.Stderr.WriteString(fmt.Sprintf("%+v\n", r))
			continue
		} else if i < len(dists) {
			nbds = nbds[0:i]
			inds = inds[0:i]
			dists = dists[0:i]
		}

		// Find the subset with total population closest to targetpop
		var k int
		rpop := 0
		for k = 0; k < len(nbds); k++ {
			rpop += nbds[k].TotalPop
			if rpop > targetpop {
				break
			}
		}
		if k < len(nbds) {
			lastpop := nbds[k].TotalPop
			if rpop-targetpop > targetpop-(rpop-lastpop) {
				k--
				rpop -= lastpop
			}
			nbds = nbds[0 : k+1]
		}

		radius := dists[len(dists)-1]
		r.RegionRadius = radius / metersPerMile

		// First pass calculates the smoothed proportions
		r.PBlack = 0
		r.PWhite = 0
		r.RegionPop = 0
		var dt, nTotal, nBlack, nWhite float64
		r.Neighbors = 0
		for j, z := range nbds {

			if z.TotalPop == 0 {
				continue
			}
			r.Neighbors++
			r.RegionPop += z.TotalPop

			var w float64
			if radius == 0 {
				w = 1
			} else {
				w = math.Exp(-escale * dists[j] / radius)
			}

			popt := float64(z.TotalPop)
			bopt := float64(z.BlackOnlyPop)
			wopt := float64(z.WhiteOnlyPop)

			nTotal += w * popt
			nBlack += w * bopt
			nWhite += w * wopt

			pBlack := bopt / popt
			pWhite := wopt / popt

			// Local entropy is only based on one region
			if j == 0 {
				pOther := 1 - pBlack - pWhite
				r.LocalEntropy = -pBlack * math.Log(pBlack)
				r.LocalEntropy -= pWhite * math.Log(pWhite)
				r.LocalEntropy -= pOther * math.Log(pOther)
			}

			r.PBlack += w * popt * pBlack
			r.PWhite += w * popt * pWhite

			dt += w * popt
		}
		r.PBlack /= dt
		r.PWhite /= dt

		// Isolation and dissimilarity measures
		{
			numer := float64(nTotal - nBlack)
			denom := float64(r.CBSATotalPop - r.CBSABlackOnlyPop)
			r.BlackIsolation = 1 - numer/denom

			qr1 := float64(nBlack) / float64(r.CBSABlackOnlyPop)
			qr2 := float64(nTotal-nBlack) / float64(r.CBSATotalPop-r.CBSABlackOnlyPop)
			r.BODissimilarity = math.Abs(qr1 - qr2)

			numer = float64(nTotal - nWhite)
			denom = float64(r.CBSATotalPop - r.CBSAWhiteOnlyPop)
			r.WhiteIsolation = 1 - numer/denom

			qr1 = float64(nWhite) / float64(r.CBSAWhiteOnlyPop)
			qr2 = float64(nTotal-nWhite) / float64(r.CBSATotalPop-r.CBSAWhiteOnlyPop)
			r.WODissimilarity = math.Abs(qr1 - qr2)
		}

		// Regional entropy
		{
			pBlack := nBlack / nTotal
			pWhite := nWhite / nTotal
			pOther := 1 - pBlack - pWhite
			r.RegionalEntropy = -pBlack * math.Log(pBlack)
			r.RegionalEntropy -= pWhite * math.Log(pWhite)
			r.RegionalEntropy -= pOther * math.Log(pOther)
		}

		err := enc.Encode(r)
		if err != nil {
			panic(err)
		}
	}
}
