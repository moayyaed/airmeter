# Air Meter

`airmeter` reads the temperature, pressure and humidity from a BME280 and publishes those readings as JSON to an MQTT endpoint.

I expect the JSON format will change over time, but currently the readings are published to `<<topicroot>>/<<location>>` with the format:

```json
{
  "Temperature":15.978877,
  "Humidity":38.132843,
  "Pressure":101718.31
}
```


# Architecture

The current goal is to simply track the data for a few locations around my home.  The architecture plan is to have multiple sensors
that publish to an MQTT broker and use [telegraf](https://github.com/influxdata/telegraf) to pull that data and insert it into an
instance of [influxdb](https://github.com/influxdata/influxdb).

# Hardware

The sensors are [Adafruit BME280 sensors](https://www.adafruit.com/product/2652) wired to the i2c bus of a [Pi Zero W](https://www.adafruit.com/product/3400).

The [mosquitto MQTT broker](https://hub.docker.com/r/pascaldevink/rpi-mosquitto/), telgraf and influx all exist on a docker swarm of (currently 3) [raspberry pi 3](https://www.adafruit.com/product/3055)s.  Visualization will come later, likely with [grafana](https://grafana.com/).  The swarm is installed and configured using [hypriotos](https://blog.hypriot.com/) with data stored on a Synology NAS over NFS.
