package sensor

import (
	"encoding/json"
	"io"

	log "github.com/sirupsen/logrus"
	"gobot.io/x/gobot/drivers/i2c"
)

// BME280Sensor is a wrapper for the BME280 sensor drivers
type BME280Sensor struct {
	Driver                               *i2c.BME280Driver
	Current                              Reading
	tempFactor, humidFactor, pressFactor float32
}

// NewBME280Sensor returns a BME280Sensor
func NewBME280Sensor(adapter i2c.Connector, units string, tf, hf, pf float32) BME280Sensor {
	return BME280Sensor{Driver: i2c.NewBME280Driver(adapter), tempFactor: tf, humidFactor: hf, pressFactor: pf}
}

// Read gets the data from the sensor.  It implements io.Reader by filling the []byte with
// the Reading struct encoded as JSON.  Every successful call returns io.EOF.
func (sensor BME280Sensor) Read(p []byte) (int, error) {
	sensor.Driver.Start()

	tem, hum, prs, err := sensor.Sample()
	if err != nil {
		return 0, err
	}
	log.Debugf("uncorrected temperature: %f, uncorrected humidity: %f, uncorrected pressure: %f", tem, hum, prs)

	// not exactly sure how to set the constant for sea level pressure and don't want to
	// copy-pasta the calculation here since my altitude wont change so its not very useful
	// i2c.bmp280SeaLevelPressure = 103400.00
	// alt, err := s.Driver.Altitude()
	// if err != nil {
	// 	return err
	// }

	sensor.Current = Reading{
		Temperature: tem + sensor.tempFactor,
		Humidity:    hum + sensor.humidFactor,
		Pressure:    prs + sensor.pressFactor,
	}

	j, err := json.Marshal(sensor.Current)
	if err != nil {
		return 0, err
	}

	copy(p, j)

	return len(j), io.EOF
}

func (sensor BME280Sensor) Sample() (float32, float32, float32, error) {
	// read the temperature from the sensor
	tem, err := sensor.Driver.Temperature()
	if err != nil {
		return 0, 0, 0, err
	}

	// read the humidity from the sensor
	hum, err := sensor.Driver.Humidity()
	if err != nil {
		return tem, 0, 0, err
	}

	// read the pressure from the sensor
	prs, err := sensor.Driver.Pressure()
	if err != nil {
		return tem, hum, 0, err
	}

	return tem, hum, prs, nil
}

func (s BME280Sensor) CleanUp() error {
	log.Debug("cleaning up")
	return nil
}
