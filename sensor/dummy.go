package sensor

import (
	"encoding/json"
	"io"
	"math/rand"

	"gobot.io/x/gobot/drivers/i2c"
)

// DummySensor is a "wrapper" for a dummy sensor drivers
type DummySensor struct {
	Current Reading
}

// NewDummySensor returns a DummySensor
func NewDummySensor(_ i2c.Connector) DummySensor {
	return DummySensor{}
}

func (sensor DummySensor) Read(p []byte) (int, error) {
	sensor.Current = Reading{
		Temperature: rand.Float32() * 100,
		Humidity:    rand.Float32() * 100,
		Pressure:    rand.Float32() * 100,
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
