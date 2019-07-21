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
	"io/ioutil"
	"os"
	"path"
	"strconv"
	"strings"

	"github.com/kshedden/segregation/seglib"
	"github.com/paulmach/orb"
)

var (
	dir string

	sumlevel seglib.RegionType

	year int

	out *gob.Encoder
)

func main() {

	flag.IntVar(&year, "year", 0, "Census year")
	var sl string
	flag.StringVar(&sl, "sumlevel", "", "Summary level ('blockgroup', 'tract', or 'ccd')")
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

	if year != 2010 {
		msg := fmt.Sprintf("Invalid year '%d'\n", year)
		panic(msg)
	}

	dir = fmt.Sprintf("data%4d", year)

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

	files, err := ioutil.ReadDir(dir)
	if err != nil {
		panic(err)
	}

	var m int
	for _, file := range files {
		name := file.Name()
		if strings.HasSuffix(name, ".zip") {
			state := name[0:2]
			n := doState(state)
			fmt.Printf("Found %d records in state %s\n", n, state)
			m += n
		}
	}
	fmt.Printf("Found %d records overall\n", m)
}

func doState(state string) int {

	pa := path.Join(dir, fmt.Sprintf("%sgeo%4d.pl.gz", state, year))
	geof, err := os.Open(pa)
	if err != nil {
		panic(err)
	}

	geoz, err := gzip.NewReader(geof)
	if err != nil {
		panic(err)
	}

	pa = path.Join(dir, fmt.Sprintf("%s00001%4d.pl.gz", state, year))
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

		// Keep only one type of region
		sumlev := georec[8 : 8+3]

		switch sumlevel {
		case seglib.CountySubdivision:
			if sumlev != "060" {
				continue
			}
		case seglib.Tract:
			if sumlev != "140" {
				continue
			}
		case seglib.BlockGroup:
			if sumlev != "150" {
				continue
			}
		default:
			panic("Unrecognized summary level\n")
		}

		stateid := georec[27 : 27+2]
		county := georec[29 : 29+3]
		cousubPart := georec[36 : 36+5]
		tractPart := strings.TrimSpace(georec[54 : 54+6])
		blkgrpPart := strings.TrimSpace(georec[60 : 60+1])

		var tract, blockgrp, cousub string
		switch sumlevel {
		case seglib.CountySubdivision:
			cousub = stateid + county + cousubPart
		case seglib.Tract:
			tract = stateid + county + tractPart
		case seglib.BlockGroup:
			blockgrp = stateid + county + tractPart + blkgrpPart
		default:
			panic("unkown sumlevel")
		}

		logrecno := georec[18 : 18+7]
		if logrecno != demorec[4] {
			panic("Record number mismatch\n")
		}

		name := strings.TrimSpace(georec[226 : 226+90])
		cbsa := georec[112 : 112+5]

		lat, err := strconv.ParseFloat(georec[336:336+11], 64)
		if err != nil {
			panic(err)
		}
		lon, err := strconv.ParseFloat(georec[347:347+12], 64)
		if err != nil {
			panic(err)
		}

		totpop, err := strconv.Atoi(demorec[76])
		if err != nil {
			panic(err)
		}

		// Non-Hispanic white (one race)
		whiteonly, err := strconv.Atoi(demorec[80])
		if err != nil {
			panic(err)
		}

		// Non-Hispanic black (one race)
		blackonly, err := strconv.Atoi(demorec[81])
		if err != nil {
			panic(err)
		}

		s := seglib.Region{
			State:        state,
			StateId:      stateid,
			County:       county,
			Cousub:       cousub,
			Tract:        tract,
			BlockGroup:   blockgrp,
			Name:         name,
			Type:         seglib.Tract,
			CBSA:         cbsa,
			Location:     orb.Point{lon, lat},
			TotalPop:     totpop,
			BlackOnlyPop: blackonly,
			WhiteOnlyPop: whiteonly,
		}

		err = out.Encode(&s)
		if err != nil {
			panic(err)
		}
		n++
	}

	return n
}
