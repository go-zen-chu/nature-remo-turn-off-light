// Package p contains an HTTP Cloud Function.
package p

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/tenntenn/natureremo"
)

const (
	illuminationThreshold  = 20
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
	ds, err := c.DeviceService.GetAll(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting devices: %s", err.Error())
		fmt.Fprint(w, "Failed")
		return
	}
	fmt.Printf("Num devices : %d\n", len(ds))
	if len(ds) == 0 {
		fmt.Fprintln(os.Stderr, "Could not find devices")
		fmt.Fprint(w, "Failed")
		return
	}
	d := ds[0] // only use the first device

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
		t := time.NewTicker(2 * time.Second)
		for {
			select {
			case <-t.C:
				if count <= 0 {
					fmt.Println("Exceed counts")
					t.Stop()
					break
				}
				count--
				// get sensor value
				sv := d.NewestEvents[natureremo.SensortypeIllumination]
				il := sv.Value
				fmt.Printf("Illumination value : %f\n", il)
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
