package main

import (
	"log"
	"time"

	"github.com/brutella/hc"
	"github.com/brutella/hc/accessory"
	"github.com/brutella/hc/characteristic"
	"github.com/brutella/hc/service"
	"github.com/pjuzeliunas/nilan"
)

// Nilan HomeKit accessory
type Nilan struct {
	*accessory.Accessory

	OutdoorTemp *service.TemperatureSensor
	IndoorTemp  *service.TemperatureSensor
	Humidity    *service.HumiditySensor
	DHWTemp     *service.TemperatureSensor
	FlowTemp    *service.TemperatureSensor
}

// NewNilan sets Nilan accessory instance up
func NewNilan(info accessory.Info) *Nilan {
	acc := Nilan{}
	acc.Accessory = accessory.New(info, accessory.TypeHeater)

	acc.OutdoorTemp = service.NewTemperatureSensor()
	acc.OutdoorTemp.AddCharacteristic(newName("Outdoor Temperature"))
	acc.OutdoorTemp.CurrentTemperature.SetMinValue(-40)
	acc.OutdoorTemp.CurrentTemperature.SetMaxValue(160)

	acc.IndoorTemp = service.NewTemperatureSensor()
	acc.IndoorTemp.AddCharacteristic(newName("Room Temperature"))
	acc.IndoorTemp.CurrentTemperature.SetMinValue(-40)
	acc.IndoorTemp.CurrentTemperature.SetMaxValue(160)

	acc.Humidity = service.NewHumiditySensor()
	acc.Humidity.AddCharacteristic(newName("Humidity"))

	acc.DHWTemp = service.NewTemperatureSensor()
	acc.DHWTemp.AddCharacteristic(newName("DHW Temperature"))
	acc.DHWTemp.CurrentTemperature.SetMinValue(-40)
	acc.DHWTemp.CurrentTemperature.SetMaxValue(160)

	acc.FlowTemp = service.NewTemperatureSensor()
	acc.FlowTemp.AddCharacteristic(newName("Flow Temperature"))
	acc.FlowTemp.CurrentTemperature.SetMinValue(-40)
	acc.FlowTemp.CurrentTemperature.SetMaxValue(160)

	acc.AddService(acc.OutdoorTemp.Service)
	acc.AddService(acc.IndoorTemp.Service)
	acc.AddService(acc.Humidity.Service)
	acc.AddService(acc.DHWTemp.Service)
	acc.AddService(acc.FlowTemp.Service)

	return &acc
}

func newName(n string) *characteristic.Characteristic {
	char := characteristic.NewName()
	char.String.SetValue(n)
	return char.Characteristic
}

func updateReadings(acc *Nilan) {
	conf := nilan.CurrentConfig() // nilan.Config{NilanAddress: "192.168.1.31:502"}
	c := nilan.Controller{Config: conf}
	r := c.FetchReadings()

	acc.OutdoorTemp.CurrentTemperature.SetValue(float64(r.OutdoorTemperature) / 10.0)
	acc.IndoorTemp.CurrentTemperature.SetValue(float64(r.RoomTemperature) / 10.0)
	acc.Humidity.CurrentRelativeHumidity.SetValue(float64(r.ActualHumidity))
	acc.DHWTemp.CurrentTemperature.SetValue(float64(r.DHWTankTopTemperature) / 10.0)
	acc.FlowTemp.CurrentTemperature.SetValue(float64(r.SupplyFlowTemperature) / 10.0)
}

func startUpdatingReadings(ac *Nilan, freq time.Duration) {
	for {
		updateReadings(ac)
		time.Sleep(freq) // 3 sec delay
	}
}

func main() {
	// create an accessory
	info := accessory.Info{Name: "Nilan"}
	ac := NewNilan(info)

	go startUpdatingReadings(ac, 5*time.Second)

	// configure the ip transport
	config := hc.Config{Pin: "00102003", Port: "55292"}
	t, err := hc.NewIPTransport(config, ac.Accessory)
	if err != nil {
		log.Panic(err)
	}

	hc.OnTermination(func() {
		<-t.Stop()
	})

	t.Start()

}
