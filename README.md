# Air Meter

`airmeter` reads the temperature, pressure and humidity from sensor and publishes those readings as JSON to an MQTT endpoint.

I expect the JSON format will change over time, but currently the readings are published to `<<topicroot>>/<<location>>` with the format:

```json
{
  "Temperature":15.978877,
  "Humidity":38.132843,
  "Pressure":101718.31
}
```

If the sensor doesn't support a value, it should not be included in the output.


# Architecture

The current goal is to simply track the data for a few locations around my home.  The architecture plan is to have multiple sensors
that publish to an MQTT broker and use [telegraf](https://github.com/influxdata/telegraf) to pull that data and insert it into an
instance of [influxdb](https://github.com/influxdata/influxdb).

# Hardware

The supported/tested sensors are:
* Dummy (returns a random temperature, humidity, and pressure)
* [Adafruit BME280 sensors](https://www.adafruit.com/product/2652)
* [Adafruit SHT31-D sensors](https://www.adafruit.com/product/2857)

 and their relatives, wired to the i2c bus of a [Pi Zero W](https://www.adafruit.com/product/3400).

# Usage

```bash
  Usage of airmeter:
  -V display version information and exit
  -a string HTTP listen address (default ":8000")
  -b string MQTT broker endpoint (default "tcp://iot.eclipse.org:1883")
  -d string Which sensor driver to use: 'dummy', 'bme280' or 'sht3x' (default "bme280")
  -e string Static correction factor for the temperature.  ie. '5', '-3', '1.2 (default "0")
  -f string frequency to collect data from the sensor (default "5s")
  -l string location for the sensor (default "home")
  -r string Static correction factor for the pressure.  ie. '5', '-3', '1.2 (default "0")
  -s Advanced: start a subscription on the MQTT topic and print to STDOUT
  -t string Advanced: Set the MQTT topic root - the topic will be 'topicroot/location' -  (default "airmeter")
  -u string Static correction factor for the humidity.  ie. '5', '-3', '1.2' (default "0")
  -v enable verbose output
```

# Development

## Build for Pi Zero W

```bash
env GOOS=linux GOARCH=arm GOARM=5 go build
```

# Backend

All backend components are simple docker containers running on a single 5 year old Intel NUC.  Nothing fancy here....

## Eclipse Mosquitto MQTT server

[mosquitto](https://hub.docker.com/_/eclipse-mosquitto)

## Telegraf

[telegraf](https://hub.docker.com/_/telegraf)

### Input

```toml
[[inputs.mqtt_consumer]]
  servers = ["tcp://mosquitto-server:1883"]
  topics = [
    "airmeter/office"
  ],
  data_format = "json"
```

### Output

```toml
[[outputs.influxdb]]
  urls = ["http://influxdb:8086"]
  database = "airmeter"
```

## InfluxDB

[influxdb](https://hub.docker.com/_/influxdb)
