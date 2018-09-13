// The MIT License (MIT)
//
// Copyright (c) 2017 aerth <aerth@riseup.net>
//
// Permission is hereby granted, free of charge, to any person obtaining a copy of
// this software and associated documentation files (the "Software"), to deal in
// the Software without restriction, including without limitation the rights to
// use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies of
// the Software, and to permit persons to whom the Software is furnished to do so,
// subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS
// FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR
// COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER
// IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN
// CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.

// playwav plays a .wav file using ALSA
package playwav

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/cryptix/wav"
	"github.com/joeky888/alsa"
)

func int16tobyte(x int16) uint8 {
	// From https://stackoverflow.com/questions/24449957/converting-a-8-bit-pcm-to-16-bit-pcm
	return uint8((x >> 8) + 128)
}

func FromFile(filename string) (wavinfo string, err error) {

	// file exists
	soundfile, err := os.Open(filename)
	if err != nil {
		return "", errors.New(fmt.Sprint("open:", err))
	}

	// stat for size
	sndfileinfo, err := os.Stat(soundfile.Name())
	if err != nil {
		return "", errors.New(fmt.Sprint("stat:", err))
	}

	// wavReader
	wavReader, err := wav.NewReader(soundfile, sndfileinfo.Size())
	if err != nil {
		return "", errors.New(fmt.Sprint("WAV reader:", err))
	}

	// require wavReader
	if wavReader == nil {
		return "", errors.New("nil wav reader")
	}

	// defaultCard := func() string {
	// 	if os.Getenv("GOARCH") == "arm64" || os.Getenv("GOARCH") == "arm" {
	// 		return "default:CARD=mtsndcard"
	// 	} else {
	// 		return "default"
	// 	}
	// }()

	cards, err := alsa.OpenCards()
	if err != nil {
		fmt.Println(err)
		return
	}
	defer alsa.CloseCards(cards)
	devices, _ := cards[0].Devices()

	fmt.Println(devices[0])
	device := devices[0]

	if err = device.Open(); err != nil {
		panic(err)
	}
	const bufsize = 4096
	wantPeriodSize := 2048 // 46ms @ 44100Hz

	periodSize, err := device.NegotiatePeriodSize(wantPeriodSize)
	if err != nil {
		panic(err)
	}
	bufferSize, err := device.NegotiateBufferSize(wantPeriodSize * 2)
	if err != nil {
		panic(err)
	}
	if err = device.Prepare(); err != nil {
		panic(err)
	}
	channels, err := device.NegotiateChannels(1, 2)
	if err != nil {
		panic(err)
	}
	sampleRate, err := device.NegotiateRate(44100, 48000)
	if err != nil {
		panic(err)
	}
	format, err := device.NegotiateFormat(alsa.S16_LE, alsa.S32_LE)
	if err != nil {
		panic(err)
	}
	fmt.Printf("Negotiated parameters: %d channels, %d hz, %v, %d period size, %d buffer size\n",
		channels, sampleRate, format, periodSize, bufferSize)

	// print .WAV info
	// wavinfo = wavReader.String()
	// fileinfo := wavReader.GetFile()
	// open default ALSA playback device
	// samplerate := int(fileinfo.SampleRate)
	// if samplerate == 0 {
	// 	samplerate = 44100
	// }
	// if samplerate > 100000 {
	// 	samplerate = 44100
	// }
	// fmt.Println(samplerate)
	// out, err := alsa.NewPlaybackDevice(defaultCard, 1, alsa.FormatS16LE, samplerate, alsa.BufferParams{})
	// if err != nil {
	// 	return wavinfo, errors.New(fmt.Sprint("alsa:", err))
	// }

	// require ALSA device
	// if out == nil {
	// 	return wavinfo, errors.New("nil ALSA device")
	// }

	// close device when finished
	// defer out.Close()

	for {
		s, err := wavReader.ReadSampleEvery(2, 0)
		var cvert []int16
		for _, b := range s {
			cvert = append(cvert, int16(b))
		}

		bytepcm := make([]byte, len(cvert))

		for i := range cvert {
			bytepcm[i] = int16tobyte(cvert[i])
		}

		if cvert != nil {
			// play!
			// out.Write(cvert)
			fmt.Println("before")
			fmt.Println(cvert)
			fmt.Println("after")
			fmt.Println(bytepcm)
			if err := device.Write(bytepcm, periodSize); err != nil {
				panic(err)
			}
		}
		cvert = []int16{}

		if err == io.EOF {
			break
		} else if err != nil {
			return wavinfo, errors.New(fmt.Sprint("WAV Decode:", err))
		}
	}

	// all done
	return wavinfo, nil
}
