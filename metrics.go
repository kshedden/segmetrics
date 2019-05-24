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

const (
	NullCBSA = "99999"
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

type neighborhoodSearch struct {
	qt    *quadtree.Quadtree
	buf   []orb.Pointer
	nbds  []*seglib.Region
	dists []float64
	inds  []int
}

func (ns *neighborhoodSearch) init(qt *quadtree.Quadtree, m int) {
	ns.qt = qt
	ns.buf = make([]orb.Pointer, m)
	ns.nbds = make([]*seglib.Region, m)
	ns.dists = make([]float64, m)
	ns.inds = make([]int, m)
}

// Find enough regions so that the total population exceeds the target population.
func (ns *neighborhoodSearch) initialCandidates(r *seglib.Region, targetpop int) {

	for k := 8; k <= 256; k *= 2 {

		ns.buf = ns.qt.KNearest(ns.buf, r.Location, k)

		pop := r.TotalPop
		for _, n := range ns.buf {
			pop += n.(*seglib.Region).TotalPop
		}

		if pop > targetpop {
			break
		}
	}
}

// Sort by increasing distance from the center.
func (ns *neighborhoodSearch) sortByDist(r *seglib.Region) {

	ns.dists = ns.dists[0:len(ns.buf)]
	for j, n := range ns.buf {
		ns.dists[j] = geo.Distance(r.Location, n.(*seglib.Region).Location)
	}
	ns.inds = ns.inds[0:len(ns.buf)]
	floats.Argsort(ns.dists, ns.inds)
	ns.nbds = ns.nbds[0:0]
	for _, j := range ns.inds {
		ns.nbds = append(ns.nbds, ns.buf[j].(*seglib.Region))
	}
}

// Exclude anything equal to or greater than the maximum radius
func (ns *neighborhoodSearch) trimRegion(r *seglib.Region) bool {

	i := sort.SearchFloat64s(ns.dists, maxradius*metersPerMile)
	if i == 0 {
		os.Stderr.WriteString("Skipping region:\n")
		os.Stderr.WriteString(fmt.Sprintf("%+v\n", r))
		return false
	} else if i < len(ns.dists) {
		ns.nbds = ns.nbds[0:i]
		ns.inds = ns.inds[0:i]
		ns.dists = ns.dists[0:i]
	}

	return true
}

// Find the subset with total population closest to targetpop
func (ns *neighborhoodSearch) matchTarget(targetpop int) {

	var k int
	rpop := 0
	for k = range ns.nbds {
		rpop += ns.nbds[k].TotalPop
		if rpop > targetpop {
			break
		}
	}

	if k > 0 {
		lastpop := ns.nbds[k].TotalPop
		if rpop-targetpop > targetpop-(rpop-lastpop) {
			k--
			rpop -= lastpop
		}
	}

	ns.nbds = ns.nbds[0 : k+1]
	ns.dists = ns.dists[0 : k+1]
}

func (ns *neighborhoodSearch) findNeighborhood(r *seglib.Region, targetpop int) ([]*seglib.Region, []float64) {

	ns.initialCandidates(r, targetpop)
	ns.sortByDist(r)

	if !ns.trimRegion(r) {
		return nil, nil
	}

	ns.matchTarget(targetpop)

	return ns.nbds, ns.dists
}

func clip01(x float64) float64 {
	if x < 0 {
		return 0
	}
	if x > 1 {
		return 1
	}
	return x
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
		msg := fmt.Sprintf("Unknown sumlevel '%s'\n", sl)
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

	var ns neighborhoodSearch
	ns.init(qt, 1000)

	for _, r := range regions {

		// The pseudo-CBSA
		nbds, dists := ns.findNeighborhood(r, 100000)
		r.PCBSATotalPop = 0
		r.PCBSABlackOnlyPop = 0
		r.PCBSAWhiteOnlyPop = 0
		for _, z := range nbds {
			r.PCBSATotalPop += z.TotalPop
			r.PCBSABlackOnlyPop += z.BlackOnlyPop
			r.PCBSAWhiteOnlyPop += z.WhiteOnlyPop
		}
		if len(dists) > 0 {
			r.PCBSARadius = dists[len(dists)-1] / metersPerMile
		}

		// The local region
		nbds, dists = ns.findNeighborhood(r, targetpop)
		if len(nbds) == 0 {
			continue
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
			var denom float64
			if r.CBSA == NullCBSA {
				denom = float64(r.PCBSATotalPop - r.PCBSABlackOnlyPop)
			} else {
				denom = float64(r.CBSATotalPop - r.CBSABlackOnlyPop)
			}
			r.BlackIsolation = clip01(1 - numer/denom)

			var qr1, qr2 float64
			if r.CBSA == NullCBSA {
				qr1 = float64(nBlack) / float64(r.PCBSABlackOnlyPop)
				qr1 = clip01(qr1)
				qr2 = float64(nTotal-nBlack) / float64(r.PCBSATotalPop-r.PCBSABlackOnlyPop)
				qr2 = clip01(qr2)
			} else {
				qr1 = float64(nBlack) / float64(r.CBSABlackOnlyPop)
				qr1 = clip01(qr1)
				qr2 = float64(nTotal-nBlack) / float64(r.CBSATotalPop-r.CBSABlackOnlyPop)
				qr2 = clip01(qr2)
			}
			r.BODissimilarity = math.Abs(qr1 - qr2)

			numer = float64(nTotal - nWhite)
			if r.CBSA == NullCBSA {
				denom = float64(r.PCBSATotalPop - r.PCBSAWhiteOnlyPop)
			} else {
				denom = float64(r.CBSATotalPop - r.CBSAWhiteOnlyPop)
			}
			r.WhiteIsolation = clip01(1 - numer/denom)

			if r.CBSA == NullCBSA {
				qr1 = float64(nWhite) / float64(r.PCBSAWhiteOnlyPop)
				qr2 = float64(nTotal-nWhite) / float64(r.PCBSATotalPop-r.PCBSAWhiteOnlyPop)
			} else {
				qr1 = float64(nWhite) / float64(r.CBSAWhiteOnlyPop)
				qr2 = float64(nTotal-nWhite) / float64(r.CBSATotalPop-r.CBSAWhiteOnlyPop)
			}
			r.WODissimilarity = math.Abs(qr1 - qr2)
		}

		// Regional entropy
		{
			pBlack := nBlack / nTotal
			if pBlack < 1e-4 {
				pBlack = 1e-4
			}
			pWhite := nWhite / nTotal
			if pWhite < 1e-4 {
				pWhite = 1e-4
			}
			pOther := 1 - pBlack - pWhite
			if pOther < 1e-4 {
				pOther = 1e-4
			}
			if nTotal > 0 {
				r.RegionalEntropy = -pBlack * math.Log(pBlack)
				r.RegionalEntropy -= pWhite * math.Log(pWhite)
				r.RegionalEntropy -= pOther * math.Log(pOther)
			}
		}

		err := enc.Encode(r)
		if err != nil {
			panic(err)
		}
	}
}
