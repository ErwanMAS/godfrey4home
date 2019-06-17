# godfrey4home

GodFrey4Home will help you to manage all devices for HomeKit  based on a GO stack software.

GodFrey4Home is provide under MIT License.

GodFrey4Home is currently a  set of independent programm.

## power-switch
* [src/power-switch.go](src/power-switch.go)

This program expose a http interface to drive a X220 relay board
=> http://gce-electronics.com/en/-usb/359-controller-usb-2-relay-board-x220-.html
This relay board :
  - drive 2 relay  : 230 Vac / 5A  or 125Vac / 10A
  - power must be supply by USB connection

Linux kernel must have these modules loaded
  - usbserial
  - ftdi_sio

Can be modify , to drive a http://gce-electronics.com/en/-usb/23-usb-relay-controller-x440.html

## homekit-switch

* [src/homekit-switch.go](src/homekit-switch.go)

A homekit bridge , that will access a relay board expose by power-switch

This brudge use the package [github.com/brutella/hc](https://github.com/brutella/hc) , for providing the HomeKit interface.

## to compile

### for your local architecture
```
cd src
go build -ldflags "-s -w" -v power-switch.go
```

### for a different architecture
For example , you want deploy the binary on a *raspberry PI* or or some *openwrt* router .

for a *raspberry PI* 
```
env GOOS=linux GOARCH=arm GOARM=5 go build   -ldflags "-s -w" -v homekit-switch.go
```
for a *cubietruck*
see debian on cubie truck (https://www.armbian.com/cubietruck/)
```
env GOOS=linux GOARCH=arm GOARM=7 go build   -ldflags "-s -w" -v homekit-switch.go
```
