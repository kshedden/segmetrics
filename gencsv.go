package main

import (
	"compress/gzip"
	"encoding/csv"
	"encoding/gob"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/kshedden/segregation/seglib"
)

func main() {

	inName := os.Args[1]
	if !strings.HasSuffix(inName, ".gob.gz") {
		panic("Invalid input file\n")
	}
	fmt.Printf("Reading regions from '%s'\n", inName)

	outName := strings.Replace(inName, ".gob.gz", ".csv.gz", 1)
	if outName == inName {
		panic("Invalid input file\n")
	}
	fmt.Printf("Writing regions to '%s'\n", outName)

	outf, err := os.Create(outName)
	if err != nil {
		panic(err)
	}
	defer outf.Close()

	outg := gzip.NewWriter(outf)
	defer outg.Close()

	outw := csv.NewWriter(outg)
	defer outw.Flush()

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

	// Write out the header
	head := []string{
		"State", "StateId", "County", "Cousub", "Tract", "BlockGroup", "CBSA", "Name", "Lon", "Lat",
		"TotalPop", "BlackOnlyPop", "WhiteOnlyPop",
		"CBSATotalPop", "CBSABlackOnlyPop", "CBSAWhiteOnlyPop",
		"PCBSATotalPop", "PCBSABlackOnlyPop", "PCBSAWhiteOnlyPop",
		"LocalEntropy", "RegionalEntropy",
		"BlackIsolation", "WhiteIsolation", "BlackIsolationResid", "WhiteIsolationResid",
		"BODissimilarity", "WODissimilarity", "BODissimilarityResid", "WODissimilarityResid",
		"Neighbors", "RegionPop", "RegionRadius",
	}
	err = outw.Write(head)
	if err != nil {
		panic(err)
	}

	cr := make([]string, len(head))
	for {
		var r seglib.Region
		err := dec.Decode(&r)
		if err == io.EOF {
			break
		} else if err != nil {
			panic(err)
		}

		cr[0] = r.State
		cr[1] = r.StateId
		cr[2] = r.County
		cr[3] = r.Cousub
		cr[4] = fmt.Sprintf("%s", r.Tract)
		cr[5] = fmt.Sprintf("%s", r.BlockGroup)
		cr[6] = fmt.Sprintf("%s", r.CBSA)
		cr[7] = r.Name
		cr[8] = fmt.Sprintf("%.3f", r.Location[0])
		cr[9] = fmt.Sprintf("%.3f", r.Location[1])
		cr[10] = fmt.Sprintf("%d", r.TotalPop)
		cr[11] = fmt.Sprintf("%d", r.BlackOnlyPop)
		cr[12] = fmt.Sprintf("%d", r.WhiteOnlyPop)
		cr[13] = fmt.Sprintf("%d", r.CBSATotalPop)
		cr[14] = fmt.Sprintf("%d", r.CBSABlackOnlyPop)
		cr[15] = fmt.Sprintf("%d", r.CBSAWhiteOnlyPop)
		cr[16] = fmt.Sprintf("%d", r.PCBSATotalPop)
		cr[17] = fmt.Sprintf("%d", r.PCBSABlackOnlyPop)
		cr[18] = fmt.Sprintf("%d", r.PCBSAWhiteOnlyPop)
		cr[19] = fmt.Sprintf("%.6f", r.LocalEntropy)
		cr[20] = fmt.Sprintf("%.6f", r.RegionalEntropy)
		cr[21] = fmt.Sprintf("%.6f", r.BlackIsolation)
		cr[22] = fmt.Sprintf("%.6f", r.WhiteIsolation)
		cr[23] = fmt.Sprintf("%.6f", r.BlackIsolationResid)
		cr[24] = fmt.Sprintf("%.6f", r.WhiteIsolationResid)
		cr[25] = fmt.Sprintf("%.6f", r.BODissimilarity)
		cr[26] = fmt.Sprintf("%.6f", r.WODissimilarity)
		cr[27] = fmt.Sprintf("%.6f", r.BODissimilarityResid)
		cr[28] = fmt.Sprintf("%.6f", r.WODissimilarityResid)
		cr[29] = fmt.Sprintf("%d", r.Neighbors)
		cr[30] = fmt.Sprintf("%d", r.RegionPop)
		cr[31] = fmt.Sprintf("%.2f", r.RegionRadius)

		if len(head) != len(cr) {
			panic("len(head) ! = len(cr)\n")
		}

		err = outw.Write(cr)
		if err != nil {
			panic(err)
		}
	}
}
