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

	CentralHeating *NilanCentralHeating

	OutdoorTemp *service.TemperatureSensor
	DHWTemp     *service.TemperatureSensor
	FlowTemp    *service.TemperatureSensor
}

type NilanCentralHeating struct {
	*service.Thermostat
	CurrentRelativeHumidity *characteristic.CurrentRelativeHumidity
}

func NewNilanCentralHeating() *NilanCentralHeating {
	svc := NilanCentralHeating{}
	svc.Thermostat = service.NewThermostat()

	svc.CurrentRelativeHumidity = characteristic.NewCurrentRelativeHumidity()
	svc.AddCharacteristic(svc.CurrentRelativeHumidity.Characteristic)

	return &svc
}

// NewNilan sets Nilan accessory instance up
func NewNilan(info accessory.Info) *Nilan {
	acc := Nilan{}
	acc.Accessory = accessory.New(info, accessory.TypeHeater)

	acc.CentralHeating = NewNilanCentralHeating()
	acc.CentralHeating.Primary = true
	acc.CentralHeating.AddCharacteristic(newName("Central Heating"))
	acc.CentralHeating.TargetHeatingCoolingState.SetValue(characteristic.TargetHeatingCoolingStateHeat)
	acc.CentralHeating.TargetHeatingCoolingState.Perms = []string{characteristic.PermRead, characteristic.PermEvents}
	acc.CentralHeating.TemperatureDisplayUnits.SetValue(characteristic.TemperatureDisplayUnitsCelsius)
	acc.CentralHeating.TargetTemperature.SetMinValue(5.0)
	acc.CentralHeating.TargetTemperature.SetMaxValue(40.0)
	acc.CentralHeating.TargetTemperature.SetStepValue(1.0)
	acc.CentralHeating.TargetTemperature.OnValueRemoteUpdate(func(tFloat float64) {
		log.Printf("Setting new Central Heating target temperature: %v\n", tFloat)
		t := int(tFloat * 10.0)
		s := nilan.Settings{DesiredRoomTemperature: &t}
		c := nilanController()
		c.SendSettings(s)
	})

	acc.OutdoorTemp = service.NewTemperatureSensor()
	acc.OutdoorTemp.AddCharacteristic(newName("Outdoor Temperature"))
	acc.OutdoorTemp.CurrentTemperature.SetMinValue(-40)
	acc.OutdoorTemp.CurrentTemperature.SetMaxValue(160)

	acc.DHWTemp = service.NewTemperatureSensor()
	acc.DHWTemp.AddCharacteristic(newName("DHW Temperature"))
	acc.DHWTemp.CurrentTemperature.SetMinValue(-40)
	acc.DHWTemp.CurrentTemperature.SetMaxValue(160)

	acc.FlowTemp = service.NewTemperatureSensor()
	acc.FlowTemp.AddCharacteristic(newName("Flow Temperature"))
	acc.FlowTemp.CurrentTemperature.SetMinValue(-40)
	acc.FlowTemp.CurrentTemperature.SetMaxValue(160)

	acc.AddService(acc.CentralHeating.Service)
	acc.AddService(acc.OutdoorTemp.Service)
	acc.AddService(acc.DHWTemp.Service)
	acc.AddService(acc.FlowTemp.Service)

	return &acc
}

func newName(n string) *characteristic.Characteristic {
	char := characteristic.NewName()
	char.String.SetValue(n)
	return char.Characteristic
}

func nilanController() nilan.Controller {
	conf := nilan.Config{NilanAddress: "192.168.1.31:502"} // TODO: undo
	// conf := nilan.CurrentConfig()
	return nilan.Controller{Config: conf}
}

func updateReadings(acc *Nilan) {
	c := nilanController()
	r := c.FetchReadings()
	s := c.FetchSettings()

	if *s.CentralHeatingIsOn && !*s.CentralHeatingPaused {
		acc.CentralHeating.CurrentHeatingCoolingState.SetValue(characteristic.CurrentHeatingCoolingStateHeat)
	} else {
		acc.CentralHeating.CurrentHeatingCoolingState.SetValue(characteristic.CurrentHeatingCoolingStateOff)
	}

	acc.CentralHeating.CurrentTemperature.SetValue(float64(r.RoomTemperature) / 10.0)
	acc.CentralHeating.TargetTemperature.SetValue(float64(*s.DesiredRoomTemperature) / 10.0)
	acc.CentralHeating.CurrentRelativeHumidity.SetValue(float64(r.ActualHumidity))

	acc.OutdoorTemp.CurrentTemperature.SetValue(float64(r.OutdoorTemperature) / 10.0)
	acc.Humidity.CurrentRelativeHumidity.SetValue(float64(r.ActualHumidity))
	acc.DHWTemp.CurrentTemperature.SetValue(float64(r.DHWTankTopTemperature) / 10.0)
	acc.FlowTemp.CurrentTemperature.SetValue(float64(r.SupplyFlowTemperature) / 10.0)
}

func startUpdatingReadings(ac *Nilan, freq time.Duration) {
	defer func() {
		if r := recover(); r != nil {
			// In case of failure: waiting and trying again
			log.Printf("Sync with Nilan did fail: %v\n", r)
			time.Sleep(freq)
			startUpdatingReadings(ac, freq)
		}
	}()
	for {
		updateReadings(ac)
		time.Sleep(freq) // 3 sec delay
	}
}

func main() {
	// create an accessory
	info := accessory.Info{Name: "Nilan-Debug"}
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
