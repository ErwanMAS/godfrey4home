// ------------------------------------------------------------------------------------------  
package main

import (
	"github.com/brutella/hc"
	"github.com/brutella/hc/accessory"
	"github.com/brutella/hc/log"
	"net/http"
	"encoding/json"
	"time"
	"io/ioutil"
	"strconv"
	"fmt"
)
// ------------------------------------------------------------------------------------------  
var myClient = &http.Client{Timeout: 10 * time.Second}

func getJson(url string, target interface{}) error {
    r, err := myClient.Get(url)
    if err != nil {
        return err
    }
    defer r.Body.Close()
    body, err := ioutil.ReadAll(r.Body)
    return json.Unmarshal(body,&target)
}
/* ------------------------------------------------------------------------------------------  
   home made switch , that can be manage by http request


# /cgi-bin/power-switch-cgi?sid=X&state=Y
#
# X can be 1 or 2
# Y can be get , on  or off

return this json when use with Y=GET

{
  "sid": 2,
  "state": "off"
}

return this json when use with Y=off

{
  "sid": 2,
  "state": "off",
  "changed": true
}

   ------------------------------------------------------------------------------------------ */
var myRemoteSwitch = "http://192.168.111.111/cgi-bin/power-switch-cgi?sid=%s&state=%s"
const MAX_Switch = 2
// ---------------------------------
type state_switch struct {
	Sid     int
	State   string
	Changed bool
}
// ---------------------------------
func ChangeSwitch ( id_switch string , new_state_bool bool ) {
	var new_state string
	if ( new_state_bool ) {
		new_state = "on"
	} else {
		new_state = "off"
	}
	c := new(state_switch)
	getJson(fmt.Sprintf(myRemoteSwitch,id_switch,new_state),c)
	log.Debug.Printf("Client changed switch %s to %s / new state %s real change %t ",id_switch,new_state,c.State,c.Changed)
}
// ------------------------------------------------------------------------------------------  
func main() {
	// ----------------------------------------------------------------------------------
	log.Debug.Enable()
	// ----------------------------------------------------------------------------------
	// use of variadic functions https://gobyexample.com/variadic-functions

	all_switchs := make([]*accessory.Switch,MAX_Switch)
	all_access := make([]*accessory.Accessory,MAX_Switch)

	for i:= 1 ; i <=MAX_Switch ; i++ {
		all_switchs[i-1] = accessory.NewSwitch(accessory.Info{Name: "Switch"+strconv.Itoa(i)})
		j := strconv.Itoa(i)
		all_switchs[i-1].Switch.On.OnValueRemoteUpdate(func(on bool) { ChangeSwitch (j,on ) })
		all_access[i-1] = all_switchs[i-1].Accessory 
	}
	// ----------------------------------------------------------------------------------
	config := hc.Config{Pin: "12344321", Port: "12345", StoragePath: "./db"}
	t, err := hc.NewIPTransport(config,all_access[0],all_access[1:]...)

	if err != nil {
		log.Info.Panic(err)
	}
	// ----------------------------------------------------------------------------------
	// Periodically check if physical status of the switch are identical to current state
	go func() {
		for {
			for i:= 1 ; i <=MAX_Switch ; i++ {
				is := strconv.Itoa(i) 
				s := new(state_switch)
				getJson(fmt.Sprintf(myRemoteSwitch,is,"get"),s)
				log.Debug.Printf("Switch %s is %s\n",is,s.State)
				if all_switchs[i-1].Switch.On.GetValue() != ( s.State == "on" ) {
					all_switchs[i-1].Switch.On.SetValue(s.State == "on")
				}
			}
			time.Sleep(3 * time.Second)
		}
	}()
	// ----------------------------------------------------------------------------------
	hc.OnTermination(func() {
		<-t.Stop()
	})
	log.Debug.Println("we are going tio run t.Start()")

	t.Start()
	log.Debug.Println("This is done")
	// ----------------------------------------------------------------------------------
}
// ------------------------------------------------------------------------------------------  


