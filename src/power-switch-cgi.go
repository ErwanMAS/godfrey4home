// ------------------------------------------------------------------------------------------  
package main

import (
        "fmt"
        "os"
        "time"     	
	"strings"
	"strconv"
	"syscall"
        "net/http/cgi"
	"github.com/jacobsa/go-serial/serial"
)
// ------------------------------------------------------------------------------------------
// This program provide a interface to drive a X220 relay board 
// => http://gce-electronics.com/en/-usb/359-controller-usb-2-relay-board-x220-.html
// This relay board :
//  - drive 2 relay  : 230 Vac / 5A  or 125Vac / 10A
//  - power must be supply by USB connection
// 
// Linux kernel must have these modules loaded 
// - usbserial
// - ftdi_sio
//
// Can be modify , to drive a http://gce-electronics.com/en/-usb/23-usb-relay-controller-x440.html
//
// i dont have a X440 , if someone want to send me one , free of charge , i will be happy , to modify my programm
//
// https://www.sohamkamani.com/blog/2017/09/13/how-to-build-a-web-application-in-golang/
//
// ------------------------------------------------------------------------------------------
const SERIAL_DEVICE = "/dev/ttyUSB0"
const LOCK_4_SERIAL_DEVICE = "/var/lock/ttyUSB0.lock"
// ------------------------------------------------------------------------------------------  
type SwitchStatus struct{
     Switch    [2]int
}

type Lock struct {
	filename string
	fd       int
}
// ------------------------------------------------------------------------------------------  
func ReadSwitchTimeout ( sta chan SwitchStatus ) {
     var t_sta SwitchStatus   
     for i:=0; i <=1 ; i++ {
       t_sta.Switch[i] = -2
     }
     time.Sleep( 5 * time.Second)
     sta <- t_sta
}

func ChangeSwitchTimeout ( sta chan bool ) {
     time.Sleep( 5 * time.Second)
     sta <- false
}

func LockTimeout ( sta chan bool ) {
     time.Sleep( 2 * time.Second)
     sta <- false
}
// ------------------------------------------------------------------------------------------  
func GetLockFile() *Lock {

     var l Lock
     l.filename=LOCK_4_SERIAL_DEVICE
     fd, err := syscall.Open(l.filename, syscall.O_CREAT|syscall.O_RDONLY, 0600)
     if err != nil {
     	return nil
     }
     l.fd = fd
     result := make(chan bool)
     
     go func() {
     	     err := syscall.Flock(l.fd, syscall.LOCK_EX)
	     if err == nil {
	     	result <- true
	     } else {
	        result <- false
	     }
     }()
     go LockTimeout(result)
     
     var lockok bool

     lockok = <- result

     if lockok {
     	return &l 
     }
     return nil 
}
// ------------------------------------------------------------------------------------------
func ReadSwitchStatus( sta chan SwitchStatus ) {

     options := serial.OpenOptions{
	      PortName: SERIAL_DEVICE,
      	      BaudRate: 9600 ,
              DataBits: 8,
      	      StopBits: 1,
      	      MinimumReadSize: 1,
    }


    var t_sta SwitchStatus   
    for i:=0; i <=1 ; i++ {
        t_sta.Switch[i] = -2
    }

    var lock *Lock 
    lock = GetLockFile()
    if lock == nil {
        syscall.Close(lock.fd)
	sta <- t_sta
	return
    }

    port, err := serial.Open(options)
    if err != nil {
        sta <- t_sta
        return
    }
    for i:=0; i <=1 ; i++ {
        t_sta.Switch[i] = -3
    }

    defer func() {
    	  port.Close()
	  syscall.Close(lock.fd)
    }()

    s := "?"
    b := []byte(s)

    _, err_w := port.Write(b)
    if err_w != nil {
      os.Exit(1)
    }

    var t_buf [8]byte 

    t_r := 0
    
    for t_r < 8 {
      Buf := make([]byte,8)
      n_r, err_r := port.Read(Buf)
      if err_r != nil {
	os.Exit(1)
      } else {
	copy(t_buf[t_r:t_r + n_r],Buf[:n_r])
        t_r = t_r + n_r
      }
    }
    for i:=0; i <=1 ; i++ {
    	if t_buf[i*4] == byte('s') && t_buf[i*4+1] == byte('1')+byte(i)  {
    	   if t_buf[i*4+2] == byte('0') {
	      t_sta.Switch[i] = 0 
	   } else {
   	      if t_buf[i*4+2] == byte('1') {
	        t_sta.Switch[i] = 1
	      }
	   }
	}
    }
    sta <- t_sta
}
// ------------------------------------------------------------------------------------------
func ChangeSwitchStatus( sta chan bool , sid int , sst int ) {

     options := serial.OpenOptions{
	      PortName: SERIAL_DEVICE,
      	      BaudRate: 9600 ,
              DataBits: 8,
      	      StopBits: 1,
      	      MinimumReadSize: 1,
    }


    var lock *Lock 
    lock = GetLockFile()
    if lock == nil {
       sta <- false
    }

    port, err := serial.Open(options)
    if err != nil {
      syscall.Close(lock.fd)
      sta <- false
    }

    defer func() {
    	  port.Close()
	  syscall.Close(lock.fd)
    }()
    s := "S00"
    b := []byte(s)

    if sid == 1 ||  sid == 2 {
       b[1]=byte('0')+byte(sid)
    }

    if sst == 0 ||  sst == 1 {
       b[2]=byte('0')+byte(sst)
    }

    _, err_w := port.Write(b)
    if err_w != nil {
       os.Exit(1)
       sta <- false
    }
    sta <- true
}
// ------------------------------------------------------------------------------------------
func main() {
	httpReq, err := cgi.Request()
        if err != nil {
                os.Exit(1)
        }
	if err = httpReq.ParseForm(); err != nil {
		os.Exit(1)
	}

	serialstatus := make(chan SwitchStatus )
	go ReadSwitchTimeout ( serialstatus )	
	go ReadSwitchStatus ( serialstatus )
	
	p_state := httpReq.FormValue("state")
        p_sid , p_sid_err := strconv.Atoi(httpReq.FormValue("sid"))
	if p_sid_err != nil {
	   p_sid=-1 
	}
	// ----------------------------------------------------------------------------------
	
	fmt.Printf("Content-type: application/json; charset=us-ascii\n\n") 

	p_state=strings.ToLower(p_state)

	t_sta := <- serialstatus

	// ----------------------------------------------------------------------------------
	states_on_off := [][]string{{"off","\"off\""},{"on","\"on\""}, }
	
	if p_state == "get" && ( p_sid == 1 || p_sid == 2 ) {
		s_sta:="null"
		if ( t_sta.Switch[p_sid-1] >=0 && t_sta.Switch[p_sid-1] <=1 )  {
			s_sta=states_on_off[t_sta.Switch[p_sid-1]][1]
		}
		fmt.Printf("{ \"sid\" : %d , \"state\": %s}\n",p_sid,s_sta)	   
	}
	if ( p_state == "on" || p_state == "off" ) && ( p_sid == 1 || p_sid == 2 ) {
		p_state_int := 0
		if p_state == "on" { p_state_int = 1 }
		
		if t_sta.Switch[p_sid-1] == p_state_int {
			fmt.Printf("{ \"sid\" : %d , \"state\": \"%s\" , \"changed\" : false }\n",p_sid,p_state)
		} else {
			modstatus := make(chan bool )
			go ChangeSwitchTimeout ( modstatus )	
			go ChangeSwitchStatus ( modstatus , p_sid , p_state_int )
			t_mod := <- modstatus
			if t_mod {
				fmt.Printf("{ \"sid\" : %d , \"state\": \"%s\" , \"changed\" : true }\n",p_sid,p_state)	   
			} else {
				fmt.Printf("{ \"sid\" : %d , \"state\": null , \"changed\" : true }\n",p_sid)	   
			}
		}
	}
	// ----------------------------------------------------------------------------------

}
// ------------------------------------------------------------------------------------------

