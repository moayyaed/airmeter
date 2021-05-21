package sensor

import (
	"fmt"
	"io"

	log "github.com/sirupsen/logrus"
	"gobot.io/x/gobot/drivers/i2c"
)

type Sensor interface {
	io.Reader
	CleanUp() error
}

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
func NewAirMeterReader(adapter i2c.Connector, driver, units string, tf, hf, pf float32) (Sensor, error) {
	switch driver {
	case "bme280":
		log.Debug("returning new BME280 sensor")
		return NewBME280Sensor(adapter, units, tf, hf, pf), nil
	case "sht3x":
		log.Debug("returning new SHT3X sensor")
		return NewSHT3xSensor(adapter, units, tf, hf, pf), nil
	case "dummy":
		log.Debug("returning new DUMMY sensor")
		return NewDummySensor(nil, tf, hf, pf), nil
	default:
		return nil, fmt.Errorf("Invalid driver '%s' or adapter", driver)
	}
}
