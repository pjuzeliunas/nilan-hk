package main

import (
	"log"
	"os"
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

	CentralHeatingSwitch  *service.Switch
	VentilationThermostat *NilanFanThermostat
	OutdoorTemp           *service.TemperatureSensor
	Fan                   *NilanFan
	HotWaterSwitch        *service.Switch
	HotWater              *service.Thermostat
	SupplyFlow            *service.Thermostat
}

// NilanFanThermostat service
type NilanFanThermostat struct {
	*service.Thermostat
	CurrentRelativeHumidity *characteristic.CurrentRelativeHumidity
}

// NewNilanFanThermostat instantiates Nilan Central Heating service
func NewNilanFanThermostat() *NilanFanThermostat {
	svc := NilanFanThermostat{}
	svc.Thermostat = service.NewThermostat()

	svc.CurrentRelativeHumidity = characteristic.NewCurrentRelativeHumidity()
	svc.AddCharacteristic(svc.CurrentRelativeHumidity.Characteristic)

	return &svc
}

// NilanFan service
type NilanFan struct {
	*service.FanV2
	RotationSpeed *characteristic.RotationSpeed
}

// NewNilanFan instantiates Nilan Fan service
func NewNilanFan() *NilanFan {
	svc := NilanFan{}
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

	acc.CentralHeatingSwitch = service.NewSwitch()
	acc.CentralHeatingSwitch.AddCharacteristic(newName("Central Heating"))
	acc.CentralHeatingSwitch.On.OnValueRemoteUpdate(func(on bool) {
		log.Printf("Setting Central Heating active: %v\n", on)

		s := nilan.Settings{}
		p := !on
		s.CentralHeatingPaused = &p
		if !on {
			s.CentralHeatingPauseDuration = new(int)
			*s.CentralHeatingPauseDuration = 180
		}

		c := nilanController()
		c.SendSettings(s)
	})

	acc.VentilationThermostat = NewNilanFanThermostat()
	acc.VentilationThermostat.Primary = true
	acc.VentilationThermostat.AddCharacteristic(newName("Room Temperature"))
	acc.VentilationThermostat.TargetHeatingCoolingState.OnValueRemoteUpdate(func(state int) {
		c := nilanController()
		switch state {
		case characteristic.TargetHeatingCoolingStateOff:
			p := true
			s := nilan.Settings{VentilationOnPause: &p}
			c.SendSettings(s)
		case characteristic.TargetHeatingCoolingStateHeat:
			p := false
			m := 2 // heating
			s := nilan.Settings{VentilationMode: &m, VentilationOnPause: &p}
			c.SendSettings(s)
		case characteristic.TargetHeatingCoolingStateCool:
			p := false
			m := 1 // cooling
			s := nilan.Settings{VentilationMode: &m, VentilationOnPause: &p}
			c.SendSettings(s)
		case characteristic.TargetHeatingCoolingStateAuto:
			p := false
			m := 0 // auto
			s := nilan.Settings{VentilationMode: &m, VentilationOnPause: &p}
			c.SendSettings(s)
		}
	})
	acc.VentilationThermostat.TemperatureDisplayUnits.SetValue(characteristic.TemperatureDisplayUnitsCelsius)
	acc.VentilationThermostat.TargetTemperature.SetMinValue(5.0)
	acc.VentilationThermostat.TargetTemperature.SetMaxValue(40.0)
	acc.VentilationThermostat.TargetTemperature.SetStepValue(1.0)
	acc.VentilationThermostat.TargetTemperature.OnValueRemoteUpdate(func(tFloat float64) {
		log.Printf("Setting new Room target temperature: %v\n", tFloat)
		t := int(tFloat * 10.0)
		if !(t >= 50 && t <= 400) {
			log.Println("Invalid Room temperature setting. Ignoring change request.")
			return
		}
		s := nilan.Settings{DesiredRoomTemperature: &t}
		c := nilanController()
		c.SendSettings(s)
	})

	acc.Fan = NewNilanFan()
	acc.Fan.AddCharacteristic(newName("Fan"))
	acc.Fan.Active.Perms = []string{characteristic.PermRead, characteristic.PermEvents}
	acc.Fan.RotationSpeed.OnValueRemoteUpdate(func(newSpeed float64) {
		log.Printf("Setting new Fan speed: %v\n", newSpeed)
		speed := nilan.FanSpeed(100 + int(newSpeed)/25)
		if !(speed >= 101 && speed <= 104) {
			log.Println("Invalid Fan speed. Ignoring change request.")
			return
		}
		s := nilan.Settings{FanSpeed: &speed}
		c := nilanController()
		c.SendSettings(s)
	})

	acc.HotWaterSwitch = service.NewSwitch()
	acc.HotWaterSwitch.AddCharacteristic(newName("Hot Water Production"))
	acc.HotWaterSwitch.On.OnValueRemoteUpdate(func(on bool) {
		log.Printf("Setting DHW active: %v\n", on)

		s := nilan.Settings{}
		p := !on
		s.DHWProductionPaused = &p

		if !on {
			s.DHWProductionPauseDuration = new(int)
			*s.DHWProductionPauseDuration = 180
		}

		c := nilanController()
		c.SendSettings(s)
	})

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
			log.Println("Invalid DHW temperature setting. Ignoring change request.")
			return
		}
		s := nilan.Settings{DesiredDHWTemperature: &t}
		c := nilanController()
		c.SendSettings(s)
	})

	acc.SupplyFlow = service.NewThermostat()
	acc.SupplyFlow.AddCharacteristic(newName("Central Heating"))
	acc.SupplyFlow.TargetHeatingCoolingState.SetValue(characteristic.TargetHeatingCoolingStateHeat)
	acc.SupplyFlow.TargetHeatingCoolingState.Perms = []string{characteristic.PermRead, characteristic.PermEvents}
	acc.SupplyFlow.TemperatureDisplayUnits.SetValue(characteristic.TemperatureDisplayUnitsCelsius)
	acc.SupplyFlow.TargetTemperature.SetMinValue(5.0)
	acc.SupplyFlow.TargetTemperature.SetMaxValue(50.0)
	acc.SupplyFlow.TargetTemperature.SetStepValue(1.0)
	acc.SupplyFlow.TargetTemperature.OnValueRemoteUpdate(func(tFloat float64) {
		log.Printf("Setting new Supply Flow target temperature: %v\n", tFloat)
		t := int(tFloat * 10.0)
		if !(t >= 50 && t <= 500) {
			log.Println("Invalid Supply Flow temperature setting. Ignoring change request.")
			return
		}
		s := nilan.Settings{SetpointSupplyTemperature: &t}
		c := nilanController()
		c.SendSettings(s)
	})

	acc.OutdoorTemp = service.NewTemperatureSensor()
	acc.OutdoorTemp.AddCharacteristic(newName("Outdoor Temperature"))
	acc.OutdoorTemp.CurrentTemperature.SetMinValue(-40)
	acc.OutdoorTemp.CurrentTemperature.SetMaxValue(160)

	acc.AddService(acc.CentralHeatingSwitch.Service)
	acc.AddService(acc.VentilationThermostat.Service)
	acc.AddService(acc.OutdoorTemp.Service)
	acc.AddService(acc.Fan.Service)
	acc.AddService(acc.HotWaterSwitch.Service)
	acc.AddService(acc.HotWater.Service)
	acc.AddService(acc.SupplyFlow.Service)

	return &acc
}

func newName(n string) *characteristic.Characteristic {
	char := characteristic.NewName()
	char.String.SetValue(n)
	return char.Characteristic
}

func nilanController() nilan.Controller {
	conf := nilan.CurrentConfig()
	return nilan.Controller{Config: conf}
}

func updateReadings(acc *Nilan) {
	c := nilanController()
	r := c.FetchReadings()
	s := c.FetchSettings()

	if *s.CentralHeatingIsOn && !*s.CentralHeatingPaused {
		acc.CentralHeatingSwitch.On.SetValue(true)
		acc.SupplyFlow.CurrentHeatingCoolingState.SetValue(characteristic.CurrentHeatingCoolingStateHeat)
	} else {
		acc.CentralHeatingSwitch.On.SetValue(false)
		acc.SupplyFlow.CurrentHeatingCoolingState.SetValue(characteristic.CurrentHeatingCoolingStateOff)
	}

	acc.VentilationThermostat.CurrentTemperature.SetValue(float64(r.RoomTemperature) / 10.0)
	acc.VentilationThermostat.TargetTemperature.SetValue(float64(*s.DesiredRoomTemperature) / 10.0)
	acc.VentilationThermostat.CurrentRelativeHumidity.SetValue(float64(r.ActualHumidity))

	if *s.VentilationOnPause {
		acc.VentilationThermostat.TargetHeatingCoolingState.SetValue(characteristic.TargetHeatingCoolingStateOff)
		acc.VentilationThermostat.CurrentHeatingCoolingState.SetValue(characteristic.CurrentHeatingCoolingStateOff)
	} else {
		switch *s.VentilationMode {
		case 0: // auto
			acc.VentilationThermostat.TargetHeatingCoolingState.SetValue(characteristic.TargetHeatingCoolingStateAuto)
			acc.VentilationThermostat.CurrentHeatingCoolingState.SetValue(characteristic.CurrentHeatingCoolingStateOff)
		case 1: // cooling
			acc.VentilationThermostat.TargetHeatingCoolingState.SetValue(characteristic.TargetHeatingCoolingStateCool)
			acc.VentilationThermostat.CurrentHeatingCoolingState.SetValue(characteristic.CurrentHeatingCoolingStateCool)
		case 2: // heating
			acc.VentilationThermostat.TargetHeatingCoolingState.SetValue(characteristic.TargetHeatingCoolingStateHeat)
			acc.VentilationThermostat.CurrentHeatingCoolingState.SetValue(characteristic.CurrentHeatingCoolingStateHeat)
		}
	}

	if *s.VentilationOnPause {
		acc.Fan.Active.SetValue(characteristic.ActiveInactive)
	} else {
		acc.Fan.Active.SetValue(characteristic.ActiveActive)
	}
	acc.Fan.RotationSpeed.SetValue((float64(*s.FanSpeed) - 100) * 25.0)

	acc.HotWater.CurrentTemperature.SetValue(float64(r.DHWTankTopTemperature) / 10.0)
	acc.HotWater.TargetTemperature.SetValue(float64(*s.DesiredDHWTemperature) / 10.0)
	if *s.DHWProductionPaused {
		acc.HotWater.CurrentHeatingCoolingState.SetValue(characteristic.CurrentHeatingCoolingStateOff)
		acc.HotWaterSwitch.On.SetValue(false)
	} else {
		acc.HotWater.CurrentHeatingCoolingState.SetValue(characteristic.CurrentHeatingCoolingStateHeat)
		acc.HotWaterSwitch.On.SetValue(true)
	}

	acc.SupplyFlow.CurrentTemperature.SetValue(float64(r.SupplyFlowTemperature) / 10.0)
	acc.SupplyFlow.TargetTemperature.SetValue(float64(*s.SetpointSupplyTemperature) / 10.0)

	acc.OutdoorTemp.CurrentTemperature.SetValue(float64(r.OutdoorTemperature) / 10.0)
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
	info := accessory.Info{Name: "Nilan"}
	ac := NewNilan(info)

	go startUpdatingReadings(ac, 5*time.Second)

	pin, pinDefined := os.LookupEnv("HK_PIN")
	if !pinDefined {
		log.Panic("HK_PIN environment variable with 8 digit PIN code must be present")
	}
	port, portDefined := os.LookupEnv("HK_PORT")

	// configure the ip transport
	config := hc.Config{Pin: pin}
	if portDefined {
		config.Port = port
	}

	t, err := hc.NewIPTransport(config, ac.Accessory)
	if err != nil {
		log.Panic(err)
	}

	hc.OnTermination(func() {
		<-t.Stop()
	})

	t.Start()

}
