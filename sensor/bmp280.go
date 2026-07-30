// Package bmp280 provides access to a Bosch BMP280 temperature and pressure
// sensor connected through an I2C bus.
package bmp280

import (
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"sync"

	"golang.org/x/sys/unix"
)

const (
	defaultI2CBus     = "/dev/i2c-1"
	defaultI2CAddress = 0x76
	i2cSlaveAddress   = 0x0703

	registerCalibration = 0x88
	registerControl     = 0xF4
	registerConfig      = 0xF5
	registerData        = 0xF7

	controlNormalMode  = 0x27
	configStandby500ms = 0xA0
)

// Sensor represents an opened BMP280 sensor.
type Sensor struct {
	mu sync.Mutex
	fd int

	temperatureCalibration [3]int32
	pressureCalibration    [9]int32
}

// Open opens the BMP280 on the default I2C bus and address, reads its
// calibration data, and configures it for normal operation.
func Open() (*Sensor, error) {
	fileDescriptor, err := unix.Open(defaultI2CBus, unix.O_RDWR, 0)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", defaultI2CBus, err)
	}

	sensor := &Sensor{fd: fileDescriptor}
	cleanup := func(openErr error) (*Sensor, error) {
		if closeErr := sensor.Close(); closeErr != nil {
			log.Printf("close BMP280 after error: %v", closeErr)
		}
		return nil, openErr
	}

	if err := unix.IoctlSetInt(fileDescriptor, i2cSlaveAddress, defaultI2CAddress); err != nil {
		return cleanup(fmt.Errorf("select I2C address 0x%02X: %w", defaultI2CAddress, err))
	}

	calibrationData, err := sensor.read(registerCalibration, 24)
	if err != nil {
		return cleanup(fmt.Errorf("read calibration data: %w", err))
	}

	sensor.temperatureCalibration[0] = int32(readUint16(calibrationData, 0))
	sensor.temperatureCalibration[1] = int32(readInt16(calibrationData, 2))
	sensor.temperatureCalibration[2] = int32(readInt16(calibrationData, 4))

	for index := range sensor.pressureCalibration {
		offset := 6 + index*2
		if index == 0 {
			sensor.pressureCalibration[index] = int32(readUint16(calibrationData, offset))
		} else {
			sensor.pressureCalibration[index] = int32(readInt16(calibrationData, offset))
		}
	}

	if sensor.temperatureCalibration[0] == 0 || sensor.pressureCalibration[0] == 0 {
		return cleanup(fmt.Errorf("invalid BMP280 calibration data"))
	}

	if err := sensor.write(registerControl, controlNormalMode); err != nil {
		return cleanup(err)
	}
	if err := sensor.write(registerConfig, configStandby500ms); err != nil {
		return cleanup(err)
	}

	return sensor, nil
}

// Close closes the sensor's I2C file descriptor. It is safe to call more than
// once.
func (s *Sensor) Close() error {
	if s.fd < 0 {
		return nil
	}

	err := unix.Close(s.fd)
	s.fd = -1
	return err
}

// Read returns the current temperature in degrees Celsius and pressure in hPa.
func (s *Sensor) Read() (temperature, pressure float64, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := s.read(registerData, 6)
	if err != nil {
		return 0, 0, err
	}

	rawPressure := int32(data[0])<<12 | int32(data[1])<<4 | int32(data[2])>>4
	rawTemperature := int32(data[3])<<12 | int32(data[4])<<4 | int32(data[5])>>4

	temperatureCalibration := s.temperatureCalibration
	pressureCalibration := s.pressureCalibration

	// Temperature compensation from the BMP280 datasheet.
	var1 := (float64(rawTemperature)/16384.0 -
		float64(temperatureCalibration[0])/1024.0) * float64(temperatureCalibration[1])
	var2 := float64(rawTemperature)/131072.0 - float64(temperatureCalibration[0])/8192.0
	var2 *= var2 * float64(temperatureCalibration[2])

	tFine := var1 + var2
	temperature = tFine / 5120.0

	// Pressure compensation from the BMP280 datasheet.
	var1 = tFine/2.0 - 64000.0
	var2 = var1 * var1 * float64(pressureCalibration[5]) / 32768.0
	var2 += var1 * float64(pressureCalibration[4]) * 2.0
	var2 = var2/4.0 + float64(pressureCalibration[3])*65536.0
	var1 = (float64(pressureCalibration[2])*var1*var1/524288.0 +
		float64(pressureCalibration[1])*var1) / 524288.0
	var1 = (1.0 + var1/32768.0) * float64(pressureCalibration[0])

	if var1 == 0 {
		return temperature, 0, fmt.Errorf("invalid pressure calibration: division by zero")
	}

	pressureRaw := 1048576.0 - float64(rawPressure)
	pressureRaw = (pressureRaw - var2/4096.0) * 6250.0 / var1
	var1 = float64(pressureCalibration[8]) * pressureRaw * pressureRaw / 2147483648.0
	var2 = pressureRaw * float64(pressureCalibration[7]) / 32768.0

	// The compensation formula returns Pa; convert it to hPa.
	pressure = (pressureRaw + (var1+var2+float64(pressureCalibration[6]))/16.0) / 100.0
	return temperature, pressure, nil
}

func (s *Sensor) read(register byte, byteCount int) ([]byte, error) {
	if err := writeAll(s.fd, []byte{register}); err != nil {
		return nil, fmt.Errorf("select register 0x%02X: %w", register, err)
	}

	data := make([]byte, byteCount)
	if err := readFull(s.fd, data); err != nil {
		return nil, fmt.Errorf("read register 0x%02X: %w", register, err)
	}
	return data, nil
}

func (s *Sensor) write(register, value byte) error {
	if err := writeAll(s.fd, []byte{register, value}); err != nil {
		return fmt.Errorf("write register 0x%02X value 0x%02X: %w", register, value, err)
	}
	return nil
}

func writeAll(fileDescriptor int, data []byte) error {
	for len(data) > 0 {
		written, err := unix.Write(fileDescriptor, data)
		if err == unix.EINTR {
			continue
		}
		if err != nil {
			return err
		}
		if written == 0 {
			return io.ErrUnexpectedEOF
		}
		data = data[written:]
	}
	return nil
}

func readFull(fileDescriptor int, data []byte) error {
	for offset := 0; offset < len(data); {
		readCount, err := unix.Read(fileDescriptor, data[offset:])
		if err == unix.EINTR {
			continue
		}
		if err != nil {
			return err
		}
		if readCount == 0 {
			return io.ErrUnexpectedEOF
		}
		offset += readCount
	}
	return nil
}

func readUint16(data []byte, offset int) uint16 {
	return binary.LittleEndian.Uint16(data[offset : offset+2])
}

func readInt16(data []byte, offset int) int16 {
	return int16(readUint16(data, offset))
}
