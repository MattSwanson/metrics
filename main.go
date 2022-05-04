package main

import (
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"time"

	"github.com/MattSwanson/ant-go"
	"github.com/google/gousb"
)

var (
	usbDriver *ant.GarminStick3
	signalChannel chan os.Signal
	currentHR int
	carsBack int
	currentSpeed float32
)

const (
	hrSensorID = 56482
)

func main() {
	signalChannel = make(chan os.Signal, 1)
	signal.Notify(signalChannel, os.Interrupt)
	usbCtx := gousb.NewContext()
	defer usbCtx.Close()
	startAntMonitor(usbCtx)

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
			case signal := <-signalChannel:
				if signal == os.Interrupt {
					cleanUp()
				}
			case <-ticker.C:
				_, err := http.PostForm("https://burtbot.app/metrics", 
				url.Values{"hr": {fmt.Sprintf("%d", currentHR)},
				"cars": {fmt.Sprintf("%d", carsBack)},
				"speed": {fmt.Sprintf("%.1f", currentSpeed)},
				})
				if err != nil {
					log.Println(err)
				}
		}
	}
}

func startAntMonitor(ctx *gousb.Context) {
	usbDriver = ant.NewGarminStick3()
	scanner := ant.NewHeartRateScanner(usbDriver)
	scanner.ListenForData(func(s *ant.HeartRateScannerState) {
		if s.DeviceID == hrSensorID {
			currentHR = int(s.ComputedHeartRate)
		}
	})
	scanner.SetOnAttachCallback(func() {
		fmt.Println("Ant sensor attached")
	})

	radar := ant.NewBikeRadarScanner(usbDriver)
	radar.ListenForData(func(s *ant.BikeRadarScannerState) {
		carsBack = 0
		for _, target := range s.Targets {
			if target != nil {
				carsBack++
			}
		}
	})
	radar.SetOnAttachCallback(func() {
		fmt.Println("Radar sensor attached")
	})

	speed := ant.NewSpeedScanner(usbDriver)
	speed.ListenForData(func(s *ant.SpeedScannerState) {
		currentSpeed = s.CalculatedSpeed
	})
	speed.SetOnAttachCallback(func() {
		fmt.Println("Speed sensor attached")
		radar.Scan()
		scanner.Scan()
	})

	usbDriver.OnStartup(func() {
		speed.Scan()
	})
	err := usbDriver.Open(ctx)
	if err != nil {
		log.Println("error opening usb driver: ", err.Error())
		usbDriver = nil
		return
	}
}

func cleanUp() {
	if usbDriver != nil {
		usbDriver.Close()
	}
	os.Exit(0)
}
