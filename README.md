# godfrey4home

GodFrey4Home will help you to manage hardware devices for a HomeKit client ( Iphone,Ipad) , based on a GO stack software.

GodFrey4Home is provide under MIT License.

GodFrey4Home is currently a  set of independent programm.

## homekit-tasmota-switch.go

* [src/homekit-tasmota-switch.go](src/homekit-tasmota-switch.go)

   control and expose 6 devices , these devices use a Tasmota firmware ( https://tasmota.github.io/docs/ )
   was installed with tuya-convert ( https://github.com/ct-Open-Source/tuya-convert )

    - 2 Gosund WP212 ( https://amzn.to/2P5Pecb )
          the template for Tasmota is https://blakadder.github.io/templates/gosund_WP212.html
    - 4 Gosund WP5   ( https://amzn.to/359w8Yy )
          the template for Tasmota is https://blakadder.github.io/templates/gosund_WP5.html

## to compile

### for your local architecture
```
cd src
go build -ldflags "-s -w" -v power-switch-cgi.go
```

### for a different architecture
For example , you want deploy the binary on a *raspberry PI* or or some *openwrt* router .

for a *raspberry PI* 
```
env GOOS=linux GOARCH=arm GOARM=5 go build   -ldflags "-s -w" -v homekit-power-switch.go
```
for a *cubietruck*
see debian on cubie truck (https://www.armbian.com/cubietruck/)
```
env GOOS=linux GOARCH=arm GOARM=7 go build   -ldflags "-s -w" -v homekit-power-switch.go
```
