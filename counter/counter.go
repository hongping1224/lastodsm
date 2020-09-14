// Copyright 2019 Hong-Ping Lo. All rights reserved.
// Use of this source code is governed by a BDS-style
// license that can be found in the LICENSE file.

package counter

import (
	"math"
	"sync"

	"github.com/jblindsay/lidario"
)

type Point struct {
	X float64
	Y float64
}

const (
	allReturn   = 0
	firstReturn = 1
	lastReturn  = 2
)

type Counter struct {
	LBCoordinate Point
	DensityMap   []lidario.LasPointer
	Mux          []sync.Mutex
	MapSize      int
	XRange       int
	YRange       int
	gap          float64
	ReadStream   chan lidario.LasPointer
	WriteStream  chan lidario.LasPointer
	WG           *sync.WaitGroup
}

func (counter *Counter) Count(start int, end int, lf *lidario.LasFile, wg *sync.WaitGroup) {

	counter.WG = wg

	for i := start; i < end; i++ {
		p, _ := lf.LasPoint(i)
		counter.ReadStream <- p
	}
	close(counter.ReadStream)
}

func (counter *Counter) Init(minx, miny, maxx, maxy, gap float64) {
	counter.ReadStream = make(chan lidario.LasPointer)
	counter.WriteStream = make(chan lidario.LasPointer)
	//DensityMap
	//LBCoordinate
	counter.LBCoordinate = Point{X: minx, Y: miny}
	counter.gap = gap
	counter.XRange = int(math.Ceil((maxx - minx) / gap))
	counter.YRange = int(math.Ceil((maxy - miny) / gap))
	defaultPoint := lidario.PointRecord0{Z: math.Inf(-1), PointSourceID: 0}
	counter.DensityMap = make([]lidario.LasPointer, counter.XRange*counter.YRange)
	for i := 0; i < counter.XRange*counter.YRange; i++ {
		counter.DensityMap[i] = &defaultPoint
	}
	counter.Mux = make([]sync.Mutex, counter.XRange*counter.YRange)
	counter.MapSize = counter.XRange * counter.YRange
}

type Reader struct {
	done bool
}

func (reader *Reader) Serve(points <-chan lidario.LasPointer, counter *Counter) {
	reader.done = false
	for {
		p, more := <-points
		if more {
			readerHandler(p, counter)
		} else {
			close(counter.WriteStream)
			break
		}
	}
	reader.done = true
}

func readerHandler(p lidario.LasPointer, counter *Counter) {
	//calculate coordinate of point and  put it into stream
	counter.WriteStream <- p
}

func xyToIndex(x float64, y float64, counter *Counter) int {
	xr := (x - counter.LBCoordinate.X) / counter.gap
	yr := (y - counter.LBCoordinate.Y) / counter.gap
	//fmt.Println(counter.LBCoordinate.X, counter.LBCoordinate.Y)
	index := int(math.Floor(xr)) + (int(math.Floor(yr)) * counter.XRange)
	return index
}

type Writer struct {
}

func (writer *Writer) Serve(points <-chan lidario.LasPointer, counter *Counter) {

	for {
		point, more := <-points
		if more {
			writerHandler(point, counter)
		} else {
			break
		}
	}
	counter.WG.Done()
}

func writerHandler(point lidario.LasPointer, counter *Counter) {
	pointdata := point.PointData()
	index := xyToIndex(pointdata.X, pointdata.Y, counter)
	if index < 0 || index >= counter.MapSize {
		//ignore point outside map
		return
	}
	counter.Mux[index].Lock()
	//check is highest
	oriData := counter.DensityMap[index].PointData()
	if oriData.Z < pointdata.Z {
		counter.DensityMap[index] = point
	}
	counter.Mux[index].Unlock()
}
