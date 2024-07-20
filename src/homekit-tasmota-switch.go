// ------------------------------------------------------------------------------------------
package main

import (
	"context"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/brutella/hap"
	"github.com/brutella/hap/accessory"
	"github.com/brutella/hap/characteristic"
	"github.com/brutella/hap/log"
	"github.com/brutella/hap/service"

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
	grpid      int
	pos_in_grp int
	len_grp    int
	host       string
	powerlabel string
	kind       string
}

// ------------------------------------------------------------------------------------------
type mainConfig struct {
	pin  int64
	port int
	db   string
}

// ------------------------------------------------------------------------------------------
func CheckObject(sw_val *fastjson.Value, sw_ind int, grpid int, last_id *int) (bool, *tasmotaSwitch) {
	if !sw_val.Exists("host") {
		log.Debug.Println("No key host in Element ", sw_ind+1, " from tasmotaswitch ")
		return false, nil
	}
	if !sw_val.Exists("powerlabel") {
		log.Debug.Println("No key powerlabel in Element ", sw_ind+1, " from tasmotaswitch ")
		return false, nil
	}
	if !sw_val.Exists("kind") {
		log.Debug.Println("No key kind in Element ", sw_ind+1, " from tasmotaswitch ")
		return false, nil
	}
	if sw_val.Get("host").Type() != fastjson.TypeString {
		log.Debug.Println("Wrong type for host in Element ", sw_ind+1, " from tasmotaswitch")
		return false, nil
	}
	if sw_val.Get("powerlabel").Type() != fastjson.TypeString {
		log.Debug.Println("Wrong type for powerlabel in Element ", sw_ind+1, " from tasmotaswitch")
		return false, nil
	}
	if sw_val.Get("kind").Type() != fastjson.TypeString {
		log.Debug.Println("Wrong type for kind in Element ", sw_ind+1, " from tasmotaswitch")
		return false, nil
	}
	*last_id++
	result := tasmotaSwitch{id: *last_id, grpid: grpid, host: string(sw_val.GetStringBytes("host")), powerlabel: string(sw_val.GetStringBytes("powerlabel")), kind: string(sw_val.GetStringBytes("kind"))}
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
func LoadConfig(config_file string) (bool, []tasmotaSwitch, *mainConfig) {
	jsondatastr, _ := os.ReadFile(config_file)

	localconfig := mainConfig{pin: 12391235, port: 22339, db: "/var/lib/homekit-tasmota-gw"}

	var p fastjson.Parser
	v, err := p.ParseBytes(jsondatastr)
	if err != nil {
		log.Debug.Println("Can not parse file")
		log.Debug.Println(jsondatastr)
		return false, nil, nil
	}
	if v.Type() != fastjson.TypeObject {
		log.Debug.Println("Root type is not a object ", v.Type())
		return false, nil, nil
	}
	// ----------------------------------------------------------------------------------
	if v.Exists("server") {
		s := v.Get("server")
		if s.Type() != fastjson.TypeObject {
			log.Debug.Println("server type is not a object")
			return false, nil, nil
		}
		if s.Exists("pin") {
			p := s.Get("pin")
			if p.Type() != fastjson.TypeNumber {
				log.Debug.Println("server.pin type is not a number")
				return false, nil, nil
			}
			localconfig.pin = p.GetInt64()
		}
		if s.Exists("port") {
			p := s.Get("port")
			if p.Type() != fastjson.TypeNumber {
				log.Debug.Println("server.port type is not a number")
				return false, nil, nil
			}
			localconfig.port = p.GetInt()
		}
		if s.Exists("db") {
			p := s.Get("db")
			if p.Type() != fastjson.TypeString {
				log.Debug.Println("server.db type is not a string")
				return false, nil, nil
			}
			localconfig.db = string(p.GetStringBytes())
		}
	}
	// ----------------------------------------------------------------------------------
	if !v.Exists("tasmotaswitchs") {
		log.Debug.Println("No key tasmotaswitchs")
		return false, nil, nil
	}
	if v.Get("tasmotaswitchs").Type() != fastjson.TypeArray {
		log.Debug.Println("Value tasmotaswitchs is not a array", v.Type())
		return false, nil, nil
	}
	var result []tasmotaSwitch
	last_id := 0
	last_grp := 0
	for sw_ind, sw_val := range v.Get("tasmotaswitchs").GetArray() {
		if sw_val.Type() == fastjson.TypeArray {
			last_grp++
			temp_err, temp_res := CheckArrayOfSwitch(sw_val, sw_ind, last_grp, &last_id)
			if !temp_err {
				return false, nil, nil
			}
			result = append(result, temp_res...)
			log.Debug.Printf("config grp %2d len %2d", last_grp, len(temp_res))
		} else {
			if sw_val.Type() != fastjson.TypeObject {
				log.Debug.Println("Element ", sw_ind+1, " from tasmotaswitchs is not a object ", v.Type())
				return false, nil, nil
			}
			last_grp++
			check_state, check_result := CheckObject(sw_val, sw_ind, last_grp, &last_id)
			if !check_state {
				return false, nil, nil
			}
			result = append(result, *check_result)
		}
	}
	if len(result) > 0 {
		// --------------------------------------------------------------------------
		grpid := math.MinInt
		minid := math.MaxInt
		maxid := math.MinInt
		for _, v := range result {
			if grpid != v.grpid {
				//
				if grpid > 0 {
					for j := minid - 1; j < maxid; j++ {
						result[j].pos_in_grp = j + 1 - minid
						result[j].len_grp = maxid + 1 - minid
					}
				}
				//
				grpid = v.grpid
				minid = v.id
			}
			maxid = v.id
		}
		// --------------------------------------------------------------------------
		if grpid > 0 {
			for j := minid - 1; j < maxid; j++ {
				result[j].pos_in_grp = j + 1 - minid
				result[j].len_grp = maxid + 1 - minid
			}
		}
		return true, result, &localconfig
	} else {
		log.Debug.Println("tasmotaswitch array is empty")
		return false, nil, nil
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
	log.Debug.Printf("Client changed switch %d to %t", cfg_a_switch.id, new_state_bool)
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
	stateload, switchconfig, serverconfig := LoadConfig("/etc/homekit-tasmota-gw/config.json")
	if !stateload || switchconfig == nil {
		log.Debug.Println("Can not load config")
		os.Exit(1)
	}
	log.Debug.Println("config is loaded")
	// ----------------------------------------------------------------------------------
	// use of variadic functions https://gobyexample.com/variadic-functions

	all_switchs := make([]*characteristic.On, len(switchconfig))
	all_access := make([]*accessory.A, len(switchconfig))

	CntAcc := 0
	LastSwitchGrp := 0

	log.Debug.Printf("switch config len %d", len(switchconfig))
	for ind, cfg_cur_switch := range switchconfig {
		local_copy_cfg := cfg_cur_switch
		if LastSwitchGrp == cfg_cur_switch.grpid {
			if local_copy_cfg.kind == "switch" {
				a_srv := service.NewSwitch()
				all_access[CntAcc-1].AddS(a_srv.S)
				all_switchs[ind] = a_srv.On
			}
			if local_copy_cfg.kind == "light" {
				a_srv := service.NewLightbulb()
				all_access[CntAcc-1].AddS(a_srv.S)
				all_switchs[ind] = a_srv.On
			}
			all_switchs[ind].OnValueRemoteUpdate(func(on bool) { ChangeSwitch(local_copy_cfg, on) })
		} else {
			if local_copy_cfg.kind == "switch" {
				a_acc := accessory.NewSwitch(accessory.Info{
					Name:  "Switch" + strconv.Itoa(CntAcc+1),
					Model: "homekit-tasmota-switch.go", Manufacturer: "MAS", SerialNumber: fmt.Sprintf("850010C7-51BB-46D2-B033-50CE%08X", CntAcc+1),
				})
				all_access[CntAcc] = a_acc.A
				all_switchs[ind] = a_acc.Switch.On
			}
			if local_copy_cfg.kind == "light" {
				a_acc := accessory.NewLightbulb(accessory.Info{
					Name:  "Light" + strconv.Itoa(CntAcc+1),
					Model: "homekit-tasmota-switch.go", Manufacturer: "MAS", SerialNumber: fmt.Sprintf("850010C7-51BB-46D2-B033-50CE%08X", CntAcc+1),
				})
				all_access[CntAcc] = a_acc.A
				all_switchs[ind] = a_acc.Lightbulb.On
			}

			LastSwitchGrp = cfg_cur_switch.grpid
			all_switchs[ind].OnValueRemoteUpdate(func(on bool) { ChangeSwitch(local_copy_cfg, on) })
			CntAcc++
		}
	}
	// ----------------------------------------------------------------------------------
	fs_store := hap.NewFsStore(serverconfig.db)
	server, err := hap.NewServer(fs_store, all_access[0], all_access[1:CntAcc]...)
	if err != nil {
		log.Info.Panic(err)
	}
	server.Pin = strconv.FormatInt(serverconfig.pin,10)
	server.Addr = fmt.Sprintf(":%d", serverconfig.port)
	// ----------------------------------------------------------------------------------
	// Periodically check if physical status of the switch are identical to current state
	for ind := range switchconfig {
		go func(j int) {
			for {
				rurl := ReturnRemoteSwitch(switchconfig[j], false)
				s := getJson(rurl)
				p := string(s.GetStringBytes(switchconfig[j].powerlabel))
				if len(p) == 0 {
					p = "UNK"
				}
				must_log := false
				if p == "ON" || p == "OFF" {
					if all_switchs[j].Value() != (p == "ON") {
						all_switchs[j].SetValue(p == "ON")
						must_log = true
					}
				} else {
					must_log = true
				}
				if must_log {
					log.Debug.Printf("Switch %2d/%2d (%s) (%s) is %s\n", switchconfig[j].id, switchconfig[j].grpid, switchconfig[j].powerlabel, rurl, p)
				}
				time.Sleep(1 * time.Second)
			}
		}(ind)
	}
	// ----------------------------------------------------------------------------------
	c := make(chan os.Signal, 24)
	signal.Notify(c, os.Interrupt)
	signal.Notify(c, syscall.SIGTERM)

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		<-c
		signal.Stop(c)
		cancel()
		log.Debug.Printf("STOP STOP STOP")
	}()
	// ----------------------------------------------------------------------------------
	log.Debug.Println("now we must listen and serve")
	log.Debug.Printf("server.Pin: %s",server.Pin)
	log.Debug.Printf("server.Addr: %s",server.Addr)
	log.Debug.Printf("db: %s",serverconfig.db)
	server.ListenAndServe(ctx)
	// ----------------------------------------------------------------------------------
}

// ------------------------------------------------------------------------------------------
