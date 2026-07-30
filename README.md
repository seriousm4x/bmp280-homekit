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

## Run with systemd

The repository includes [`bmp280.service`](bmp280.service), which sets the working directory explicitly so `./db` resolves to `/opt/bmp280-homekit/db` instead of a systemd default directory.

Install the application and service as follows:

```sh
sudo useradd --system --home /opt/bmp280-homekit --shell /usr/sbin/nologin bmp280
sudo install -d -o bmp280 -g bmp280 /opt/bmp280-homekit
sudo install -o bmp280 -g bmp280 bmp280 /opt/bmp280-homekit/bmp280
sudo install -o bmp280 -g bmp280 -d /opt/bmp280-homekit/db
sudo install -o root -g root -m 0644 bmp280.service /etc/systemd/system/bmp280.service
sudo usermod -aG i2c bmp280
sudo systemctl daemon-reload
sudo systemctl enable --now bmp280.service
```

Keep `/opt/bmp280-homekit/db` when replacing the binary, otherwise HomeKit pairing information will be lost. Check the service with `sudo systemctl status bmp280.service` and view logs with `sudo journalctl -u bmp280.service`.

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
