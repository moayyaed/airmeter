package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"time"

	MQTT "github.com/eclipse/paho.mqtt.golang"
	uuid "github.com/satori/go.uuid"
	log "github.com/sirupsen/logrus"

	"gobot.io/x/gobot"
	"gobot.io/x/gobot/drivers/i2c"
	"gobot.io/x/gobot/platforms/raspi"
)

var (
	version = "0.1.0"

	vers = flag.Bool("v", false, "display version information and exit")

	frequency  = flag.String("f", "5s", "frequency to collect data from the sensor")
	location   = flag.String("l", "home", "location for the sensor")
	mqttBroker = flag.String("b", "tcp://iot.eclipse.org:1883", "MQTT broker endpoint")

	// Advanced options
	topicroot       = flag.String("t", "airmeter", "Advanced: Set the MQTT topic root - the topic will be 'topicroot/location' - ")
	startSubscriber = flag.Bool("s", false, "Advanced: start a subscription on the MQTT topic and print to STDOUT")
)

// publishHandler is a simple "print to STDOUT" handler for the MQTT topic subscription
// define a function for the default message handler
var publishHandler MQTT.MessageHandler = func(client MQTT.Client, msg MQTT.Message) {
	fmt.Println("Message from MQTT")
	fmt.Printf("TOPIC: %s\n", msg.Topic())
	fmt.Printf("MSG: %s\n", msg.Payload())
}

// Sensor is the driver for the sensor and the most recent (ie. Current) reading
type Sensor struct {
	driver  *i2c.BME280Driver
	Current Reading
}

// Reading holds air sensor readings
// Temperature is in C
// Humidity is %
// Pressure is in Pa
type Reading struct {
	Temperature float32
	Humidity    float32
	Pressure    float32
}

func main() {
	flag.Parse()

	if *vers {
		fmt.Println("Airmeter version:", version)
		os.Exit(0)
	}

	freq, err := time.ParseDuration(*frequency)
	if err != nil {
		log.Fatalf("Cannot parse frequency duration: %s", *frequency)
	}

	topic := fmt.Sprintf("%s/%s", *topicroot, *location)

	mqttClient := newMQTTClient()

	if *startSubscriber {
		// if startSubscriber flag is passed, start a goroutine to subscribe to the MQTT topic
		// this is primarily added for debugging and not expected to be used most of the time.
		log.Println("Starting MQTT subscription on topic:", topic)
		go subscribe(mqttClient, topic)
	}

	r := raspi.NewAdaptor()
	sensor := &Sensor{
		driver: i2c.NewBME280Driver(r),
	}

	log.Infoln("Outside gobot")
	_, err = ioutil.ReadAll(sensor)
	if err != nil {
		log.Fatal(err)
	}
	sensor.Print()

	work := func() {
		gobot.Every(freq, func() {
			b, e := ioutil.ReadAll(sensor)
			if e != nil {
				log.Fatalln(err)
			}
			token := mqttClient.Publish(topic, 0, false, string(b))
			token.Wait()
		})
	}

	robot := gobot.NewRobot("airBot",
		[]gobot.Connection{r},
		[]gobot.Device{sensor.driver},
		work,
	)

	err = robot.Start()
	if err != nil {
		log.Fatalln("Error starting robot!")
	}

	mqttClient.Disconnect(1)
}

// Read gets the data from the sensor.  It implements io.Reader by filling the []byte with
// the Reading struct encoded as JSON.  Every successful call returns io.EOF.
func (s *Sensor) Read(p []byte) (int, error) {
	s.driver.Start()

	// read the humidity from the sensor
	hum, err := s.driver.Humidity()
	if err != nil {
		return 0, err
	}

	// read the temperature from the sensor
	tem, err := s.driver.Temperature()
	if err != nil {
		return 0, err
	}

	// read the pressure from the sensor
	prs, err := s.driver.Pressure()
	if err != nil {
		return 0, err
	}

	// not exactly sure how to set the constant for sea level pressure and don't want to
	// copy-pasta the calculation here since my altitude wont change so its not very useful
	// i2c.bmp280SeaLevelPressure = 103400.00
	// alt, err := s.driver.Altitude()
	// if err != nil {
	// 	return err
	// }

	s.Current = Reading{
		Temperature: tem,
		Humidity:    hum,
		Pressure:    prs,
	}

	j, err := json.Marshal(s.Current)
	if err != nil {
		return 0, err
	}

	// fill the slice of bytes from the values marshaled to JSON
	for i, b := range j {
		p[i] = b
	}

	return len(j), io.EOF
}

// Print displays the current sensor data
func (sensor *Sensor) Print() {
	fmt.Printf("Temp: %fC\n", sensor.Current.Temperature)
	fmt.Printf("Humidity: %f\n", sensor.Current.Humidity)
	fmt.Printf("Pressure: %fPa\n\n", sensor.Current.Pressure)
}

// newMQTTClient returns a new MQTT client with a random client ID and the broker endpoint set
// by the flag for mqttBroker
func newMQTTClient() MQTT.Client {
	clientID := uuid.NewV4()
	opts := MQTT.NewClientOptions().AddBroker(*mqttBroker)
	log.Infof("Setting MQTT broker: %s", *mqttBroker)

	opts.SetClientID(clientID.String())
	log.Infof("Setting MQTT client ID: %s", clientID.String())

	opts.SetDefaultPublishHandler(publishHandler)

	// create and start a client using the above ClientOptions
	c := MQTT.NewClient(opts)
	if token := c.Connect(); token.Wait() && token.Error() != nil {
		panic(token.Error())
	}

	return c
}

// subscribe subscribes to the MQTT topic defined at startup and handles messages with the default handler
func subscribe(mqttClient MQTT.Client, topic string) {
	if token := mqttClient.Subscribe(topic, 0, nil); token.Wait() && token.Error() != nil {
		log.Errorln(token.Error())
		os.Exit(1)
	}
}
