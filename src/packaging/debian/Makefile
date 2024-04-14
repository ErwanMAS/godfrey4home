.PHONY: all clean install

all: homekit-tasmota-gw

clean:
	rm -f homekit-tasmota-gw

homekit-tasmota-gw: homekit-tasmota-gw.go
	go build homekit-tasmota-gw.go

install:
	mkdir -p $(DESTDIR)/usr/bin
	cp homekit-tasmota-gw $(DESTDIR)/usr/bin/homekit-tasmota-gw

