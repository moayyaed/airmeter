package sensor

import (
	"encoding/json"
	"io"

	log "github.com/sirupsen/logrus"
	"gobot.io/x/gobot/drivers/i2c"
)

// SHT3xSensor is a wrapper for the SHT3x sensor drivers
type SHT3xSensor struct {
	Driver                  *i2c.SHT3xDriver
	Current                 Reading
	tempFactor, humidFactor float32
}

// NewSHT3xSensor returns a SHT3xSensor
func NewSHT3xSensor(adapter i2c.Connector, units string, tf, hf, pf float32) SHT3xSensor {
	s := i2c.NewSHT3xDriver(adapter)
	s.Units = units

	return SHT3xSensor{Driver: s, tempFactor: tf, humidFactor: hf}
}

func (sensor SHT3xSensor) Read(p []byte) (int, error) {
	if err := sensor.Driver.Start(); err != nil {
		return 0, err
	}

	if err := sensor.Driver.SetAccuracy(i2c.SHT3xAccuracyHigh); err != nil {
		return 0, err
	}

	tem, hum, err := sensor.Driver.Sample()
	if err != nil {
		return 0, err
	}
	log.Debugf("uncorrected temperature: %f, uncorrected humidity: %f", tem, hum)

	sensor.Current = Reading{
		Temperature: tem + sensor.tempFactor,
		Humidity:    hum + sensor.humidFactor,
	}

	j, err := json.Marshal(sensor.Current)
	if err != nil {
		return 0, err
	}

	copy(p, j)

	return len(j), io.EOF
}

func (s SHT3xSensor) CleanUp() error {
	log.Debug("cleaning up")
	return nil
}
