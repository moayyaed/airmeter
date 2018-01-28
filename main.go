package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"sync"
	"time"

	MQTT "github.com/eclipse/paho.mqtt.golang"
	"github.com/fishnix/airmeter/sensor"
	uuid "github.com/satori/go.uuid"
	log "github.com/sirupsen/logrus"

	"gobot.io/x/gobot/platforms/raspi"
)

var (
	version = "0.2.0"

	vers         = flag.Bool("v", false, "display version information and exit")
	sensorDriver = flag.String("d", "bme280", "Which sensor driver to use: 'bme280' or 'sht3x'")

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

type job struct {
	ticker     *time.Ticker
	waitgroup  *sync.WaitGroup
	sensor     io.Reader
	mqttTopic  string
	mqttClient MQTT.Client
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
	sensor, err := sensor.NewAirMeterReader(r, *sensorDriver)
	if err != nil {
		log.Fatalf("Couldn't configure sensor! %s", err)
	}

	// Setup context to allow goroutines to be cancelled
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(1)
	Start(ctx, job{
		ticker:     time.NewTicker(freq),
		waitgroup:  &wg,
		sensor:     sensor,
		mqttTopic:  topic,
		mqttClient: mqttClient,
	})
	wg.Wait()

	mqttClient.Disconnect(1)
}

func Start(ctx context.Context, j job) {
	go func() {
		defer j.waitgroup.Done()
		for {
			select {
			case <-j.ticker.C:
				b, e := ioutil.ReadAll(j.sensor)
				if e != nil {
					log.Fatalln(e)
				}
				token := j.mqttClient.Publish(j.mqttTopic, 0, false, string(b))
				token.Wait()
			case <-ctx.Done():
				log.Info("Shutdown...")
				return
			}
		}
	}()
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
