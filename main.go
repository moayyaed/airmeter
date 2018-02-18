package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"sync"
	"time"

	MQTT "github.com/eclipse/paho.mqtt.golang"
	"github.com/fishnix/airmeter/sensor"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	uuid "github.com/satori/go.uuid"
	log "github.com/sirupsen/logrus"

	"gobot.io/x/gobot/platforms/raspi"
)

var (
	version = "0.2.0"

	vers         = flag.Bool("v", false, "display version information and exit")
	sensorDriver = flag.String("d", "bme280", "Which sensor driver to use: 'dummy', 'bme280' or 'sht3x'")

	frequency  = flag.String("f", "5s", "frequency to collect data from the sensor")
	location   = flag.String("l", "home", "location for the sensor")
	mqttBroker = flag.String("b", "tcp://iot.eclipse.org:1883", "MQTT broker endpoint")

	httpAddress = flag.String("a", ":8000", "HTTP listen address")

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

// CommandRequest defines a command request recieved over the API
type CommandRequest struct {
	name string
	args map[string]string
}

type job struct {
	ticker         *time.Ticker
	waitgroup      *sync.WaitGroup
	sensor         io.Reader
	mqttTopic      string
	mqttClient     MQTT.Client
	requestChannel chan *CommandRequest
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
	cmdChan := make(chan *CommandRequest)
	wg.Add(1)
	respChan := Start(ctx, cancel, job{
		ticker:         time.NewTicker(freq),
		waitgroup:      &wg,
		sensor:         sensor,
		mqttTopic:      topic,
		mqttClient:     mqttClient,
		requestChannel: cmdChan,
	})

	StartHTTP(ctx, cmdChan, respChan)
	log.Info("Waiting for threads to exit")
	wg.Wait()

	mqttClient.Disconnect(1)
}

// Start begins reading the sensor data and writing it to MQTT
func Start(ctx context.Context, cancel context.CancelFunc, j job) chan []byte {
	responseChannel := make(chan []byte)
	go func() {
		defer j.waitgroup.Done()
		for {
			select {
			case <-j.ticker.C:
				log.Debug("Reading from sensor and writing to MQTT")
				b, e := ioutil.ReadAll(j.sensor)
				if e != nil {
					log.Errorf("%s", e)
				} else {
					token := j.mqttClient.Publish(j.mqttTopic, 0, false, string(b))
					token.Wait()
				}
			case cmd := <-j.requestChannel:
				log.Debugf("Got a request on the command request channel: %s", cmd.name)
				switch cmd.name {
				case "shutdown":
					log.Warn("Got request to shutdown.")
					cancel()
					return
				case "reading":
					log.Info("Got request for reading.")
					b, e := ioutil.ReadAll(j.sensor)
					if e != nil {
						log.Errorf("%s", e)
					} else {
						responseChannel <- b
					}
				default:
					log.Errorf("Got unrecognized command on command request channel: %s", cmd.name)
				}
			case <-ctx.Done():
				log.Warn("Shutdown...")
				return
			}
		}
	}()
	return responseChannel
}

// StartHTTP starts the HTTP subsystem
func StartHTTP(ctx context.Context, cmdChan chan *CommandRequest, respChan chan []byte) {
	r := mux.NewRouter()
	r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(IndexHTML))
	})

	api := r.PathPrefix("/api").Subrouter()
	api.HandleFunc("/{cmd}", func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		switch vars["cmd"] {
		case "shutdown":
			if r.Method != http.MethodPost {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			cmdChan <- &CommandRequest{name: "shutdown"}
		case "reading":
			if r.Method != http.MethodGet {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			timeout := time.After(30 * time.Second)
			cmdChan <- &CommandRequest{name: "reading"}

			select {
			case <-timeout:
				w.WriteHeader(http.StatusRequestTimeout)
				return
			case reading, ok := <-respChan:
				if !ok {
					log.Error("Response channel closed")
					w.WriteHeader(http.StatusRequestTimeout)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				w.Write(reading)
			}
		default:
			w.WriteHeader(http.StatusBadRequest)
		}
	})

	headersOk := handlers.AllowedHeaders([]string{"X-Requested-With", "Auth-Token"})
	originsOk := handlers.AllowedOrigins([]string{"*"})
	methodsOk := handlers.AllowedMethods([]string{"GET", "HEAD", "OPTIONS"})
	srv := &http.Server{
		Handler:      handlers.CORS(originsOk, headersOk, methodsOk)(handlers.LoggingHandler(os.Stdout, r)),
		Addr:         *httpAddress,
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}

	log.Infof("Starting HTTP server on %s", *httpAddress)
	go func() {
		if err := srv.ListenAndServe(); err != nil {
			log.Fatal(err)
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
