package sensor

import (
	"encoding/json"
	"fmt"
	"io"

	"gobot.io/x/gobot/drivers/i2c"
)

// Reading holds air sensor readings
// Temperature is in C
// Humidity is %
// Pressure is in Pa
type Reading struct {
	Temperature float32 `json:",omitempty"`
	Humidity    float32 `json:",omitempty"`
	Pressure    float32 `json:",omitempty"`
}

// NewAirMeter returns the proper i2c airmeter driver
func NewAirMeterReader(adapter i2c.Connector, driver string) (io.Reader, error) {
	switch driver {
	case "bme280":
		return NewBME280Sensor(adapter), nil
	case "sht3x":
		return NewSHT3xSensor(adapter), nil
	default:
		return nil, fmt.Errorf("Invalid driver '%s' or adapter", driver)
	}
}

// BME280Sensor is a wrapper for the BME280 sensor drivers
type BME280Sensor struct {
	Driver  *i2c.BME280Driver
	Current Reading
}

// NewBME280Sensor returns a BME280Sensor
func NewBME280Sensor(adapter i2c.Connector) BME280Sensor {
	return BME280Sensor{Driver: i2c.NewBME280Driver(adapter)}
}

// Read gets the data from the sensor.  It implements io.Reader by filling the []byte with
// the Reading struct encoded as JSON.  Every successful call returns io.EOF.
func (sensor BME280Sensor) Read(p []byte) (int, error) {
	sensor.Driver.Start()

	// read the humidity from the sensor
	hum, err := sensor.Driver.Humidity()
	if err != nil {
		return 0, err
	}

	// read the temperature from the sensor
	tem, err := sensor.Driver.Temperature()
	if err != nil {
		return 0, err
	}

	// read the pressure from the sensor
	prs, err := sensor.Driver.Pressure()
	if err != nil {
		return 0, err
	}

	// not exactly sure how to set the constant for sea level pressure and don't want to
	// copy-pasta the calculation here since my altitude wont change so its not very useful
	// i2c.bmp280SeaLevelPressure = 103400.00
	// alt, err := s.Driver.Altitude()
	// if err != nil {
	// 	return err
	// }

	sensor.Current = Reading{
		Temperature: tem,
		Humidity:    hum,
		Pressure:    prs,
	}

	j, err := json.Marshal(sensor.Current)
	if err != nil {
		return 0, err
	}

	// fill the slice of bytes from the values marshaled to JSON
	for i, b := range j {
		p[i] = b
	}

	return len(j), io.EOF
}

// SHT3xSensor is a wrapper for the SHT3x sensor drivers
type SHT3xSensor struct {
	Driver  *i2c.SHT3xDriver
	Current Reading
}

// NewSHT3xSensor returns a SHT3xSensor
func NewSHT3xSensor(adapter i2c.Connector) SHT3xSensor {
	return SHT3xSensor{Driver: i2c.NewSHT3xDriver(adapter)}
}

func (sensor SHT3xSensor) Read(p []byte) (int, error) {
	sensor.Driver.Start()

	tem, hum, err := sensor.Driver.Sample()
	if err != nil {
		return 0, err
	}

	sensor.Current = Reading{
		Temperature: tem,
		Humidity:    hum,
	}

	j, err := json.Marshal(sensor.Current)
	if err != nil {
		return 0, err
	}

	// fill the slice of bytes from the values marshaled to JSON
	for i, b := range j {
		p[i] = b
	}

	return len(j), io.EOF
}
