package gst

/*
#cgo pkg-config: gstreamer-1.0 gstreamer-app-1.0

#include "gst.h"

*/
import "C"
import (
	"os"
	"syscall"
	"os/signal"
	"fmt"
	"sync"
	"unsafe"

	"github.com/joeky888/gplay/opusdec"
	"github.com/joeky888/gplay/alsa"
	"github.com/pions/webrtc"
)

const (
	videoClockRate = 90000
	audioClockRate = 48000
	channels = 1
	format = alsa.FormatS16LE
)

var player *alsa.PlaybackDevice
var ctrlc = make(chan os.Signal)

func init() {
	var err error
	player, err = alsa.NewPlaybackDevice("default", channels, format, audioClockRate,
		alsa.BufferParams{BufferFrames: 0, PeriodFrames: 960, Periods: 960})
	if err != nil {
		panic(err)
	}
	signal.Notify(ctrlc, os.Interrupt, syscall.SIGTERM)
	go cleanup(player)
	go C.gstreamer_send_start_mainloop()
}

// Pipeline is a wrapper for a GStreamer Pipeline
type Pipeline struct {
	Pipeline  *C.GstElement
	in        chan<- webrtc.RTCSample
	id        int
	codecName string
}

var pipelines = make(map[int]*Pipeline)
var pipelinesLock sync.Mutex

// CreatePipeline creates a GStreamer Pipeline
func CreatePipeline(codecName string, in chan<- webrtc.RTCSample) *Pipeline {
	pipelineStr := "appsink name=appsink"
	switch codecName {
	case webrtc.VP8:
		pipelineStr = "videotestsrc ! vp8enc ! " + pipelineStr
	case webrtc.VP9:
		pipelineStr = "videotestsrc ! vp9enc ! " + pipelineStr
	case webrtc.H264:
		pipelineStr = "videotestsrc ! video/x-raw,format=I420 ! x264enc bframes=0 speed-preset=veryfast key-int-max=60 ! video/x-h264,stream-format=byte-stream ! " + pipelineStr
	case webrtc.Opus:
		pipelineStr = "audiotestsrc ! opusenc ! " + pipelineStr
	default:
		panic("Unhandled codec " + codecName)
	}

	pipelineStrUnsafe := C.CString(pipelineStr)
	defer C.free(unsafe.Pointer(pipelineStrUnsafe))

	pipelinesLock.Lock()
	defer pipelinesLock.Unlock()

	pipeline := &Pipeline{
		Pipeline:  C.gstreamer_send_create_pipeline(pipelineStrUnsafe),
		in:        in,
		id:        len(pipelines),
		codecName: codecName,
	}

	pipelines[pipeline.id] = pipeline
	return pipeline
}

// Start starts the GStreamer Pipeline
func (p *Pipeline) Start() {
	C.gstreamer_send_start_pipeline(p.Pipeline, C.int(p.id))
}

// Stop stops the GStreamer Pipeline
func (p *Pipeline) Stop() {
	C.gstreamer_send_stop_pipeline(p.Pipeline)
}

//export goHandlePipelineBuffer
func goHandlePipelineBuffer(buffer unsafe.Pointer, bufferLen C.int, duration C.int, pipelineID C.int) {
	pipelinesLock.Lock()
	defer pipelinesLock.Unlock()

	pcm := make([]int16, 960)
	dec, err := opusdec.NewDecoder(audioClockRate, 2)
	if err != nil {
		panic(err)
	}

	if pipeline, ok := pipelines[int(pipelineID)]; ok {
		var samples uint32
		if pipeline.codecName == webrtc.Opus {
			samples = uint32(audioClockRate * (float32(duration) / 1000000000))
			// fmt.Println(C.GoBytes(buffer, bufferLen))
			// fmt.Println(samples)
			_, err := dec.Decode(C.GoBytes(buffer, bufferLen), pcm)
			if err != nil {
				panic(err)
			}
			fmt.Println(pcm)
			player.Write(pcm)
		} else {
			samples = uint32(videoClockRate * (float32(duration) / 1000000000))
		}
		pipeline.in <- webrtc.RTCSample{Data: C.GoBytes(buffer, bufferLen), Samples: samples}
	} else {
		fmt.Printf("discarding buffer, no pipeline with id %d", int(pipelineID))
	}
	C.free(buffer)
}

func cleanup(player *alsa.PlaybackDevice) {
	// User hit Ctrl-C, clean up
	<-ctrlc
	fmt.Println("Close devices")
	player.Close()
	os.Exit(1)
}