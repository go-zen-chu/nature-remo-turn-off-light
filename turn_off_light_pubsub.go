// Package p contains an HTTP Cloud Function.
package p

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/tenntenn/natureremo"
)

const (
	illuminationThreshold  = 100
	lightTurnOffInterval   = 15 * time.Second
	lightApplianceName     = "Light" // check with yout nature remo app
	lightTurnOffSignalName = "off"
)

type PubSubMessage struct {
	Data []byte `json:"data"`
}

// TurnOffLight : Turn off light via Nature Remo API
// Make sure you set environment variable and token
func TurnOffLight(pubSubCtx context.Context, m PubSubMessage) error {
	fmt.Printf("%s\n", string(m.Data))
	// get token
	token := os.Getenv("NATURE_REMO_GLOBAL_TOKEN")
	if token == "" {
		return errors.New("Error getting env var: NATURE_REMO_GLOBAL_TOKEN")
	}
	// create client
	c := natureremo.NewClient(token)
	ctx := context.Background()
	// get devices
	dvs, err := c.DeviceService.GetAll(ctx)
	if err != nil {
		return fmt.Errorf("Error getting devices: %s", err.Error())
	}
	fmt.Printf("Num devices : %d\n", len(dvs))
	if len(dvs) == 0 {
		return errors.New("Could not find devices")
	}
	var dv *natureremo.Device
	for _, d := range dvs {
		if strings.Contains(d.FirmwareVersion, "Remo-mini") {
			fmt.Fprintln(os.Stderr, "NatureRemo mini does not support illumination value")
		} else {
			dv = d // only use the first device
			break
		}
	}
	if dv == nil {
		return errors.New("There was no device supporting measuring illumination value")
	}

	// get appliances and turn off signal
	acs, err := c.ApplianceService.GetAll(ctx)
	if err != nil {
		return fmt.Errorf("Error getting appliances: %s", err.Error())
	}
	fmt.Printf("Num appliances : %d\n", len(acs))
	if len(acs) == 0 {
		return errors.New("Could not find appliances")
	}
	var ac *natureremo.Appliance
	for _, a := range acs {
		if a.Nickname == lightApplianceName {
			ac = a
		}
	}
	if ac == nil {
		return fmt.Errorf("Could not find light with nickname : %s", lightApplianceName)
	}
	var sg *natureremo.Signal
	for _, s := range ac.Signals {
		if s.Name == lightTurnOffSignalName {
			sg = s
		}
	}
	if sg == nil {
		return fmt.Errorf("Could not find turn off signal : %s", lightTurnOffSignalName)
	}

	// turn off light until a room gets dark
	count := 10
	go func() {
		t := time.NewTicker(lightTurnOffInterval)
		for {
			select {
			case <-t.C:
				if count <= 0 {
					fmt.Println("Exceed counts")
					t.Stop()
					break
				}
				count--
				dv, err = c.DeviceService.Update(ctx, dv)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error updating device : %s", err.Error())
					continue
				}
				// get sensor value
				sv := dv.NewestEvents[natureremo.SensortypeIllumination]
				il := sv.Value
				fmt.Printf("Illumination value : %f\n", il)
				// if failed to get sensor value, it is zero
				if il >= illuminationThreshold {
					err := c.SignalService.Send(ctx, sg)
					if err != nil {
						fmt.Fprintf(os.Stderr, "Error executing signal : %s", err.Error())
					}
				} else {
					fmt.Println("A room gets dark")
					t.Stop()
					break
				}
			}
		}
	}()
	return nil
}
