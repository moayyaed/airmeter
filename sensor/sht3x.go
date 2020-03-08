package sensor

import (
	"encoding/json"
	"io"

	"gobot.io/x/gobot/drivers/i2c"
)

// SHT3xSensor is a wrapper for the SHT3x sensor drivers
type SHT3xSensor struct {
	Driver                  *i2c.SHT3xDriver
	Current                 Reading
	tempFactor, humidFactor float32
}

// NewSHT3xSensor returns a SHT3xSensor
func NewSHT3xSensor(adapter i2c.Connector, tf, hf, pf float32) SHT3xSensor {
	return SHT3xSensor{Driver: i2c.NewSHT3xDriver(adapter), tempFactor: tf, humidFactor: hf}
}

func (sensor SHT3xSensor) Read(p []byte) (int, error) {
	sensor.Driver.Start()

	tem, hum, err := sensor.Driver.Sample()
	if err != nil {
		return 0, err
	}

	sensor.Current = Reading{
		Temperature: tem + sensor.tempFactor,
		Humidity:    hum + sensor.humidFactor,
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
