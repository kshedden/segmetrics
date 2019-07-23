// Download census redistricting data.
//
// Data codebooks:
// 1990  ???
// 2000  https://www.census.gov/prod/cen2000/doc/pl94-171.pdf
// 2010  https://www.census.gov/prod/cen2010/doc/pl94-171.pdf

package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path"

	"github.com/kshedden/segregation/seglib"
)

const (
	baseDir = "/dsi/stage/stage/cscar-census"

	// The URLs for the data files
	base1990 = "www2.census.gov/census_1990/????"
	base2000 = "www2.census.gov/census_2000/datasets/redistricting_file--pl_94-171"
	base2010 = "www2.census.gov/census_2010/01-Redistricting_File--PL_94-171/"
)

var (
	// The data directory name
	dir string
)

func getStateFiles(year int, state string) ([]string, []string) {

	switch year {
	case 2010:
		zip := []string{fmt.Sprintf("%s%d.pl.zip", state, year)}
		fi := []string{
			fmt.Sprintf("%s00001%4d.pl", state, year),
			fmt.Sprintf("%s00002%4d.pl", state, year),
			fmt.Sprintf("%sgeo%4d.pl", state, year),
		}
		return zip, fi
	case 2000:
		zip := []string{
			fmt.Sprintf("%s00001.upl.zip", state),
			fmt.Sprintf("%s00002.upl.zip", state),
			fmt.Sprintf("%sgeo.upl.zip", state),
		}
		fi := []string{
			fmt.Sprintf("%s00001.upl", state),
			fmt.Sprintf("%s00002.upl", state),
			fmt.Sprintf("%sgeo.upl", state),
		}
		return zip, fi
	case 1990:
		return nil, nil // TODO
	default:
		panic("unknown year")
	}
}

func main() {

	yearx := flag.Int("year", 2010, "year of census data to download")
	flag.Parse()
	year := *yearx
	if year != 1990 && year != 2000 && year != 2010 {
		panic("Invalid year")
	}
	println(fmt.Sprintf("Downloading census data for year %d\n", year))

	var wwwbase string
	switch year {
	case 2010:
		wwwbase = base2010
	case 2000:
		wwwbase = base2000
	case 19990:
		wwwbase = base1990
	default:
		panic("invalid year")
	}

	dir = path.Join(baseDir, "redistricting-data", fmt.Sprintf("%4d", year))

	for _, state := range seglib.States {

		// The names of all state archive files
		zipnames, finames := getStateFiles(year, state[1])

		for _, zipname := range zipnames {

			// Check if we already have the archive
			pa := path.Join(dir, zipname)
			_, err := os.Stat(pa)
			if err != nil && os.IsNotExist(err) {

				// Get the zip file if we don't already have it
				fmt.Printf("Getting %s\n", zipname)
				pa = path.Join(wwwbase, state[0], zipname)
				cmds := []string{"wget", "https://" + pa, fmt.Sprintf("--directory-prefix=%s", dir)}
				cmd := exec.Command(cmds[0], cmds[1:]...)
				cmd.Stderr = os.Stderr
				err = cmd.Run()
				if err != nil {
					panic(err)
				}
			} else if err != nil {
				panic(err)
			} else {
				// Don't get the zip file if we already have it
				fmt.Printf("Skipping download of %s\n", zipname)
			}

			// Unzip the zip file
			fmt.Printf("Unzipping %s\n", zipname)
			pa = path.Join(dir, zipname)
			cmd := exec.Command("unzip", "-o", "-d", dir, pa)
			cmd.Stderr = os.Stderr
			if err := cmd.Run(); err != nil {
				panic(err)
			}
		}

		// gzip compress the files that we want
		for _, f := range finames {
			pa := path.Join(dir, f)
			cmd := exec.Command("gzip", "-f", pa)
			cmd.Stderr = os.Stderr
			if err := cmd.Run(); err != nil {
				panic(err)
			}
		}
	}
}
