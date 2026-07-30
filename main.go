package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/brutella/hap"
	"github.com/brutella/hap/accessory"
	bmp280 "github.com/seriousm4x/bmp280/sensor"
)

const (
	homeKitStoragePath = "./db"
	readingInterval    = 5 * time.Second
)

func main() {
	sensor, err := bmp280.Open()
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		if err := sensor.Close(); err != nil {
			log.Printf("close BMP280: %v", err)
		}
	}()

	shutdownContext, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stop()

	accessory := accessory.NewTemperatureSensor(accessory.Info{
		Name:         "Temperature",
		Manufacturer: "ASUS",
		Model:        "Tinker Board + BMP280",
		SerialNumber: "BMP280-001",
		Firmware:     "1.0.0",
	})

	store := hap.NewFsStore(homeKitStoragePath)
	server, err := hap.NewServer(store, accessory.A)
	if err != nil {
		log.Fatal(err)
	}
	server.Pin = "00102003"

	var updateWaitGroup sync.WaitGroup
	updateWaitGroup.Go(func() {
		updateHomeKit(shutdownContext, accessory, sensor)
	})

	log.Println("HomeKit server starting...")
	log.Println("Pairing PIN: 001-02-003")

	serverError := server.ListenAndServe(shutdownContext)
	stop()
	updateWaitGroup.Wait()

	if serverError != nil && shutdownContext.Err() == nil {
		log.Fatal(serverError)
	}
	log.Println("HomeKit server stopped")
}

func updateHomeKit(
	ctx context.Context,
	accessory *accessory.Thermometer,
	sensor *bmp280.Sensor,
) {
	updateReading := func() {
		temperature, pressure, err := sensor.Read()
		if err != nil {
			log.Printf("BMP280 read failed: %v", err)
			return
		}

		accessory.TempSensor.CurrentTemperature.SetValue(temperature)
		// Pressure is logged because this accessory currently has no native
		// HomeKit pressure characteristic.
		log.Printf("%.2f °C   %.2f hPa", temperature, pressure)
	}

	updateReading()

	ticker := time.NewTicker(readingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			updateReading()
		}
	}
}
