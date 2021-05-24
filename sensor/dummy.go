package sensor

import (
	"encoding/json"
	"io"
	"math/rand"

	log "github.com/sirupsen/logrus"
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
	tem, hum, prs, err := sensor.Sample()
	if err != nil {
		return 0, err
	}
	log.Debugf("uncorrected temperature: %f, uncorrected humidity: %f, uncorrected pressure: %f", tem, hum, prs)

	sensor.Current = Reading{
		Temperature: tem + sensor.tempFactor,
		Humidity:    hum + sensor.humidFactor,
		Pressure:    prs + sensor.pressFactor,
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

func (sensor DummySensor) Sample() (float32, float32, float32, error) {
	return rand.Float32() * 100, rand.Float32() * 100, rand.Float32() * 100, nil
}

func (s DummySensor) CleanUp() error {
	log.Debug("cleaning up")
	return nil
}
