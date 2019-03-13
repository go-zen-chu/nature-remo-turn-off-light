// Package p contains an HTTP Cloud Function.
package p

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/tenntenn/natureremo"
)

const (
	illuminationThreshold  = 50
	lightTurnOffInterval   = 15 * time.Second
	lightApplianceName     = "Light" // check with yout nature remo app
	lightTurnOffSignalName = "off"
)

// TurnOffLight : Turn off light via Nature Remo API
// Make sure you set environment variable and token
func TurnOffLight(w http.ResponseWriter, r *http.Request) {
	// get token
	token := os.Getenv("NATURE_REMO_GLOBAL_TOKEN")
	if token == "" {
		fmt.Fprintln(os.Stderr, "Error getting env var: NATURE_REMO_GLOBAL_TOKEN")
		fmt.Fprintf(w, "Failed")
		return
	}
	// create client
	c := natureremo.NewClient(token)
	ctx := context.Background()
	// get devices
	dvs, err := c.DeviceService.GetAll(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting devices: %s", err.Error())
		fmt.Fprint(w, "Failed")
		return
	}
	fmt.Printf("Num devices : %d\n", len(dvs))
	if len(dvs) == 0 {
		fmt.Fprintln(os.Stderr, "Could not find devices")
		fmt.Fprint(w, "Failed")
		return
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
		fmt.Fprintln(os.Stderr, "There was no device supporting measuring illumination value")
		fmt.Fprint(w, "Failed")
		return
	}

	// get appliances and turn off signal
	acs, err := c.ApplianceService.GetAll(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting appliances: %s", err.Error())
		fmt.Fprint(w, "Failed")
		return
	}
	fmt.Printf("Num appliances : %d\n", len(acs))
	if len(acs) == 0 {
		fmt.Fprintln(os.Stderr, "Could not find appliances")
		fmt.Fprint(w, "Failed")
		return
	}
	var ac *natureremo.Appliance
	for _, a := range acs {
		if a.Nickname == lightApplianceName {
			ac = a
		}
	}
	if ac == nil {
		fmt.Fprintf(os.Stderr, "Could not find light with nickname : %s", lightApplianceName)
		fmt.Fprint(w, "Failed")
		return
	}
	var sg *natureremo.Signal
	for _, s := range ac.Signals {
		if s.Name == lightTurnOffSignalName {
			sg = s
		}
	}
	if sg == nil {
		fmt.Fprintf(os.Stderr, "Could not find turn off signal : %s", lightTurnOffSignalName)
		fmt.Fprint(w, "Failed")
		return
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
	fmt.Fprint(w, "OK")
}
