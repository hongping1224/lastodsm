// Copyright 2019 Hong-Ping Lo. All rights reserved.
// Use of this source code is governed by a BDS-style
// license that can be found in the LICENSE file.

package main

import (
	"encoding/binary"
	"flag"
	"fmt"
	"image"
	"log"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/hongping1224/lastodsm/counter"
	tiff32 "github.com/hongping1224/lastodsm/tiff"
	"github.com/jblindsay/lidario"
)

var numOFCPU int

func main() {

	numOFCPU = runtime.NumCPU()
	flag.IntVar(&numOFCPU, "cpuCount", numOFCPU, "Cpu use for compute")
	dir := "./"
	flag.StringVar(&dir, "dir", dir, "directory to process")
	gap := float64(0.01)
	flag.Float64Var(&gap, "size", gap, "pixel size")
	outPath := ""
	flag.StringVar(&outPath, "out", outPath, "Output Dir")
	flag.Parse()
	start := time.Now()
	defer exit(start)
	fmt.Printf("Running Program on %v Thread(s)\n", numOFCPU)
	runtime.GOMAXPROCS(numOFCPU)
	//run on a directory
	//check directory exist
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		log.Fatal(err)
		return
	}

	//find all las file
	fmt.Println(dir)
	lasfile := findFile(dir, ".las")
	//if mode 0, output each time

	for _, path := range lasfile {
		o := checkOutPath(outPath, path, 0)
		fmt.Println("Calculating", path)
		fmt.Println("out path ", o)
		Run(path, gap, o)
	}

}

func exit(start time.Time) {
	end := time.Now()
	elp := end.Sub(start)
	fmt.Println("Finish Job, Used :", elp)
}

//SaveLabelTiff Save DensityMap into tiff
func SaveLabelTiff(filename string, DensityMap []lidario.LasPointer, w, h int, gap float64, LB counter.Point) {
	//convert to 2d array
	//index := int(math.Floor(xr)) + (int(math.Floor(yr)) * w)
	fmt.Println("Creating Image")
	im := tiff32.NewGray32(image.Rectangle{Min: image.Point{X: 0, Y: 0}, Max: image.Point{X: w, Y: h}})
	for y := h - 1; y > -1; y-- {
		for x := 0; x < w; x++ {
			i := x + (y * w)
			data := DensityMap[i].PointData()
			im.SetGray32(x, h-1-y, tiff32.Gray32Color{Y: uint32(data.PointSourceID)})
		}
	}
	//write tfw file
	fmt.Println("Saving Tiff file at:", filename)
	f, err := os.Create(filename + "_label.tiff")
	if err != nil {
		log.Fatal(err)
		return
	}
	err = tiff32.Encode(f, im, nil)
	if err != nil {
		log.Fatal(err)
	}
	f.Close()
	tfw, err := os.Create(filename + "_label.tfw")
	gaps := fmt.Sprintf("%g", gap)
	tfw.WriteString(gaps + "\n0\n0\n-" + gaps + "\n")
	tfw.WriteString(fmt.Sprintf("%.5f", LB.X+(gap/2)) + "\n")
	tfw.WriteString(fmt.Sprintf("%.5f", LB.Y+(gap*(float64(h)-0.5))))
	return
}

//SaveDSMTiff Save DensityMap into tiff
func SaveDSMTiff(filename string, DensityMap []lidario.LasPointer, w, h int, gap float64, LB counter.Point) {
	//convert to 2d array
	//index := int(math.Floor(xr)) + (int(math.Floor(yr)) * w)
	fmt.Println("Creating Image")
	im := tiff32.NewGrayFloat32(image.Rectangle{Min: image.Point{X: 0, Y: 0}, Max: image.Point{X: w, Y: h}})
	for y := h - 1; y > -1; y-- {
		for x := 0; x < w; x++ {
			i := x + (y * w)
			data := DensityMap[i].PointData()
			im.SetGray32(x, h-1-y, tiff32.GrayFloat32Color{Y: uint32frombytes(float32bytes(float32(data.Z)))})
		}
	}
	//write tfw file
	fmt.Println("Saving Tiff file at:", filename)
	f, err := os.Create(filename + "_dsm.tiff")
	if err != nil {
		log.Fatal(err)
		return
	}
	err = tiff32.Encode(f, im, nil)
	if err != nil {
		log.Fatal(err)
	}
	f.Close()
	tfw, err := os.Create(filename + "_dsm.tfw")
	gaps := fmt.Sprintf("%g", gap)
	tfw.WriteString(gaps + "\n0\n0\n-" + gaps + "\n")
	tfw.WriteString(fmt.Sprintf("%.5f", LB.X+(gap/2)) + "\n")
	tfw.WriteString(fmt.Sprintf("%.5f", LB.Y+(gap*(float64(h)-0.5))))
	return
}

//Run Calculate Point Density On a Single File
func Run(filepath string, gap float64, outPath string) {
	lf, err := openLasFile(filepath)
	if err != nil {
		log.Println(err)
		return
	}
	densityMap, xrange, yrange := Calculate(lf, lf.Header.MinX, lf.Header.MinY, lf.Header.MaxX, lf.Header.MaxY, gap)
	fmt.Println(xrange, yrange)
	LB := counter.Point{X: lf.Header.MinX, Y: lf.Header.MinY}
	SaveDSMTiff(outPath, densityMap, xrange, yrange, gap, LB)
	SaveLabelTiff(outPath, densityMap, xrange, yrange, gap, LB)
}

func totalPoint(files []string) (total int) {
	total = 0
	for _, path := range files {
		las, err := openLasHeader(path)
		if err != nil {
			log.Fatal(err)
		}
		total += las.Header.NumberPoints
		las.Close()
	}
	return
}

func findBoundary(files []string) (minx, miny, maxx, maxy float64) {
	minx = math.MaxFloat64
	miny = math.MaxFloat64
	maxx = 0.0
	maxy = 0.0
	for _, path := range files {
		las, err := openLasHeader(path)
		if err != nil {
			log.Fatal(err)
		}
		minx = math.Min(las.Header.MinX, minx)
		miny = math.Min(las.Header.MinY, miny)
		maxx = math.Max(las.Header.MaxX, maxx)
		maxy = math.Max(las.Header.MaxY, maxy)
		las.Close()
	}
	return
}

//Calculate setup 1 reader and 1 writer for each Counter
//each Counter reads write 1/NumOfCPU of points, and add em up together in the end
func Calculate(lf *lidario.LasFile, MinX, MinY, MaxX, MaxY, gap float64) ([]lidario.LasPointer, int, int) {
	cpu := numOFCPU / 2
	counters := make([]*counter.Counter, cpu)
	//make counter accoding to num CPU
	for i := 0; i < cpu; i++ {
		counters[i] = &counter.Counter{}
		counters[i].Init(MinX, MinY, MaxX, MaxY, gap)
		read := counter.Reader{}
		go read.Serve(counters[i].ReadStream, counters[i])
		write := counter.Writer{}
		go write.Serve(counters[i].WriteStream, counters[i])
	}
	//open las file
	//assign stating point, and ending point

	var wg sync.WaitGroup

	nOP := make([]int, cpu+1)
	nOP[0] = 0
	nOP[cpu] = lf.Header.NumberPoints

	for i := 1; i < cpu; i++ {
		nOP[i] = lf.Header.NumberPoints / cpu * i
	}

	for i := 0; i < cpu; i++ {
		wg.Add(1)
		go counters[i].Count(nOP[i], nOP[i+1], lf, &wg)
	}

	wg.Wait()
	//add em up when all done.
	fmt.Println("Merging Result")
	mapsize := len(counters[0].DensityMap)
	for j := 1; j < cpu; j++ {
		for i := 0; i < mapsize; i++ {
			other := counters[j].DensityMap[i].PointData().Z
			self := counters[0].DensityMap[i].PointData().Z
			if other > self {
				counters[0].DensityMap[i] = counters[j].DensityMap[i]
			}
		}
	}
	return counters[0].DensityMap, counters[0].XRange, counters[0].YRange
}

//0 for file , 1 for dir
func checkOutPath(outPath, inPath string, mode int) string {
	outdir, outfile := filepath.Split(outPath)
	indir, infile := filepath.Split(inPath)
	fmt.Println(outfile)
	if mode == 0 {
		if outdir == "" {
			outdir = indir
		}
		if outfile == "" {
			outfile = "PointDensity_" + strings.TrimSuffix(infile, ".las")
		} else {
			outfile = strings.TrimSuffix(outfile, filepath.Ext(outfile))
		}
	}
	if mode == 1 {
		outfile = "PointDensity"
		if outdir == "" {
			outdir = indir
		}
	}
	return filepath.Join(outdir, outfile)
}

func float32bytes(float float32) []byte {
	bits := math.Float32bits(float)
	bytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(bytes, bits)
	return bytes
}
func uint32frombytes(b []byte) uint32 {
	tt := binary.LittleEndian.Uint32(b)
	return tt
}
func uint32bytes(usignint uint32) []byte {
	buf := make([]byte, 4)
	buf[0] = byte(usignint)
	buf[1] = byte(usignint >> 8)
	buf[2] = byte(usignint >> 16)
	buf[3] = byte(usignint >> 24)
	return buf
}

func float32frombytes(bytes []byte) float32 {
	bits := binary.LittleEndian.Uint32(bytes)
	float := math.Float32frombits(bits)
	return float
}
