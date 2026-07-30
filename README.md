# BMP280 HomeKit Sensor

Small Go application for reading temperature and air pressure from a Bosch BMP280 connected over I2C and exposing the temperature reading to Apple HomeKit through `go-hap` (`github.com/brutella/hap`).

The project runs on an ASUS Tinker Board Rev. 1.2 with Armbian. The BMP280 is expected at I2C address `0x76` on `/dev/i2c-1`.

## What it does

- Reads temperature and pressure from the BMP280.
- Publishes the temperature as a HomeKit temperature sensor.
- Logs the pressure in hPa because the current HomeKit accessory does not expose a native pressure characteristic.
- Stores HomeKit pairing data in the local `./db` directory.
- Uses pairing PIN `001-02-003`.

The BMP280 driver is implemented separately in [`sensor/bmp280.go`](sensor/bmp280.go).

## Requirements

- Go 1.25 or newer.
- Armbian/Linux with an enabled I2C interface.
- A BMP280 connected to the configured I2C bus.
- Access to `/dev/i2c-1` for the user running the application.

On the Tinker Board, verify the I2C device exists before starting:

```sh
ls -l /dev/i2c-1
```

## Build and run

From the project directory:

```sh
go build .
./bmp280
```

The application prints the current temperature and pressure once per second. Pair the accessory from the Home app using PIN `001-02-003`.

The pairing database is created in `./db`. Keep this directory when upgrading or restarting the application, otherwise HomeKit pairing information will be lost.

## Configuration

The I2C bus and BMP280 address are currently constants in [`sensor/bmp280.go`](sensor/bmp280.go):

```go
const (
	defaultI2CBus     = "/dev/i2c-1"
	defaultI2CAddress = 0x76
)
```

Change these values if your wiring uses a different bus or sensor address.

## Disclaimer

This is a personal project for a private use case. It is not intended to be a general-purpose library or production-ready solution. No support, maintenance, compatibility guarantees, or troubleshooting assistance will be provided.

**Is this vibe coded? Yes.**
