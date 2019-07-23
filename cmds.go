package main

import (
	"fmt"
	"os"
	"strings"
)

const (
	base1 = "go run metrics.go -sumlevel=SUMLEVEL -targetpop=TARGETPOP -year=YEAR -outfile=segregation_SUMLEVEL_YEAR_TARGETPOP.gob.gz"
	base2 = "go run gencsv.go segregation_SUMLEVEL_YEAR_TARGETPOP_norm.gob.gz"
	base3 = "go run normalize.go segregation_SUMLEVEL_YEAR_TARGETPOP.gob.gz"
	base4 = "rclone copy segregation_SUMLEVEL_YEAR_TARGETPOP_norm.csv.gz"
)

func main() {

	cmd := os.Args[1]
	sumlevel := os.Args[2]

	yearsr := os.Args[3]
	years := strings.Split(yearsr, ",")

	popr := os.Args[4]
	pops := strings.Split(popr, ",")

	var base string
	switch cmd {
	case "metrics":
		base = base1
	case "gencsv":
		base = base2
	case "normalize":
		base = base3
	case "upload":
		base = base4
	default:
		panic("unknown cmd\n")
	}

	if sumlevel != "cousub" {
		for _, pop := range pops {
			for _, year := range years {
				b := strings.ReplaceAll(base, "SUMLEVEL", sumlevel)
				b = strings.ReplaceAll(b, "YEAR", year)
				b = strings.ReplaceAll(b, "TARGETPOP", pop)
				fmt.Printf("%s\n", b)
			}
		}
	} else {
		// No target population size for county subdivisions
		basex := base
		basex = strings.ReplaceAll(base, "_TARGETPOP", "")
		basex = strings.ReplaceAll(basex, "-targetpop=TARGETPOP", "")
		for _, year := range years {
			b := strings.ReplaceAll(basex, "SUMLEVEL", sumlevel)
			b = strings.ReplaceAll(b, "YEAR", year)
			fmt.Printf("%s\n", b)
		}
	}
}
