// Reference for Getis-Ord statistics
// https://onlinelibrary.wiley.com/doi/pdf/10.1111/j.1538-4632.1995.tb00912.x

package main

import (
	"compress/gzip"
	"encoding/gob"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"math"
	"os"
	"path"
	"sort"
	"strings"

	shp "github.com/jonas-p/go-shp"
	"github.com/kshedden/segregation/seglib"
	"github.com/paulmach/orb"
	"github.com/paulmach/orb/geo"
	"github.com/paulmach/orb/quadtree"
	"gonum.org/v1/gonum/floats"
)

var (
	year int

	// 99999 for 2010, 9999 for 2000
	nullCBSA string

	sumlevel seglib.RegionType

	regions []*seglib.Region

	// Bounding boxes of the regions (only used for COUSUBs)
	regboxes map[string]orb.Bound

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
	case seglib.CountySubdivision:
		fname = fmt.Sprintf("segregation_raw_cousub_%4d.gob.gz", year)
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

// Find the quadrant where q lies relative to r.
func quad(r, q orb.Point) int {
	angle := math.Atan2(q[1]-r[1], q[0]-r[0])
	return int(math.Floor(2 * (1 + angle/math.Pi)))
}

// Find enough regions so that the total population exceeds the target population.
func selectByDirection(qt *quadtree.Quadtree, r *seglib.Region) ([4]*seglib.Region, int) {

	var nbrs [4]*seglib.Region
	var buf []orb.Pointer
	var n int

	for k := 4; k <= 10; k++ {

		buf = qt.KNearest(buf, r.Location, k)

		for _, n := range buf {
			nr := n.(*seglib.Region)
			q := quad(r.Location, nr.Location)
			nbrs[q] = nr
		}

		n = 0
		for _, x := range nbrs {
			if x != nil {
				n++
			}
		}
	}

	return nbrs, n
}

func getShapes() map[string]orb.Bound {

	shapes := make(map[string]orb.Bound)

	files, err := ioutil.ReadDir(path.Join("shapefiles", "cousub"))
	if err != nil {
		panic(err)
	}

	for _, file := range files {

		if !strings.HasSuffix(file.Name(), ".shp") {
			continue
		}

		sf := path.Join("shapefiles", "cousub", file.Name())
		shapef, err := shp.Open(sf)
		if err != nil {
			panic(err)
		}
		defer shapef.Close()

		// fields from the attribute table (DBF)
		fields := shapef.Fields()
		if fields[0].String() != "GEO_ID" {
			panic("inconsistent layout")
		}

		// Loop through all features in the shapefile
		for shapef.Next() {
			n, p := shapef.Shape()
			geoid := shapef.ReadAttribute(n, 0)
			cousub := geoid[9:] // State + County + Cousub
			pb := p.BBox()
			pmin := orb.Point{pb.MinX, pb.MinY}
			pmax := orb.Point{pb.MaxX, pb.MaxY}
			box := orb.MultiPoint{pmin, pmax}.Bound()

			shapes[cousub] = box
		}
	}

	return shapes
}

func findNeighbors(qt *quadtree.Quadtree, r *seglib.Region) []*seglib.Region {

	buf := qt.KNearest(nil, r.Location, 20)
	var nbd []*seglib.Region

	tbox := regboxes[r.Cousub]

	for _, q := range buf {
		qr := q.(*seglib.Region)
		qbox := regboxes[qr.Cousub]
		if tbox.Pad(0.001).Intersects(qbox) {
			nbd = append(nbd, qr)
		}
	}

	return nbd
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
	flag.StringVar(&sl, "sumlevel", "", "Summary level ('blockgroup', 'cousub', or 'tract')")
	flag.IntVar(&targetpop, "targetpop", 0, "Target population")
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
	case "cousub":
		sumlevel = seglib.CountySubdivision
	case "tract":
		sumlevel = seglib.Tract
	case "blockgroup":
		sumlevel = seglib.BlockGroup
	default:
		msg := fmt.Sprintf("Unknown sumlevel '%s'\n", sl)
		panic(msg)
	}

	switch year {
	case 2010:
		nullCBSA = "99999"
	case 2000:
		nullCBSA = "9999"
	default:
		panic("Invalid year")
	}

	if sumlevel == seglib.CountySubdivision && targetpop != 0 {
		msg := "When using county subdivisions, do not set targetpop"
		panic(msg)
	}

	getRegions()
	getCBSAStats()

	if sumlevel == seglib.CountySubdivision {
		regboxes = getShapes()
	}

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

		// The outer container based on cardinal directions
		if sumlevel == seglib.CountySubdivision {
			cdn := findNeighbors(qt, r)
			r.PCBSATotalPop = r.TotalPop
			r.PCBSABlackOnlyPop = r.BlackOnlyPop
			r.PCBSAWhiteOnlyPop = r.WhiteOnlyPop
			for _, z := range cdn {
				if z != nil {
					r.PCBSATotalPop += z.TotalPop
					r.PCBSABlackOnlyPop += z.BlackOnlyPop
					r.PCBSAWhiteOnlyPop += z.WhiteOnlyPop
				}
			}
		}

		// The local region
		var nbds []*seglib.Region
		var dists []float64
		if sumlevel != seglib.CountySubdivision {
			nbds, dists = ns.findNeighborhood(r, targetpop)
			if len(nbds) == 0 {
				continue
			}
		} else {
			nbds = []*seglib.Region{r}
			dists = []float64{0}
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

			// Use pseudocounts to avoid log(0) in entropy.
			popt := 2 + float64(z.TotalPop)
			bopt := 1 + float64(z.BlackOnlyPop)
			wopt := 1 + float64(z.WhiteOnlyPop)

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
			var numer, denom float64
			if r.CBSA == nullCBSA {
				numer = float64(r.TotalPop - r.BlackOnlyPop)
				denom = float64(r.PCBSATotalPop - r.PCBSABlackOnlyPop)
			} else {
				numer = float64(nTotal - nBlack)
				denom = float64(r.CBSATotalPop - r.CBSABlackOnlyPop)
			}
			r.BlackIsolation = clip01(1 - numer/denom)

			var qr1, qr2 float64
			if r.CBSA == nullCBSA {
				qr1 = float64(r.BlackOnlyPop) / float64(r.PCBSABlackOnlyPop)
				qr2 = float64(r.TotalPop-r.BlackOnlyPop) / float64(r.PCBSATotalPop-r.PCBSABlackOnlyPop)
			} else {
				qr1 = float64(nBlack) / float64(r.CBSABlackOnlyPop)
				qr2 = float64(nTotal-nBlack) / float64(r.CBSATotalPop-r.CBSABlackOnlyPop)
			}
			r.BODissimilarity = math.Abs(clip01(qr1) - clip01(qr2))

			if r.CBSA == nullCBSA {
				numer = float64(r.TotalPop - r.WhiteOnlyPop)
				denom = float64(r.PCBSATotalPop - r.PCBSAWhiteOnlyPop)
			} else {
				numer = float64(nTotal - nWhite)
				denom = float64(r.CBSATotalPop - r.CBSAWhiteOnlyPop)
			}
			r.WhiteIsolation = clip01(1 - numer/denom)

			if r.CBSA == nullCBSA {
				qr1 = float64(nWhite) / float64(r.PCBSAWhiteOnlyPop)
				qr2 = float64(nTotal-nWhite) / float64(r.PCBSATotalPop-r.PCBSAWhiteOnlyPop)
			} else {
				qr1 = float64(nWhite) / float64(r.CBSAWhiteOnlyPop)
				qr2 = float64(nTotal-nWhite) / float64(r.CBSATotalPop-r.CBSAWhiteOnlyPop)
			}
			r.WODissimilarity = math.Abs(clip01(qr1) - clip01(qr2))
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

		err = enc.Encode(r)
		if err != nil {
			panic(err)
		}
	}
}
