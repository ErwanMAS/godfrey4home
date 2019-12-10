// ------------------------------------------------------------------------------------------  
package main

import (
	"github.com/brutella/hc"
	"github.com/brutella/hc/accessory"
	"github.com/brutella/hc/service"	
	"github.com/brutella/hc/log"
	"net/http"
	"github.com/valyala/fastjson"
	"time"
	"io/ioutil"
	"strconv"
	"fmt"
)
// ------------------------------------------------------------------------------------------  
var myClient = &http.Client{Timeout: 10 * time.Second}

func getJson(url string) (error,*fastjson.Value) {
    r, err := myClient.Get(url)
    if err != nil {
        return err,nil
    }
    defer r.Body.Close()
    body, err := ioutil.ReadAll(r.Body)
    var p fastjson.Parser
    v , err := p.ParseBytes(body)
    return err,v
}
/* ------------------------------------------------------------------------------------------  
   control and expose 6 devices , these devices use a tasmota firmware ( https://tasmota.github.io/ ) 

    - 4 Gosund WP5   ( https://blakadder.github.io/templates/gosund_WP5.html )
    - 2 Gosund WP212 ( https://blakadder.github.io/templates/gosund_WP212.html )


   For fetch the status on WP5
      curl http://192.168.111.111/cm?cmnd=Power1

   For fetch the status on WP211
      curl http://192.168.111.118/cm?cmnd=Power1
    or
      curl http://192.168.111.118/cm?cmnd=Power2

   To set the power Off 
      curl http://192.168.111.111/cm?cmnd=Power1%20Off

   To set the power On
      curl http://192.168.111.111/cm?cmnd=Power1%20On

   The reply is a json strean :
      {"POWER1":"ON"} or {"POWER2":"OFF"}


   ------------------------------------------------------------------------------------------ */
const MAX_Switch = 8
// ---------------------------------
type state_switch struct {
	POWER   string
}
// ---------------------------------
func ReturnRemoteSwitch ( id_switch int , change bool ) (string,int) {
	var myRemoteSwitch string
	var j int

	switch {
	case id_switch == 5 : { myRemoteSwitch="http://192.168.111.111/cm?cmnd=POWER1" ; j = 0 ; }
	case id_switch == 6 : { myRemoteSwitch="http://192.168.111.112/cm?cmnd=POWER1" ; j = 0 ; }
	case id_switch == 7 : { myRemoteSwitch="http://192.168.111.113/cm?cmnd=POWER1" ; j = 0 ; }
	case id_switch == 8 : { myRemoteSwitch="http://192.168.111.114/cm?cmnd=POWER1" ; j = 0 ; }
	case ( id_switch >= 3 && id_switch <= 4 ) : { myRemoteSwitch=fmt.Sprintf("http://192.168.111.115/cm?cmnd=POWER%d",id_switch-2) ; j=id_switch-2 ; }
	case ( id_switch >= 1 && id_switch <= 2 ) : { myRemoteSwitch=fmt.Sprintf("http://192.168.111.116/cm?cmnd=POWER%d",id_switch)   ; j=id_switch ; }
	}
	if ( change ) {
		return myRemoteSwitch+"%%20%s",j  ;
	} else {
		return myRemoteSwitch,j ;
	}
}

func ChangeSwitch ( id_switch string , new_state_bool bool ) {
	var id_switch_num int
	var j int
	var new_state string
	var myRemoteSwitch string
	id_switch_num , _ = strconv.Atoi(id_switch)
	myRemoteSwitch,j=ReturnRemoteSwitch(id_switch_num,true)
	if ( new_state_bool ) {
		new_state = "On"
	} else {
		new_state = "Off"
	}
	_,c := getJson(fmt.Sprintf(myRemoteSwitch,new_state))
	if ( j == 0 ) {
		log.Debug.Printf("Client changed switch %d to %s / new state %s ",id_switch,new_state,c.GetStringBytes("POWER"))
	} else { 
		log.Debug.Printf("Client changed switch %d to %s / new state %s ",id_switch,new_state,c.GetStringBytes("POWER"+strconv.Itoa(j)))
	}
}
// ------------------------------------------------------------------------------------------  
func main() {
	// ----------------------------------------------------------------------------------
	log.Debug.Enable()
	// ----------------------------------------------------------------------------------
	// use of variadic functions https://gobyexample.com/variadic-functions

	all_switchs := make([]*service.Switch,MAX_Switch)
	all_access := make([]*accessory.Accessory,MAX_Switch)

	CntA :=0
	
	for i:= 1 ; i <=MAX_Switch ; i++ {
		j := strconv.Itoa(i)
		if ( i >=2 && i <= 4 ) {
			a_srv:=service.NewSwitch()
			a_srv.On.OnValueRemoteUpdate(func(on bool) { ChangeSwitch (j,on ) })
			all_access[0].AddService(a_srv.Service)
			all_switchs[i-1] =  a_srv
		} else {
			a_acc := accessory.NewSwitch(accessory.Info{Name: "Switch"+j})
			all_switchs[i-1] =  a_acc.Switch
			a_acc.Switch.On.OnValueRemoteUpdate(func(on bool) { ChangeSwitch (j,on ) })
			all_access[CntA] = a_acc.Accessory
			CntA++
		}
	}
	// ----------------------------------------------------------------------------------
	config := hc.Config{Pin: "12344321", Port: "12345", StoragePath: "./db"}
	t, err := hc.NewIPTransport(config,all_access[0],all_access[1:5]...)

	if err != nil {
		log.Info.Panic(err)
	}
	// ----------------------------------------------------------------------------------
	// Periodically check if physical status of the switch are identical to current state
	go func() {
		for {
			for i:= 1 ; i <=MAX_Switch ; i++ {
				var rurl string
				var rj   int
				rurl,rj = ReturnRemoteSwitch(i,false)
				_,s := getJson(rurl)
				var p string
				if ( rj == 0 ) {
					p=string(s.GetStringBytes("POWER"))
				} else {
					p=string(s.GetStringBytes("POWER"+strconv.Itoa(rj)))
				}
				log.Debug.Printf("Switch %d (%d) (%s) is %s\n",i,rj,rurl,p)
				if all_switchs[i-1].On.GetValue() != ( p == "ON" ) {
					all_switchs[i-1].On.SetValue( p == "ON")
				}
			}
			time.Sleep(6 * time.Second)
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


