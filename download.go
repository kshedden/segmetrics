package main

import (
	"fmt"
	"os"
	"os/exec"
	"path"

	"github.com/kshedden/segregation/seglib"
)

const (
	// Download data for this year
	year int = 2010
)

var (
	// The data directory name
	dir string

	// The URL for the data files
	base = "ftp.census.gov/census_2010/01-Redistricting_File--PL_94-171/"
)

func main() {

	dir = fmt.Sprintf("data%4d", year)

	for _, state := range seglib.States {

		zipname := fmt.Sprintf("%s%d.pl.zip", state[1], year)

		pa := path.Join(dir, zipname)
		_, err := os.Stat(pa)
		if err != nil && os.IsNotExist(err) {
			// Get the zip file since we don't already have it
			fmt.Printf("Getting %s\n", zipname)
			pa = path.Join(base, state[0], zipname)
			cmd := exec.Command("wget", "ftp://"+pa, fmt.Sprintf("--directory-prefix=%s", dir))
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

		// Compress the files
		for _, f := range []string{
			fmt.Sprintf("geo%4d.pl", year),
			fmt.Sprintf("00001%4d.pl", year),
			fmt.Sprintf("00002%4d.pl", year),
		} {
			pa = path.Join(dir, state[1]+f)
			cmd := exec.Command("gzip", "-f", pa)
			cmd.Stderr = os.Stderr
			if err := cmd.Run(); err != nil {
				panic(err)
			}
		}
	}
}
