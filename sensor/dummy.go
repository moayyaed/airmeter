package sensor

import (
	"encoding/json"
	"io"
	"math/rand"

	"gobot.io/x/gobot/drivers/i2c"
)

// DummySensor is a "wrapper" for a dummy sensor drivers
type DummySensor struct {
	Current                              Reading
	tempFactor, humidFactor, pressFactor float32
}

// NewDummySensor returns a DummySensor
func NewDummySensor(_ i2c.Connector, tf, hf, pf float32) DummySensor {
	return DummySensor{tempFactor: tf, humidFactor: hf, pressFactor: pf}
}

func (sensor DummySensor) Read(p []byte) (int, error) {
	sensor.Current = Reading{
		Temperature: (rand.Float32() * 100) + sensor.tempFactor,
		Humidity:    (rand.Float32() * 100) + sensor.humidFactor,
		Pressure:    (rand.Float32() * 100) + sensor.pressFactor,
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
