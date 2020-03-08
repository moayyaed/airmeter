package sensor

import (
	"encoding/json"
	"io"

	"gobot.io/x/gobot/drivers/i2c"
)

// BME280Sensor is a wrapper for the BME280 sensor drivers
type BME280Sensor struct {
	Driver                               *i2c.BME280Driver
	Current                              Reading
	tempFactor, humidFactor, pressFactor float32
}

// NewBME280Sensor returns a BME280Sensor
func NewBME280Sensor(adapter i2c.Connector, tf, hf, pf float32) BME280Sensor {
	return BME280Sensor{Driver: i2c.NewBME280Driver(adapter), tempFactor: tf, humidFactor: hf, pressFactor: pf}
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
