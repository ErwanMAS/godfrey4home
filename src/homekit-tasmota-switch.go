// ------------------------------------------------------------------------------------------
package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/brutella/hc"
	"github.com/brutella/hc/accessory"
	"github.com/brutella/hc/log"
	"github.com/brutella/hc/service"
	"github.com/valyala/fastjson"
)

// ------------------------------------------------------------------------------------------
var myClient = &http.Client{Timeout: 10 * time.Second}

func getJson(url string) *fastjson.Value {
	r, err := myClient.Get(url)
	if err != nil {
		return nil
	}
	defer r.Body.Close()
	body, _ := io.ReadAll(r.Body)
	var p fastjson.Parser
	v, _ := p.ParseBytes(body)
	return v
}

/* ------------------------------------------------------------------------------------------
   control and expose tasmota devices ( https://tasmota.github.io/ )

    - 6 Gosund WP5   ( https://blakadder.github.io/templates/gosund_WP5.html )
    - 3 Gosund WP212 ( https://blakadder.github.io/templates/gosund_WP212.html )


   For fetch the status on WP5
      curl http://192.168.111.111/cm?cmnd=Power

   For fetch the status on WP211
      curl http://192.168.111.118/cm?cmnd=Power1
    or
      curl http://192.168.111.118/cm?cmnd=Power2

   To set the power Off
      curl http://192.168.111.111/cm?cmnd=Power%20Off

   To set the power On
      curl http://192.168.111.118/cm?cmnd=Power1%20On

   The reply is a json strean :
      {"POWER1":"ON"} or {"POWER2":"OFF"} or {"POWER":"OFF"}


   the configuration is defined in a json file named homekit-tasmota-config.json

   ------------------------------------------------------------------------------------------ */
// ------------------------------------------------------------------------------------------
type tasmotaSwitch struct {
	id         int
	grp        int
	host       string
	powerlabel string
}

// ------------------------------------------------------------------------------------------
func CheckObject(sw_val *fastjson.Value, sw_ind int, grpid int, last_id *int) (bool, *tasmotaSwitch) {
	if !sw_val.Exists("host") {
		log.Debug.Println("No key host in Element ", sw_ind+1, " from tasmotaswitch ")
		return false, nil
	}
	if !sw_val.Exists("powerlabel") {
		log.Debug.Println("No key host in Element ", sw_ind+1, " from tasmotaswitch ")
		return false, nil
	}
	if sw_val.Get("host").Type() != fastjson.TypeString {
		log.Debug.Println("Wrong type for host in Element ", sw_ind+1, " from tasmotaswitch")
		return false, nil
	}
	if sw_val.Get("powerlabel").Type() != fastjson.TypeString {
		log.Debug.Println("Wrong type for host in Element ", sw_ind+1, " from tasmotaswitch")
		return false, nil
	}
	*last_id++
	result := tasmotaSwitch{id: *last_id, grp: grpid, host: string(sw_val.GetStringBytes("host")), powerlabel: string(sw_val.GetStringBytes("powerlabel"))}
	return true, &result
}

// ------------------------------------------------------------------------------------------
func CheckArrayOfSwitch(v *fastjson.Value, sw_ind_offset int, grpid int, last_id *int) (bool, []tasmotaSwitch) {
	var result []tasmotaSwitch
	for sw_ind, sw_val := range v.GetArray() {
		if sw_val.Type() != fastjson.TypeObject {
			log.Debug.Println("Element ", sw_ind+1, " from tasmotaswitchs is not a object ", v.Type())
			return false, nil
		}
		check_state, check_result := CheckObject(sw_val, sw_ind_offset+sw_ind, grpid, last_id)
		if !check_state {
			return false, nil
		}
		result = append(result, *check_result)
	}
	return true, result
}

// ------------------------------------------------------------------------------------------
func LoadConfig(config_file string) (bool, []tasmotaSwitch) {
	jsondatastr, _ := os.ReadFile(config_file)

	var p fastjson.Parser
	v, err := p.ParseBytes(jsondatastr)
	if err != nil {
		log.Debug.Println("Can not parse file")
		log.Debug.Println(jsondatastr)
		return false, nil
	}
	if v.Type() != fastjson.TypeObject {
		log.Debug.Println("Root type is not a object ", v.Type())
		return false, nil
	}
	if !v.Exists("tasmotaswitchs") {
		log.Debug.Println("No key tasmotaswitchs")
		return false, nil
	}
	if v.Get("tasmotaswitchs").Type() != fastjson.TypeArray {
		log.Debug.Println("Value tasmotaswitchs is not a array", v.Type())
		return false, nil
	}
	var result []tasmotaSwitch
	last_id := 0
	last_grp := 0
	for sw_ind, sw_val := range v.Get("tasmotaswitchs").GetArray() {
		if sw_val.Type() == fastjson.TypeArray {
			last_grp++
			temp_err, temp_res := CheckArrayOfSwitch(sw_val, sw_ind, last_grp, &last_id)
			if !temp_err {
				return false, nil
			}
			result = append(result, temp_res...)
			log.Debug.Printf("config grp %2d len %2d", last_grp, len(temp_res))
		} else {
			if sw_val.Type() != fastjson.TypeObject {
				log.Debug.Println("Element ", sw_ind+1, " from tasmotaswitchs is not a object ", v.Type())
				return false, nil
			}
			last_grp++
			check_state, check_result := CheckObject(sw_val, sw_ind, last_grp, &last_id)
			if !check_state {
				return false, nil
			}
			result = append(result, *check_result)
		}
	}
	if len(result) > 0 {
		return true, result
	} else {
		log.Debug.Println("tasmotaswitch array is empty")
		return false, nil
	}
}

// ------------------------------------------------------------------------------------------
func ReturnRemoteSwitch(cfg_a_switch tasmotaSwitch, change bool) string {
	myRemoteSwitch := fmt.Sprintf("http://%s/cm?cmnd=%s", cfg_a_switch.host, cfg_a_switch.powerlabel)
	if change {
		return myRemoteSwitch + "%%20%s"
	} else {
		return myRemoteSwitch
	}
}

// ------------------------------------------------------------------------------------------
func ChangeSwitch(cfg_a_switch tasmotaSwitch, new_state_bool bool) {
	log.Debug.Printf("Client changed switch %d to %d", cfg_a_switch.id, new_state_bool)
	var new_state string
	myRemoteSwitch := ReturnRemoteSwitch(cfg_a_switch, true)
	if new_state_bool {
		new_state = "On"
	} else {
		new_state = "Off"
	}
	c := getJson(fmt.Sprintf(myRemoteSwitch, new_state))
	log.Debug.Printf("Client changed switch %d to %s / new state %s ", cfg_a_switch.id, new_state, c.GetStringBytes(cfg_a_switch.powerlabel))
}

// ------------------------------------------------------------------------------------------
func main() {
	// ----------------------------------------------------------------------------------
	log.Debug.Enable()
	// ----------------------------------------------------------------------------------
	stateload, switchconfig := LoadConfig("./homekit-tasmota-config.json")
	if !stateload || switchconfig == nil {
		log.Debug.Println("Can not load config")
		os.Exit(1)
	}
	// ----------------------------------------------------------------------------------
	// use of variadic functions https://gobyexample.com/variadic-functions

	all_switchs := make([]*service.Switch, len(switchconfig))
	all_access := make([]*accessory.Accessory, len(switchconfig))

	CntAcc := 0
	LastSwitchGrp := 0

	log.Debug.Printf("switch config len %d", len(switchconfig))
	for ind, cfg_cur_switch := range switchconfig {
		local_copy_cfg := cfg_cur_switch
		if LastSwitchGrp == cfg_cur_switch.grp {
			a_srv := service.NewSwitch()
			a_srv.On.OnValueRemoteUpdate(func(on bool) { ChangeSwitch(local_copy_cfg, on) })
			all_access[CntAcc-1].AddService(a_srv.Service)
			all_switchs[ind] = a_srv
		} else {
			a_acc := accessory.NewSwitch(accessory.Info{
				Name:  "Switch" + strconv.Itoa(CntAcc+1),
				Model: "homekit-tasmota-switch.go", Manufacturer: "MAS", SerialNumber: "850010C7-51BB-46D2-B033-" + strconv.Itoa(CntAcc+1),
			})
			all_switchs[ind] = a_acc.Switch
			a_acc.Switch.On.OnValueRemoteUpdate(func(on bool) { ChangeSwitch(local_copy_cfg, on) })
			all_access[CntAcc] = a_acc.Accessory
			CntAcc++
			LastSwitchGrp = cfg_cur_switch.grp
		}
	}
	// ----------------------------------------------------------------------------------
	config := hc.Config{Pin: "12345321", Port: "12345", StoragePath: "./db"}

	t, err := hc.NewIPTransport(config, all_access[0], all_access[1:CntAcc]...)
	if err != nil {
		log.Info.Panic(err)
	}
	// ----------------------------------------------------------------------------------
	// Periodically check if physical status of the switch are identical to current state
	go func() {
		for {
			for i := 1; i <= len(switchconfig); i++ {
				rurl := ReturnRemoteSwitch(switchconfig[i-1], false)
				s := getJson(rurl)
				p := string(s.GetStringBytes(switchconfig[i-1].powerlabel))
				log.Debug.Printf("Switch %2d/%2d (%s) (%s) is %s\n", switchconfig[i-1].id, switchconfig[i-1].grp, switchconfig[i-1].powerlabel, rurl, p)
				if all_switchs[i-1].On.GetValue() != (p == "ON") {
					all_switchs[i-1].On.SetValue(p == "ON")
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
