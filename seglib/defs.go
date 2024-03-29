package seglib

import (
	"github.com/paulmach/orb"
)

type RegionType uint8

const (
	Tract RegionType = iota
	BlockGroup
	CountySubdivision
)

type Region struct {

	// These values depend only on this region
	State        string
	StateId      string
	County       string
	Cousub       string
	Tract        string
	BlockGroup   string
	Name         string
	CBSA         string
	Type         RegionType
	Location     orb.Point
	TotalPop     int
	BlackOnlyPop int
	WhiteOnlyPop int

	CBSATotalPop     int
	CBSABlackOnlyPop int
	CBSAWhiteOnlyPop int

	// Pseudo-CBSA
	PCBSATotalPop     int
	PCBSABlackOnlyPop int
	PCBSAWhiteOnlyPop int

	// These values depend on the region's neighbors
	RegionPop    int
	RegionRadius float64

	// The smoothed proportion of each race in this region
	PBlack float64
	PWhite float64

	// Isolation measures
	BlackIsolation      float64
	WhiteIsolation      float64
	BlackIsolationResid float64
	WhiteIsolationResid float64

	// Dissimilarity measures
	BODissimilarity      float64
	WODissimilarity      float64
	BODissimilarityResid float64
	WODissimilarityResid float64

	LocalEntropy    float64
	RegionalEntropy float64

	Neighbors int
}

// point allows Region to satisfy the orb.Pointer interface
func (r *Region) Point() orb.Point {
	return r.Location
}
