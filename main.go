package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"time"

	_ "embed"

	MQTT "github.com/eclipse/paho.mqtt.golang"
	"github.com/fishnix/airmeter/sensor"
	"github.com/google/uuid"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"

	"gobot.io/x/gobot/platforms/raspi"
)

var (
	version = "unknown"
	commit  = "unknown"
	date    = "unknown"
	builtBy = "unknown"

	//go:embed static/index.html
	IndexHTML string

	vers         = flag.Bool("V", false, "display version information and exit")
	sensorDriver = flag.String("d", "bme280", "Which sensor driver to use: 'dummy', 'bme280' or 'sht3x'")
	tempUnits    = flag.String("U", "C", "units (F, C)")
	tempFactor   = flag.String("e", "0", "Static correction factor for the temperature.  ie. '5', '-3', '1.2")
	humidFactor  = flag.String("u", "0", "Static correction factor for the humidity.  ie. '5', '-3', '1.2'")
	pressFactor  = flag.String("r", "0", "Static correction factor for the pressure.  ie. '5', '-3', '1.2")

	frequency   = flag.String("f", "5s", "frequency to collect data from the sensor")
	location    = flag.String("l", "home", "location for the sensor")
	mqttBroker  = flag.String("b", "tcp://iot.eclipse.org:1883", "MQTT broker endpoint")
	mqttEnabled = flag.Bool("m", false, "enable mqtt")

	logLevel = flag.String("L", "info", "set the log level (debug, info, warn, error)")

	httpAddress = flag.String("a", ":8000", "HTTP listen address")

	// Advanced options
	topicroot       = flag.String("t", "airmeter", "Advanced: Set the MQTT topic root - the topic will be 'topicroot/location' - ")
	startSubscriber = flag.Bool("s", false, "Advanced: start a subscription on the MQTT topic and print to STDOUT")

	verbose = flag.Bool("v", false, "enable verbose output")
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
		fmt.Printf("Airmeter version %s (%s %s - %s)\n", version, commit, date, builtBy)
		os.Exit(0)
	}

	switch *logLevel {
	case "error":
		log.SetLevel(log.ErrorLevel)
	case "warn":
		log.SetLevel(log.WarnLevel)
	case "info":
		log.SetLevel(log.InfoLevel)
	case "debug":
		log.SetLevel(log.DebugLevel)
	default:
		log.SetLevel(log.InfoLevel)
	}

	if *verbose {
		log.SetLevel(log.DebugLevel)
		log.Debug("Starting profiler on 127.0.0.1:6080")
		go http.ListenAndServe("127.0.0.1:6080", nil)
	}

	freq, err := time.ParseDuration(*frequency)
	if err != nil {
		log.Fatalf("Cannot parse frequency duration '%s': %s", *frequency, err)
	}

	topic := fmt.Sprintf("%s/%s", *topicroot, *location)

	tf, err := parseFactor(*tempFactor)
	if err != nil {
		log.Fatalf("Cannot parse temperature factor as float '%s': %s", *tempFactor, err)
	}
	log.Infof("set temperature factor to %f", tf)

	hf, err := parseFactor(*humidFactor)
	if err != nil {
		log.Fatalf("Cannot parse temperature factor as float '%s': %s", *humidFactor, err)
	}
	log.Infof("set humidity factor to %f", hf)

	pf, err := parseFactor(*pressFactor)
	if err != nil {
		log.Fatalf("Cannot parse temperature factor as float '%s': %s", *pressFactor, err)
	}
	log.Infof("set pressure factor to %f", pf)

	var wg sync.WaitGroup

	// Setup context to allow goroutines to be cancelled
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var mqttClient MQTT.Client
	if *mqttEnabled {
		log.Infof("Starting MQTT integration...")

		mqttClient, err = newMQTTClient(ctx, &wg, *mqttBroker)
		if err != nil {
			log.Fatalf("Failed to setup MQTT client: %s", err)
		}

		if *startSubscriber {
			// if startSubscriber flag is passed, start a goroutine to subscribe to the MQTT topic
			// this is primarily added for debugging and not expected to be used most of the time.
			log.Infof("Starting MQTT subscription on topic: %s", topic)
			go subscribe(mqttClient, topic)
		}
	}

	r := raspi.NewAdaptor()
	sensor, err := sensor.NewAirMeterReader(r, *sensorDriver, *tempUnits, tf, hf, pf)
	if err != nil {
		log.Fatalf("Couldn't configure sensor! %s", err)
	}

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

	// catch os interrupts and shutdown
	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt)
		<-c
		cancel()
	}()

	log.Info("Waiting for threads to exit")

	wg.Wait()

	if err := sensor.CleanUp(); err != nil {
		log.Warnf("error running cleanup on sensor: %s", err)
	}
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
					continue
				}

				log.Infof("Sensor reading: %s", string(b))

				if j.mqttClient != nil {
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
						continue
					}
					responseChannel <- b
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

// newMQTTClient returns a new MQTT client with a random client ID and the broker endpoint
func newMQTTClient(ctx context.Context, wg *sync.WaitGroup, broker string) (MQTT.Client, error) {
	clientID := uuid.New()

	opts := MQTT.NewClientOptions().AddBroker(broker)
	log.Infof("Setting MQTT broker: %s", broker)

	opts.SetClientID(clientID.String())
	log.Infof("Setting MQTT client ID: %s", clientID.String())

	opts.SetDefaultPublishHandler(publishHandler)

	c := MQTT.NewClient(opts)

	if err := retry(3, 30*time.Second, func() error {
		log.Infof("attemting to connect MQTT client...")

		t := c.Connect()
		if t.Wait() && t.Error() != nil {
			return t.Error()
		}

		log.Infof("successfully connected MQTT client!")

		return nil
	}); err != nil {
		return nil, err
	}

	go func() {
		<-ctx.Done()
		log.Warn("disconnecting MQTT client")
		c.Disconnect(1)
		wg.Done()
	}()

	return c, nil
}

type stop struct {
	error
}

// retry is stolen from https://upgear.io/blog/simple-golang-retry-function/
func retry(attempts int, sleep time.Duration, f func() error) error {
	if err := f(); err != nil {
		if s, ok := err.(stop); ok {
			// Return the original error for later checking
			return s.error
		}

		if attempts--; attempts > 0 {
			// Add some randomness to prevent creating a Thundering Herd
			jitter := time.Duration(rand.Int63n(int64(sleep)))
			sleep = sleep + jitter/2

			time.Sleep(sleep)
			return retry(attempts, 2*sleep, f)
		}
		return err
	}

	return nil
}

// subscribe subscribes to the MQTT topic defined at startup and handles messages with the default handler
func subscribe(mqttClient MQTT.Client, topic string) {
	if token := mqttClient.Subscribe(topic, 0, nil); token.Wait() && token.Error() != nil {
		log.Errorln(token.Error())
		os.Exit(1)
	}
}

func parseFactor(f string) (float32, error) {
	log.Debugf("parsing correction factor %s", f)
	factor, err := strconv.ParseFloat(f, 32)
	if err != nil {
		return float32(0), err
	}

	log.Debugf("parsed correction factor %f", factor)
	return float32(factor), nil
}
