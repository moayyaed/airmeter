package sensor

import (
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
	case "dummy":
		return NewDummySensor(nil), nil
	default:
		return nil, fmt.Errorf("Invalid driver '%s' or adapter", driver)
	}
}
