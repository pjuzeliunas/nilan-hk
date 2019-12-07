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

// Nilan CTS700 accessory
type Nilan struct {
	*accessory.Accessory

	CentralHeating *NilanCentralHeating
	Ventilation    *NilanVentilation
	HotWater       *service.Thermostat

	OutdoorTemp *service.TemperatureSensor
	FlowTemp    *service.TemperatureSensor
}

// NilanCentralHeating service
type NilanCentralHeating struct {
	*service.Thermostat
	CurrentRelativeHumidity *characteristic.CurrentRelativeHumidity
}

// NewNilanCentralHeating instantiates Nilan Central Heating service
func NewNilanCentralHeating() *NilanCentralHeating {
	svc := NilanCentralHeating{}
	svc.Thermostat = service.NewThermostat()

	svc.CurrentRelativeHumidity = characteristic.NewCurrentRelativeHumidity()
	svc.AddCharacteristic(svc.CurrentRelativeHumidity.Characteristic)

	return &svc
}

// NilanVentilation service
type NilanVentilation struct {
	*service.FanV2
	RotationSpeed *characteristic.RotationSpeed
}

// NewNilanVentilation instantiates Nilan Ventilation service
func NewNilanVentilation() *NilanVentilation {
	svc := NilanVentilation{}
	svc.FanV2 = service.NewFanV2()

	svc.RotationSpeed = characteristic.NewRotationSpeed()
	svc.RotationSpeed.SetMinValue(25)
	svc.RotationSpeed.SetMaxValue(100)
	svc.RotationSpeed.SetStepValue(25)
	svc.AddCharacteristic(svc.RotationSpeed.Characteristic)

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
		if !(t >= 50 && t <= 400) {
			log.Println("Invalid Central Heating temperature setting. Ignoring change request.")
			return
		}
		s := nilan.Settings{DesiredRoomTemperature: &t}
		c := nilanController()
		c.SendSettings(s)
	})

	acc.Ventilation = NewNilanVentilation()
	acc.Ventilation.AddCharacteristic(newName("Ventilation"))
	acc.Ventilation.Active.Perms = []string{characteristic.PermRead, characteristic.PermEvents}
	acc.Ventilation.RotationSpeed.OnValueRemoteUpdate(func(newSpeed float64) {
		log.Printf("Setting new Ventilation speed: %v\n", newSpeed)
		speed := nilan.FanSpeed(100 + int(newSpeed)/25)
		if !(speed >= 101 && speed <= 104) {
			log.Println("Invalid Ventilation mode. Ignoring change request.")
			return
		}
		s := nilan.Settings{FanSpeed: &speed}
		c := nilanController()
		c.SendSettings(s)
	})
	// acc.Ventilation.Active.OnValueRemoteUpdate(func(isActive int) {
	// 	switch isActive {
	// 	case characteristic.ActiveInactive:
	// 		p := true
	// 		s := nilan.Settings{VentilationOnPause: &p}
	// 		c := nilanController()
	// 		c.SendSettings(s)
	// 	case characteristic.ActiveActive:
	// 		p := false
	// 		s := nilan.Settings{VentilationOnPause: &p}
	// 		c := nilanController()
	// 		c.SendSettings(s)
	// 	}
	// })

	acc.HotWater = service.NewThermostat()
	acc.HotWater.AddCharacteristic(newName("Hot Water"))
	acc.HotWater.TargetHeatingCoolingState.SetValue(characteristic.TargetHeatingCoolingStateHeat)
	acc.HotWater.TargetHeatingCoolingState.Perms = []string{characteristic.PermRead, characteristic.PermEvents}
	acc.HotWater.TemperatureDisplayUnits.SetValue(characteristic.TemperatureDisplayUnitsCelsius)
	acc.HotWater.TargetTemperature.SetMinValue(10.0)
	acc.HotWater.TargetTemperature.SetMaxValue(60.0)
	acc.HotWater.TargetTemperature.SetStepValue(1.0)
	acc.HotWater.TargetTemperature.OnValueRemoteUpdate(func(tFloat float64) {
		log.Printf("Setting new DHW target temperature: %v\n", tFloat)
		t := int(tFloat * 10.0)
		if !(t >= 100 && t <= 600) {
			log.Println("InvalidDHW temperature setting. Ignoring change request.")
			return
		}
		s := nilan.Settings{DesiredDHWTemperature: &t}
		c := nilanController()
		c.SendSettings(s)
	})

	acc.OutdoorTemp = service.NewTemperatureSensor()
	acc.OutdoorTemp.AddCharacteristic(newName("Outdoor Temperature"))
	acc.OutdoorTemp.CurrentTemperature.SetMinValue(-40)
	acc.OutdoorTemp.CurrentTemperature.SetMaxValue(160)

	acc.FlowTemp = service.NewTemperatureSensor()
	acc.FlowTemp.AddCharacteristic(newName("Flow Temperature"))
	acc.FlowTemp.CurrentTemperature.SetMinValue(-40)
	acc.FlowTemp.CurrentTemperature.SetMaxValue(160)

	acc.AddService(acc.CentralHeating.Service)
	acc.AddService(acc.OutdoorTemp.Service)
	acc.AddService(acc.Ventilation.Service)
	acc.AddService(acc.HotWater.Service)

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

	if *s.VentilationOnPause {
		acc.Ventilation.Active.SetValue(characteristic.ActiveInactive)
	} else {
		acc.Ventilation.Active.SetValue(characteristic.ActiveActive)
	}
	acc.Ventilation.RotationSpeed.SetValue((float64(*s.FanSpeed) - 100) * 25.0)

	acc.HotWater.CurrentTemperature.SetValue(float64(r.DHWTankTopTemperature) / 10.0)
	acc.HotWater.TargetTemperature.SetValue(float64(*s.DesiredDHWTemperature) / 10.0)
	if *s.DHWProductionPaused {
		acc.HotWater.CurrentHeatingCoolingState.SetValue(characteristic.CurrentHeatingCoolingStateOff)
	} else {
		acc.HotWater.CurrentHeatingCoolingState.SetValue(characteristic.CurrentHeatingCoolingStateHeat)
	}

	acc.OutdoorTemp.CurrentTemperature.SetValue(float64(r.OutdoorTemperature) / 10.0)
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
