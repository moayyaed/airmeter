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

I can't say for sure, but I decided to implement the sensors as io.Reader where Read() should return the JSON encoded `[]byte` of data from the sensor.  This seems like it will make it 'easy' to implement any other sensors (at least via i2c and gobot!).

# Hardware

The supported/tested sensors are:
* Dummy (returns a random temperature, humidity, and pressure)
* [Adafruit BME280 sensors](https://www.adafruit.com/product/2652)
* [Adafruit SHT31-D sensors](https://www.adafruit.com/product/2857)

 and their relatives, wired to the i2c bus of a [Pi Zero W](https://www.adafruit.com/product/3400).

The [mosquitto MQTT broker](https://hub.docker.com/r/pascaldevink/rpi-mosquitto/), telgraf and influx all exist on a docker swarm of (currently 3) [raspberry pi 3](https://www.adafruit.com/product/3055)s.  Visualization will come later, likely with [grafana](https://grafana.com/).  The swarm is installed and configured using [hypriotos](https://blog.hypriot.com/) with data stored on a Synology NAS over NFS.
