package main

import (
	"log"

	"github.com/brutella/hc"
	"github.com/brutella/hc/accessory"
	"github.com/brutella/hc/characteristic"
	"github.com/brutella/hc/service"
)

type Nilan struct {
	*accessory.Accessory
	OutdoorTemp *service.TemperatureSensor
	IndoorTemp  *service.TemperatureSensor
	Humidity    *service.HumiditySensor
	DHWTemp     *service.TemperatureSensor
	FlowTemp    *service.TemperatureSensor

	CentralHeating *service.Switch
	DHW            *service.Switch
	Ventilation    *service.Fan
}

func NewNilan(info accessory.Info) *Nilan {
	acc := Nilan{}
	acc.Accessory = accessory.New(info, accessory.TypeHeater)

	acc.OutdoorTemp = service.NewTemperatureSensor()
	acc.OutdoorTemp.AddCharacteristic(newName("Outdoor Temperature"))
	acc.OutdoorTemp.CurrentTemperature.SetMinValue(-100)
	acc.OutdoorTemp.CurrentTemperature.SetMaxValue(100)
	acc.OutdoorTemp.CurrentTemperature.SetValue(4)

	acc.IndoorTemp = service.NewTemperatureSensor()
	acc.IndoorTemp.AddCharacteristic(newName("Room Temperature"))
	acc.IndoorTemp.CurrentTemperature.SetValue(21.4)

	acc.CentralHeating = service.NewSwitch()
	name := characteristic.NewName()
	name.String.SetValue("Central Heating")
	acc.CentralHeating.AddCharacteristic(name.Characteristic)

	acc.DHW = service.NewSwitch()
	name = characteristic.NewName()
	name.String.SetValue("DHW")
	acc.DHW.AddCharacteristic(name.Characteristic)

	acc.Ventilation = service.NewFan()
	name = characteristic.NewName()
	name.String.SetValue("Ventilation")
	acc.Ventilation.AddCharacteristic(name.Characteristic)

	acc.AddService(acc.OutdoorTemp.Service)
	acc.AddService(acc.IndoorTemp.Service)
	acc.AddService(acc.CentralHeating.Service)
	acc.AddService(acc.DHW.Service)
	acc.AddService(acc.Ventilation.Service)

	return &acc
}

func newName(n string) *characteristic.Characteristic {
	char := characteristic.NewName()
	char.String.SetValue(n)
	return char.Characteristic
}

func main() {
	// create an accessory
	info := accessory.Info{Name: "Nilan"}
	ac := NewNilan(info)

	// configure the ip transport
	config := hc.Config{Pin: "00102003"}
	t, err := hc.NewIPTransport(config, ac.Accessory)
	if err != nil {
		log.Panic(err)
	}

	hc.OnTermination(func() {
		<-t.Stop()
	})

	t.Start()
}
