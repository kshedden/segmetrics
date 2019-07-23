// https://www.census.gov/prod/cen2010/doc/pl94-171.pdf

package main

import (
	"bufio"
	"compress/gzip"
	"encoding/csv"
	"encoding/gob"
	"flag"
	"fmt"
	"io"
	"os"
	"path"
	"strconv"
	"strings"

	"github.com/kshedden/segregation/seglib"
	"github.com/paulmach/orb"
)

const (
	baseDir = "/dsi/stage/stage/cscar-census"
)

var (
	dir string

	sumlevel seglib.RegionType

	year int

	// 0, 1, 2 correspond to the target sumlevel code for cousub, tract, and blockgroup,
	// respectively
	sumlevelCodes []string

	out *gob.Encoder
)

func main() {

	flag.IntVar(&year, "year", 0, "Census year")
	var sl string
	flag.StringVar(&sl, "sumlevel", "", "Summary level ('blockgroup', 'tract', or 'cousub')")
	flag.Parse()

	switch sl {
	case "cousub":
		sumlevel = seglib.CountySubdivision
	case "tract":
		sumlevel = seglib.Tract
	case "blockgroup":
		sumlevel = seglib.BlockGroup
	default:
		msg := fmt.Sprintf("Unkown sumlevel '%s'\n", sl)
		panic(msg)
	}

	switch year {
	case 2010:
		sumlevelCodes = []string{"060", "140", "150"}
	case 2000:
		sumlevelCodes = []string{"060", "140", "740"}
	default:
		panic("invalid year")
	}

	dir = path.Join(baseDir, "redistricting-data", fmt.Sprintf("%4d", year))

	var fname string
	switch sumlevel {
	case seglib.CountySubdivision:
		fname = fmt.Sprintf("segregation_raw_cousub_%4d.gob.gz", year)
	case seglib.Tract:
		fname = fmt.Sprintf("segregation_raw_tract_%4d.gob.gz", year)
	case seglib.BlockGroup:
		fname = fmt.Sprintf("segregation_raw_blockgroup_%4d.gob.gz", year)
	default:
		panic("Unrecognized summary level\n")
	}

	fid, err := os.Create(fname)
	if err != nil {
		panic(err)
	}
	defer fid.Close()

	gid := gzip.NewWriter(fid)
	defer gid.Close()

	out = gob.NewEncoder(gid)

	var m int
	for _, state := range seglib.States {
		n := doState(state[1])
		fmt.Printf("Found %d records in state %s\n", n, state[1])
		m += n
	}
	fmt.Printf("Found %d records overall\n", m)
}

type demorect struct {
	logrecno  string
	totpop    int
	whiteonly int
	blackonly int
}

func (dr *demorect) parse2010(demorec []string) {

	dr.logrecno = demorec[4]

	var err error
	dr.totpop, err = strconv.Atoi(demorec[76])
	if err != nil {
		panic(err)
	}

	dr.whiteonly, err = strconv.Atoi(demorec[80])
	if err != nil {
		panic(err)
	}

	dr.blackonly, err = strconv.Atoi(demorec[81])
	if err != nil {
		panic(err)
	}
}

func (dr *demorect) parse2000(demorec []string) {

	// It appears to be the same as 2010
	dr.parse2010(demorec)
}

type georect struct {
	sumlevel   string
	stateid    string
	county     string
	cousubPart string
	tractPart  string
	blkgrpPart string
	logrecno   string
	name       string
	cbsa       string
	lat        float64
	lon        float64
}

func (gr *georect) parse2010(georec string) {

	gr.sumlevel = georec[8 : 8+3]
	gr.stateid = georec[27 : 27+2]
	gr.county = georec[29 : 29+3]
	gr.cousubPart = georec[36 : 36+5]
	gr.tractPart = strings.TrimSpace(georec[54 : 54+6])
	gr.blkgrpPart = strings.TrimSpace(georec[60 : 60+1])
	gr.logrecno = georec[18 : 18+7]
	gr.name = strings.TrimSpace(georec[226 : 226+90])
	gr.cbsa = georec[112 : 112+5]

	var err error
	gr.lat, err = strconv.ParseFloat(georec[336:336+11], 64)
	if err != nil {
		panic(err)
	}
	gr.lon, err = strconv.ParseFloat(georec[347:347+12], 64)
	if err != nil {
		panic(err)
	}
}

func (gr *georect) parse2000(georec string) {

	gr.sumlevel = georec[8 : 8+3]
	gr.stateid = georec[29 : 29+2]
	gr.county = georec[31 : 31+3]
	gr.cousubPart = georec[36 : 36+5]
	gr.tractPart = strings.TrimSpace(georec[55 : 55+6])
	gr.blkgrpPart = strings.TrimSpace(georec[61 : 61+1])
	gr.logrecno = georec[18 : 18+7]
	gr.name = strings.TrimSpace(georec[200 : 200+90])
	gr.cbsa = georec[106 : 106+4] // CMSA

	var err error
	gr.lat, err = strconv.ParseFloat(georec[310:310+9], 64)
	if err != nil {
		panic(err)
	}
	gr.lon, err = strconv.ParseFloat(georec[319:319+10], 64)
	if err != nil {
		panic(err)
	}

	// No decimal place in the these files
	gr.lat /= 1e6
	gr.lon /= 1e6
}

func doState(state string) int {

	var gfn string
	switch year {
	case 1990:
		panic("invalid year")
	case 2000:
		gfn = fmt.Sprintf("%sgeo.upl.gz", state)
	case 2010:
		gfn = fmt.Sprintf("%sgeo%4d.pl.gz", state, year)
	}

	pa := path.Join(dir, gfn)
	geof, err := os.Open(pa)
	if err != nil {
		panic(err)
	}

	geoz, err := gzip.NewReader(geof)
	if err != nil {
		panic(err)
	}

	var dfn string
	switch year {
	case 1990:
		panic("invalid year")
	case 2000:
		dfn = fmt.Sprintf("%s00001.upl.gz", state)
	case 2010:
		dfn = fmt.Sprintf("%s00001%4d.pl.gz", state, year)
	}

	pa = path.Join(dir, dfn)
	demof, err := os.Open(pa)
	if err != nil {
		panic(err)
	}

	demoz, err := gzip.NewReader(demof)
	if err != nil {
		panic(err)
	}

	democsv := csv.NewReader(demoz)
	geoscanner := bufio.NewScanner(geoz)

	var n int
	grt := new(georect)
	drt := new(demorect)
	for {
		demorec, err := democsv.Read()
		if err == io.EOF {
			break
		} else if err != nil {
			panic(err)
		}

		if !geoscanner.Scan() {
			break
		}
		georec := geoscanner.Text()

		switch year {
		case 2010:
			drt.parse2010(demorec)
			grt.parse2010(georec)
		case 2000:
			drt.parse2000(demorec)
			grt.parse2000(georec)
		default:
			panic("invalid year")
		}

		switch sumlevel {
		case seglib.CountySubdivision:
			if grt.sumlevel != sumlevelCodes[0] {
				continue
			}
		case seglib.Tract:
			if grt.sumlevel != sumlevelCodes[1] {
				continue
			}
		case seglib.BlockGroup:
			if grt.sumlevel != sumlevelCodes[2] {
				continue
			}
		default:
			panic("Unrecognized summary level\n")
		}

		var tract, blockgrp, cousub string
		switch sumlevel {
		case seglib.CountySubdivision:
			cousub = grt.stateid + grt.county + grt.cousubPart
		case seglib.Tract:
			tract = grt.stateid + grt.county + grt.tractPart
		case seglib.BlockGroup:
			blockgrp = grt.stateid + grt.county + grt.tractPart + grt.blkgrpPart
		default:
			panic("unknown sumlevel")
		}

		if grt.logrecno != drt.logrecno {
			panic("Record number mismatch\n")
		}

		s := seglib.Region{
			State:        state,
			StateId:      grt.stateid,
			County:       grt.county,
			Cousub:       cousub,
			Tract:        tract,
			BlockGroup:   blockgrp,
			Name:         grt.name,
			Type:         seglib.Tract,
			CBSA:         grt.cbsa,
			Location:     orb.Point{grt.lon, grt.lat},
			TotalPop:     drt.totpop,
			BlackOnlyPop: drt.blackonly,
			WhiteOnlyPop: drt.whiteonly,
		}

		err = out.Encode(&s)
		if err != nil {
			panic(err)
		}
		n++
	}

	return n
}
